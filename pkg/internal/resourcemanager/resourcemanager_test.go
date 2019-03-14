package resourcemanager_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/pkg/apis/rabbitmq/v1beta1"
	. "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/plans"
	"github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/plans/plansfakes"
	. "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/resourcegenerator"
	"github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/resourcegenerator/resourcegeneratorfakes"
	. "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/resourcemanager"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Resourcemanager", func() {
	var (
		plans           *plansfakes.FakePlans
		instance        *rabbitmqv1beta1.RabbitmqCluster
		generator       *resourcegeneratorfakes.FakeResourceGenerator
		resourceManager ResourceManager
	)
	Describe("Configure", func() {
		BeforeEach(func() {
			plans = new(plansfakes.FakePlans)
			generator = new(resourcegeneratorfakes.FakeResourceGenerator)

			resourceManager = &RabbitResourceManager{}

			instance = &rabbitmqv1beta1.RabbitmqCluster{
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Plan: "plan1",
				},
				Status: rabbitmqv1beta1.RabbitmqClusterStatus{},
				TypeMeta: metav1.TypeMeta{
					Kind:       "RabbitmqCluster",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq",
					Namespace: "rabbitmq",
				},
			}
		})
		It("retrieves the plan", func() {
			resourceManager.Configure(instance, plans, generator)
			planName := plans.GetArgsForCall(0)

			Expect(instance.Spec.Plan).To(Equal(planName))
		})

		It("returns an empty target resource and an error if plan retrieval fails", func() {
			planErr := errors.New("")
			plans.GetReturns(Configuration{}, planErr)

			targetResource, err := resourceManager.Configure(instance, plans, generator)

			Expect(targetResource).To(Equal([]TargetResource{}))
			Expect(err).To(BeIdenticalTo(planErr))
		})

		It("builds the context from the plan and instance to pass to the generator", func() {
			var TestBuild = func(nodes int32) {
				configuration := Configuration{
					Nodes: nodes,
				}
				plans.GetReturns(configuration, nil)

				resourceManager.Configure(instance, plans, generator)
				generationContext := generator.BuildArgsForCall(0)

				Expect(generationContext.Namespace).To(Equal(instance.Namespace))
				Expect(generationContext.InstanceName).To(Equal(instance.Name))
				Expect(generationContext.Nodes).To(Equal(configuration.Nodes))
			}
			TestBuild(int32(GinkgoRandomSeed()))
		})

	})
})
