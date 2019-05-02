package integration_tests_test

import (
	"encoding/json"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
)

const catalogURL = baseURL + "catalog"

var _ = Describe("/v2/catalog", func() {
	It("succeeds with HTTP 200 and returns a valid catalog", func() {
		response, body := doRequest(http.MethodGet, catalogURL, nil)
		Expect(response.StatusCode).To(Equal(http.StatusOK))

		catalog := make(map[string][]brokerapi.Service)
		Expect(json.Unmarshal(body, &catalog)).To(Succeed())

		Expect(catalog["services"]).To(HaveLen(1))

	})
})
