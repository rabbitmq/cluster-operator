package scaling_test

import (
	"fmt"
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/v2/internal/scaling"
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
			Resources: corev1.VolumeResourceRequirements{
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
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: size,
				},
			},
		},
	}
}

type actionMatcher struct {
	expectedVerb         string
	expectedResourceType string
	expectedNamespace    string
	actualAction         k8stesting.Action
}

func beGetActionOnResource(expectedResourceType, expectedResourceName, expectedNamespace string) types.GomegaMatcher {
	return &getActionMatcher{
		actionMatcher{
			expectedVerb:         "get",
			expectedResourceType: expectedResourceType,
			expectedNamespace:    expectedNamespace,
		},
		expectedResourceName,
	}
}

type getActionMatcher struct {
	actionMatcher
	expectedResourceName string
}

func (matcher *getActionMatcher) Match(actual any) (bool, error) {
	genericAction, ok := actual.(k8stesting.Action)
	if !ok {
		return false, fmt.Errorf("beGetActionOnResource must be passed an Action from the fakeClientset")
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

func (matcher *getActionMatcher) FailureMessage(actual any) string {
	return fmt.Sprintf("Expected '%s' on resource '%s' named '%s' in namespace '%s' to match the observed action:\n%+v\n",
		matcher.expectedVerb, matcher.expectedResourceType, matcher.expectedResourceName, matcher.expectedNamespace, matcher.actualAction)
}

func (matcher *getActionMatcher) NegatedFailureMessage(actual any) string {
	return fmt.Sprintf("Expected '%s' on resource '%s' named '%s' in namespace '%s' not to match the observed action:\n%+v\n",
		matcher.expectedVerb, matcher.expectedResourceType, matcher.expectedResourceName, matcher.expectedNamespace, matcher.actualAction)
}

func beDeleteActionOnResource(expectedResourceType, expectedResourceName, expectedNamespace string) types.GomegaMatcher {
	return &deleteActionMatcher{
		actionMatcher{
			expectedVerb:         "delete",
			expectedResourceType: expectedResourceType,
			expectedNamespace:    expectedNamespace,
		},
		expectedResourceName,
	}
}

type deleteActionMatcher struct {
	actionMatcher
	expectedResourceName string
}

func (matcher *deleteActionMatcher) Match(actual any) (bool, error) {
	genericAction, ok := actual.(k8stesting.Action)
	if !ok {
		return false, fmt.Errorf("beDeleteActionOnResource must be passed an Action from the fakeClientset")
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

func (matcher *deleteActionMatcher) FailureMessage(actual any) string {
	return fmt.Sprintf("Expected '%s' on resource '%s' named '%s' in namespace '%s' to match the observed action:\n%+v\n",
		matcher.expectedVerb, matcher.expectedResourceType, matcher.expectedResourceName, matcher.expectedNamespace, matcher.actualAction)
}

func (matcher *deleteActionMatcher) NegatedFailureMessage(actual any) string {
	return fmt.Sprintf("Expected '%s' on resource '%s' named '%s' in namespace '%s' not to match the observed action:\n%+v\n",
		matcher.expectedVerb, matcher.expectedResourceType, matcher.expectedResourceName, matcher.expectedNamespace, matcher.actualAction)
}

func beUpdateActionOnResource(expectedResourceType, expectedResourceName, expectedNamespace string, updatedResourceMatcher types.GomegaMatcher) types.GomegaMatcher {
	return &updateActionMatcher{
		actionMatcher{
			expectedVerb:         "update",
			expectedResourceType: expectedResourceType,
			expectedNamespace:    expectedNamespace,
		},
		expectedResourceName,
		PointTo(updatedResourceMatcher),
		false,
	}
}

type updateActionMatcher struct {
	actionMatcher
	expectedResourceName         string
	updatedResourceMatcher       types.GomegaMatcher
	failedUpdatedResourceMatcher bool
}

func (matcher *updateActionMatcher) Match(actual any) (bool, error) {
	genericAction, ok := actual.(k8stesting.Action)
	if !ok {
		return false, fmt.Errorf("beUpdateActionOnResource must be passed an Action from the fakeClientset")
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
	if !action.Matches(matcher.expectedVerb, matcher.expectedResourceType) ||
		action.GetNamespace() != matcher.expectedNamespace ||
		objMeta.GetName() != matcher.expectedResourceName {
		return false, nil
	}

	passedUpdatedResourceMatcher, err := matcher.updatedResourceMatcher.Match(action.GetObject())
	if err != nil {
		return false, fmt.Errorf("failed to run embedded matcher: %w", err)
	}
	matcher.failedUpdatedResourceMatcher = !passedUpdatedResourceMatcher

	return passedUpdatedResourceMatcher, nil
}

func (matcher *updateActionMatcher) FailureMessage(actual any) string {
	if matcher.failedUpdatedResourceMatcher {
		return matcher.updatedResourceMatcher.FailureMessage(actual)
	}
	return fmt.Sprintf("Expected '%s' on resource '%s' named '%s' in namespace '%s' to match the observed action:\n%+v\n",
		matcher.expectedVerb, matcher.expectedResourceType, matcher.expectedResourceName, matcher.expectedNamespace, matcher.actualAction)
}

func (matcher *updateActionMatcher) NegatedFailureMessage(actual any) string {
	if matcher.failedUpdatedResourceMatcher {
		return matcher.updatedResourceMatcher.NegatedFailureMessage(actual)
	}
	return fmt.Sprintf("Expected '%s' on resource '%s' named '%s' in namespace '%s' not to match the observed action:\n%+v\n",
		matcher.expectedVerb, matcher.expectedResourceType, matcher.expectedResourceName, matcher.expectedNamespace, matcher.actualAction)
}
