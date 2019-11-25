package resource

import (
	"fmt"

	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	rabbitmqImage             string = "rabbitmq:3.8.1"
	defaultPersistentCapacity string = "10Gi"
	defaultMemoryLimit        string = "2Gi"
	defaultCPULimit           string = "500m"
	defaultMemoryRequest      string = "2Gi"
	defaultCPURequest         string = "100m"
	initContainerCPU          string = "100m"
	initContainerMemory       string = "500Mi"
)

type ComputeResourceQuantities struct {
	CPU    k8sresource.Quantity
	Memory k8sresource.Quantity
}

type ResourceRequirementQuantities struct {
	Limit   ComputeResourceQuantities
	Request ComputeResourceQuantities
}

type ComputeResource struct {
	CPU    string
	Memory string
}

type ResourceRequirements struct {
	Limit   ComputeResource
	Request ComputeResource
}

type StatefulSetConfiguration struct {
	ImageReference             string
	ImagePullSecret            string
	PersistentStorageClassName *string
	PersistentStorage          k8sresource.Quantity
	ResourceRequirementsConfig ResourceRequirementQuantities
	Scheme                     *runtime.Scheme
}

func (cluster *RabbitmqResourceBuilder) StatefulSet() (*appsv1.StatefulSet, error) {
	automountServiceAccountToken := true
	rabbitmqGID := int64(999)
	rabbitmqUID := int64(999)

	replicas := int32(cluster.Instance.Spec.Replicas)
	if replicas == 0 {
		replicas = int32(1)
	}

	statefulSetConfiguration, err := cluster.statefulSetConfigurations()
	if err != nil {
		return nil, err
	}

	var imagePullSecrets []corev1.LocalObjectReference
	if statefulSetConfiguration.ImagePullSecret != "" {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: statefulSetConfiguration.ImagePullSecret})
	}

	pvc, err := persistentVolumeClaim(cluster.Instance, statefulSetConfiguration)
	if err != nil {
		return nil, err
	}

	cpuRequest, err := k8sresource.ParseQuantity(initContainerCPU)
	if err != nil {
		return nil, err
	}
	memoryRequest, err := k8sresource.ParseQuantity(initContainerMemory)
	if err != nil {
		return nil, err
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Instance.ChildResourceName("server"),
			Namespace: cluster.Instance.Namespace,
			Labels:    metadata.Label(cluster.Instance.Name),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: cluster.Instance.ChildResourceName(headlessServiceName),
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: metadata.LabelSelector(cluster.Instance.Name),
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				*pvc,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: metadata.Label(cluster.Instance.Name),
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup:    &rabbitmqGID,
						RunAsGroup: &rabbitmqGID,
						RunAsUser:  &rabbitmqUID,
					},
					ServiceAccountName:           cluster.Instance.ChildResourceName(serviceAccountName),
					AutomountServiceAccountToken: &automountServiceAccountToken,
					ImagePullSecrets:             imagePullSecrets,
					InitContainers: []corev1.Container{
						{
							Name: "copy-config",
							Command: []string{
								"sh", "-c", "cp /tmp/rabbitmq/rabbitmq.conf /etc/rabbitmq/rabbitmq.conf && echo '' >> /etc/rabbitmq/rabbitmq.conf ; " +
									"cp /tmp/erlang-cookie-secret/.erlang.cookie /var/lib/rabbitmq/.erlang.cookie " +
									"&& chown 999:999 /var/lib/rabbitmq/.erlang.cookie " +
									"&& chmod 600 /var/lib/rabbitmq/.erlang.cookie",
							},
							Image: statefulSetConfiguration.ImageReference,
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
					Containers: []corev1.Container{
						{
							Name:  "rabbitmq",
							Image: statefulSetConfiguration.ImageReference,
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    statefulSetConfiguration.ResourceRequirementsConfig.Limit.CPU,
									corev1.ResourceMemory: statefulSetConfiguration.ResourceRequirementsConfig.Limit.Memory,
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    statefulSetConfiguration.ResourceRequirementsConfig.Request.CPU,
									corev1.ResourceMemory: statefulSetConfiguration.ResourceRequirementsConfig.Request.Memory,
								},
							},
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
									Value: cluster.Instance.ChildResourceName("headless"),
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
								{
									Name:      "rabbitmq-erlang-cookie",
									MountPath: "/var/lib/rabbitmq/",
								},
							},
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
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "rabbitmq-admin",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: cluster.Instance.ChildResourceName(adminSecretName),
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
										Name: cluster.Instance.ChildResourceName(serverConfigMapName),
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
									SecretName: cluster.Instance.ChildResourceName(erlangCookieName),
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

func persistentVolumeClaim(instance *rabbitmqv1beta1.RabbitmqCluster, statefulSetConfigureation StatefulSetConfiguration) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "persistence",
			Labels: metadata.Label(instance.Name),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: statefulSetConfigureation.PersistentStorage,
				},
			},
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: statefulSetConfigureation.PersistentStorageClassName,
		},
	}

	if err := controllerutil.SetControllerReference(instance, pvc, statefulSetConfigureation.Scheme); err != nil {
		return nil, fmt.Errorf("failed setting controller reference: %v", err)
	}

	return pvc, nil
}

func (cluster *RabbitmqResourceBuilder) statefulSetConfigurations() (StatefulSetConfiguration, error) {
	var err error
	statefulSetConfiguration := StatefulSetConfiguration{
		ImageReference:  rabbitmqImage,
		ImagePullSecret: "",
		Scheme:          cluster.DefaultConfiguration.Scheme,
	}

	if cluster.Instance.Spec.Image != "" {
		statefulSetConfiguration.ImageReference = cluster.Instance.Spec.Image
	} else if cluster.DefaultConfiguration.ImageReference != "" {
		statefulSetConfiguration.ImageReference = cluster.DefaultConfiguration.ImageReference
	}

	if cluster.Instance.Spec.ImagePullSecret != "" {
		statefulSetConfiguration.ImagePullSecret = cluster.Instance.Spec.ImagePullSecret
	} else if cluster.DefaultConfiguration.ImagePullSecret != "" {
		statefulSetConfiguration.ImagePullSecret = RegistrySecretName(cluster.Instance.Name)
	}

	cpuLimit := defaultCPULimit
	if cluster.DefaultConfiguration.ResourceRequirements.Limit.CPU != "" {
		cpuLimit = cluster.DefaultConfiguration.ResourceRequirements.Limit.CPU
	}
	statefulSetConfiguration.ResourceRequirementsConfig.Limit.CPU, err = k8sresource.ParseQuantity(cpuLimit)
	if err != nil {
		return statefulSetConfiguration, err
	}

	cpuRequest := defaultCPURequest
	if cluster.DefaultConfiguration.ResourceRequirements.Request.CPU != "" {
		cpuRequest = cluster.DefaultConfiguration.ResourceRequirements.Request.CPU
	}
	statefulSetConfiguration.ResourceRequirementsConfig.Request.CPU, err = k8sresource.ParseQuantity(cpuRequest)
	if err != nil {
		return statefulSetConfiguration, err
	}

	memoryLimit := defaultMemoryLimit
	if cluster.DefaultConfiguration.ResourceRequirements.Limit.Memory != "" {
		memoryLimit = cluster.DefaultConfiguration.ResourceRequirements.Limit.Memory
	}
	statefulSetConfiguration.ResourceRequirementsConfig.Limit.Memory, err = k8sresource.ParseQuantity(memoryLimit)
	if err != nil {
		return statefulSetConfiguration, err
	}

	memoryRequest := defaultMemoryRequest
	if cluster.DefaultConfiguration.ResourceRequirements.Request.Memory != "" {
		memoryRequest = cluster.DefaultConfiguration.ResourceRequirements.Request.Memory
	}
	statefulSetConfiguration.ResourceRequirementsConfig.Request.Memory, err = k8sresource.ParseQuantity(memoryRequest)
	if err != nil {
		return statefulSetConfiguration, err
	}

	statefulSetConfiguration.PersistentStorage, err = k8sresource.ParseQuantity(defaultPersistentCapacity)
	if err != nil {
		return statefulSetConfiguration, err
	}

	if cluster.Instance.Spec.Persistence.Storage != "" {
		statefulSetConfiguration.PersistentStorage, err = k8sresource.ParseQuantity(cluster.Instance.Spec.Persistence.Storage)
		if err != nil {
			return statefulSetConfiguration, err
		}
	} else if cluster.DefaultConfiguration.PersistentStorage != "" {
		statefulSetConfiguration.PersistentStorage, err = k8sresource.ParseQuantity(cluster.DefaultConfiguration.PersistentStorage)
		if err != nil {
			return statefulSetConfiguration, err
		}
	}

	statefulSetConfiguration.PersistentStorageClassName = nil
	if cluster.Instance.Spec.Persistence.StorageClassName != "" {
		statefulSetConfiguration.PersistentStorageClassName = &cluster.Instance.Spec.Persistence.StorageClassName
	} else if cluster.DefaultConfiguration.PersistentStorageClassName != "" {
		statefulSetConfiguration.PersistentStorageClassName = &cluster.DefaultConfiguration.PersistentStorageClassName
	}

	return statefulSetConfiguration, nil
}
