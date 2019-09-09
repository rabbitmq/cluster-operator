package system_tests

import (
	"testing"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
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
	k8sClient                      client.Client
	clientSet                      *kubernetes.Clientset
	namespace                      string
	operatorConMapStorageClassName string
	mgr                            manager.Manager
)

const (
	k8sResourcePrefix = "p-rmq-"
	serviceSuffix     = "-rabbitmq-ingress"
	statefulSetSuffix = "-rabbitmq-server"
)

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

})
