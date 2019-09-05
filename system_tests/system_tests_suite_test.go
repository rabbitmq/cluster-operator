package system_tests

import (
	"context"
	v1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/pivotal/rabbitmq-for-kubernetes/internal/config"
)

func TestSystemTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SystemTests Suite")
}

var k8sClient client.Client
var clientSet *kubernetes.Clientset
var namespace, operatorConMapStorageClassName string
var mgr manager.Manager

var _ = BeforeSuite(func() {
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

	configMap, err := clientSet.CoreV1().ConfigMaps(namespace).Get("pivotal-rabbitmq-manager-config", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	operatorConMap, err := config.NewConfig([]byte(configMap.Data["CONFIG"]))
	Expect(err).NotTo(HaveOccurred())

	operatorConMapStorageClassName = operatorConMap.Persistence.StorageClassName

	storageClass := &v1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: operatorConMapStorageClassName,
		},
		Provisioner: "kubernetes.io/gce-pd",
	}

	err = k8sClient.Create(context.TODO(), storageClass)
	if !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}
})
