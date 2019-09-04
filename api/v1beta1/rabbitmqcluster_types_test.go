/*
Copyright 2019 Pivotal.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("RabbitmqCluster", func() {

	Context("Create API", func() {

		It("should create an object successfully", func() {
			key := types.NamespacedName{
				Name:      "cluster",
				Namespace: "default",
			}

			created := generateRabbitmqClusterObject()
			By("creating an API obj", func() {
				Expect(k8sClient.Create(context.TODO(), created)).To(Succeed())

				fetched := &RabbitmqCluster{}
				Expect(k8sClient.Get(context.TODO(), key, fetched)).To(Succeed())
				Expect(fetched).To(Equal(created))
			})

			By("deleting the created object", func() {
				Expect(k8sClient.Delete(context.TODO(), created)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), key, created)).ToNot(Succeed())
			})

			By("validating the provided replicas", func() {
				invalidReplica := generateRabbitmqClusterObject()
				invalidReplica.Spec.Replicas = 5
				Expect(k8sClient.Create(context.TODO(), invalidReplica)).To(MatchError(ContainSubstring("validation failure list:\nspec.replicas in body should be one of [1]")))
			})

			By("validating the provided service type", func() {
				invalidService := generateRabbitmqClusterObject()
				invalidService.Spec.Service.Type = "ihateservices"
				Expect(k8sClient.Create(context.TODO(), invalidService)).To(MatchError(ContainSubstring("validation failure list:\nspec.service.type in body should be one of [ClusterIP LoadBalancer NodePort]")))
			})
		})
	})
})

func generateRabbitmqClusterObject() *RabbitmqCluster {
	return &RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: "default",
		},
		Spec: RabbitmqClusterSpec{
			Replicas: 1,
			Image: RabbitmqClusterImageSpec{
				Repository: "my-private-repo",
			},
			ImagePullSecret: "some-secret-name",
			Service: RabbitmqClusterServiceSpec{
				Type: "LoadBalancer",
			},
			Persistence: RabbitmqClusterPersistenceSpec{
				Storage:          "some-storage",
				StorageClassName: "some-storage-class-name",
			},
		},
	}
}
