// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

package v1beta1

import (
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AllReplicasReady",type="string",JSONPath=".status.conditions[?(@.type == 'AllReplicasReady')].status"
// +kubebuilder:printcolumn:name="ReconcileSuccess",type="string",JSONPath=".status.conditions[?(@.type == 'ReconcileSuccess')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:shortName={"rmq"},categories=all;rabbitmq
// RabbitmqCluster is the Schema for the RabbitmqCluster API. Each instance of this object
// corresponds to a single RabbitMQ cluster.
type RabbitmqCluster struct {
	// Embedded metadata identifying a Kind and API Verison of an object.
	// For more info, see: https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#TypeMeta
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the RabbitmqCluster Custom Resource.
	Spec RabbitmqClusterSpec `json:"spec,omitempty"`
	// Status presents the observed state of RabbitmqCluster
	Status RabbitmqClusterStatus `json:"status,omitempty"`
}

// Spec is the desired state of the RabbitmqCluster Custom Resource.
type RabbitmqClusterSpec struct {
	// Replicas is the number of nodes in the RabbitMQ cluster. Each node is deployed as a Replica in a StatefulSet. Only 1, 3, 5 replicas clusters are tested.
	// This value should be an odd number to ensure the resultant cluster can establish exactly one quorum of nodes
	// in the event of a fragmenting network partition.
	// +optional
	// +kubebuilder:validation:Minimum:=0
	// +kubebuilder:default:=1
	Replicas *int32 `json:"replicas,omitempty"`
	// Image is the name of the RabbitMQ docker image to use for RabbitMQ nodes in the RabbitmqCluster.
	// Must be provided together with ImagePullSecrets in order to use an image in a private registry.
	Image string `json:"image,omitempty"`
	// List of Secret resource containing access credentials to the registry for the RabbitMQ image. Required if the docker registry is private.
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// The desired state of the Kubernetes Service to create for the cluster.
	// +kubebuilder:default:={type: "ClusterIP"}
	Service RabbitmqClusterServiceSpec `json:"service,omitempty"`
	// The desired persistent storage configuration for each Pod in the cluster.
	// +kubebuilder:default:={storage: "10Gi"}
	Persistence RabbitmqClusterPersistenceSpec `json:"persistence,omitempty"`
	// The desired compute resource requirements of Pods in the cluster.
	// +kubebuilder:default:={limits: {cpu: "2000m", memory: "2Gi"}, requests: {cpu: "1000m", memory: "2Gi"}}
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// Affinity scheduling rules to be applied on created Pods.
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// Tolerations is the list of Toleration resources attached to each Pod in the RabbitmqCluster.
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// Configuration options for RabbitMQ Pods created in the cluster.
	Rabbitmq RabbitmqClusterConfigurationSpec `json:"rabbitmq,omitempty"`
	// TLS-related configuration for the RabbitMQ cluster.
	TLS TLSSpec `json:"tls,omitempty"`
	// Provides the ability to override the generated manifest of several child resources.
	Override RabbitmqClusterOverrideSpec `json:"override,omitempty"`
	// If unset, or set to false, the cluster will run `rabbitmq-queues rebalance all` whenever the cluster is updated.
	// Set to true to prevent the operator rebalancing queue leaders after a cluster update.
	// Has no effect if the cluster only consists of one node.
	// For more information, see https://www.rabbitmq.com/rabbitmq-queues.8.html#rebalance
	SkipPostDeploySteps bool `json:"skipPostDeploySteps,omitempty"`
	// TerminationGracePeriodSeconds is the timeout that each rabbitmqcluster pod will have to terminate gracefully.
	// It defaults to 604800 seconds ( a week long) to ensure that the container preStop lifecycle hook can finish running.
	// For more information, see: https://github.com/rabbitmq/cluster-operator/blob/main/docs/design/20200520-graceful-pod-termination.md
	// +kubebuilder:validation:Minimum:=0
	// +kubebuilder:default:=604800
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
	// Secret backend configuration for the RabbitmqCluster.
	// Enables to fetch default user credentials and certificates from K8s external secret stores.
	SecretBackend SecretBackend `json:"secretBackend,omitempty"`
}

// SecretBackend configures a single secret backend.
// Today, only Vault exists as supported secret backend.
// Future secret backends could be Secrets Store CSI Driver.
// If not configured, K8s Secrets will be used.
type SecretBackend struct {
	Vault *VaultSpec `json:"vault,omitempty"`
}

// VaultSpec will add Vault annotations (see https://www.vaultproject.io/docs/platform/k8s/injector/annotations)
// to RabbitMQ Pods. It requires a Vault Agent Sidecar Injector (https://www.vaultproject.io/docs/platform/k8s/injector)
// to be installed in the K8s cluster. The injector is a K8s Mutation Webhook Controller that alters RabbitMQ Pod specifications
// (based on the added Vault annotations) to include Vault Agent containers that render Vault secrets to the volume.
type VaultSpec struct {
	// Role in Vault.
	// If vault.defaultUserPath is set, this role must have capability to read the pre-created default user credential in Vault.
	// If vault.tls is set, this role must have capability to create and update certificates in the Vault PKI engine for the domains
	// "<namespace>" and "<namespace>.svc".
	Role string `json:"role,omitempty"`
	// Vault annotations that override the Vault annotations set by the cluster-operator.
	// For a list of valid Vault annotations, see https://www.vaultproject.io/docs/platform/k8s/injector/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// Path in Vault to access a KV (Key-Value) secret with the fields username and password for the default user.
	// For example "secret/data/rabbitmq/config".
	DefaultUserPath string `json:"defaultUserPath,omitempty"`
	// Sidecar container that updates the default user's password in RabbitMQ when it changes in Vault.
	// Additionally, it updates /var/lib/rabbitmq/.rabbitmqadmin.conf (used by rabbitmqadmin CLI).
	// Set to empty string to disable the sidecar container.
	DefaultUserUpdaterImage *string      `json:"defaultUserUpdaterImage,omitempty"`
	TLS                     VaultTLSSpec `json:"tls,omitempty"`
}

type VaultTLSSpec struct {
	// Path in Vault PKI engine.
	// For example "pki/issue/hashicorp-com".
	// required
	PKIIssuerPath string `json:"pkiIssuerPath,omitempty"`
	// Specifies the requested certificate Common Name (CN).
	// Defaults to <serviceName>.<namespace>.svc if not provided.
	// +optional
	CommonName string `json:"commonName,omitempty"`
	// Specifies the requested Subject Alternative Names (SANs), in a comma-delimited list.
	// These will be appended to the SANs added by the cluster-operator.
	// The cluster-operator will add SANs:
	// "<RabbitmqCluster name>-server-<index>.<RabbitmqCluster name>-nodes.<namespace>" for each pod,
	// e.g. "myrabbit-server-0.myrabbit-nodes.default".
	// +optional
	AltNames string `json:"altNames,omitempty"`
	// Specifies the requested IP Subject Alternative Names, in a comma-delimited list.
	// +optional
	IpSans string `json:"ipSans,omitempty"`
}

func (spec *VaultSpec) TLSEnabled() bool {
	return spec.TLS.PKIIssuerPath != ""
}
func (spec *VaultSpec) DefaultUserSecretEnabled() bool {
	return spec.DefaultUserPath != ""
}

// Provides the ability to override the generated manifest of several child resources.
type RabbitmqClusterOverrideSpec struct {
	// Override configuration for the RabbitMQ StatefulSet.
	StatefulSet *StatefulSet `json:"statefulSet,omitempty"`
	// Override configuration for the Service created to serve traffic to the cluster.
	Service *Service `json:"service,omitempty"`
}

// Override configuration for the Service created to serve traffic to the cluster.
// Allows for the manifest of the created Service to be overwritten with custom configuration.
type Service struct {
	// +optional
	*EmbeddedLabelsAnnotations `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Spec defines the behavior of a Service.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec *corev1.ServiceSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// Override configuration for the RabbitMQ StatefulSet.
// Allows for the manifest of the created StatefulSet to be overwritten with custom configuration.
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
	// replicas corresponds to the desired number of Pods in the StatefulSet.
	// For more info, see https://pkg.go.dev/k8s.io/api/apps/v1#StatefulSetSpec
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

// EmbeddedLabelsAnnotations is an embedded subset of the fields included in k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta.
// Only labels and annotations are included.
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

// EmbeddedObjectMeta is an embedded subset of the fields included in k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta.
// Only fields which are relevant to embedded resources are included.
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
	// Embedded metadata identifying a Kind and API Verison of an object.
	// For more info, see: https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#TypeMeta
	metav1.TypeMeta `json:",inline"`
	// +optional
	EmbeddedObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the desired characteristics of a volume requested by a pod author.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims
	// +optional
	Spec corev1.PersistentVolumeClaimSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// Allows for the configuration of TLS certificates to be used by RabbitMQ. Also allows for non-TLS traffic to be disabled.
type TLSSpec struct {
	// Name of a Secret in the same Namespace as the RabbitmqCluster, containing the server's private key & public certificate for TLS.
	// The Secret must store these as tls.key and tls.crt, respectively.
	// This Secret can be created by running `kubectl create secret tls tls-secret --cert=path/to/tls.cert --key=path/to/tls.key`
	SecretName string `json:"secretName,omitempty"`
	// Name of a Secret in the same Namespace as the RabbitmqCluster, containing the Certificate Authority's public certificate for TLS.
	// The Secret must store this as ca.crt.
	// This Secret can be created by running `kubectl create secret generic ca-secret --from-file=ca.crt=path/to/ca.cert`
	// Used for mTLS, and TLS for rabbitmq_web_stomp and rabbitmq_web_mqtt.
	CaSecretName string `json:"caSecretName,omitempty"`
	// When set to true, the RabbitmqCluster disables non-TLS listeners for RabbitMQ, management plugin and for any enabled plugins in the following list: stomp, mqtt, web_stomp, web_mqtt.
	// Only TLS-enabled clients will be able to connect.
	DisableNonTLSListeners bool `json:"disableNonTLSListeners,omitempty"`
}

// kubebuilder validating tags 'Pattern' and 'MaxLength' must be specified on string type.
// Alias type 'string' as 'Plugin' to specify schema validation on items of the list 'AdditionalPlugins'

// A Plugin to enable on the RabbitmqCluster.
// +kubebuilder:validation:Pattern:="^\\w+$"
// +kubebuilder:validation:MaxLength=100
type Plugin string

// RabbitMQ-related configuration.
type RabbitmqClusterConfigurationSpec struct {
	// List of plugins to enable in addition to essential plugins: rabbitmq_management, rabbitmq_prometheus, and rabbitmq_peer_discovery_k8s.
	// +kubebuilder:validation:MaxItems:=100
	AdditionalPlugins []Plugin `json:"additionalPlugins,omitempty"`
	// Modify to add to the rabbitmq.conf file in addition to default configurations set by the operator.
	// Modifying this property on an existing RabbitmqCluster will trigger a StatefulSet rolling restart and will cause rabbitmq downtime.
	// For more information on this config, see https://www.rabbitmq.com/configure.html#config-file
	// +kubebuilder:validation:MaxLength:=2000
	AdditionalConfig string `json:"additionalConfig,omitempty"`
	// Specify any rabbitmq advanced.config configurations to apply to the cluster.
	// For more information on advanced config, see https://www.rabbitmq.com/configure.html#advanced-config-file
	// +kubebuilder:validation:MaxLength:=100000
	AdvancedConfig string `json:"advancedConfig,omitempty"`
	// Modify to add to the rabbitmq-env.conf file. Modifying this property on an existing RabbitmqCluster will trigger a StatefulSet rolling restart and will cause rabbitmq downtime.
	// For more information on env config, see https://www.rabbitmq.com/man/rabbitmq-env.conf.5.html
	// +kubebuilder:validation:MaxLength:=100000
	EnvConfig string `json:"envConfig,omitempty"`
}

// The settings for the persistent storage desired for each Pod in the RabbitmqCluster.
type RabbitmqClusterPersistenceSpec struct {
	// The name of the StorageClass to claim a PersistentVolume from.
	StorageClassName *string `json:"storageClassName,omitempty"`
	// The requested size of the persistent volume attached to each Pod in the RabbitmqCluster.
	// The format of this field matches that defined by kubernetes/apimachinery.
	// See https://pkg.go.dev/k8s.io/apimachinery/pkg/api/resource#Quantity for more info on the format of this field.
	// +kubebuilder:default:="10Gi"
	Storage *k8sresource.Quantity `json:"storage,omitempty"`
}

// Settable attributes for the Service resource.
type RabbitmqClusterServiceSpec struct {
	// Type of Service to create for the cluster. Must be one of: ClusterIP, LoadBalancer, NodePort.
	// For more info see https://pkg.go.dev/k8s.io/api/core/v1#ServiceType
	// +kubebuilder:validation:Enum=ClusterIP;LoadBalancer;NodePort
	// +kubebuilder:default:="ClusterIP"
	Type corev1.ServiceType `json:"type,omitempty"`
	// Annotations to add to the Service.
	Annotations map[string]string `json:"annotations,omitempty"`
}

func (cluster *RabbitmqCluster) TLSEnabled() bool {
	return cluster.SecretTLSEnabled() || cluster.VaultTLSEnabled()
}
func (cluster *RabbitmqCluster) SecretTLSEnabled() bool {
	return cluster.Spec.TLS.SecretName != ""
}

func (cluster *RabbitmqCluster) MutualTLSEnabled() bool {
	return (cluster.SecretTLSEnabled() && cluster.Spec.TLS.CaSecretName != "") || cluster.VaultTLSEnabled()
}

func (cluster *RabbitmqCluster) MemoryLimited() bool {
	return cluster.Spec.Resources != nil && cluster.Spec.Resources.Limits != nil && !cluster.Spec.Resources.Limits.Memory().IsZero()
}

func (cluster *RabbitmqCluster) SingleTLSSecret() bool {
	return cluster.MutualTLSEnabled() && cluster.Spec.TLS.CaSecretName == cluster.Spec.TLS.SecretName
}

func (cluster *RabbitmqCluster) DisableNonTLSListeners() bool {
	return cluster.Spec.TLS.DisableNonTLSListeners
}

func (cluster *RabbitmqCluster) AdditionalPluginEnabled(plugin Plugin) bool {
	for _, p := range cluster.Spec.Rabbitmq.AdditionalPlugins {
		if p == plugin {
			return true
		}
	}
	return false
}

// the OSR plugin `rabbitmq_multi_dc_replication` enables `rabbitmq_stream` as a dependency
func (cluster *RabbitmqCluster) StreamNeeded() bool {
	return cluster.AdditionalPluginEnabled("rabbitmq_stream") || cluster.AdditionalPluginEnabled("rabbitmq_multi_dc_replication")
}

func (cluster *RabbitmqCluster) VaultEnabled() bool {
	return cluster.Spec.SecretBackend.Vault != nil
}

func (cluster *RabbitmqCluster) UsesDefaultUserUpdaterImage() bool {
	return cluster.VaultEnabled() && cluster.Spec.SecretBackend.Vault.DefaultUserUpdaterImage == nil
}

func (cluster *RabbitmqCluster) VaultDefaultUserSecretEnabled() bool {
	return cluster.VaultEnabled() && cluster.Spec.SecretBackend.Vault.DefaultUserSecretEnabled()
}

func (cluster *RabbitmqCluster) VaultTLSEnabled() bool {
	return cluster.VaultEnabled() && cluster.Spec.SecretBackend.Vault.TLSEnabled()
}

func (cluster *RabbitmqCluster) ServiceSubDomain() string {
	return fmt.Sprintf("%s.%s.svc", cluster.Name, cluster.Namespace)
}

// +kubebuilder:object:root=true

// RabbitmqClusterList contains a list of RabbitmqClusters.
type RabbitmqClusterList struct {
	// Embedded metadata identifying a Kind and API Verison of an object.
	// For more info, see: https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#TypeMeta
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// Array of RabbitmqCluster resources.
	Items []RabbitmqCluster `json:"items"`
}

func (cluster RabbitmqCluster) ChildResourceName(name string) string {
	return strings.TrimSuffix(strings.Join([]string{cluster.Name, name}, "-"), "-")
}

func (cluster RabbitmqCluster) PVCName(i int) string {
	return strings.Join([]string{"persistence", cluster.Name, "server", strconv.Itoa(i)}, "-")
}

func init() {
	SchemeBuilder.Register(&RabbitmqCluster{}, &RabbitmqClusterList{})
}
