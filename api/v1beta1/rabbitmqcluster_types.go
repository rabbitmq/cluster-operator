// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

package v1beta1

import (
	"strings"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/rabbitmq/cluster-operator/internal/status"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +kubebuilder:object:root=true

// RabbitmqCluster is the Schema for the rabbitmqclusters API
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.clusterStatus"
// +kubebuilder:resource:shortName={"rmq"}
type RabbitmqCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RabbitmqClusterSpec   `json:"spec,omitempty"`
	Status RabbitmqClusterStatus `json:"status,omitempty"`
}

// Spec is the desired state of the RabbitmqCluster Custom Resource.
type RabbitmqClusterSpec struct {
	// Replicas is the number of nodes in the RabbitMQ cluster. Each node is deployed as a Replica in a StatefulSet. Only 1, 3, 5 replicas clusters are tested.
	// +optional
	// +kubebuilder:validation:Minimum:=0
	// +kubebuilder:default:=1
	Replicas *int32 `json:"replicas,omitempty"`
	// Image is the name of the RabbitMQ docker image to use for RabbitMQ nodes in the RabbitmqCluster.
	// +kubebuilder:default:="rabbitmq:3.8.9-management"
	Image string `json:"image,omitempty"`
	// List of Secret resource containing access credentials to the registry for the RabbitMQ image. Required if the docker registry is private.
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// +kubebuilder:default:={type: "ClusterIP"}
	Service RabbitmqClusterServiceSpec `json:"service,omitempty"`
	// +kubebuilder:default:={storage: "10Gi"}
	Persistence RabbitmqClusterPersistenceSpec `json:"persistence,omitempty"`
	// +kubebuilder:default:={limits: {cpu: "2000m", memory: "2Gi"}, requests: {cpu: "1000m", memory: "2Gi"}}
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	Affinity  *corev1.Affinity             `json:"affinity,omitempty"`
	// Tolerations is the list of Toleration resources attached to each Pod in the RabbitmqCluster.
	Tolerations []corev1.Toleration              `json:"tolerations,omitempty"`
	Rabbitmq    RabbitmqClusterConfigurationSpec `json:"rabbitmq,omitempty"`
	TLS         TLSSpec                          `json:"tls,omitempty"`
	Override    RabbitmqClusterOverrideSpec      `json:"override,omitempty"`
	// If unset, or set to false, the cluster will run `rabbitmq-queues rebalance all` whenever the cluster is updated.
	// Has no effect if the cluster only consists of one node.
	// For more information, see https://www.rabbitmq.com/rabbitmq-queues.8.html#rebalance
	SkipPostDeploySteps bool `json:"skipPostDeploySteps,omitempty"`
	// TerminationGracePeriodSeconds is the timeout that each rabbitmqcluster pod will have to terminate gracefully.
	// It defaults to 604800 seconds ( a week long) to ensure that the container preStop lifecycle hook can finish running.
	// For more information, see: https://github.com/rabbitmq/cluster-operator/blob/main/docs/design/20200520-graceful-pod-termination.md
	// +kubebuilder:validation:Minimum:=0
	// +kubebuilder:default:=604800
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
}

type RabbitmqClusterOverrideSpec struct {
	StatefulSet *StatefulSet `json:"statefulSet,omitempty"`
	Service     *Service     `json:"service,omitempty"`
}

type Service struct {
	// +optional
	*EmbeddedLabelsAnnotations `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Spec defines the behavior of a service.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec *corev1.ServiceSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

type StatefulSet struct {
	// +optional
	*EmbeddedLabelsAnnotations `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Spec defines the desired identities of pods in this set.
	// +optional
	Spec *StatefulSetSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// StatefulSetSpec contains a subset of the fields included in k8s.io/api/apps/v1.StatefulSetSpec.
// Field RevisionHistoryLimit is omitted.
// Every field is made optional.
type StatefulSetSpec struct {
	// replicas is the desired number of replicas of the given Template.
	// These are replicas in the sense that they are instantiations of the
	// same Template, but individual replicas also have a consistent identity.
	// +optional
	Replicas *int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`

	// selector is a label query over pods that should match the replica count.
	// It must match the pod template's labels.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty" protobuf:"bytes,2,opt,name=selector"`

	// template is the object that describes the pod that will be created if
	// insufficient replicas are detected. Each pod stamped out by the StatefulSet
	// will fulfill this Template, but have a unique identity from the rest
	// of the StatefulSet.
	// +optional
	Template *PodTemplateSpec `json:"template,omitempty" protobuf:"bytes,3,opt,name=template"`

	// volumeClaimTemplates is a list of claims that pods are allowed to reference.
	// The StatefulSet controller is responsible for mapping network identities to
	// claims in a way that maintains the identity of a pod. Every claim in
	// this list must have at least one matching (by name) volumeMount in one
	// container in the template. A claim in this list takes precedence over
	// any volumes in the template, with the same name.
	// +optional
	VolumeClaimTemplates []PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty" protobuf:"bytes,4,rep,name=volumeClaimTemplates"`

	// serviceName is the name of the service that governs this StatefulSet.
	// This service must exist before the StatefulSet, and is responsible for
	// the network identity of the set. Pods get DNS/hostnames that follow the
	// pattern: pod-specific-string.serviceName.default.svc.cluster.local
	// where "pod-specific-string" is managed by the StatefulSet controller.
	// +optional
	ServiceName string `json:"serviceName,omitempty" protobuf:"bytes,5,opt,name=serviceName"`

	// podManagementPolicy controls how pods are created during initial scale up,
	// when replacing pods on nodes, or when scaling down. The default policy is
	// `OrderedReady`, where pods are created in increasing order (pod-0, then
	// pod-1, etc) and the controller will wait until each pod is ready before
	// continuing. When scaling down, the pods are removed in the opposite order.
	// The alternative policy is `Parallel` which will create pods in parallel
	// to match the desired scale without waiting, and on scale down will delete
	// all pods at once.
	// +optional
	PodManagementPolicy appsv1.PodManagementPolicyType `json:"podManagementPolicy,omitempty" protobuf:"bytes,6,opt,name=podManagementPolicy,casttype=PodManagementPolicyType"`

	// updateStrategy indicates the StatefulSetUpdateStrategy that will be
	// employed to update Pods in the StatefulSet when a revision is made to
	// Template.
	// +optional
	UpdateStrategy *appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty" protobuf:"bytes,7,opt,name=updateStrategy"`
}

// It is used in Service and StatefulSet
type EmbeddedLabelsAnnotations struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,11,rep,name=labels"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,12,rep,name=annotations"`
}

// EmbeddedObjectMeta contains a subset of the fields included in k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
// Only fields which are relevant to embedded resources are included.
// It is used in PersistentVolumeClaim and PodTemplate
type EmbeddedObjectMeta struct {
	// Name must be unique within a namespace. Is required when creating resources, although
	// some resources may allow a client to request the generation of an appropriate name
	// automatically. Name is primarily intended for creation idempotence and configuration
	// definition.
	// Cannot be updated.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#names
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// Namespace defines the space within each name must be unique. An empty namespace is
	// equivalent to the "default" namespace, but "default" is the canonical representation.
	// Not all objects are required to be scoped to a namespace - the value of this field for
	// those objects will be empty.
	//
	// Must be a DNS_LABEL.
	// Cannot be updated.
	// More info: http://kubernetes.io/docs/user-guide/namespaces
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`

	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,11,rep,name=labels"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,12,rep,name=annotations"`
}

// PodTemplateSpec is an embedded version of k8s.io/api/core/v1.PodTemplateSpec.
// It contains a reduced ObjectMeta.
type PodTemplateSpec struct {
	// +optional
	*EmbeddedObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the pod.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec *corev1.PodSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// PersistentVolumeClaim is an embedded version of k8s.io/api/core/v1.PersistentVolumeClaim.
// It contains TypeMeta and a reduced ObjectMeta.
// Field status is omitted.
type PersistentVolumeClaim struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	EmbeddedObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the desired characteristics of a volume requested by a pod author.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims
	// +optional
	Spec corev1.PersistentVolumeClaimSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

type TLSSpec struct {
	// Name of a Secret in the same Namespace as the RabbitmqCluster, containing the server's private key & public certificate for TLS.
	// The Secret must store these as tls.key and tls.crt, respectively.
	SecretName string `json:"secretName,omitempty"`
	// Name of a Secret in the same Namespace as the RabbitmqCluster, containing the Certificate Authority's public certificate for TLS.
	// The Secret must store this as ca.crt.
	// Used for mTLS, and TLS for rabbitmq_web_stomp and rabbitmq_web_mqtt.
	CaSecretName string `json:"caSecretName,omitempty"`
}

// kubebuilder validating tags 'Pattern' and 'MaxLength' must be specified on string type.
// Alias type 'string' as 'Plugin' to specify schema validation on items of the list 'AdditionalPlugins'
// +kubebuilder:validation:Pattern:="^\\w+$"
// +kubebuilder:validation:MaxLength=100
type Plugin string

// Rabbitmq related configurations
type RabbitmqClusterConfigurationSpec struct {
	// List of plugins to enable in addition to essential plugins: rabbitmq_management, rabbitmq_prometheus, and rabbitmq_peer_discovery_k8s.
	// +kubebuilder:validation:MaxItems:=100
	AdditionalPlugins []Plugin `json:"additionalPlugins,omitempty"`
	// Modify to add to the rabbitmq.conf file in addition to default configurations set by the operator. Modifying this property on an existing RabbitmqCluster will trigger a StatefulSet rolling restart and will cause rabbitmq downtime.
	// +kubebuilder:validation:MaxLength:=2000
	AdditionalConfig string `json:"additionalConfig,omitempty"`
	// Specify any rabbitmq advanced.config configurations
	// +kubebuilder:validation:MaxLength:=100000
	AdvancedConfig string `json:"advancedConfig,omitempty"`
	// Modify to add to the rabbitmq-env.conf file. Modifying this property on an existing RabbitmqCluster will trigger a StatefulSet rolling restart and will cause rabbitmq downtime.
	// +kubebuilder:validation:MaxLength:=100000
	EnvConfig string `json:"envConfig,omitempty"`
}

// The settings for the persistent storage desired for each Pod in the RabbitmqCluster.
type RabbitmqClusterPersistenceSpec struct {
	// StorageClassName is the name of the StorageClass to claim a PersistentVolume from.
	StorageClassName *string `json:"storageClassName,omitempty"`
	// The requested size of the persistent volume attached to each Pod in the RabbitmqCluster.
	// +kubebuilder:default:="10Gi"
	Storage *k8sresource.Quantity `json:"storage,omitempty"`
}

// Settable attributes for the Service resource.
type RabbitmqClusterServiceSpec struct {
	// +kubebuilder:validation:Enum=ClusterIP;LoadBalancer;NodePort
	// +kubebuilder:default:="ClusterIP"
	Type corev1.ServiceType `json:"type,omitempty"`
	// Annotations to add to the Service.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Status presents the observed state of RabbitmqCluster
type RabbitmqClusterStatus struct {
	ClusterStatus string `json:"clusterStatus,omitempty"`
	// Set of Conditions describing the current state of the RabbitmqCluster
	Conditions []status.RabbitmqClusterCondition `json:"conditions"`

	// Identifying information on internal resources
	DefaultUser *RabbitmqClusterDefaultUser `json:"defaultUser,omitempty"`
}

type RabbitmqClusterDefaultUser struct {
	SecretReference  *RabbitmqClusterSecretReference  `json:"secretReference,omitempty"`
	ServiceReference *RabbitmqClusterServiceReference `json:"serviceReference,omitempty"`
}

type RabbitmqClusterSecretReference struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Keys      map[string]string `json:"keys"`
}

type RabbitmqClusterServiceReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

func (cluster *RabbitmqCluster) TLSEnabled() bool {
	return cluster.Spec.TLS.SecretName != ""
}

func (cluster *RabbitmqCluster) MutualTLSEnabled() bool {
	return cluster.TLSEnabled() && cluster.Spec.TLS.CaSecretName != ""
}

func (cluster *RabbitmqCluster) MemoryLimited() bool {
	return cluster.Spec.Resources != nil && cluster.Spec.Resources.Limits != nil && !cluster.Spec.Resources.Limits.Memory().IsZero()
}

func (cluster *RabbitmqCluster) SingleTLSSecret() bool {
	return cluster.MutualTLSEnabled() && cluster.Spec.TLS.CaSecretName == cluster.Spec.TLS.SecretName
}

func (cluster *RabbitmqCluster) AdditionalPluginEnabled(plugin Plugin) bool {
	for _, p := range cluster.Spec.Rabbitmq.AdditionalPlugins {
		if p == plugin {
			return true
		}
	}
	return false
}

func (clusterStatus *RabbitmqClusterStatus) SetConditions(resources []runtime.Object) {
	var oldAllPodsReadyCondition *status.RabbitmqClusterCondition
	var oldClusterAvailableCondition *status.RabbitmqClusterCondition
	var oldNoWarningsCondition *status.RabbitmqClusterCondition
	var oldReconcileCondition *status.RabbitmqClusterCondition

	for _, condition := range clusterStatus.Conditions {
		switch condition.Type {
		case status.AllReplicasReady:
			oldAllPodsReadyCondition = condition.DeepCopy()
		case status.ClusterAvailable:
			oldClusterAvailableCondition = condition.DeepCopy()
		case status.NoWarnings:
			oldNoWarningsCondition = condition.DeepCopy()
		case status.ReconcileSuccess:
			oldReconcileCondition = condition.DeepCopy()
		}
	}

	allReplicasReadyCond := status.AllReplicasReadyCondition(resources, oldAllPodsReadyCondition)
	clusterAvailableCond := status.ClusterAvailableCondition(resources, oldClusterAvailableCondition)
	noWarningsCond := status.NoWarningsCondition(resources, oldNoWarningsCondition)

	var reconciledCondition status.RabbitmqClusterCondition
	if oldReconcileCondition != nil {
		reconciledCondition = *oldReconcileCondition
	} else {
		reconciledCondition = status.ReconcileSuccessCondition(corev1.ConditionUnknown, "Initialising", "")
	}

	clusterStatus.Conditions = []status.RabbitmqClusterCondition{
		allReplicasReadyCond,
		clusterAvailableCond,
		noWarningsCond,
		reconciledCondition,
	}
}

func (clusterStatus *RabbitmqClusterStatus) SetCondition(condType status.RabbitmqClusterConditionType,
	condStatus corev1.ConditionStatus, reason string, messages ...string) {
	for i := range clusterStatus.Conditions {
		if clusterStatus.Conditions[i].Type == condType {
			clusterStatus.Conditions[i].UpdateState(condStatus)
			clusterStatus.Conditions[i].UpdateReason(reason, messages...)
			break
		}
	}
}

// +kubebuilder:object:root=true

// RabbitmqClusterList contains a list of RabbitmqCluster
type RabbitmqClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RabbitmqCluster `json:"items"`
}

func (cluster RabbitmqCluster) ChildResourceName(name string) string {
	return strings.TrimSuffix(strings.Join([]string{cluster.Name, name}, "-"), "-")
}

func init() {
	SchemeBuilder.Register(&RabbitmqCluster{}, &RabbitmqClusterList{})
}
