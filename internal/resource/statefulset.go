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
	initContainerCPU    string = "100m"
	initContainerMemory string = "500Mi"
	DeletionMarker      string = "skipPreStopChecks"
)

type StatefulSetBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
	Scheme   *runtime.Scheme
}

func (builder *RabbitmqResourceBuilder) StatefulSet() *StatefulSetBuilder {
	return &StatefulSetBuilder{
		Instance: builder.Instance,
		Scheme:   builder.Scheme,
	}
}

func (builder *StatefulSetBuilder) Build() (runtime.Object, error) {
	// PVC, ServiceName & Selector: can't be updated without deleting the statefulset
	pvc, err := persistentVolumeClaim(builder.Instance, builder.Scheme)
	if err != nil {
		return nil, err
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName("server"),
			Namespace: builder.Instance.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: builder.Instance.ChildResourceName(headlessServiceSuffix),
			Selector: &metav1.LabelSelector{
				MatchLabels: metadata.LabelSelector(builder.Instance.Name),
			},
			VolumeClaimTemplates: pvc,
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

		if len(overrideSts.Spec.VolumeClaimTemplates) != 0 {
			override := overrideSts.Spec.VolumeClaimTemplates
			pvcList := make([]corev1.PersistentVolumeClaim, len(override))
			for i := range override {
				copyObjectMeta(&pvcList[i].ObjectMeta, override[i].EmbeddedObjectMeta)
				pvcList[i].Spec = override[i].Spec
				if err := controllerutil.SetControllerReference(builder.Instance, &pvcList[i], builder.Scheme); err != nil {
					return nil, fmt.Errorf("failed setting controller reference: %v", err)
				}
				disableBlockOwnerDeletion(pvcList[i])
			}
			sts.Spec.VolumeClaimTemplates = pvcList
		}
	}

	return sts, nil
}

func (builder *StatefulSetBuilder) Update(object runtime.Object) error {
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
	defaultPodAnnotations := map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "15692",
	}
	podAnnotations := metadata.ReconcileAnnotations(defaultPodAnnotations, metadata.ReconcileAndFilterAnnotations(sts.Spec.Template.Annotations, builder.Instance.Annotations))

	//Labels
	updatedLabels := metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	sts.Labels = updatedLabels

	sts.Spec.Template = builder.podTemplateSpec(podAnnotations, updatedLabels)

	if !sts.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Equal(*sts.Spec.Template.Spec.Containers[0].Resources.Requests.Memory()) {
		logger := ctrl.Log.WithName("statefulset").WithName("RabbitmqCluster")
		logger.Info(fmt.Sprintf("Warning: Memory request and limit are not equal for \"%s\". It is recommended that they be set to the same value", sts.GetName()))
	}

	if builder.Instance.Spec.Override.StatefulSet != nil {
		if err := applyStsOverride(sts, builder.Instance.Spec.Override.StatefulSet); err != nil {
			return fmt.Errorf("failed applying StatefulSet override: %v", err)
		}
	}

	if err := controllerutil.SetControllerReference(builder.Instance, sts, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}
	return nil
}

func applyStsOverride(sts *appsv1.StatefulSet, stsOverride *rabbitmqv1beta1.StatefulSet) error {
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
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "persistence",
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
		return []corev1.PersistentVolumeClaim{}, fmt.Errorf("failed setting controller reference: %v", err)
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
		return corev1.PodSpec{}, fmt.Errorf("error marshalling statefulSet podSpec: %v", err)
	}

	patch, err := json.Marshal(podSpecOverride)
	if err != nil {
		return corev1.PodSpec{}, fmt.Errorf("error marshalling statefulSet podSpec override: %v", err)
	}

	patchedJSON, err := strategicpatch.StrategicMergePatch(originalPodSpec, patch, corev1.PodSpec{})
	if err != nil {
		return corev1.PodSpec{}, fmt.Errorf("error patching podSpec: %v", err)
	}

	patchedPodSpec := corev1.PodSpec{}
	err = json.Unmarshal(patchedJSON, &patchedPodSpec)
	if err != nil {
		return corev1.PodSpec{}, fmt.Errorf("error unmarshalling patched Stateful Set: %v", err)
	}

	// handle the rabbitmq container envVar list as a special case if it's overwritten
	// we need to ensure that MY_POD_NAME, MY_POD_NAMESPACE and K8S_SERVICE_NAME are defined first so other envVars values can reference them
	if rmqContainer := containerRabbitmq(podSpecOverride.Containers); rmqContainer.Env != nil {
		sortEnvVar(patchedPodSpec.Containers[0].Env)
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

func (builder *StatefulSetBuilder) podTemplateSpec(annotations, labels map[string]string) corev1.PodTemplateSpec {
	//Init Container resources
	cpuRequest := k8sresource.MustParse(initContainerCPU)
	memoryRequest := k8sresource.MustParse(initContainerMemory)

	automountServiceAccountToken := true
	rabbitmqGID := int64(999)
	rabbitmqUID := int64(999)

	volumes := []corev1.Volume{
		{
			Name: "server-conf",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: builder.Instance.ChildResourceName(ServerConfigMapName),
					},
				},
			},
		},
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
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: builder.Instance.ChildResourceName(DefaultUserSecretName),
								},
								Items: []corev1.KeyToPath{
									{
										Key:  "default_user.conf",
										Path: "default_user.conf",
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

	rabbitmqContainerVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "persistence",
			MountPath: "/var/lib/rabbitmq/mnesia/",
		},
		{
			Name:      "rabbitmq-plugins",
			MountPath: "/operator",
		},
		{
			Name:      "server-conf",
			MountPath: "/etc/rabbitmq/rabbitmq.conf",
			SubPath:   "rabbitmq.conf",
		},
		{
			Name:      "rabbitmq-confd",
			MountPath: "/etc/rabbitmq/conf.d/default_user.conf",
			SubPath:   "default_user.conf",
		},
		{
			Name:      "rabbitmq-erlang-cookie",
			MountPath: "/var/lib/rabbitmq/",
		},
		{
			Name:      "pod-info",
			MountPath: "/etc/pod-info/",
		},
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
	if builder.Instance.TLSEnabled() {
		// add tls port
		ports = append(ports, corev1.ContainerPort{
			Name:          "amqps",
			ContainerPort: 5671,
		},
			corev1.ContainerPort{
				Name:          "https",
				ContainerPort: 15671,
			},
		)

		// add tls volume
		filePermissions := int32(400)
		secretEnforced := true
		tlsSecretName := tlsSpec.SecretName
		volumes = append(volumes, corev1.Volume{
			Name: "rabbitmq-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  tlsSecretName,
					DefaultMode: &filePermissions,
					Optional:    &secretEnforced,
				},
			},
		})

		// add volume mount
		rabbitmqContainerVolumeMounts = append(rabbitmqContainerVolumeMounts, corev1.VolumeMount{
			Name:      "rabbitmq-tls",
			MountPath: "/etc/rabbitmq-tls/tls.crt",
			SubPath:   "tls.crt",
			ReadOnly:  true,
		})
		rabbitmqContainerVolumeMounts = append(rabbitmqContainerVolumeMounts, corev1.VolumeMount{
			Name:      "rabbitmq-tls",
			MountPath: "/etc/rabbitmq-tls/tls.key",
			SubPath:   "tls.key",
			ReadOnly:  true,
		})

		if builder.Instance.MutualTLSEnabled() {
			caCertName := builder.Instance.Spec.TLS.CaCertName

			if builder.Instance.SingleTLSSecret() {
				//Mount CaCert in TLS Secret
				rabbitmqContainerVolumeMounts = append(rabbitmqContainerVolumeMounts, corev1.VolumeMount{
					Name:      "rabbitmq-tls",
					MountPath: fmt.Sprintf("/etc/rabbitmq-tls/%s", caCertName),
					SubPath:   caCertName,
					ReadOnly:  true,
				})
			} else {
				// add tls volume
				filePermissions := int32(400)
				secretEnforced := true
				volumes = append(volumes, corev1.Volume{
					Name: "rabbitmq-mutual-tls",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  tlsSpec.CaSecretName,
							DefaultMode: &filePermissions,
							Optional:    &secretEnforced,
						},
					},
				})
				//Mount new volume to same path as TLS cert and key
				rabbitmqContainerVolumeMounts = append(rabbitmqContainerVolumeMounts, corev1.VolumeMount{
					Name:      "rabbitmq-mutual-tls",
					MountPath: fmt.Sprintf("/etc/rabbitmq-tls/%s", caCertName),
					SubPath:   caCertName,
					ReadOnly:  true,
				})
			}
		}
	}

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: annotations,
			Labels:      labels,
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
				FSGroup:    &rabbitmqGID,
				RunAsGroup: &rabbitmqGID,
				RunAsUser:  &rabbitmqUID,
			},
			ImagePullSecrets:              builder.Instance.Spec.ImagePullSecrets,
			TerminationGracePeriodSeconds: builder.Instance.Spec.TerminationGracePeriodSeconds,
			ServiceAccountName:            builder.Instance.ChildResourceName(serviceAccountName),
			AutomountServiceAccountToken:  &automountServiceAccountToken,
			Affinity:                      builder.Instance.Spec.Affinity,
			Tolerations:                   builder.Instance.Spec.Tolerations,
			InitContainers: []corev1.Container{
				{
					Name:  "setup-container",
					Image: builder.Instance.Spec.Image,
					SecurityContext: &corev1.SecurityContext{
						RunAsUser: pointer.Int64Ptr(0),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
							Add:  []corev1.Capability{"CHOWN", "FOWNER"},
						},
					},
					Command: []string{
						"sh", "-c", "cp /tmp/erlang-cookie-secret/.erlang.cookie /var/lib/rabbitmq/.erlang.cookie " +
							"&& chown 999:999 /var/lib/rabbitmq/.erlang.cookie " +
							"&& chmod 600 /var/lib/rabbitmq/.erlang.cookie ; " +
							"cp /tmp/rabbitmq-plugins/enabled_plugins /operator/enabled_plugins " +
							"&& chown 999:999 /operator/enabled_plugins ; " +
							"chgrp 999 /var/lib/rabbitmq/mnesia/ ; " +
							"echo '[default]' > /var/lib/rabbitmq/.rabbitmqadmin.conf " +
							"&& sed -e 's/default_user/username/' -e 's/default_pass/password/' /tmp/default_user.conf >> /var/lib/rabbitmq/.rabbitmqadmin.conf " +
							"&& chown 999:999 /var/lib/rabbitmq/.rabbitmqadmin.conf " +
							"&& chmod 600 /var/lib/rabbitmq/.rabbitmqadmin.conf",
					},
					Resources: corev1.ResourceRequirements{
						Limits: map[corev1.ResourceName]k8sresource.Quantity{
							"cpu":    cpuRequest,
							"memory": memoryRequest,
						},
						Requests: map[corev1.ResourceName]k8sresource.Quantity{
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
						{
							Name:      "rabbitmq-confd",
							MountPath: "/tmp/default_user.conf",
							SubPath:   "default_user.conf",
						},
					},
				},
			},
			Volumes: volumes,
			Containers: []corev1.Container{
				{
					Name:      "rabbitmq",
					Resources: *builder.Instance.Spec.Resources,
					Image:     builder.Instance.Spec.Image,
					Env: []corev1.EnvVar{
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
							Name:  "RABBITMQ_ENABLED_PLUGINS_FILE",
							Value: "/operator/enabled_plugins",
						},
						{
							Name:  "K8S_SERVICE_NAME",
							Value: builder.Instance.ChildResourceName(headlessServiceSuffix),
						},
						{
							Name:  "RABBITMQ_USE_LONGNAME",
							Value: "true",
						},
						{
							Name:  "RABBITMQ_NODENAME",
							Value: "rabbit@$(MY_POD_NAME).$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE)",
						},
						{
							Name:  "K8S_HOSTNAME_SUFFIX",
							Value: ".$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE)",
						},
					},
					Ports:        ports,
					VolumeMounts: rabbitmqContainerVolumeMounts,
					// Why using a tcp readiness probe instead of running `rabbitmq-diagnostics check_port_connectivity`?
					// Using rabbitmq-diagnostics command as the probe could cause context deadline exceeded errors
					// Pods could be stuck at terminating at deletion as a result of that
					// More details see issue: https://github.com/rabbitmq/cluster-operator/issues/409
					ReadinessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.IntOrString{
									Type:   intstr.String,
									StrVal: "amqp",
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
						PreStop: &corev1.Handler{
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
}

func copyLabelsAnnotations(base *metav1.ObjectMeta, override rabbitmqv1beta1.EmbeddedLabelsAnnotations) {
	if override.Labels != nil {
		base.Labels = mergeMap(base.Labels, override.Labels)
	}

	if override.Annotations != nil {
		base.Annotations = mergeMap(base.Annotations, override.Annotations)
	}
}

func copyObjectMeta(base *metav1.ObjectMeta, override rabbitmqv1beta1.EmbeddedObjectMeta) {
	if override.Name != "" {
		base.Name = override.Name
	}

	if override.Namespace != "" {
		base.Namespace = override.Namespace
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
