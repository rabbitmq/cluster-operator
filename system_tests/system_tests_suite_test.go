package system_tests

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSystemTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SystemTests Suite")
}

var (
	rmqClusterClient          client.Client
	clientSet                 *kubernetes.Clientset
	namespace                 string
	mgr                       manager.Manager
	specifiedStorageClassName = "persistent-test"
	specifiedStorageCapacity  = "1Gi"
)

var _ = SynchronizedBeforeSuite(func() []byte {
	scheme := runtime.NewScheme()
	Expect(rabbitmqv1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(defaultscheme.AddToScheme(scheme)).NotTo(HaveOccurred())

	restConfig, err := createRestConfig()
	Expect(err).NotTo(HaveOccurred())

	rmqClusterClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	clientSet, err = createClientSet()
	Expect(err).NotTo(HaveOccurred())

	namespace = MustHaveEnv("NAMESPACE")

	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: specifiedStorageClassName,
		},
		Provisioner: "kubernetes.io/gce-pd",
	}
	err = rmqClusterClient.Create(context.TODO(), storageClass)
	if !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}

	// Patch/update configMap
	operatorConfigMapName := "p-rmq-operator-config"
	configMap, err := clientSet.CoreV1().ConfigMaps(namespace).Get(operatorConfigMapName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	operatorConMap, err := config.NewConfig([]byte(configMap.Data["config"]))
	Expect(err).NotTo(HaveOccurred())

	operatorConMap.Persistence.StorageClassName = specifiedStorageClassName
	operatorConMap.Persistence.Storage = specifiedStorageCapacity
	configBytes, err := toYamlBytes(operatorConMap)
	Expect(err).NotTo(HaveOccurred())
	configMap.Data["config"] = string(configBytes)

	_, err = clientSet.CoreV1().ConfigMaps(namespace).Update(configMap)
	Expect(err).NotTo(HaveOccurred())

	// Delete Operator pod
	var operatorPod *corev1.Pod
	pods, err := clientSet.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	for _, pod := range pods.Items {
		if strings.Contains(pod.Name, "operator") {
			operatorPod = &pod
		}
	}
	if operatorPod == nil {
		Fail("Operator pod cannot be found")
	}

	Expect(clientSet.CoreV1().Pods(namespace).Delete(operatorPod.Name, &metav1.DeleteOptions{})).To(Succeed())

	Eventually(func() []byte {
		output, err := kubectl(
			"-n",
			namespace,
			"get",
			"deployment",
			"-l",
			"control-plane=controller-manager",
		)

		Expect(err).NotTo(HaveOccurred())

		return output
	}, 10, 1).Should(ContainSubstring("1/1"))

	return nil
}, func(data []byte) {
	scheme := runtime.NewScheme()
	Expect(rabbitmqv1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(defaultscheme.AddToScheme(scheme)).NotTo(HaveOccurred())

	restConfig, err := createRestConfig()
	Expect(err).NotTo(HaveOccurred())

	rmqClusterClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

})
