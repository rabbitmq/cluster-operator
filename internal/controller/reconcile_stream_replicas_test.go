package controllers

import (
	"context"
	"fmt"

	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/v2/internal/rabbitmqclient"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type streamReplicaTestExecutor struct {
	calls [][]string
}

func (e *streamReplicaTestExecutor) Exec(_ *kubernetes.Clientset, _ *rest.Config, namespace, podName, containerName string, command ...string) (string, string, error) {
	e.calls = append(e.calls, append([]string{namespace, podName, containerName}, command...))
	return "", "", nil
}

type streamReplicaTestRabbitmqClient struct {
	queues []rabbithole.QueueInfo
	err    error
}

func (c *streamReplicaTestRabbitmqClient) Overview() (*rabbithole.Overview, error) {
	return &rabbithole.Overview{}, nil
}

func (c *streamReplicaTestRabbitmqClient) HealthCheckNodeIsQuorumCritical() (rabbithole.HealthCheckStatus, error) {
	return rabbithole.HealthCheckStatus{Status: "ok"}, nil
}

func (c *streamReplicaTestRabbitmqClient) ListDeprecatedFeaturesUsed() ([]rabbithole.DeprecatedFeature, error) {
	return nil, nil
}

func (c *streamReplicaTestRabbitmqClient) ListQueues() ([]rabbithole.QueueInfo, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.queues, nil
}

type streamReplicaTestRabbitmqClientFactory struct {
	client rabbitmqclient.RabbitmqClient
}

func (f *streamReplicaTestRabbitmqClientFactory) GetClientForPod(_ context.Context, _ client.Reader, _ *rabbitmqv1beta1.RabbitmqCluster, _ string) (rabbitmqclient.RabbitmqClient, error) {
	return f.client, nil
}

func (f *streamReplicaTestRabbitmqClientFactory) GetClientForService(_ context.Context, _ client.Reader, _ *rabbitmqv1beta1.RabbitmqCluster) (rabbitmqclient.RabbitmqClient, error) {
	return f.client, nil
}

var _ = Describe("Reconcile stream replicas", func() {
	It("adds missing stream replicas only for the new broker nodes", func() {
		cluster := &rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stream-scale",
				Namespace: "default",
			},
			Spec: rabbitmqv1beta1.RabbitmqClusterSpec{Replicas: ptrTo(int32(3))},
		}
		sts := &appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Replicas: ptrTo(int32(3))}}
		executor := &streamReplicaTestExecutor{}
		factory := &streamReplicaTestRabbitmqClientFactory{
			client: &streamReplicaTestRabbitmqClient{queues: []rabbithole.QueueInfo{{
				Name:    "orders",
				Vhost:   "/",
				Type:    "stream",
				Members: []string{"rabbit@stream-scale-server-0.stream-scale-nodes.default", "rabbit@stream-scale-server-1.stream-scale-nodes.default"},
			}}},
		}
		reconciler := &RabbitmqClusterReconciler{
			Clientset:             &kubernetes.Clientset{},
			PodExecutor:           executor,
			RabbitmqClientFactory: factory,
		}

		reconciler.reconcileStreamReplicas(context.Background(), cluster, sts)

		Expect(executor.calls).To(HaveLen(1))
		Expect(executor.calls[0]).To(Equal([]string{
			"default",
			"stream-scale-server-0",
			"rabbitmq",
			"sh",
			"-c",
			fmt.Sprintf("rabbitmq-streams add_replica --vhost %q %q %q", "/", "orders", "rabbit@stream-scale-server-2.stream-scale-nodes.default"),
		}))
	})

	It("does not add stream replicas when every desired node is already a member", func() {
		cluster := &rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stream-scale",
				Namespace: "default",
			},
			Spec: rabbitmqv1beta1.RabbitmqClusterSpec{Replicas: ptrTo(int32(3))},
		}
		sts := &appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Replicas: ptrTo(int32(3))}}
		executor := &streamReplicaTestExecutor{}
		factory := &streamReplicaTestRabbitmqClientFactory{
			client: &streamReplicaTestRabbitmqClient{queues: []rabbithole.QueueInfo{{
				Name:  "orders",
				Vhost: "/",
				Type:  "stream",
				Members: []string{
					"rabbit@stream-scale-server-0.stream-scale-nodes.default",
					"rabbit@stream-scale-server-1.stream-scale-nodes.default",
					"rabbit@stream-scale-server-2.stream-scale-nodes.default",
				},
			}}},
		}
		reconciler := &RabbitmqClusterReconciler{
			Clientset:             &kubernetes.Clientset{},
			PodExecutor:           executor,
			RabbitmqClientFactory: factory,
		}

		reconciler.reconcileStreamReplicas(context.Background(), cluster, sts)

		Expect(executor.calls).To(BeEmpty())
	})
})

func ptrTo[T any](v T) *T {
	return &v
}
