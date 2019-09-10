package system_tests

import (
	"context"
	"fmt"
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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSystemTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SystemTests Suite")
}

var (
	k8sClient                 client.Client
	clientSet                 *kubernetes.Clientset
	namespace                 string
	mgr                       manager.Manager
	specifiedStorageClassName string
	specifiedStorageCapacity  string
)

const (
	k8sResourcePrefix = "p-rmq-"
	serviceSuffix     = "-rabbitmq-ingress"
	statefulSetSuffix = "-rabbitmq-server"
	secretSuffix      = "-rabbitmq-admin"
	configMapSuffix   = "-rabbitmq-plugins"
)

var _ = SynchronizedBeforeSuite(func() []byte {
	scheme := runtime.NewScheme()
	Expect(rabbitmqv1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(defaultscheme.AddToScheme(scheme)).NotTo(HaveOccurred())

	restConfig, err := createRestConfig()
	Expect(err).NotTo(HaveOccurred())

	mgr, err = ctrl.NewManager(restConfig, ctrl.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	k8sClient = mgr.GetClient()

	clientSet, err = createClientSet()
	Expect(err).NotTo(HaveOccurred())

	namespace = MustHaveEnv("NAMESPACE")

	specifiedStorageClassName = "persistent-test"
	specifiedStorageCapacity = "1Gi"

	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: specifiedStorageClassName,
		},
		Provisioner: "kubernetes.io/gce-pd",
	}
	err = k8sClient.Create(context.TODO(), storageClass)
	if !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}

	// Patch/update configMap
	operatorConfigMapName := k8sResourcePrefix + "operator-config"
	configMap, err := clientSet.CoreV1().ConfigMaps(namespace).Get(operatorConfigMapName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	operatorConMap, err := config.NewConfig([]byte(configMap.Data["CONFIG"]))
	Expect(err).NotTo(HaveOccurred())

	operatorConMap.Persistence.StorageClassName = specifiedStorageClassName
	operatorConMap.Persistence.Storage = specifiedStorageCapacity
	configBytes, err := toYamlBytes(operatorConMap)
	Expect(err).NotTo(HaveOccurred())
	configMap.Data["CONFIG"] = string(configBytes)

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

	Eventually(func() string {
		pod, err := clientSet.CoreV1().Pods(namespace).Get(operatorPod.Name, metav1.GetOptions{})
		if err != nil {
			Expect(err).To(MatchError(fmt.Sprintf("pods \"%s\" not found", operatorPod.Name)))
			return ""
		}

		return fmt.Sprintf("%v", pod.Status.Conditions)
	}, podCreationTimeout, 5).Should(ContainSubstring("ContainersReady True"))

	return nil
}, func(data []byte) {
	scheme := runtime.NewScheme()
	Expect(rabbitmqv1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(defaultscheme.AddToScheme(scheme)).NotTo(HaveOccurred())

	restConfig, err := createRestConfig()
	Expect(err).NotTo(HaveOccurred())

	mgr, err = ctrl.NewManager(restConfig, ctrl.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	k8sClient = mgr.GetClient()

	clientSet, err = createClientSet()
	Expect(err).NotTo(HaveOccurred())

	namespace = MustHaveEnv("NAMESPACE")

	specifiedStorageClassName = "persistent-test"
	specifiedStorageCapacity = "1Gi"

})
