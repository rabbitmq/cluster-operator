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

var _ = Describe("RabbitmqCluster spec", func() {

	It("can be created with a single replica", func() {
		created := generateRabbitmqClusterObject("rabbit1", 1)

		Expect(k8sClient.Create(context.TODO(), created)).To(Succeed())

		fetched := &RabbitmqCluster{}
		Expect(k8sClient.Get(context.TODO(), getKey(created), fetched)).To(Succeed())
		Expect(fetched).To(Equal(created))
	})

	It("can be created with three replicas", func() {
		created := generateRabbitmqClusterObject("rabbit2", 3)

		Expect(k8sClient.Create(context.TODO(), created)).To(Succeed())

		fetched := &RabbitmqCluster{}
		Expect(k8sClient.Get(context.TODO(), getKey(created), fetched)).To(Succeed())
		Expect(fetched).To(Equal(created))
	})

	It("can be deleted", func() {
		created := generateRabbitmqClusterObject("rabbit3", 1)
		Expect(k8sClient.Create(context.TODO(), created)).To(Succeed())

		Expect(k8sClient.Delete(context.TODO(), created)).To(Succeed())
		Expect(k8sClient.Get(context.TODO(), getKey(created), created)).ToNot(Succeed())
	})

	It("is validated", func() {
		By("checking the replica count", func() {
			invalidReplica := generateRabbitmqClusterObject("rabbit4", 1)
			invalidReplica.Spec.Replicas = 5
			Expect(k8sClient.Create(context.TODO(), invalidReplica)).To(MatchError(ContainSubstring("validation failure list:\nspec.replicas in body should be one of [1 3]")))
		})

		By("checking the service type", func() {
			invalidService := generateRabbitmqClusterObject("rabbit5", 1)
			invalidService.Spec.Service.Type = "ihateservices"
			Expect(k8sClient.Create(context.TODO(), invalidService)).To(MatchError(ContainSubstring("validation failure list:\nspec.service.type in body should be one of [ClusterIP LoadBalancer NodePort]")))
		})
	})

	Describe("ChildResourceName", func() {
		It("prefixes the passed string with the name of the RabbitmqCluster name", func() {
			resource := generateRabbitmqClusterObject("iam", 1)
			Expect(resource.ChildResourceName("great")).To(Equal("iam-great"))
		})
	})
})

func getKey(cluster *RabbitmqCluster) types.NamespacedName {
	return types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}
}

func generateRabbitmqClusterObject(clusterName string, numReplicas int) *RabbitmqCluster {
	return &RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: "default",
		},
		Spec: RabbitmqClusterSpec{
			Replicas: numReplicas,
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
