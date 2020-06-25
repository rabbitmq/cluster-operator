// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
)

var _ = Context("ClientServices", func() {
	var (
		instance   rabbitmqv1beta1.RabbitmqCluster
		rmqBuilder resource.RabbitmqResourceBuilder
		scheme     *runtime.Scheme
	)

	Context("Build", func() {
		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
			instance = generateRabbitmqCluster()
			rmqBuilder = resource.RabbitmqResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}
		})

		It("Builds using the values from the CR", func() {
			serviceBuilder := rmqBuilder.ClientService()
			obj, err := serviceBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			service := obj.(*corev1.Service)

			By("generates a service object with the correct name and labels", func() {
				expectedName := instance.ChildResourceName("client")
				Expect(service.Name).To(Equal(expectedName))
			})

			By("generates a service object with the correct namespace", func() {
				Expect(service.Namespace).To(Equal(instance.Namespace))
			})
		})
	})

	Context("Update", func() {
		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
			instance = generateRabbitmqCluster()
			rmqBuilder = resource.RabbitmqResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}
		})

		Context("TLS", func() {
			It("opens port 5671 on the service", func() {
				instance := &rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: v1.ObjectMeta{
						Name:      "foo",
						Namespace: "foo-namespace",
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						TLS: rabbitmqv1beta1.TLSSpec{
							SecretName: "tls-secret",
						},
					},
				}
				rmqBuilder.Instance = instance
				serviceBuilder := rmqBuilder.ClientService()
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-service",
						Namespace: "foo-namespace",
					},
				}

				Expect(serviceBuilder.Update(svc)).To(Succeed())
				Expect(svc.Spec.Ports).Should(ContainElement(corev1.ServicePort{
					Name:     "amqps",
					Protocol: "TCP",
					Port:     5671,
				}))
			})
		})

		Context("Annotations", func() {
			When("CR instance does have service annotations specified", func() {
				It("generates a service object with the annotations as specified", func() {
					serviceAnno := map[string]string{
						"service_annotation_a":        "0.0.0.0/0",
						"kubernetes.io/other":         "i-like-this",
						"kubectl.kubernetes.io/other": "i-like-this",
						"k8s.io/other":                "i-like-this",
					}
					expectedAnnotations := map[string]string{
						"service_annotation_a":             "0.0.0.0/0",
						"app.kubernetes.io/part-of":        "rabbitmq",
						"app.k8s.io/something":             "something-amazing",
						"this-was-the-previous-annotation": "should-be-preserved",
						"kubernetes.io/other":              "i-like-this",
						"kubectl.kubernetes.io/other":      "i-like-this",
						"k8s.io/other":                     "i-like-this",
					}

					service := updateServiceWithAnnotations(rmqBuilder, nil, serviceAnno)
					Expect(service.ObjectMeta.Annotations).To(Equal(expectedAnnotations))
				})
			})

			When("CR instance does not have service annotations specified", func() {
				It("generates the service annotations as specified", func() {
					expectedAnnotations := map[string]string{
						"app.kubernetes.io/part-of":        "rabbitmq",
						"app.k8s.io/something":             "something-amazing",
						"this-was-the-previous-annotation": "should-be-preserved",
					}

					var serviceAnnotations map[string]string = nil
					var instanceAnnotations map[string]string = nil
					service := updateServiceWithAnnotations(rmqBuilder, instanceAnnotations, serviceAnnotations)
					Expect(service.ObjectMeta.Annotations).To(Equal(expectedAnnotations))
				})
			})

			When("CR instance does not have service annotations specified, but does have metadata annotations specified", func() {
				It("sets the instance annotations on the service", func() {
					instanceMetadataAnnotations := map[string]string{
						"kubernetes.io/name":         "i-do-not-like-this",
						"kubectl.kubernetes.io/name": "i-do-not-like-this",
						"k8s.io/name":                "i-do-not-like-this",
						"my-annotation":              "i-like-this",
					}

					var serviceAnnotations map[string]string = nil
					service := updateServiceWithAnnotations(rmqBuilder, instanceMetadataAnnotations, serviceAnnotations)
					expectedAnnotations := map[string]string{
						"my-annotation":                    "i-like-this",
						"app.kubernetes.io/part-of":        "rabbitmq",
						"app.k8s.io/something":             "something-amazing",
						"this-was-the-previous-annotation": "should-be-preserved",
					}

					Expect(service.Annotations).To(Equal(expectedAnnotations))
				})
			})

			When("CR instance has service annotations specified, and has metadata annotations specified", func() {
				It("merges the annotations", func() {
					serviceAnnotations := map[string]string{
						"kubernetes.io/other":         "i-like-this",
						"kubectl.kubernetes.io/other": "i-like-this",
						"k8s.io/other":                "i-like-this",
						"service_annotation_a":        "0.0.0.0/0",
						"my-annotation":               "i-like-this-more",
					}
					instanceAnnotations := map[string]string{
						"kubernetes.io/name":         "i-do-not-like-this",
						"kubectl.kubernetes.io/name": "i-do-not-like-this",
						"k8s.io/name":                "i-do-not-like-this",
						"my-annotation":              "i-like-this",
						"my-second-annotation":       "i-like-this-also",
					}

					expectedAnnotations := map[string]string{
						"kubernetes.io/other":              "i-like-this",
						"kubectl.kubernetes.io/other":      "i-like-this",
						"k8s.io/other":                     "i-like-this",
						"my-annotation":                    "i-like-this-more",
						"my-second-annotation":             "i-like-this-also",
						"service_annotation_a":             "0.0.0.0/0",
						"app.kubernetes.io/part-of":        "rabbitmq",
						"app.k8s.io/something":             "something-amazing",
						"this-was-the-previous-annotation": "should-be-preserved",
					}

					service := updateServiceWithAnnotations(rmqBuilder, instanceAnnotations, serviceAnnotations)

					Expect(service.ObjectMeta.Annotations).To(Equal(expectedAnnotations))
				})
			})
		})

		Context("Labels", func() {
			var (
				serviceBuilder *resource.ClientServiceBuilder
				svc            *corev1.Service
			)
			BeforeEach(func() {
				serviceBuilder = rmqBuilder.ClientService()
				instance = rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rabbit-labelled",
					},
				}
				instance.Labels = map[string]string{
					"app.kubernetes.io/foo": "bar",
					"foo":                   "bar",
					"rabbitmq":              "is-great",
					"foo/app.kubernetes.io": "edgecase",
				}

				svc = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app.kubernetes.io/name":      instance.Name,
							"app.kubernetes.io/part-of":   "rabbitmq",
							"this-was-the-previous-label": "should-be-deleted",
						},
					},
				}
				err := serviceBuilder.Update(svc)
				Expect(err).NotTo(HaveOccurred())
			})

			It("adds labels from the CR", func() {
				testLabels(svc.Labels)
			})

			It("restores the default labels", func() {
				labels := svc.Labels
				Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
				Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
				Expect(labels["app.kubernetes.io/part-of"]).To(Equal("rabbitmq"))
			})

			It("deletes the labels that are removed from the CR", func() {
				Expect(svc.Labels).NotTo(HaveKey("this-was-the-previous-label"))
			})
		})

		Context("Service Type", func() {
			var (
				svc            *corev1.Service
				serviceBuilder *resource.ClientServiceBuilder
			)

			BeforeEach(func() {
				serviceBuilder = rmqBuilder.ClientService()
				instance = generateRabbitmqCluster()

				svc = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rabbit-service-type-update-service",
						Namespace: "foo-namespace",
					},
				}
			})

			It("sets the service type to the value specified in the CR instance by default", func() {
				err := serviceBuilder.Update(svc)
				Expect(err).NotTo(HaveOccurred())

				expectedServiceType := "this-is-a-service"
				Expect(string(svc.Spec.Type)).To(Equal(expectedServiceType))
			})

			It("adds a label selector with the instance name", func() {
				err := serviceBuilder.Update(svc)
				Expect(err).NotTo(HaveOccurred())

				Expect(svc.Spec.Selector["app.kubernetes.io/name"]).To(Equal(instance.Name))
			})

			It("sets the onwer reference", func() {
				err := serviceBuilder.Update(svc)
				Expect(err).NotTo(HaveOccurred())

				Expect(svc.ObjectMeta.OwnerReferences[0].Name).To(Equal("foo"))
			})

			It("exposes the required ports", func() {
				err := serviceBuilder.Update(svc)
				Expect(err).NotTo(HaveOccurred())

				amqpPort := corev1.ServicePort{
					Name:     "amqp",
					Port:     5672,
					Protocol: corev1.ProtocolTCP,
				}
				managementPort := corev1.ServicePort{
					Name:     "management",
					Port:     15672,
					Protocol: corev1.ProtocolTCP,
				}
				Expect(svc.Spec.Ports).Should(ConsistOf(amqpPort, managementPort))
			})

			It("updates the service type from ClusterIP to NodePort", func() {
				svc.Spec.Type = corev1.ServiceTypeClusterIP
				serviceBuilder.Instance.Spec.Service.Type = "NodePort"
				err := serviceBuilder.Update(svc)
				Expect(err).NotTo(HaveOccurred())

				expectedServiceType := "NodePort"
				Expect(string(svc.Spec.Type)).To(Equal(expectedServiceType))
			})

			It("preserves the same node ports after updating from LoadBalancer to NodePort", func() {
				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				svc.Spec.Ports = []corev1.ServicePort{
					corev1.ServicePort{
						Protocol: corev1.ProtocolTCP,
						Port:     5672,
						Name:     "amqp",
						NodePort: 12345,
					},
					corev1.ServicePort{
						Protocol: corev1.ProtocolTCP,
						Port:     15672,
						Name:     "management",
						NodePort: 1234,
					},
				}

				serviceBuilder.Instance.Spec.Service.Type = "NodePort"
				err := serviceBuilder.Update(svc)
				Expect(err).NotTo(HaveOccurred())

				expectedAmqpServicePort := corev1.ServicePort{
					Name:     "amqp",
					Protocol: corev1.ProtocolTCP,
					Port:     5672,
					NodePort: 12345,
				}
				expectedManagementServicePort := corev1.ServicePort{
					Protocol: corev1.ProtocolTCP,
					Port:     15672,
					Name:     "management",
					NodePort: 1234,
				}

				Expect(svc.Spec.Ports).To(ContainElement(expectedAmqpServicePort))
				Expect(svc.Spec.Ports).To(ContainElement(expectedManagementServicePort))
			})

			It("unsets nodePort after updating from NodePort to ClusterIP", func() {
				svc.Spec.Type = corev1.ServiceTypeNodePort
				svc.Spec.Ports = []corev1.ServicePort{
					corev1.ServicePort{
						Protocol: corev1.ProtocolTCP,
						Port:     5672,
						Name:     "amqp",
						NodePort: 12345,
					},
				}

				serviceBuilder.Instance.Spec.Service.Type = "ClusterIP"
				err := serviceBuilder.Update(svc)
				Expect(err).NotTo(HaveOccurred())

				// We cant set nodePort to nil because its a primitive
				// For Kubernetes API, setting it to 0 is the same as not setting it at all
				expectedServicePort := corev1.ServicePort{
					Name:     "amqp",
					Protocol: corev1.ProtocolTCP,
					Port:     5672,
					NodePort: 0,
				}

				Expect(svc.Spec.Ports).To(ContainElement(expectedServicePort))
			})

			It("unsets the service type and node ports when service type is deleted from CR spec", func() {
				svc.Spec.Type = corev1.ServiceTypeNodePort
				svc.Spec.Ports = []corev1.ServicePort{
					corev1.ServicePort{
						Protocol: corev1.ProtocolTCP,
						Port:     5672,
						Name:     "amqp",
						NodePort: 12345,
					},
				}

				serviceBuilder.Instance.Spec.Service.Type = ""
				err := serviceBuilder.Update(svc)
				Expect(err).NotTo(HaveOccurred())

				expectedServicePort := corev1.ServicePort{
					Name:     "amqp",
					Protocol: corev1.ProtocolTCP,
					Port:     5672,
					NodePort: 0,
				}

				Expect(svc.Spec.Ports).To(ContainElement(expectedServicePort))
			})
		})
	})
})

func updateServiceWithAnnotations(rmqBuilder resource.RabbitmqResourceBuilder, instanceAnnotations, serviceAnnotations map[string]string) *corev1.Service {
	instance := &rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: v1.ObjectMeta{
			Name:        "foo",
			Namespace:   "foo-namespace",
			Annotations: instanceAnnotations,
		},
		Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
			Service: rabbitmqv1beta1.RabbitmqClusterServiceSpec{
				Annotations: serviceAnnotations,
			},
		},
	}

	rmqBuilder.Instance = instance
	serviceBuilder := rmqBuilder.ClientService()
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-service",
			Namespace: "foo-namespace",
			Annotations: map[string]string{
				"this-was-the-previous-annotation": "should-be-preserved",
				"app.kubernetes.io/part-of":        "rabbitmq",
				"app.k8s.io/something":             "something-amazing",
			},
		},
	}
	Expect(serviceBuilder.Update(svc)).To(Succeed())
	return svc
}
