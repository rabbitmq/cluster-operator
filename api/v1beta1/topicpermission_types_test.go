package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("TopicPermission", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates a topic permission when username is provided", func() {
		permission := TopicPermission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-permission-1",
				Namespace: namespace,
			},
			Spec: TopicPermissionSpec{
				User:  "test",
				Vhost: "/test",
				Permissions: TopicPermissionConfig{
					Exchange: "some",
					Read:     "^?",
					Write:    ".*",
				},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &permission)).To(Succeed())
		fetchedTopicPermission := &TopicPermission{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      permission.Name,
			Namespace: permission.Namespace,
		}, fetchedTopicPermission)).To(Succeed())
		Expect(fetchedTopicPermission.Spec.User).To(Equal("test"))
		Expect(fetchedTopicPermission.Spec.Vhost).To(Equal("/test"))
		Expect(fetchedTopicPermission.Spec.RabbitmqClusterReference.Name).To(Equal("some-cluster"))

		Expect(fetchedTopicPermission.Spec.Permissions.Exchange).To(Equal("some"))
		Expect(fetchedTopicPermission.Spec.Permissions.Write).To(Equal(".*"))
		Expect(fetchedTopicPermission.Spec.Permissions.Read).To(Equal("^?"))
	})

	It("creates a permission object with user reference is provided", func() {
		permission := TopicPermission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "user-ref-permission",
				Namespace: namespace,
			},
			Spec: TopicPermissionSpec{
				UserReference: &corev1.LocalObjectReference{
					Name: "a-created-user",
				},
				Vhost:       "/test",
				Permissions: TopicPermissionConfig{},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &permission)).To(Succeed())
		fetchedTopicPermission := &TopicPermission{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      permission.Name,
			Namespace: permission.Namespace,
		}, fetchedTopicPermission)).To(Succeed())
		Expect(fetchedTopicPermission.Spec.UserReference.Name).To(Equal("a-created-user"))
		Expect(fetchedTopicPermission.Spec.User).To(Equal(""))
		Expect(fetchedTopicPermission.Spec.Vhost).To(Equal("/test"))
		Expect(fetchedTopicPermission.Spec.RabbitmqClusterReference.Name).To(Equal("some-cluster"))
	})
})
