package rabbitmqclient_test

import (
	"crypto/tls"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/ghttp"
	"github.com/rabbitmq/cluster-operator/internal/topology/testutils"
	"net/url"
	"strconv"
	"testing"
)

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rabbitmqclient Suite")
}

func mockRabbitMQServer() *ghttp.Server {
	return ghttp.NewServer()
}

func mockRabbitMQTLSServer() (*ghttp.Server, string, string, string) {
	fakeRabbitMQServer := ghttp.NewUnstartedServer()

	// create cert files
	serverCertPath, serverCertFile := testutils.CreateCertFile(1, "server.crt")
	serverKeyPath, serverKeyFile := testutils.CreateCertFile(1, "server.key")
	caCertPath, caCertFile := testutils.CreateCertFile(1, "ca.crt")

	// generate and write cert and key to file
	_, _ = testutils.CreateCertificateChain(1, "127.0.0.1", caCertFile, serverCertFile, serverKeyFile)

	cert, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	fakeRabbitMQServer.HTTPTestServer.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	fakeRabbitMQServer.HTTPTestServer.StartTLS()
	return fakeRabbitMQServer, serverCertPath, serverKeyPath, caCertPath
}

func mockRabbitMQURLPort(fakeRabbitMQServer *ghttp.Server) (*url.URL, int, error) {
	fakeRabbitMQURL, err := url.Parse(fakeRabbitMQServer.URL())
	if err != nil {
		return nil, 0, err
	}
	fakeRabbitMQPort, err := strconv.Atoi(fakeRabbitMQURL.Port())
	if err != nil {
		return nil, 0, err
	}
	return fakeRabbitMQURL, fakeRabbitMQPort, nil
}
