package resource

import (
	"fmt"
	"strings"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type RabbitmqResourceBuilder struct {
	Instance             *rabbitmqv1beta1.RabbitmqCluster
	DefaultConfiguration DefaultConfiguration
}

type DefaultConfiguration struct {
	ServiceAnnotations         map[string]string
	ServiceType                string
	Scheme                     *runtime.Scheme
	ImageReference             string
	ImagePullSecret            string
	PersistentStorage          string
	PersistentStorageClassName string
	ResourceRequirements       ResourceRequirements
	OperatorRegistrySecret     *corev1.Secret
}

type ResourceBuilder interface {
	Update(runtime.Object) error
	Build() (runtime.Object, error)
}

func (builder *RabbitmqResourceBuilder) Resources() (resources []runtime.Object, err error) {
	serverConf := builder.ServerConfigMap()
	resources = append(resources, serverConf)

	headlessService := builder.HeadlessService()
	resources = append(resources, headlessService)

	adminSecret, err := builder.AdminSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate admin secret: %v ", err)
	}
	resources = append(resources, adminSecret)

	erlangCookie, err := builder.ErlangCookie()
	if err != nil {
		return nil, fmt.Errorf("failed to generate erlang cookie: %v ", err)
	}
	resources = append(resources, erlangCookie)

	if builder.DefaultConfiguration.OperatorRegistrySecret != nil {
		clusterRegistrySecret := builder.RegistrySecret()
		resources = append(resources, clusterRegistrySecret)
	}

	serviceAccount := builder.ServiceAccount()
	resources = append(resources, serviceAccount)

	role := builder.Role()
	resources = append(resources, role)

	roleBinding := builder.RoleBinding()
	resources = append(resources, roleBinding)

	return resources, nil
}

func (builder *RabbitmqResourceBuilder) updateLabels(objectMeta *metav1.ObjectMeta) {
	if builder.Instance.Labels != nil {
		if objectMeta.Labels == nil {
			objectMeta.Labels = make(map[string]string)
		}
		for label, value := range builder.Instance.Labels {
			if !strings.HasPrefix(label, "app.kubernetes.io") {
				// TODO if a label is in the StatefulSet and in the CR, the value in the CR will overwrite the value in STS
				objectMeta.Labels[label] = value
			}
		}
	}
}
