package scaling_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Scaling", func() {
	When("the PVC and StatefulSet already exist", func() {
		BeforeEach(func() {
			initialAPIObjects = []runtime.Object{&existingSts, &existingPVC}
		})
		It("scales the PVC", func() {
			Expect(persistenceScaler.Scale(context.Background(), existingCluster, fifteenG)).To(Succeed())
			Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
				"0": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
				"1": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
				"2": BeDeleteActionOnResource("statefulsets", "rabbit-server", namespace),
				"3": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
				"4": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
				"5": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace, MatchFields(IgnoreExtras, Fields{
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
			Expect(persistenceScaler.Scale(context.Background(), existingCluster, fifteenG)).To(Succeed())
			Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
				"0": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
			}))
		})
	})

	When("the PVC exists, but the StatefulSet does not exist", func() {
		BeforeEach(func() {
			initialAPIObjects = []runtime.Object{&existingPVC}
		})
		It("does not delete the StatefulSet, but still updates the PVC", func() {
			Expect(persistenceScaler.Scale(context.Background(), existingCluster, fifteenG)).To(Succeed())
			Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
				"0": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
				"1": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
				"2": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
				"3": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace, MatchFields(IgnoreExtras, Fields{
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
			Expect(persistenceScaler.Scale(context.Background(), existingCluster, oneG)).To(MatchError("shrinking persistent volumes is not supported"))
			Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
				"0": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
			}))
		})
	})

	When("the existing cluster is using ephemeral storage", func() {
		BeforeEach(func() {
			existingPVC := generatePVC(existingCluster, 0, ephemeralStorage)
			initialAPIObjects = []runtime.Object{&existingSts, &existingPVC}
		})
		It("raises an error if trying to move to persistent storage", func() {
			Expect(persistenceScaler.Scale(context.Background(), existingCluster, tenG)).To(MatchError("changing from ephemeral to persistent storage is not supported"))
			Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
				"0": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
			}))
		})
		It("does nothing if remaining as ephemeral storage", func() {
			Expect(persistenceScaler.Scale(context.Background(), existingCluster, ephemeralStorage)).To(Succeed())
			Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
				"0": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
			}))
		})
	})

	When("the cluster has more than one replica", func() {
		When("all the PVCs exist and are the same size", func() {
			BeforeEach(func() {
				existingCluster.Spec.Replicas = &three
				existingPVC0 := generatePVC(existingCluster, 0, tenG)
				existingPVC1 := generatePVC(existingCluster, 1, tenG)
				existingPVC2 := generatePVC(existingCluster, 2, tenG)
				initialAPIObjects = []runtime.Object{&existingSts, &existingPVC0, &existingPVC1, &existingPVC2}
			})
			It("deletes the statefulset and updates each individual PVC", func() {
				Expect(persistenceScaler.Scale(context.Background(), existingCluster, fifteenG)).To(Succeed())
				Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
					"0": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
					"1": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-1", namespace),
					"2": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace),
					"3": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
					"4": BeDeleteActionOnResource("statefulsets", "rabbit-server", namespace),
					"5": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
					"6": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
					"7": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace, MatchFields(IgnoreExtras, Fields{
						"Spec": MatchFields(IgnoreExtras, Fields{
							"Resources": MatchFields(IgnoreExtras, Fields{
								"Requests": MatchAllKeys(Keys{
									corev1.ResourceStorage: Equal(fifteenG),
								}),
							}),
						}),
					})),
					"8": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-1", namespace),
					"9": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-1", namespace, MatchFields(IgnoreExtras, Fields{
						"Spec": MatchFields(IgnoreExtras, Fields{
							"Resources": MatchFields(IgnoreExtras, Fields{
								"Requests": MatchAllKeys(Keys{
									corev1.ResourceStorage: Equal(fifteenG),
								}),
							}),
						}),
					})),
					"10": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace),
					"11": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace, MatchFields(IgnoreExtras, Fields{
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
				existingCluster.Spec.Replicas = &three
				existingPVC0 := generatePVC(existingCluster, 0, tenG)
				existingPVC2 := generatePVC(existingCluster, 2, tenG)
				initialAPIObjects = []runtime.Object{&existingSts, &existingPVC0, &existingPVC2}
			})
			It("deletes the statefulset and updates the PVCs that exist", func() {
				Expect(persistenceScaler.Scale(context.Background(), existingCluster, fifteenG)).To(Succeed())
				Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
					"0": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
					"1": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-1", namespace),
					"2": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace),
					"3": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
					"4": BeDeleteActionOnResource("statefulsets", "rabbit-server", namespace),
					"5": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
					"6": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
					"7": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace, MatchFields(IgnoreExtras, Fields{
						"Spec": MatchFields(IgnoreExtras, Fields{
							"Resources": MatchFields(IgnoreExtras, Fields{
								"Requests": MatchAllKeys(Keys{
									corev1.ResourceStorage: Equal(fifteenG),
								}),
							}),
						}),
					})),
					"8": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace),
					"9": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace, MatchFields(IgnoreExtras, Fields{
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
				existingCluster.Spec.Replicas = &three
				existingPVC0 := generatePVC(existingCluster, 0, fifteenG)
				existingPVC1 := generatePVC(existingCluster, 1, fifteenG)
				existingPVC2 := generatePVC(existingCluster, 2, tenG)
				initialAPIObjects = []runtime.Object{&existingSts, &existingPVC0, &existingPVC1, &existingPVC2}
			})
			It("deletes the statefulset and updates the PVCs that exist", func() {
				Expect(persistenceScaler.Scale(context.Background(), existingCluster, fifteenG)).To(Succeed())
				Expect(fakeClientset.Actions()).To(MatchAllElementsWithIndex(IndexIdentity, Elements{
					"0": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-0", namespace),
					"1": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-1", namespace),
					"2": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace),
					"3": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
					"4": BeDeleteActionOnResource("statefulsets", "rabbit-server", namespace),
					"5": BeGetActionOnResource("statefulsets", "rabbit-server", namespace),
					"6": BeGetActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace),
					"7": BeUpdateActionOnResource("persistentvolumeclaims", "persistence-rabbit-server-2", namespace, MatchFields(IgnoreExtras, Fields{
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
