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

		Expect(catalog["services"][0]).To(Equal(brokerapi.Service{
			ID:          "00000000-0000-0000-0000-000000000000",
			Name:        "p-rabbitmq-k8s",
			Description: "RabbitMQ on K8s",
			Bindable:    true,
			Plans: []brokerapi.ServicePlan{
				brokerapi.ServicePlan{
					ID:          "11111111-1111-1111-1111-111111111111",
					Name:        "ha",
					Description: "HA RabbitMQ on K8s",
				},
				brokerapi.ServicePlan{
					ID:          "22222222-2222-2222-2222-222222222222",
					Name:        "single",
					Description: "Single-node RabbitMQ on K8s",
				},
			},
		}))
	})
})
