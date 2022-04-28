package scaling_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/scaling"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("Scaling", func() {
	BeforeEach(func() {
		rmq = rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rabbit",
				Namespace: namespace,
			},
		}
		existingPVC = generatePVC(rmq, 0, tenG)
		existingSts = appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rabbit-server",
				Namespace: namespace,
			},
			Spec: appsv1.StatefulSetSpec{
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{generatePVCTemplate(tenG)},
			},
		}
	})
	JustBeforeEach(func() {
		fakeClientset = fake.NewSimpleClientset(initialAPIObjects...)
		persistenceScaler = scaling.NewPersistenceScaler(fakeClientset)
	})

	When("the PVC and StatefulSet already exist", func() {
		BeforeEach(func() {
			initialAPIObjects = []runtime.Object{&existingSts, &existingPVC}
		})
		It("scales the PVC", func() {
			Expect(persistenceScaler.Scale(context.Background(), rmq, fifteenG)).To(Succeed())
			Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
				"0": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
				"1": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
				"2": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
				"3": BeDeleteActionOnResource("statefulsets", "rabbit-server", namespace),
				"4": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
				"5": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
				"6": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace, MatchFields(IgnoreExtras, Fields{
					"Spec": MatchFields(IgnoreExtras, Fields{
						"Resources": MatchFields(IgnoreExtras, Fields{
							"Requests": MatchAllKeys(Keys{
								corev1.ResourceStorage: Equal(fifteenG),
							}),
						}),
					}),
				})),
			}))
		})
	})

	When("the PVC does not yet exist", func() {
		BeforeEach(func() {
			initialAPIObjects = []runtime.Object{&existingSts}
		})
		It("performs no actions other than checking for the PVC's existence", func() {
			Expect(persistenceScaler.Scale(context.Background(), rmq, fifteenG)).To(Succeed())
			Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
				"0": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
				"1": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
			}))
		})
	})

	When("the PVC exists, but the StatefulSet does not exist", func() {
		BeforeEach(func() {
			initialAPIObjects = []runtime.Object{&existingPVC}
		})
		It("does not delete the StatefulSet, but still updates the PVC", func() {
			Expect(persistenceScaler.Scale(context.Background(), rmq, fifteenG)).To(Succeed())
			Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
				"0": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
				"1": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
				"2": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
				"3": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
				"4": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace, MatchFields(IgnoreExtras, Fields{
					"Spec": MatchFields(IgnoreExtras, Fields{
						"Resources": MatchFields(IgnoreExtras, Fields{
							"Requests": MatchAllKeys(Keys{
								corev1.ResourceStorage: Equal(fifteenG),
							}),
						}),
					}),
				})),
			}))
		})
	})

	When("the desired PVC capacity is lower than the existing PVC", func() {
		BeforeEach(func() {
			initialAPIObjects = []runtime.Object{&existingSts, &existingPVC}
		})
		It("raises an error", func() {
			Expect(persistenceScaler.Scale(context.Background(), rmq, oneG)).To(MatchError("shrinking persistent volumes is not supported"))
			Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
				"0": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
			}))
		})
	})

	When("the existing cluster is using ephemeral storage", func() {
		BeforeEach(func() {
			existingSts.Spec.VolumeClaimTemplates = nil
			initialAPIObjects = []runtime.Object{&existingSts}
		})
		It("raises an error if trying to move to persistent storage", func() {
			Expect(persistenceScaler.Scale(context.Background(), rmq, tenG)).To(MatchError("changing from ephemeral to persistent storage is not supported"))
			Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
				"0": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
			}))
		})
		It("does nothing if remaining as ephemeral storage", func() {
			Expect(persistenceScaler.Scale(context.Background(), rmq, ephemeralStorage)).To(Succeed())
			Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
				"0": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
				"1": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
			}))
		})
	})

	When("the cluster has more than one replica", func() {
		When("all the PVCs exist and are the same size", func() {
			BeforeEach(func() {
				rmq.Spec.Replicas = &three
				existingPVC0 := generatePVC(rmq, 0, tenG)
				existingPVC1 := generatePVC(rmq, 1, tenG)
				existingPVC2 := generatePVC(rmq, 2, tenG)
				initialAPIObjects = []runtime.Object{&existingSts, &existingPVC0, &existingPVC1, &existingPVC2}
			})
			It("deletes the statefulset and updates each individual PVC", func() {
				Expect(persistenceScaler.Scale(context.Background(), rmq, fifteenG)).To(Succeed())
				Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
					"0": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
					"1": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
					"2": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-1", namespace),
					"3": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace),
					"4": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
					"5": BeDeleteActionOnResource("statefulsets", "rabbit-server", namespace),
					"6": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
					"7": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
					"8": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace, MatchFields(IgnoreExtras, Fields{
						"Spec": MatchFields(IgnoreExtras, Fields{
							"Resources": MatchFields(IgnoreExtras, Fields{
								"Requests": MatchAllKeys(Keys{
									corev1.ResourceStorage: Equal(fifteenG),
								}),
							}),
						}),
					})),
					"9": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-1", namespace),
					"10": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-1", namespace, MatchFields(IgnoreExtras, Fields{
						"Spec": MatchFields(IgnoreExtras, Fields{
							"Resources": MatchFields(IgnoreExtras, Fields{
								"Requests": MatchAllKeys(Keys{
									corev1.ResourceStorage: Equal(fifteenG),
								}),
							}),
						}),
					})),
					"11": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace),
					"12": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace, MatchFields(IgnoreExtras, Fields{
						"Spec": MatchFields(IgnoreExtras, Fields{
							"Resources": MatchFields(IgnoreExtras, Fields{
								"Requests": MatchAllKeys(Keys{
									corev1.ResourceStorage: Equal(fifteenG),
								}),
							}),
						}),
					})),
				}))
			})
		})

		When("some of the PVCs don't exist yet", func() {
			BeforeEach(func() {
				rmq.Spec.Replicas = &three
				existingPVC0 := generatePVC(rmq, 0, tenG)
				existingPVC2 := generatePVC(rmq, 2, tenG)
				initialAPIObjects = []runtime.Object{&existingSts, &existingPVC0, &existingPVC2}
			})
			It("deletes the statefulset and updates the PVCs that exist", func() {
				Expect(persistenceScaler.Scale(context.Background(), rmq, fifteenG)).To(Succeed())
				Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
					"0": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
					"1": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
					"2": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-1", namespace),
					"3": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace),
					"4": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
					"5": BeDeleteActionOnResource("statefulsets", "rabbit-server", namespace),
					"6": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
					"7": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
					"8": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace, MatchFields(IgnoreExtras, Fields{
						"Spec": MatchFields(IgnoreExtras, Fields{
							"Resources": MatchFields(IgnoreExtras, Fields{
								"Requests": MatchAllKeys(Keys{
									corev1.ResourceStorage: Equal(fifteenG),
								}),
							}),
						}),
					})),
					"9": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace),
					"10": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace, MatchFields(IgnoreExtras, Fields{
						"Spec": MatchFields(IgnoreExtras, Fields{
							"Resources": MatchFields(IgnoreExtras, Fields{
								"Requests": MatchAllKeys(Keys{
									corev1.ResourceStorage: Equal(fifteenG),
								}),
							}),
						}),
					})),
				}))
			})
		})

		When("some of the PVCs have already been resized", func() {
			BeforeEach(func() {
				rmq.Spec.Replicas = &three
				existingPVC0 := generatePVC(rmq, 0, fifteenG)
				existingPVC1 := generatePVC(rmq, 1, fifteenG)
				existingPVC2 := generatePVC(rmq, 2, tenG)
				initialAPIObjects = []runtime.Object{&existingSts, &existingPVC0, &existingPVC1, &existingPVC2}
			})
			It("deletes the statefulset and updates the PVCs that exist", func() {
				Expect(persistenceScaler.Scale(context.Background(), rmq, fifteenG)).To(Succeed())
				Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
					"0": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
					"1": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
					"2": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-1", namespace),
					"3": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace),
					"4": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
					"5": BeDeleteActionOnResource("statefulsets", "rabbit-server", namespace),
					"6": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
					"7": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace),
					"8": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace, MatchFields(IgnoreExtras, Fields{
						"Spec": MatchFields(IgnoreExtras, Fields{
							"Resources": MatchFields(IgnoreExtras, Fields{
								"Requests": MatchAllKeys(Keys{
									corev1.ResourceStorage: Equal(fifteenG),
								}),
							}),
						}),
					})),
				}))
			})
		})
	})
})
