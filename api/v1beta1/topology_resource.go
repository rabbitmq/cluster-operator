package v1beta1

import "sigs.k8s.io/controller-runtime/pkg/client"

var _ TopologyResource = &Binding{}
var _ TopologyResource = &Exchange{}
var _ TopologyResource = &Federation{}
var _ TopologyResource = &Permission{}
var _ TopologyResource = &Queue{}
var _ TopologyResource = &SchemaReplication{}
var _ TopologyResource = &Shovel{}
var _ TopologyResource = &User{}
var _ TopologyResource = &Vhost{}

// +k8s:deepcopy-gen=false
type TopologyResource interface {
	client.Object
	RabbitReference() RabbitmqClusterReference
	SetStatusConditions([]Condition)
}
