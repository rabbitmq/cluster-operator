package managedresource

import (
	"fmt"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type SuperStreamBindingBuilder struct {
	*Builder
	partitionIndex  int
	routingKey      string
	vhost           string
	rabbitmqCluster *rabbitmqv1beta1.RabbitmqClusterReference
}

func (builder *Builder) SuperStreamBinding(partitionIndex int, routingKey, vhost string, rabbitmqCluster *rabbitmqv1beta1.RabbitmqClusterReference) *SuperStreamBindingBuilder {
	return &SuperStreamBindingBuilder{builder, partitionIndex, routingKey, vhost, rabbitmqCluster}
}

func (builder *SuperStreamBindingBuilder) partitionSuffix() string {
	return fmt.Sprintf("-binding-%d", builder.partitionIndex)
}

func (builder *SuperStreamBindingBuilder) Build() (client.Object, error) {
	return &rabbitmqv1beta1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.GenerateChildResourceName(builder.partitionSuffix()),
			Namespace: builder.ObjectOwner.GetNamespace(),
			Labels: map[string]string{
				AnnotationSuperStream:           builder.ObjectOwner.GetName(),
				AnnotationSuperStreamRoutingKey: builder.routingKey,
			},
		},
	}, nil
}

func (builder *SuperStreamBindingBuilder) Update(object client.Object) error {
	binding := object.(*rabbitmqv1beta1.Binding)

	binding.Spec.Source = builder.ObjectOwner.GetName()
	binding.Spec.DestinationType = "queue"
	binding.Spec.Destination = fmt.Sprintf("%s-%s", builder.ObjectOwner.GetName(), builder.routingKey)
	binding.Spec.RoutingKey = builder.routingKey
	binding.Spec.Vhost = builder.vhost
	binding.Spec.RabbitmqClusterReference = *builder.rabbitmqCluster

	argumentString := fmt.Sprintf(`{"x-stream-partition-order": %d}`, builder.partitionIndex)
	binding.Spec.Arguments = &runtime.RawExtension{Raw: []byte(argumentString)}

	if err := controllerutil.SetControllerReference(builder.ObjectOwner, object, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %w", err)
	}

	return nil
}

func (builder *SuperStreamBindingBuilder) ResourceType() string { return "Binding" }
