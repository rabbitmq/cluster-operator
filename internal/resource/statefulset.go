package resource

import (
	"fmt"

	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	rabbitmqManagementImage    string = "rabbitmq:3.8-rc-management"
	defaultPersistenceCapacity string = "10Gi"
)

func GenerateStatefulSet(instance rabbitmqv1beta1.RabbitmqCluster, imageRepository, imagePullSecret, persistenceStorageClassName, persistenceStorage string, scheme *runtime.Scheme) (*appsv1.StatefulSet, error) {
	t := true
	image := rabbitmqManagementImage
	rabbitmqGID := int64(999)

	replicas := int32(instance.Spec.Replicas)
	if replicas == 0 {
		replicas = int32(1)
	}

	if instance.Spec.Image.Repository != "" {
		image = fmt.Sprintf("%s/%s", instance.Spec.Image.Repository, image)
	} else if imageRepository != "" {
		image = fmt.Sprintf("%s/%s", imageRepository, image)
	}

	imagePullSecrets := []corev1.LocalObjectReference{}
	if instance.Spec.ImagePullSecret != "" {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: instance.Spec.ImagePullSecret})
	} else if imagePullSecret != "" {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: imagePullSecret})
	}

	pvc, err := generatePersistentVolumeClaim(instance, persistenceStorageClassName, persistenceStorage, scheme)
	if err != nil {
		return nil, err
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.ChildResourceName("server"),
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"app":             "pivotal-rabbitmq",
				"RabbitmqCluster": instance.Name,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: instance.ChildResourceName(headlessServiceName),
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": instance.Name},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				*pvc,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": instance.Name}},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: &rabbitmqGID,
					},
					ServiceAccountName:           instance.ChildResourceName(serviceAccountName),
					AutomountServiceAccountToken: &t,
					ImagePullSecrets:             imagePullSecrets,
					InitContainers:               generateInitContainers(image),
					Containers: []corev1.Container{
						{
							Name:  "rabbitmq",
							Image: image,
							Env: []corev1.EnvVar{
								{
									Name:  "RABBITMQ_ENABLED_PLUGINS_FILE",
									Value: "/opt/server-conf/enabled_plugins",
								},
								{
									Name:  "RABBITMQ_DEFAULT_PASS_FILE",
									Value: "/opt/rabbitmq-secret/rabbitmq-password",
								},
								{
									Name:  "RABBITMQ_DEFAULT_USER_FILE",
									Value: "/opt/rabbitmq-secret/rabbitmq-username",
								},
								{
									Name:  "RABBITMQ_MNESIA_BASE",
									Value: "/var/lib/rabbitmq/db",
								},

								{
									Name:  "RABBITMQ_ERLANG_COOKIE",
									Value: "NEEDTOCHANGETHIS",
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
									Value: instance.ChildResourceName("headless"),
								},
								{
									Name:  "RABBITMQ_USE_LONGNAME",
									Value: "true",
								},
								{
									Name:  "RABBITMQ_NODENAME",
									Value: "rabbit@$(MY_POD_NAME).$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE).svc.cluster.local",
								},
								{
									Name:  "K8S_HOSTNAME_SUFFIX",
									Value: ".$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE).svc.cluster.local",
								},
							},
							Ports: []corev1.ContainerPort{
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
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "server-conf",
									MountPath: "/opt/server-conf/",
								},
								{
									Name:      "rabbitmq-admin",
									MountPath: "/opt/rabbitmq-secret/",
								},
								{
									Name:      "persistence",
									MountPath: "/var/lib/rabbitmq/db/",
								},
								{
									Name:      "rabbitmq-etc",
									MountPath: "/etc/rabbitmq/",
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"/bin/sh", "-c", "rabbitmq-diagnostics check_running && rabbitmq-diagnostics check_port_connectivity"},
									},
								},
								InitialDelaySeconds: 10,
								TimeoutSeconds:      5,
								PeriodSeconds:       30,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "rabbitmq-admin",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: instance.ChildResourceName(adminSecretName),
									Items: []corev1.KeyToPath{
										{
											Key:  "rabbitmq-username",
											Path: "rabbitmq-username",
										},
										{
											Key:  "rabbitmq-password",
											Path: "rabbitmq-password",
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
										Name: instance.ChildResourceName(serverConfigMapName),
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
					},
				},
			},
		},
	}, nil
}

func generatePersistentVolumeClaim(instance rabbitmqv1beta1.RabbitmqCluster, persistenceStorageClassName, persistenceStorage string, scheme *runtime.Scheme) (*corev1.PersistentVolumeClaim, error) {
	var err error
	q, err := k8sresource.ParseQuantity(defaultPersistenceCapacity)
	if err != nil {
		return nil, err
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "persistence",
			Labels: map[string]string{
				"app": instance.Name,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: q,
				},
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		},
	}

	if instance.Spec.Persistence.Storage != "" {
		pvc.Spec.Resources.Requests["storage"], err = k8sresource.ParseQuantity(instance.Spec.Persistence.Storage)
		if err != nil {
			return nil, err
		}
	} else if persistenceStorage != "" {
		pvc.Spec.Resources.Requests["storage"], err = k8sresource.ParseQuantity(persistenceStorage)
		if err != nil {
			return nil, err
		}
	}

	if instance.Spec.Persistence.StorageClassName != "" {
		pvc.Spec.StorageClassName = &instance.Spec.Persistence.StorageClassName
	} else if persistenceStorageClassName != "" {
		pvc.Spec.StorageClassName = &persistenceStorageClassName
	}

	if err := controllerutil.SetControllerReference(&instance, pvc, scheme); err != nil {
		return nil, fmt.Errorf("failed setting controller reference: %v", err)
	}

	return pvc, nil
}

func generateInitContainers(image string) []corev1.Container {
	return []corev1.Container{
		{
			Name: "copy-config",
			Command: []string{
				"sh", "-c", "cp /tmp/rabbitmq/rabbitmq.conf /etc/rabbitmq/rabbitmq.conf && echo '' >> /etc/rabbitmq/rabbitmq.conf",
			},
			Image: image,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "server-conf",
					MountPath: "/tmp/rabbitmq/",
				},
				{
					Name:      "rabbitmq-etc",
					MountPath: "/etc/rabbitmq/",
				},
			},
		},
	}
}
