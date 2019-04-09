package integration_tests_test

import (
	"io"
	"io/ioutil"
	"net/http"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const (
	baseURL  = "http://localhost:8123/v2/"
	username = "p1-rabbit"
	password = "p1-rabbit-testpwd"
)

var (
	session *gexec.Session
)

func TestIntegrationtests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integrationtests Suite")
}

var _ = BeforeSuite(func() {
	pathToServiceBroker, err := gexec.Build("servicebroker")
	Expect(err).NotTo(HaveOccurred())

	path, err := filepath.Abs(filepath.Join("fixtures", "config.yaml"))
	Expect(err).ToNot(HaveOccurred())

	command := exec.Command(pathToServiceBroker, "-configPath", path)
	session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(brokerIsServing).Should(BeTrue())
})

var _ = AfterSuite(func() {
	session.Kill().Wait()
	gexec.CleanupBuildArtifacts()
})

func doRequest(method, url string, body io.Reader) (*http.Response, []byte) {
	req, err := http.NewRequest(method, url, body)
	Expect(err).NotTo(HaveOccurred())

	req.SetBasicAuth(username, password)
	req.Header.Set("X-Broker-API-Version", "2.14")

	req.Close = true
	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())

	bodyContent, err := ioutil.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	Expect(resp.Body.Close()).To(Succeed())
	return resp, bodyContent
}

func brokerIsServing() bool {
	_, err := http.Get(baseURL)
	return err == nil
}
