package scaling_test

import (
	"fmt"
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/scaling"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestScaling(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scaling Suite")
}

const namespace = "exampleNamespace"

var (
	initialAPIObjects []runtime.Object
	fakeClientset     *fake.Clientset
	persistenceScaler scaling.PersistenceScaler
	rmq               rabbitmqv1beta1.RabbitmqCluster
	existingSts       appsv1.StatefulSet
	existingPVC       corev1.PersistentVolumeClaim
	three             = int32(3)
	oneG              = k8sresource.MustParse("1Gi")
	tenG              = k8sresource.MustParse("10Gi")
	fifteenG          = k8sresource.MustParse("15Gi")
	ephemeralStorage  = k8sresource.MustParse("0")
)

func generatePVCTemplate(size k8sresource.Quantity) corev1.PersistentVolumeClaim {
	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "persistence",
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: size,
				},
			},
		},
	}
}

func generatePVC(rmq rabbitmqv1beta1.RabbitmqCluster, index int, size k8sresource.Quantity) corev1.PersistentVolumeClaim {
	name := rmq.PVCName(index)
	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: size,
				},
			},
		},
	}
}

type ActionMatcher struct {
	expectedVerb         string
	expectedResourceType string
	expectedNamespace    string
	actualAction         k8stesting.Action
}

func BeGetActionOnResource(expectedResourceType, expectedResourceName, expectedNamespace string) types.GomegaMatcher {
	return &GetActionMatcher{
		ActionMatcher{
			expectedVerb:         "get",
			expectedResourceType: expectedResourceType,
			expectedNamespace:    expectedNamespace,
		},
		expectedResourceName,
	}
}

type GetActionMatcher struct {
	ActionMatcher
	expectedResourceName string
}

func (matcher *GetActionMatcher) Match(actual interface{}) (bool, error) {
	genericAction, ok := actual.(k8stesting.Action)
	if !ok {
		return false, fmt.Errorf("BeGetActionOnResource must be passed an Action from the fakeClientset")
	}
	matcher.actualAction = genericAction

	action, ok := actual.(k8stesting.GetAction)
	if !ok {
		return false, nil
	}
	return action.Matches(matcher.expectedVerb, matcher.expectedResourceType) &&
		action.GetNamespace() == matcher.expectedNamespace &&
		action.GetName() == matcher.expectedResourceName, nil
}

func (matcher *GetActionMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected '%s' on resource '%s' named '%s' in namespace '%s' to match the observed action:\n%+v\n",
		matcher.expectedVerb, matcher.expectedResourceType, matcher.expectedResourceName, matcher.expectedNamespace, matcher.actualAction)
}

func (matcher *GetActionMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected '%s' on resource '%s' named '%s' in namespace '%s' not to match the observed action:\n%+v\n",
		matcher.expectedVerb, matcher.expectedResourceType, matcher.expectedResourceName, matcher.expectedNamespace, matcher.actualAction)
}

func BeDeleteActionOnResource(expectedResourceType, expectedResourceName, expectedNamespace string) types.GomegaMatcher {
	return &DeleteActionMatcher{
		ActionMatcher{
			expectedVerb:         "delete",
			expectedResourceType: expectedResourceType,
			expectedNamespace:    expectedNamespace,
		},
		expectedResourceName,
	}
}

type DeleteActionMatcher struct {
	ActionMatcher
	expectedResourceName string
}

func (matcher *DeleteActionMatcher) Match(actual interface{}) (bool, error) {
	genericAction, ok := actual.(k8stesting.Action)
	if !ok {
		return false, fmt.Errorf("BeDeleteActionOnResource must be passed an Action from the fakeClientset")
	}
	matcher.actualAction = genericAction

	action, ok := actual.(k8stesting.DeleteAction)
	if !ok {
		return false, nil
	}
	return action.Matches(matcher.expectedVerb, matcher.expectedResourceType) &&
		action.GetNamespace() == matcher.expectedNamespace &&
		action.GetName() == matcher.expectedResourceName, nil

}

func (matcher *DeleteActionMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected '%s' on resource '%s' named '%s' in namespace '%s' to match the observed action:\n%+v\n",
		matcher.expectedVerb, matcher.expectedResourceType, matcher.expectedResourceName, matcher.expectedNamespace, matcher.actualAction)
}

func (matcher *DeleteActionMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected '%s' on resource '%s' named '%s' in namespace '%s' not to match the observed action:\n%+v\n",
		matcher.expectedVerb, matcher.expectedResourceType, matcher.expectedResourceName, matcher.expectedNamespace, matcher.actualAction)
}

func BeUpdateActionOnResource(expectedResourceType, expectedResourceName, expectedNamespace string, updatedResourceMatcher types.GomegaMatcher) types.GomegaMatcher {
	return &UpdateActionMatcher{
		ActionMatcher{
			expectedVerb:         "update",
			expectedResourceType: expectedResourceType,
			expectedNamespace:    expectedNamespace,
		},
		expectedResourceName,
		PointTo(updatedResourceMatcher),
		false,
	}
}

type UpdateActionMatcher struct {
	ActionMatcher
	expectedResourceName         string
	updatedResourceMatcher       types.GomegaMatcher
	failedUpdatedResourceMatcher bool
}

func (matcher *UpdateActionMatcher) Match(actual interface{}) (bool, error) {
	genericAction, ok := actual.(k8stesting.Action)
	if !ok {
		return false, fmt.Errorf("BeUpdateActionOnResource must be passed an Action from the fakeClientset")
	}
	matcher.actualAction = genericAction

	action, ok := actual.(k8stesting.UpdateAction)
	if !ok {
		return false, nil
	}

	updatedObject := reflect.ValueOf(action.GetObject()).Elem()
	objMeta, ok := updatedObject.FieldByName("ObjectMeta").Interface().(metav1.ObjectMeta)
	if !ok {
		return false, fmt.Errorf("object of action was not an object with ObjectMeta")
	}

	// Check the object's Name, Namespace, resource type and the verb of the action first. If this fails, there's
	// no point in running the extra matchers on the updated object.
	if !(action.Matches(matcher.expectedVerb, matcher.expectedResourceType) &&
		action.GetNamespace() == matcher.expectedNamespace &&
		objMeta.GetName() == matcher.expectedResourceName) {
		return false, nil
	}

	passedUpdatedResourceMatcher, err := matcher.updatedResourceMatcher.Match(action.GetObject())
	if err != nil {
		return false, fmt.Errorf("failed to run embedded matcher: %w", err)
	}
	matcher.failedUpdatedResourceMatcher = !passedUpdatedResourceMatcher

	return passedUpdatedResourceMatcher, nil
}

func (matcher *UpdateActionMatcher) FailureMessage(actual interface{}) string {
	if matcher.failedUpdatedResourceMatcher {
		return matcher.updatedResourceMatcher.FailureMessage(actual)
	}
	return fmt.Sprintf("Expected '%s' on resource '%s' named '%s' in namespace '%s' to match the observed action:\n%+v\n",
		matcher.expectedVerb, matcher.expectedResourceType, matcher.expectedResourceName, matcher.expectedNamespace, matcher.actualAction)
}

func (matcher *UpdateActionMatcher) NegatedFailureMessage(actual interface{}) string {
	if matcher.failedUpdatedResourceMatcher {
		return matcher.updatedResourceMatcher.NegatedFailureMessage(actual)
	}
	return fmt.Sprintf("Expected '%s' on resource '%s' named '%s' in namespace '%s' not to match the observed action:\n%+v\n",
		matcher.expectedVerb, matcher.expectedResourceType, matcher.expectedResourceName, matcher.expectedNamespace, matcher.actualAction)
}
