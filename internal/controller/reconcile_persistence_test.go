package controllers_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/v2/internal/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Persistence", func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		defaultNamespace = "default"
		ctx              = context.Background()
	)

	BeforeEach(func() {
		cluster = &rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("rabbitmq-persistence-%d", time.Now().UnixNano()),
				Namespace: defaultNamespace,
			},
			Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
				Replicas: ptr.To(int32(5)),
			},
		}
		Expect(client.Create(ctx, cluster)).To(Succeed())
		waitForClusterCreation(ctx, cluster, client)
	})

	AfterEach(func() {
		err := client.Delete(ctx, cluster)
		Expect(err == nil || apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("does not allow PVC shrink", func() {
		By("not updating statefulSet volume claim storage capacity", func() {
			tenG := k8sresource.MustParse("10Gi")
			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				storage := k8sresource.MustParse("1Gi")
				cluster.Spec.Persistence.Storage = &storage
			})).To(Succeed())
			Consistently(func() k8sresource.Quantity {
				sts, err := clientSet.AppsV1().StatefulSets(defaultNamespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
			}, 10, 1).Should(Equal(tenG))
		})

		By("setting 'Warning' events", func() {
			Expect(aggregateEventMsgs(ctx, cluster, "FailedReconcilePersistence")).To(
				ContainSubstring("shrinking persistent volumes is not supported"))
		})

		By("setting ReconcileSuccess to 'false' with failed reason and message", func() {
			Eventually(func() string {
				rabbit := &rabbitmqv1beta1.RabbitmqCluster{}
				Expect(client.Get(ctx, runtimeClient.ObjectKey{
					Name:      cluster.Name,
					Namespace: defaultNamespace,
				}, rabbit)).To(Succeed())

				for i := range rabbit.Status.Conditions {
					if rabbit.Status.Conditions[i].Type == status.ReconcileSuccess {
						return fmt.Sprintf(
							"ReconcileSuccess status: %s, with reason: %s and message: %s",
							rabbit.Status.Conditions[i].Status,
							rabbit.Status.Conditions[i].Reason,
							rabbit.Status.Conditions[i].Message)
					}
				}
				return "ReconcileSuccess status: condition not present"
			}, 5).Should(Equal("ReconcileSuccess status: False, " +
				"with reason: FailedReconcilePVC " +
				"and message: shrinking persistent volumes is not supported"))
		})
	})

	It("removes RabbitmqCluster owner references from existing PVCs when retaining PVCs after deletion", func() {
		pvcName := cluster.PVCName(0)
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: defaultNamespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         rabbitmqv1beta1.GroupVersion.String(),
						Kind:               "RabbitmqCluster",
						Name:               cluster.Name,
						UID:                cluster.UID,
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(false),
					},
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: k8sresource.MustParse("10Gi"),
					},
				},
			},
		}
		_, err := clientSet.CoreV1().PersistentVolumeClaims(defaultNamespace).Create(ctx, pvc, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
			r.Spec.Override.StatefulSet = &rabbitmqv1beta1.StatefulSet{
				Spec: &rabbitmqv1beta1.StatefulSetSpec{
					PersistentVolumeClaimRetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
						WhenDeleted: appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
						WhenScaled:  appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
					},
				},
			}
		})).To(Succeed())

		Eventually(func() []metav1.OwnerReference {
			pvc, err := clientSet.CoreV1().PersistentVolumeClaims(defaultNamespace).Get(ctx, pvcName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return pvc.OwnerReferences
		}, 5).Should(BeEmpty())
	})
})
