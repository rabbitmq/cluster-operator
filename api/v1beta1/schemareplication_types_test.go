package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("schemaReplication spec", func() {
	It("creates a schemaReplication", func() {
		replication := SchemaReplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "replication",
				Namespace: "default",
			},
			Spec: SchemaReplicationSpec{
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
				UpstreamSecret: &corev1.LocalObjectReference{
					Name: "a-secret",
				},
				Endpoints: "abc.rmq.com:1234",
			}}
		Expect(k8sClient.Create(context.Background(), &replication)).To(Succeed())

		fetched := &SchemaReplication{}
		Expect(k8sClient.Get(context.Background(), types.NamespacedName{
			Name:      replication.Name,
			Namespace: replication.Namespace,
		}, fetched)).To(Succeed())
		Expect(fetched.Spec.RabbitmqClusterReference).To(Equal(RabbitmqClusterReference{
			Name: "some-cluster",
		}))
		Expect(fetched.Spec.UpstreamSecret.Name).To(Equal("a-secret"))
		Expect(fetched.Spec.Endpoints).To(Equal("abc.rmq.com:1234"))
	})
})
