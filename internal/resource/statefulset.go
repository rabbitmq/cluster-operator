// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package resource

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/util/intstr"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/metadata"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	stsSuffix           string = "server"
	initContainerCPU    string = "100m"
	initContainerMemory string = "500Mi"
	defaultPVCName      string = "persistence"
	DeletionMarker      string = "skipPreStopChecks"
)

type StatefulSetBuilder struct {
	*RabbitmqResourceBuilder
}

func (builder *RabbitmqResourceBuilder) StatefulSet() *StatefulSetBuilder {
	return &StatefulSetBuilder{builder}
}

func (builder *StatefulSetBuilder) Build() (client.Object, error) {
	// PVC, ServiceName & Selector: can't be updated without deleting the statefulset
	pvc, err := persistentVolumeClaim(builder.Instance, builder.Scheme)
	if err != nil {
		return nil, err
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(stsSuffix),
			Namespace: builder.Instance.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: builder.Instance.ChildResourceName(headlessServiceSuffix),
			Selector: &metav1.LabelSelector{
				MatchLabels: metadata.LabelSelector(builder.Instance.Name),
			},
			VolumeClaimTemplates: pvc,
			PodManagementPolicy:  appsv1.ParallelPodManagement,
		},
	}

	// StatefulSet Override
	// override is applied to PVC, ServiceName & Selector
	// other fields are handled in Update()
	overrideSts := builder.Instance.Spec.Override.StatefulSet
	if overrideSts != nil && overrideSts.Spec != nil {
		if overrideSts.Spec.Selector != nil {
			sts.Spec.Selector = overrideSts.Spec.Selector
		}

		if overrideSts.Spec.ServiceName != "" {
			sts.Spec.ServiceName = overrideSts.Spec.ServiceName
		}
	}

	return sts, nil
}

// updates to storage capacity will recreate sts
func (builder *StatefulSetBuilder) UpdateMayRequireStsRecreate() bool {
	return true
}

func (builder *StatefulSetBuilder) Update(object client.Object) error {
	sts := object.(*appsv1.StatefulSet)

	//Replicas
	sts.Spec.Replicas = builder.Instance.Spec.Replicas

	//Update Strategy
	sts.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
		RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
			Partition: pointer.Int32Ptr(0),
		},
		Type: appsv1.RollingUpdateStatefulSetStrategyType,
	}

	//Annotations
	sts.Annotations = metadata.ReconcileAndFilterAnnotations(sts.Annotations, builder.Instance.Annotations)

	//Labels
	sts.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)

	// PVC storage capacity
	updatePersistenceStorageCapacity(&sts.Spec.VolumeClaimTemplates, builder.Instance.Spec.Persistence.Storage)

	// pod template
	sts.Spec.Template = builder.podTemplateSpec(sts.Spec.Template.Annotations)

	if !sts.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Equal(*sts.Spec.Template.Spec.Containers[0].Resources.Requests.Memory()) {
		logger := ctrl.Log.WithName("statefulset").WithName("RabbitmqCluster")
		logger.Info(fmt.Sprintf("Warning: Memory request and limit are not equal for \"%s\". It is recommended that they be set to the same value", sts.GetName()))
	}

	if builder.Instance.Spec.Override.StatefulSet != nil {
		if err := applyStsOverride(builder.Instance, builder.Scheme, sts, builder.Instance.Spec.Override.StatefulSet); err != nil {
			return fmt.Errorf("failed applying StatefulSet override: %w", err)
		}
	}

	if err := controllerutil.SetControllerReference(builder.Instance, sts, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %w", err)
	}
	return nil
}

func updatePersistenceStorageCapacity(templates *[]corev1.PersistentVolumeClaim, capacity *k8sresource.Quantity) {
	for _, t := range *templates {
		if t.Name == defaultPVCName {
			t.Spec.Resources.Requests[corev1.ResourceStorage] = *capacity
		}
	}
}

func applyStsOverride(instance *rabbitmqv1beta1.RabbitmqCluster, scheme *runtime.Scheme, sts *appsv1.StatefulSet, stsOverride *rabbitmqv1beta1.StatefulSet) error {
	if stsOverride.EmbeddedLabelsAnnotations != nil {
		copyLabelsAnnotations(&sts.ObjectMeta, *stsOverride.EmbeddedLabelsAnnotations)
	}

	if stsOverride.Spec == nil {
		return nil
	}
	if stsOverride.Spec.Replicas != nil {
		sts.Spec.Replicas = stsOverride.Spec.Replicas
	}
	if stsOverride.Spec.UpdateStrategy != nil {
		sts.Spec.UpdateStrategy = *stsOverride.Spec.UpdateStrategy
	}
	if stsOverride.Spec.PodManagementPolicy != "" {
		sts.Spec.PodManagementPolicy = stsOverride.Spec.PodManagementPolicy
	}

	if len(stsOverride.Spec.VolumeClaimTemplates) != 0 {
		// If spec.persistence.storage == 0, ignore PVC overrides.
		// Main reason for that is that there is no default PVC in such case (emptyDir is used instead)
		// other PVCs could technically be still overridden/added but we would be entering a very confusing territory
		// where storage is set to 0 and yet there are PVCs with data
		if instance.Spec.Persistence.Storage.Cmp(k8sresource.MustParse("0Gi")) == 0 {
			logger := ctrl.Log.WithName("statefulset").WithName("RabbitmqCluster")
			logger.Info(fmt.Sprintf("Warning: persistentVolumeClaim overrides are ignored for cluster \"%s\", becasue spec.persistence.storage is set to zero.", sts.GetName()))
		} else {
			volumeClaimTemplatesOverride := stsOverride.Spec.VolumeClaimTemplates
			pvcOverride := make([]corev1.PersistentVolumeClaim, len(volumeClaimTemplatesOverride))
			for i := range volumeClaimTemplatesOverride {
				copyObjectMeta(&pvcOverride[i].ObjectMeta, volumeClaimTemplatesOverride[i].EmbeddedObjectMeta)
				pvcOverride[i].Namespace = sts.Namespace // PVC should always be in the same namespace as the Stateful Set
				pvcOverride[i].Spec = volumeClaimTemplatesOverride[i].Spec
				if err := controllerutil.SetControllerReference(instance, &pvcOverride[i], scheme); err != nil {
					return fmt.Errorf("failed setting controller reference: %w", err)
				}
				disableBlockOwnerDeletion(pvcOverride[i])
			}
			sts.Spec.VolumeClaimTemplates = pvcOverride
		}
	}

	if stsOverride.Spec.Template == nil {
		return nil
	}
	if stsOverride.Spec.Template.EmbeddedObjectMeta != nil {
		copyObjectMeta(&sts.Spec.Template.ObjectMeta, *stsOverride.Spec.Template.EmbeddedObjectMeta)
	}
	if stsOverride.Spec.Template.Spec != nil {
		patchedPodSpec, err := patchPodSpec(&sts.Spec.Template.Spec, stsOverride.Spec.Template.Spec)
		if err != nil {
			return err
		}
		sts.Spec.Template.Spec = patchedPodSpec
	}

	return nil
}

func persistentVolumeClaim(instance *rabbitmqv1beta1.RabbitmqCluster, scheme *runtime.Scheme) ([]corev1.PersistentVolumeClaim, error) {
	zero := k8sresource.MustParse("0Gi")
	if instance.Spec.Persistence.Storage.Cmp(zero) == 0 {
		return []corev1.PersistentVolumeClaim{}, nil
	}

	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        defaultPVCName,
			Namespace:   instance.GetNamespace(),
			Labels:      metadata.Label(instance.Name),
			Annotations: metadata.ReconcileAndFilterAnnotations(map[string]string{}, instance.Annotations),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: *instance.Spec.Persistence.Storage,
				},
			},
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: instance.Spec.Persistence.StorageClassName,
		},
	}

	if err := controllerutil.SetControllerReference(instance, &pvc, scheme); err != nil {
		return []corev1.PersistentVolumeClaim{}, fmt.Errorf("failed setting controller reference: %w", err)
	}
	disableBlockOwnerDeletion(pvc)

	return []corev1.PersistentVolumeClaim{pvc}, nil
}

// required for OpenShift compatibility, see https://github.com/rabbitmq/cluster-operator/issues/234
func disableBlockOwnerDeletion(pvc corev1.PersistentVolumeClaim) {
	refs := pvc.OwnerReferences
	for i := range refs {
		refs[i].BlockOwnerDeletion = pointer.BoolPtr(false)
	}
}

func patchPodSpec(podSpec, podSpecOverride *corev1.PodSpec) (corev1.PodSpec, error) {
	originalPodSpec, err := json.Marshal(podSpec)
	if err != nil {
		return corev1.PodSpec{}, fmt.Errorf("error marshalling statefulSet podSpec: %w", err)
	}

	patch, err := json.Marshal(podSpecOverride)
	if err != nil {
		return corev1.PodSpec{}, fmt.Errorf("error marshalling statefulSet podSpec override: %w", err)
	}

	patchedJSON, err := strategicpatch.StrategicMergePatch(originalPodSpec, patch, corev1.PodSpec{})
	if err != nil {
		return corev1.PodSpec{}, fmt.Errorf("error patching podSpec: %w", err)
	}

	patchedPodSpec := corev1.PodSpec{}
	err = json.Unmarshal(patchedJSON, &patchedPodSpec)
	if err != nil {
		return corev1.PodSpec{}, fmt.Errorf("error unmarshalling patched Stateful Set: %w", err)
	}

	rmqContainer := containerRabbitmq(podSpecOverride.Containers)
	// handle the rabbitmq container envVar list as a special case if it's overwritten
	// we need to ensure that MY_POD_NAME, MY_POD_NAMESPACE and K8S_SERVICE_NAME are defined first so other envVars values can reference them
	if rmqContainer.Env != nil {
		sortEnvVar(patchedPodSpec.Containers[0].Env)
	}
	// handle the rabbitmq container volumeMounts list as a special case if it's overwritten
	// we need to ensure that '/var/lib/rabbitmq/' always mounts before '/var/lib/rabbitmq/mnesia/' to avoid shadowing
	if rmqContainer.VolumeMounts != nil {
		sortVolumeMounts(patchedPodSpec.Containers[0].VolumeMounts)
	}

	// A user may wish to override the controller-set securityContext for the RabbitMQ & init containers so that the
	// container runtime can override them. If the securityContext has been set to an empty struct, `strategicpatch.StrategicMergePatch`
	// won't pick this up, so manually override it here.
	if podSpecOverride.SecurityContext != nil && reflect.DeepEqual(*podSpecOverride.SecurityContext, corev1.PodSecurityContext{}) {
		patchedPodSpec.SecurityContext = nil
	}
	for i := range podSpecOverride.InitContainers {
		if podSpecOverride.InitContainers[i].SecurityContext != nil && reflect.DeepEqual(*podSpecOverride.InitContainers[i].SecurityContext, corev1.SecurityContext{}) {
			patchedPodSpec.InitContainers[i].SecurityContext = nil
		}
	}

	return patchedPodSpec, nil
}

// sortEnvVar ensures that 'MY_POD_NAME', 'MY_POD_NAMESPACE' and 'K8S_SERVICE_NAME' envVars are defined first in the list
// this is to enable other envVars to reference them as variables successfully
func sortEnvVar(envVar []corev1.EnvVar) {
	for i, e := range envVar {
		if e.Name == "MY_POD_NAME" {
			envVar[0], envVar[i] = envVar[i], envVar[0]
			continue
		}
		if e.Name == "MY_POD_NAMESPACE" {
			envVar[1], envVar[i] = envVar[i], envVar[1]
			continue
		}
		if e.Name == "K8S_SERVICE_NAME" {
			envVar[2], envVar[i] = envVar[i], envVar[2]
		}
	}
}

// sortVolumeMounts always returns '/var/lib/rabbitmq/' and '/var/lib/rabbitmq/mnesia/' first in the list.
// this is to ensure '/var/lib/rabbitmq/' always mounts before '/var/lib/rabbitmq/mnesia/' to avoid shadowing
// popular open-sourced container runtimes like docker and containerD will sort mounts in alphabetical order to
// avoid this issue, but there's no guarantee that all container runtime would do so
func sortVolumeMounts(mounts []corev1.VolumeMount) {
	for i, m := range mounts {
		if m.Name == "rabbitmq-erlang-cookie" {
			mounts[0], mounts[i] = mounts[i], mounts[0]
			continue
		}
		if m.Name == defaultPVCName {
			mounts[1], mounts[i] = mounts[i], mounts[1]
		}
	}
}

func (builder *StatefulSetBuilder) podTemplateSpec(previousPodAnnotations map[string]string) corev1.PodTemplateSpec {
	// default pod annotations
	defaultPodAnnotations := make(map[string]string, 0)

	if builder.Instance.VaultEnabled() {
		defaultPodAnnotations = appendVaultAnnotations(defaultPodAnnotations, builder.Instance)
	}

	readinessProbePort := "amqp"
	if builder.Instance.DisableNonTLSListeners() {
		readinessProbePort = "amqps"
	}

	volumes := []corev1.Volume{
		{
			Name: "plugins-conf",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: builder.Instance.ChildResourceName(PluginsConfigName),
					},
				},
			},
		},
		{
			Name: "rabbitmq-confd",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{

						{
							ConfigMap: &corev1.ConfigMapProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: builder.Instance.ChildResourceName(ServerConfigMapName),
								},
								Items: []corev1.KeyToPath{
									{
										Key:  "operatorDefaults.conf",
										Path: "operatorDefaults.conf",
									},
									{
										Key:  "userDefinedConfiguration.conf",
										Path: "userDefinedConfiguration.conf",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "rabbitmq-erlang-cookie",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "erlang-cookie-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: builder.Instance.ChildResourceName(erlangCookieName),
				},
			},
		},
		{
			Name: "rabbitmq-plugins",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "pod-info",
			VolumeSource: corev1.VolumeSource{
				DownwardAPI: &corev1.DownwardAPIVolumeSource{
					Items: []corev1.DownwardAPIVolumeFile{
						{
							Path: DeletionMarker,
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: fmt.Sprintf("metadata.labels['%s']", DeletionMarker),
							},
						},
					},
				},
			},
		},
	}

	if !builder.Instance.VaultDefaultUserSecretEnabled() {
		appendDefaultUserSecretVolumeProjection(volumes, builder.Instance)
	}

	if builder.Instance.Spec.Rabbitmq.AdvancedConfig != "" || builder.Instance.Spec.Rabbitmq.EnvConfig != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "server-conf",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: builder.Instance.ChildResourceName(ServerConfigMapName),
					}}}})
	}

	zero := k8sresource.MustParse("0Gi")
	if builder.Instance.Spec.Persistence.Storage.Cmp(zero) == 0 {
		volumes = append(volumes, corev1.Volume{
			Name: "persistence",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	rabbitmqContainerVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "rabbitmq-erlang-cookie",
			MountPath: "/var/lib/rabbitmq/",
		},
		{
			Name:      "persistence",
			MountPath: "/var/lib/rabbitmq/mnesia/",
		},
		{
			Name:      "rabbitmq-plugins",
			MountPath: "/operator",
		},
		{
			Name:      "rabbitmq-confd",
			MountPath: "/etc/rabbitmq/conf.d/10-operatorDefaults.conf",
			SubPath:   "operatorDefaults.conf",
		},
		{
			Name:      "rabbitmq-confd",
			MountPath: "/etc/rabbitmq/conf.d/90-userDefinedConfiguration.conf",
			SubPath:   "userDefinedConfiguration.conf",
		},
		{
			Name:      "pod-info",
			MountPath: "/etc/pod-info/",
		},
	}

	if !builder.Instance.VaultDefaultUserSecretEnabled() {
		rabbitmqContainerVolumeMounts = append(rabbitmqContainerVolumeMounts, corev1.VolumeMount{
			Name: "rabbitmq-confd", MountPath: "/etc/rabbitmq/conf.d/11-default_user.conf", SubPath: "default_user.conf",
		})
	}

	if builder.Instance.Spec.Rabbitmq.EnvConfig != "" {
		rabbitmqContainerVolumeMounts = append(rabbitmqContainerVolumeMounts, corev1.VolumeMount{
			Name: "server-conf", MountPath: "/etc/rabbitmq/rabbitmq-env.conf", SubPath: "rabbitmq-env.conf",
		})
	}

	if builder.Instance.Spec.Rabbitmq.AdvancedConfig != "" {
		rabbitmqContainerVolumeMounts = append(rabbitmqContainerVolumeMounts, corev1.VolumeMount{
			Name: "server-conf", MountPath: "/etc/rabbitmq/advanced.config", SubPath: "advanced.config",
		})
	}

	tlsSpec := builder.Instance.Spec.TLS
	if builder.Instance.SecretTLSEnabled() {
		rabbitmqContainerVolumeMounts = append(rabbitmqContainerVolumeMounts, corev1.VolumeMount{
			Name:      "rabbitmq-tls",
			MountPath: "/etc/rabbitmq-tls/",
			ReadOnly:  true,
		})

		secretEnforced := true
		filePermissions := pointer.Int32Ptr(400)
		tlsProjectedVolume := corev1.Volume{
			Name: "rabbitmq-tls",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: tlsSpec.SecretName,
								},
								Optional: &secretEnforced,
							},
						},
					},
					DefaultMode: filePermissions,
				},
			},
		}

		if builder.Instance.MutualTLSEnabled() && !builder.Instance.SingleTLSSecret() {
			caSecretProjection := corev1.VolumeProjection{
				Secret: &corev1.SecretProjection{
					LocalObjectReference: corev1.LocalObjectReference{Name: tlsSpec.CaSecretName},
					Optional:             &secretEnforced,
				},
			}
			tlsProjectedVolume.VolumeSource.Projected.Sources = append(tlsProjectedVolume.VolumeSource.Projected.Sources, caSecretProjection)
		}

		volumes = append(volumes, tlsProjectedVolume)
	}

	rabbitmqUID := int64(999)
	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: metadata.ReconcileAnnotations(previousPodAnnotations, defaultPodAnnotations),
			Labels:      metadata.Label(builder.Instance.Name),
		},
		Spec: corev1.PodSpec{
			TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
				{
					MaxSkew: 1,
					// "topology.kubernetes.io/zone" is a well-known label.
					// It is automatically set by kubelet if the cloud provider provides the zone information.
					// See: https://kubernetes.io/docs/reference/kubernetes-api/labels-annotations-taints/#topologykubernetesiozone
					TopologyKey:       "topology.kubernetes.io/zone",
					WhenUnsatisfiable: corev1.ScheduleAnyway,
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: metadata.LabelSelector(builder.Instance.Name),
					},
				},
			},
			SecurityContext: &corev1.PodSecurityContext{
				FSGroup:   pointer.Int64(0),
				RunAsUser: &rabbitmqUID,
			},
			ImagePullSecrets:              builder.Instance.Spec.ImagePullSecrets,
			TerminationGracePeriodSeconds: builder.Instance.Spec.TerminationGracePeriodSeconds,
			ServiceAccountName:            builder.Instance.ChildResourceName(serviceAccountName),
			AutomountServiceAccountToken:  pointer.Bool(true),
			Affinity:                      builder.Instance.Spec.Affinity,
			Tolerations:                   builder.Instance.Spec.Tolerations,
			InitContainers:                []corev1.Container{setupContainer(builder.Instance)},
			Volumes:                       volumes,
			Containers: []corev1.Container{
				{
					Name:      "rabbitmq",
					Resources: *builder.Instance.Spec.Resources,
					Image:     builder.Instance.Spec.Image,
					Env: append(envVarsK8sObjects(builder.Instance),
						corev1.EnvVar{
							Name:  "RABBITMQ_ENABLED_PLUGINS_FILE",
							Value: "/operator/enabled_plugins",
						},
						corev1.EnvVar{
							Name:  "RABBITMQ_USE_LONGNAME",
							Value: "true",
						},
						corev1.EnvVar{
							Name:  "RABBITMQ_NODENAME",
							Value: "rabbit@$(MY_POD_NAME).$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE)",
						},
						corev1.EnvVar{
							Name:  "K8S_HOSTNAME_SUFFIX",
							Value: ".$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE)",
						},
					),
					Ports:        builder.updateContainerPorts(),
					VolumeMounts: rabbitmqContainerVolumeMounts,
					// Why using a tcp readiness probe instead of running `rabbitmq-diagnostics check_port_connectivity`?
					// Using rabbitmq-diagnostics command as the probe could cause context deadline exceeded errors
					// Pods could be stuck at terminating at deletion as a result of that
					// More details see issue: https://github.com/rabbitmq/cluster-operator/issues/409
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.IntOrString{
									Type:   intstr.String,
									StrVal: readinessProbePort,
								},
							},
						},
						InitialDelaySeconds: 10,
						TimeoutSeconds:      5,
						PeriodSeconds:       10,
						SuccessThreshold:    1,
						FailureThreshold:    3,
					},
					Lifecycle: &corev1.Lifecycle{
						PreStop: &corev1.LifecycleHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"/bin/bash", "-c",
									fmt.Sprintf("if [ ! -z \"$(cat /etc/pod-info/%s)\" ]; then exit 0; fi;", DeletionMarker) +
										fmt.Sprintf(" rabbitmq-upgrade await_online_quorum_plus_one -t %d;"+
											" rabbitmq-upgrade await_online_synchronized_mirror -t %d;"+
											" rabbitmq-upgrade drain -t %d",
											*builder.Instance.Spec.TerminationGracePeriodSeconds,
											*builder.Instance.Spec.TerminationGracePeriodSeconds,
											*builder.Instance.Spec.TerminationGracePeriodSeconds),
								},
							},
						},
					},
				},
			},
		},
	}
	if builder.Instance.VaultDefaultUserSecretEnabled() &&
		builder.Instance.Spec.SecretBackend.Vault.DefaultUserUpdaterImage != nil &&
		*builder.Instance.Spec.SecretBackend.Vault.DefaultUserUpdaterImage != "" {
		podTemplateSpec.Spec.Containers = append(podTemplateSpec.Spec.Containers,
			defaultUserCredentialUpdater(builder.Instance))
	}
	return podTemplateSpec
}

func defaultUserCredentialUpdater(instance *rabbitmqv1beta1.RabbitmqCluster) corev1.Container {
	managementURI := "http://127.0.0.1:15672"
	if instance.TLSEnabled() {
		// RabbitMQ certificate SAN must include this host name.
		// (Alternatively, we could have put 127.0.0.1 in the management URI and put 127.0.0.1 into certificate IP SAN.
		// However, that approach seems to be less common.)
		managementURI = "https://$(HOSTNAME_DOMAIN):15671"
	}
	container := corev1.Container{
		Name: "default-user-credential-updater",
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"cpu":    k8sresource.MustParse("500m"),
				"memory": k8sresource.MustParse("128Mi"),
			},
			Requests: corev1.ResourceList{
				"cpu":    k8sresource.MustParse("10m"),
				"memory": k8sresource.MustParse("512Ki"),
			},
		},
		Image: *instance.Spec.SecretBackend.Vault.DefaultUserUpdaterImage,
		Args: []string{
			"--management-uri", managementURI,
			"-v", "4"},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "rabbitmq-erlang-cookie",
				MountPath: "/var/lib/rabbitmq/",
			},
			// VolumeMount /etc/rabbitmq/conf.d/11-default-user will be added by Vault injector.
		},
		Env: append(envVarsK8sObjects(instance),
			corev1.EnvVar{
				Name:  "HOSTNAME_DOMAIN",
				Value: "$(MY_POD_NAME).$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE)",
			}),
	}

	if instance.SecretTLSEnabled() {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "rabbitmq-tls",
			MountPath: "/etc/rabbitmq-tls/",
			ReadOnly:  true,
		})
	}
	// If instance.VaultTLSEnabled() volume mount /etc/rabbitmq-tls/ will be added by Vault injector.

	return container
}

func envVarsK8sObjects(instance *rabbitmqv1beta1.RabbitmqCluster) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "MY_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath:  "metadata.name",
					APIVersion: "v1",
				},
			},
		},
		{
			Name: "MY_POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath:  "metadata.namespace",
					APIVersion: "v1",
				},
			},
		},
		{
			Name:  "K8S_SERVICE_NAME",
			Value: instance.ChildResourceName(headlessServiceSuffix),
		},
	}
}

func setupContainer(instance *rabbitmqv1beta1.RabbitmqCluster) corev1.Container {
	//Init Container resources
	cpuRequest := k8sresource.MustParse(initContainerCPU)
	memoryRequest := k8sresource.MustParse(initContainerMemory)

	setupContainer := corev1.Container{
		Name:  "setup-container",
		Image: instance.Spec.Image,
		Command: []string{
			"sh", "-c",
			"cp /tmp/erlang-cookie-secret/.erlang.cookie /var/lib/rabbitmq/.erlang.cookie " +
				"&& chmod 600 /var/lib/rabbitmq/.erlang.cookie ; " +
				"cp /tmp/rabbitmq-plugins/enabled_plugins /operator/enabled_plugins ; " +
				"echo '[default]' > /var/lib/rabbitmq/.rabbitmqadmin.conf " +
				"&& sed -e 's/default_user/username/' -e 's/default_pass/password/' %s >> /var/lib/rabbitmq/.rabbitmqadmin.conf " +
				"&& chmod 600 /var/lib/rabbitmq/.rabbitmqadmin.conf",
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"cpu":    cpuRequest,
				"memory": memoryRequest,
			},
			Requests: corev1.ResourceList{
				"cpu":    cpuRequest,
				"memory": memoryRequest,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "plugins-conf",
				MountPath: "/tmp/rabbitmq-plugins/",
			},
			{
				Name:      "rabbitmq-erlang-cookie",
				MountPath: "/var/lib/rabbitmq/",
			},
			{
				Name:      "erlang-cookie-secret",
				MountPath: "/tmp/erlang-cookie-secret/",
			},
			{
				Name:      "rabbitmq-plugins",
				MountPath: "/operator",
			},
			{
				Name:      "persistence",
				MountPath: "/var/lib/rabbitmq/mnesia/",
			},
		},
	}

	if instance.VaultDefaultUserSecretEnabled() {
		// Vault annotation automatically mounts the volume
		setupContainer.Command[2] = fmt.Sprintf(setupContainer.Command[2], "/etc/rabbitmq/conf.d/11-default_user.conf")
	} else {
		setupContainer.Command[2] = fmt.Sprintf(setupContainer.Command[2], "/tmp/default_user.conf")
		setupContainer.VolumeMounts = append(setupContainer.VolumeMounts, corev1.VolumeMount{
			Name:      "rabbitmq-confd",
			MountPath: "/tmp/default_user.conf",
			SubPath:   "default_user.conf",
		})
	}
	return setupContainer
}

func appendDefaultUserSecretVolumeProjection(volumes []corev1.Volume, instance *rabbitmqv1beta1.RabbitmqCluster) {
	for _, value := range volumes {
		if value.Name == "rabbitmq-confd" {
			value.VolumeSource.Projected.Sources = append(value.VolumeSource.Projected.Sources,
				corev1.VolumeProjection{
					Secret: &corev1.SecretProjection{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: instance.ChildResourceName(DefaultUserSecretName),
						},
						Items: []corev1.KeyToPath{
							{
								Key:  "default_user.conf",
								Path: "default_user.conf",
							},
						},
					},
				},
			)
		}
	}
}

func appendVaultAnnotations(currentAnnotations map[string]string, instance *rabbitmqv1beta1.RabbitmqCluster) map[string]string {
	vault := instance.Spec.SecretBackend.Vault

	vaultAnnotations := map[string]string{
		"vault.hashicorp.com/agent-inject":     "true",
		"vault.hashicorp.com/agent-init-first": "true",
		"vault.hashicorp.com/role":             vault.Role,
	}

	if vault.DefaultUserSecretEnabled() {
		secretName := "11-default_user.conf"
		vaultAnnotations["vault.hashicorp.com/secret-volume-path-"+secretName] = "/etc/rabbitmq/conf.d"
		vaultAnnotations["vault.hashicorp.com/agent-inject-perms-"+secretName] = "0640"
		vaultAnnotations["vault.hashicorp.com/agent-inject-secret-"+secretName] = vault.DefaultUserPath
		vaultAnnotations["vault.hashicorp.com/agent-inject-template-"+secretName] = fmt.Sprintf(`
{{- with secret "%s" -}}
default_user = {{ .Data.data.username }}
default_pass = {{ .Data.data.password }}
{{- end }}`, vault.DefaultUserPath)
	}

	if vault.TLSEnabled() {
		pathCert := vault.TLS.PKIIssuerPath
		commonName := instance.ServiceSubDomain()
		if vault.TLS.CommonName != "" {
			commonName = vault.TLS.CommonName
		}

		altNames := podHostNames(instance)
		if vault.TLS.AltNames != "" {
			altNames = fmt.Sprintf("%s,%s", altNames, vault.TLS.AltNames)
		}

		certDir := strings.TrimSuffix(tlsCertDir, "/")
		vaultAnnotations["vault.hashicorp.com/secret-volume-path-"+tlsCertFilename] = certDir
		vaultAnnotations["vault.hashicorp.com/agent-inject-secret-"+tlsCertFilename] = pathCert
		vaultAnnotations["vault.hashicorp.com/agent-inject-template-"+tlsCertFilename] = generateVaultTLSTemplate(commonName, altNames, vault, "certificate")

		vaultAnnotations["vault.hashicorp.com/secret-volume-path-"+tlsKeyFilename] = certDir
		vaultAnnotations["vault.hashicorp.com/agent-inject-secret-"+tlsKeyFilename] = pathCert
		vaultAnnotations["vault.hashicorp.com/agent-inject-template-"+tlsKeyFilename] = generateVaultTLSTemplate(commonName, altNames, vault, "private_key")

		vaultAnnotations["vault.hashicorp.com/secret-volume-path-"+caCertFilename] = certDir
		vaultAnnotations["vault.hashicorp.com/agent-inject-secret-"+caCertFilename] = pathCert
		vaultAnnotations["vault.hashicorp.com/agent-inject-template-"+caCertFilename] = generateVaultTLSTemplate(commonName, altNames, vault, "issuing_ca")
	}

	return metadata.ReconcileAnnotations(currentAnnotations, vaultAnnotations, vault.Annotations)
}

func podHostNames(instance *rabbitmqv1beta1.RabbitmqCluster) string {
	altNames := ""
	var i int32
	for i = 0; i < pointer.Int32PtrDerefOr(instance.Spec.Replicas, 1); i++ {
		altNames += fmt.Sprintf(",%s", fmt.Sprintf("%s-%d.%s.%s", instance.ChildResourceName(stsSuffix), i, instance.ChildResourceName(headlessServiceSuffix), instance.Namespace))
	}
	return strings.TrimPrefix(altNames, ",")
}

func generateVaultTLSTemplate(commonName, altNames string, vault *rabbitmqv1beta1.VaultSpec, tlsAttribute string) string {
	return fmt.Sprintf(`
{{- with secret "%s" "common_name=%s" "alt_names=%s" "ip_sans=%s" -}}
{{ .Data.%s }}
{{- end }}`, vault.TLS.PKIIssuerPath, commonName, altNames, vault.TLS.IpSans, tlsAttribute)
}

func (builder *StatefulSetBuilder) updateContainerPorts() []corev1.ContainerPort {
	if builder.Instance.DisableNonTLSListeners() {
		return builder.updateContainerPortsOnlyTLSListeners()
	}

	ports := []corev1.ContainerPort{
		{
			Name:          "epmd",
			ContainerPort: 4369,
		},
		{
			Name:          "amqp",
			ContainerPort: 5672,
		},
		{
			Name:          "management",
			ContainerPort: 15672,
		},
		{
			Name:          "prometheus",
			ContainerPort: 15692,
		},
	}

	if builder.Instance.AdditionalPluginEnabled("rabbitmq_mqtt") {
		ports = append(ports, corev1.ContainerPort{
			Name:          "mqtt",
			ContainerPort: 1883,
		})
	}
	if builder.Instance.AdditionalPluginEnabled("rabbitmq_web_mqtt") {
		ports = append(ports, corev1.ContainerPort{
			Name:          "web-mqtt",
			ContainerPort: 15675,
		})
	}
	if builder.Instance.AdditionalPluginEnabled("rabbitmq_stomp") {
		ports = append(ports, corev1.ContainerPort{
			Name:          "stomp",
			ContainerPort: 61613,
		})
	}
	if builder.Instance.AdditionalPluginEnabled("rabbitmq_web_stomp") {
		ports = append(ports, corev1.ContainerPort{
			Name:          "web-stomp",
			ContainerPort: 15674,
		})
	}

	if builder.Instance.StreamNeeded() {
		ports = append(ports, corev1.ContainerPort{
			Name:          "stream",
			ContainerPort: 5552,
		})
	}

	if builder.Instance.TLSEnabled() {
		ports = append(ports, corev1.ContainerPort{
			Name:          "amqps",
			ContainerPort: 5671,
		},
			corev1.ContainerPort{
				Name:          "management-tls",
				ContainerPort: 15671,
			},
			corev1.ContainerPort{
				Name:          "prometheus-tls",
				ContainerPort: 15691,
			},
		)

		// enable tls ports for plugins
		if builder.Instance.AdditionalPluginEnabled("rabbitmq_mqtt") {
			ports = append(ports, corev1.ContainerPort{
				Name:          "mqtts",
				ContainerPort: 8883,
			})
		}

		if builder.Instance.AdditionalPluginEnabled("rabbitmq_stomp") {
			ports = append(ports, corev1.ContainerPort{
				Name:          "stomps",
				ContainerPort: 61614,
			})
		}

		if builder.Instance.StreamNeeded() {
			ports = append(ports, corev1.ContainerPort{
				Name:          "streams",
				ContainerPort: 5551,
			})
		}

		if builder.Instance.MutualTLSEnabled() {
			if builder.Instance.AdditionalPluginEnabled("rabbitmq_web_mqtt") {
				ports = append(ports, corev1.ContainerPort{
					Name:          "web-mqtt-tls",
					ContainerPort: 15676,
				})
			}

			if builder.Instance.AdditionalPluginEnabled("rabbitmq_web_stomp") {
				ports = append(ports, corev1.ContainerPort{
					Name:          "web-stomp-tls",
					ContainerPort: 15673,
				})
			}
		}
	}

	return ports
}

func (builder *StatefulSetBuilder) updateContainerPortsOnlyTLSListeners() []corev1.ContainerPort {
	ports := []corev1.ContainerPort{
		{
			Name:          "epmd",
			ContainerPort: 4369,
		},
		{
			Name:          "amqps",
			ContainerPort: 5671,
		},
		{
			Name:          "management-tls",
			ContainerPort: 15671,
		},
		{
			Name:          "prometheus-tls",
			ContainerPort: 15691,
		},
	}

	if builder.Instance.AdditionalPluginEnabled("rabbitmq_mqtt") {
		ports = append(ports, corev1.ContainerPort{
			Name:          "mqtts",
			ContainerPort: 8883,
		})
	}

	if builder.Instance.AdditionalPluginEnabled("rabbitmq_stomp") {
		ports = append(ports, corev1.ContainerPort{
			Name:          "stomps",
			ContainerPort: 61614,
		})
	}

	if builder.Instance.StreamNeeded() {
		ports = append(ports, corev1.ContainerPort{
			Name:          "streams",
			ContainerPort: 5551,
		})
	}

	if builder.Instance.MutualTLSEnabled() {
		if builder.Instance.AdditionalPluginEnabled("rabbitmq_web_mqtt") {
			ports = append(ports, corev1.ContainerPort{
				Name:          "web-mqtt-tls",
				ContainerPort: 15676,
			})
		}

		if builder.Instance.AdditionalPluginEnabled("rabbitmq_web_stomp") {
			ports = append(ports, corev1.ContainerPort{
				Name:          "web-stomp-tls",
				ContainerPort: 15673,
			})
		}
	}
	return ports
}

func copyLabelsAnnotations(base *metav1.ObjectMeta, override rabbitmqv1beta1.EmbeddedLabelsAnnotations) {
	if override.Labels != nil {
		base.Labels = mergeMap(base.Labels, override.Labels)
	}

	if override.Annotations != nil {
		base.Annotations = mergeMap(base.Annotations, override.Annotations)
	}
}

// copyObjectMeta copies name, labels, and annotations from a given EmbeddedObjectMeta to a metav1.ObjectMeta
// there is no need to copy the namespace because both PVCs and Pod have to be in the same namespace as its StatefulSet
func copyObjectMeta(base *metav1.ObjectMeta, override rabbitmqv1beta1.EmbeddedObjectMeta) {
	if override.Name != "" {
		base.Name = override.Name
	}

	if override.Labels != nil {
		base.Labels = mergeMap(base.Labels, override.Labels)
	}

	if override.Annotations != nil {
		base.Annotations = mergeMap(base.Annotations, override.Annotations)
	}
}

func mergeMap(base, override map[string]string) map[string]string {
	result := base
	if result == nil {
		result = make(map[string]string)
	}

	for k, v := range override {
		result[k] = v
	}

	return result
}

func containerRabbitmq(containers []corev1.Container) corev1.Container {
	for _, container := range containers {
		if container.Name == "rabbitmq" {
			return container
		}
	}
	return corev1.Container{}
}
