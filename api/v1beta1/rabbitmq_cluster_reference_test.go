package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("RabbitmqClusterReference", func() {
	var reference *RabbitmqClusterReference

	BeforeEach(func() {
		reference = &RabbitmqClusterReference{
			Name:      "a-name",
			Namespace: "a-ns",
			ConnectionSecret: &v1.LocalObjectReference{
				Name: "a-secret-name",
			},
		}
	})

	Context("Matches", func() {
		When("name is different", func() {
			It("returns false", func() {
				new := reference.DeepCopy()
				new.Name = "new-name"
				Expect(reference.Matches(new)).To(BeFalse())
			})
		})

		When("namespace is different", func() {
			It("returns false", func() {
				new := reference.DeepCopy()
				new.Namespace = "new-ns"
				Expect(reference.Matches(new)).To(BeFalse())
			})
		})

		When("connectionSecret.name is different", func() {
			It("returns false", func() {
				new := reference.DeepCopy()
				new.ConnectionSecret.Name = "new-secret-name"
				Expect(reference.Matches(new)).To(BeFalse())
			})
		})

		When("connectionSecret is removed", func() {
			It("returns false", func() {
				new := reference.DeepCopy()
				new.ConnectionSecret = nil
				Expect(reference.Matches(new)).To(BeFalse())
			})
		})

		When("connectionSecret is added", func() {
			It("returns false", func() {
				reference.ConnectionSecret = nil
				new := reference.DeepCopy()
				new.ConnectionSecret = &v1.LocalObjectReference{
					Name: "a-secret-name",
				}
				Expect(reference.Matches(new)).To(BeFalse())
			})
		})

		When("RabbitmqClusterReference stayed the same", func() {
			It("returns true", func() {
				new := reference.DeepCopy()
				Expect(reference.Matches(new)).To(BeTrue())
			})
		})
	})

	Context("ValidateOnCreate", func() {
		When("name is provided", func() {
			It("returns no error", func() {
				reference.ConnectionSecret = nil
				reference.Name = "a-name"
				Expect(reference.ValidateOnCreate(schema.GroupResource{}, "a-resource")).To(Succeed())
			})
		})

		When("connectionSecret is provided", func() {
			It("returns no error", func() {
				reference.ConnectionSecret = &v1.LocalObjectReference{Name: "a-secret-name"}
				reference.Name = ""
				Expect(reference.ValidateOnCreate(schema.GroupResource{}, "a-resource")).To(Succeed())
			})
		})

		When("name and connectionSecrets are both provided", func() {
			It("returns a forbidden api error", func() {
				reference.Name = "a-cluster"
				reference.ConnectionSecret = &v1.LocalObjectReference{Name: "a-secret-name"}
				Expect(apierrors.IsForbidden(reference.ValidateOnCreate(schema.GroupResource{}, "a-resource"))).To(BeTrue())
			})
		})

		When("name and connectionSecrets are both empty", func() {
			It("returns a forbidden api error", func() {
				reference.ConnectionSecret = nil
				reference.Name = ""
				Expect(apierrors.IsForbidden(reference.ValidateOnCreate(schema.GroupResource{}, "a-resource"))).To(BeTrue())
			})
		})
	})

})
