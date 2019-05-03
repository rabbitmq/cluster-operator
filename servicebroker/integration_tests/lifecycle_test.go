package integration_tests_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("the lifecycle of a service instance", func() {
	const (
		serviceID = "00000000-0000-0000-0000-000000000000"
		planID    = "11111111-1111-1111-1111-111111111111"
		bindingID = "22222222-2222-2222-2222-222222222222"
	)

	It("succeeds for one SI and binding", func() {
		serviceInstanceID := uuid.New().String()

		By("sending a provision request")
		provisionResponse, provisionBody := provision(serviceInstanceID, serviceID, planID)
		Expect(provisionResponse.StatusCode).To(Equal(http.StatusAccepted), string(provisionBody))

		By("checking that the rabbitmq pod is created with the correct plan")
		planCommand := exec.Command("kubectl", "-n", "rabbitmq-for-kubernetes", "get", "rabbitmqcluster", serviceInstanceID, "-o=jsonpath='{.spec.plan}'")
		planSession, err := gexec.Start(planCommand, GinkgoWriter, GinkgoWriter)
		planSession.Wait(30 * time.Second)
		Expect(planSession.ExitCode()).To(Equal(0))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(planSession.Out.Contents())).To(Equal("'ha'"))

		By("sending a binding request")
		var bindBody []byte
		Eventually(func() int {
			var bindResponse *http.Response
			bindResponse, bindBody = bind(bindingID, serviceInstanceID, serviceID, planID)
			return bindResponse.StatusCode
		}, "60s").Should(Equal(http.StatusCreated), string(bindBody))

		By("checking the binding credentials")
		var binding map[string]interface{}
		Expect(json.Unmarshal(bindBody, &binding)).To(Succeed())
		Expect(binding).To(HaveKeyWithValue("credentials", SatisfyAll(
			HaveKeyWithValue("username", bindingID),
			HaveKeyWithValue("vhost", serviceInstanceID))))
	})
})

func provisionURL(serviceInstanceID string) string {
	return fmt.Sprintf("%sservice_instances/%s", baseURL, serviceInstanceID)
}

func provision(serviceInstanceID, serviceID, planID string) (*http.Response, []byte) {
	provisionDetails, err := json.Marshal(map[string]string{
		"service_id":        serviceID,
		"plan_id":           planID,
		"organization_guid": "fake-org-guid",
		"space_guid":        "fake-space-guid",
	})
	Expect(err).NotTo(HaveOccurred())

	return doRequest(http.MethodPut, provisionURL(serviceInstanceID), bytes.NewReader(provisionDetails))
}

func bindURL(serviceInstanceID, bindingID string) string {
	return fmt.Sprintf("%sservice_instances/%s/service_bindings/%s", baseURL, serviceInstanceID, bindingID)
}

func bind(bindingID, serviceInstanceID, serviceID, planID string) (*http.Response, []byte) {
	bindDetails, err := json.Marshal(map[string]string{
		"service_id": serviceID,
		"plan_id":    planID,
	})
	Expect(err).NotTo(HaveOccurred())

	return doRequest(http.MethodPut, bindURL(serviceInstanceID, bindingID), bytes.NewReader(bindDetails))
}
