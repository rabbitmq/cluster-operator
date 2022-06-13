package profiling_test

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var testenv *envtest.Environment
var cfg *rest.Config
var clientset *kubernetes.Clientset

func TestProfiling(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Profiling Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testenv = &envtest.Environment{}

	var err error
	cfg, err = testenv.Start()
	Expect(err).NotTo(HaveOccurred())

})

var _ = AfterSuite(func() {
	Expect(testenv.Stop()).To(Succeed())
})

func getFreePort() (string, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return "", err
	}
	defer l.Close()
	return l.Addr().String(), nil
}
