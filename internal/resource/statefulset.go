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

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/metadata"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	defaultGracePeriodTimeoutSeconds int64  = 60 * 60 * 24 * 7
	initContainerCPU                 string = "100m"
	initContainerMemory              string = "500Mi"
	DeletionMarker                   string = "skipPreStopChecks"
)

func (builder *RabbitmqResourceBuilder) StatefulSet() *StatefulSetBuilder {
	return &StatefulSetBuilder{
		Instance: builder.Instance,
		Scheme:   builder.Scheme,
	}
}

type StatefulSetBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
	Scheme   *runtime.Scheme
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
			ServiceName: builder.Instance.ChildResourceName(headlessServiceName),
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
			}
			sts.Spec.VolumeClaimTemplates = pvcList
		}
	}

	return sts, nil
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

	return []corev1.PersistentVolumeClaim{pvc}, nil
}

func (builder *StatefulSetBuilder) Update(object runtime.Object) error {
	sts := object.(*appsv1.StatefulSet)

	//Replicas
	sts.Spec.Replicas = builder.Instance.Spec.Replicas

	//Update Strategy
	zero := int32(0)
	sts.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
		RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
			Partition: &zero,
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

	sts.Spec.Template = builder.podTemplateSpec(podAnnotations, updatedLabels, builder.Instance.Spec.TLS)

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
	return patchedPodSpec, nil
}

func (builder *StatefulSetBuilder) podTemplateSpec(annotations, labels map[string]string, tlsSpec rabbitmqv1beta1.TLSSpec) corev1.PodTemplateSpec {
	//Init Container resources
	cpuRequest := k8sresource.MustParse(initContainerCPU)
	memoryRequest := k8sresource.MustParse(initContainerMemory)

	//Image Pull Secret
	imagePullSecrets := []corev1.LocalObjectReference{}
	if builder.Instance.Spec.ImagePullSecret != "" {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: builder.Instance.Spec.ImagePullSecret})
	}

	automountServiceAccountToken := true
	rabbitmqGID := int64(999)
	rabbitmqUID := int64(999)

	terminationGracePeriod := defaultGracePeriodTimeoutSeconds

	volumes := []corev1.Volume{
		{
			Name: "rabbitmq-admin",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: builder.Instance.ChildResourceName(AdminSecretName),
					Items: []corev1.KeyToPath{
						{
							Key:  "username",
							Path: "username",
						},
						{
							Key:  "password",
							Path: "password",
						},
					},
				},
			},
		},
		{
			Name: "server-conf",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: builder.Instance.ChildResourceName(serverConfigMapName),
					},
				},
			},
		},
		{
			Name: "rabbitmq-etc",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
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
			Name:          "http",
			ContainerPort: 15672,
		},
		{
			Name:          "prometheus",
			ContainerPort: 15692,
		},
	}

	rabbitmqContainerVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "rabbitmq-admin",
			MountPath: "/opt/rabbitmq-secret/",
		},
		{
			Name:      "persistence",
			MountPath: "/var/lib/rabbitmq/mnesia/",
		},
		{
			Name:      "rabbitmq-etc",
			MountPath: "/etc/rabbitmq/",
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

	if tlsSpec.SecretName != "" {
		// add tls port
		ports = append(ports, corev1.ContainerPort{
			Name:          "amqps",
			ContainerPort: 5671,
		})

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

			if !builder.Instance.SingleTLSSecret() {
				// add tls volume
				filePermissions := int32(400)
				secretEnforced := true
				tlsCaSecretName := tlsSpec.CaSecretName
				volumes = append(volumes, corev1.Volume{
					Name: "rabbitmq-mutual-tls",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  tlsCaSecretName,
							DefaultMode: &filePermissions,
							Optional:    &secretEnforced,
						},
					},
				})
				//Mount new volume to same path as TLS cert and key
				rabbitmqContainerVolumeMounts = append(rabbitmqContainerVolumeMounts, corev1.VolumeMount{
					Name:      "rabbitmq-mutual-tls",
					MountPath: fmt.Sprintf("/etc/rabbitmq-tls/%s", builder.Instance.Spec.TLS.CaCertName),
					SubPath:   builder.Instance.Spec.TLS.CaCertName,
					ReadOnly:  true,
				})

			} else {
				//Mount CaCert in TLS Secret
				rabbitmqContainerVolumeMounts = append(rabbitmqContainerVolumeMounts, corev1.VolumeMount{
					Name:      "rabbitmq-tls",
					MountPath: fmt.Sprintf("/etc/rabbitmq-tls/%s", builder.Instance.Spec.TLS.CaCertName),
					SubPath:   builder.Instance.Spec.TLS.CaCertName,
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
			SecurityContext: &corev1.PodSecurityContext{
				FSGroup:    &rabbitmqGID,
				RunAsGroup: &rabbitmqGID,
				RunAsUser:  &rabbitmqUID,
			},
			ImagePullSecrets:              imagePullSecrets,
			TerminationGracePeriodSeconds: &terminationGracePeriod,
			ServiceAccountName:            builder.Instance.ChildResourceName(serviceAccountName),
			AutomountServiceAccountToken:  &automountServiceAccountToken,
			Affinity:                      builder.Instance.Spec.Affinity,
			Tolerations:                   builder.Instance.Spec.Tolerations,
			InitContainers: []corev1.Container{
				{
					Name:  "copy-config",
					Image: builder.Instance.Spec.Image,
					Command: []string{
						"sh", "-c", "cp /tmp/rabbitmq/rabbitmq.conf /etc/rabbitmq/rabbitmq.conf && echo '' >> /etc/rabbitmq/rabbitmq.conf ; " +
							"cp /tmp/rabbitmq/advanced.config /etc/rabbitmq/advanced.config ; " +
							"cp /tmp/rabbitmq/rabbitmq-env.conf /etc/rabbitmq/rabbitmq-env.conf ; " +
							"cp /tmp/erlang-cookie-secret/.erlang.cookie /var/lib/rabbitmq/.erlang.cookie " +
							"&& chown 999:999 /var/lib/rabbitmq/.erlang.cookie " +
							"&& chmod 600 /var/lib/rabbitmq/.erlang.cookie ; " +
							"cp /tmp/rabbitmq/enabled_plugins /etc/rabbitmq/enabled_plugins " +
							"&& chown 999:999 /etc/rabbitmq/enabled_plugins",
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
							Name:      "server-conf",
							MountPath: "/tmp/rabbitmq/",
						},
						{
							Name:      "rabbitmq-etc",
							MountPath: "/etc/rabbitmq/",
						},
						{
							Name:      "rabbitmq-erlang-cookie",
							MountPath: "/var/lib/rabbitmq/",
						},
						{
							Name:      "erlang-cookie-secret",
							MountPath: "/tmp/erlang-cookie-secret/",
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
							Name:  "RABBITMQ_DEFAULT_PASS_FILE",
							Value: "/opt/rabbitmq-secret/password",
						},
						{
							Name:  "RABBITMQ_DEFAULT_USER_FILE",
							Value: "/opt/rabbitmq-secret/username",
						},
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
							Value: builder.Instance.ChildResourceName("headless"),
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
					ReadinessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							Exec: &corev1.ExecAction{
								Command: []string{"/bin/sh", "-c", "rabbitmq-diagnostics check_port_connectivity"},
							},
						},
						InitialDelaySeconds: 10,
						TimeoutSeconds:      5,
						PeriodSeconds:       30,
						SuccessThreshold:    1,
						FailureThreshold:    3,
					},
					Lifecycle: &corev1.Lifecycle{
						PreStop: &corev1.Handler{
							Exec: &corev1.ExecAction{
								Command: []string{
									"/bin/bash", "-c", fmt.Sprintf("if [ ! -z \"$(cat /etc/pod-info/%s)\" ]; then exit 0; fi;", DeletionMarker) +
										" while true; do rabbitmq-queues check_if_node_is_quorum_critical" +
										" 2>&1; if [ $(echo $?) -eq 69 ]; then sleep 2; continue; fi;" +
										" rabbitmq-queues check_if_node_is_mirror_sync_critical" +
										" 2>&1; if [ $(echo $?) -eq 69 ]; then sleep 2; continue; fi; break;" +
										" done",
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
