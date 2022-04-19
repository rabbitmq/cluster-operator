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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
)

var _ = Context("Services", func() {
	var (
		instance rabbitmqv1beta1.RabbitmqCluster
		builder  resource.RabbitmqResourceBuilder
		scheme   *runtime.Scheme
	)

	Describe("Build", func() {
		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
			instance = generateRabbitmqCluster()
			builder = resource.RabbitmqResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}
		})

		It("Builds using the values from the CR", func() {
			serviceBuilder := builder.Service()
			obj, err := serviceBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			service := obj.(*corev1.Service)

			By("generates a service object with the correct name and labels", func() {
				expectedName := instance.Name
				Expect(service.Name).To(Equal(expectedName))
			})

			By("generates a service object with the correct namespace", func() {
				Expect(service.Namespace).To(Equal(instance.Namespace))
			})
		})
	})

	Describe("Update", func() {
		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
			instance = generateRabbitmqCluster()
			builder = resource.RabbitmqResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}
		})

		Context("TLS", func() {
			var (
				svc            *corev1.Service
				serviceBuilder *resource.ServiceBuilder
			)
			BeforeEach(func() {
				svc = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-service",
						Namespace: "foo-namespace",
					},
				}
				serviceBuilder = builder.Service()
				instance = generateRabbitmqCluster()
				instance.Spec.TLS = rabbitmqv1beta1.TLSSpec{
					SecretName: "tls-secret",
				}
			})
			It("opens ports for amqps, management-tls, and prometheus-tls on the service", func() {
				Expect(serviceBuilder.Update(svc)).To(Succeed())
				Expect(svc.Spec.Ports).To(ContainElements([]corev1.ServicePort{
					{
						Name:        "amqps",
						Protocol:    corev1.ProtocolTCP,
						Port:        5671,
						TargetPort:  intstr.FromInt(5671),
						AppProtocol: pointer.String("amqps"),
					},
					{
						Name:        "management-tls",
						Protocol:    corev1.ProtocolTCP,
						Port:        15671,
						TargetPort:  intstr.FromInt(15671),
						AppProtocol: pointer.String("https"),
					},
					{
						Name:        "prometheus-tls",
						Protocol:    corev1.ProtocolTCP,
						Port:        15691,
						TargetPort:  intstr.FromInt(15691),
						AppProtocol: pointer.String("prometheus.io/metric-tls"),
					},
				}))
				Expect(svc.Spec.Ports).ToNot(ContainElement(corev1.ServicePort{
					Name:        "prometheus",
					Protocol:    corev1.ProtocolTCP,
					Port:        15692,
					TargetPort:  intstr.FromInt(15692),
					AppProtocol: pointer.String("prometheus.io/metric"),
				},
				))
			})

			When("mqtt, stomp and stream are enabled", func() {
				It("opens ports for those plugins", func() {
					instance.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{"rabbitmq_mqtt", "rabbitmq_stomp", "rabbitmq_stream"}
					Expect(serviceBuilder.Update(svc)).To(Succeed())
					Expect(svc.Spec.Ports).To(ContainElements([]corev1.ServicePort{
						{
							Name:        "mqtts",
							Protocol:    corev1.ProtocolTCP,
							Port:        8883,
							TargetPort:  intstr.FromInt(8883),
							AppProtocol: pointer.String("mqtts"),
						},
						{
							Name:        "stomps",
							Protocol:    corev1.ProtocolTCP,
							Port:        61614,
							TargetPort:  intstr.FromInt(61614),
							AppProtocol: pointer.String("stomp.github.io/stomp-tls"),
						},
						{
							Name:        "streams",
							Protocol:    corev1.ProtocolTCP,
							Port:        5551,
							TargetPort:  intstr.FromInt(5551),
							AppProtocol: pointer.String("rabbitmq.com/stream-tls"),
						},
					}))
				})
			})

			When("rabbitmq_web_mqtt and rabbitmq_web_stomp are enabled", func() {
				It("opens ports for those plugins", func() {
					instance.Spec.TLS.CaSecretName = "caname"
					instance.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{"rabbitmq_web_mqtt", "rabbitmq_web_stomp"}
					Expect(serviceBuilder.Update(svc)).To(Succeed())
					Expect(svc.Spec.Ports).To(ContainElements([]corev1.ServicePort{
						{
							Name:        "web-mqtt-tls",
							Protocol:    corev1.ProtocolTCP,
							Port:        15676,
							TargetPort:  intstr.FromInt(15676),
							AppProtocol: pointer.String("https"),
						},
						{
							Name:        "web-stomp-tls",
							Protocol:    corev1.ProtocolTCP,
							Port:        15673,
							TargetPort:  intstr.FromInt(15673),
							AppProtocol: pointer.String("https"),
						},
					}))
				})
			})

			When("rabbitmq_multi_dc_replication is enabled", func() {
				It("opens port for streams", func() {
					instance.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{"rabbitmq_multi_dc_replication"}
					Expect(serviceBuilder.Update(svc)).To(Succeed())
					Expect(svc.Spec.Ports).To(ContainElements([]corev1.ServicePort{
						{
							Name:        "streams",
							Protocol:    corev1.ProtocolTCP,
							Port:        5551,
							TargetPort:  intstr.FromInt(5551),
							AppProtocol: pointer.String("rabbitmq.com/stream-tls"),
						},
					}))
				})
			})

			When("DisableNonTLSListeners is set to true", func() {
				It("only exposes tls ports in the service", func() {
					instance.Spec.TLS.DisableNonTLSListeners = true
					Expect(serviceBuilder.Update(svc)).To(Succeed())
					Expect(svc.Spec.Ports).To(ConsistOf([]corev1.ServicePort{
						{
							Name:        "amqps",
							Protocol:    corev1.ProtocolTCP,
							Port:        5671,
							TargetPort:  intstr.FromInt(5671),
							AppProtocol: pointer.String("amqps"),
						},
						{
							Name:        "management-tls",
							Protocol:    corev1.ProtocolTCP,
							Port:        15671,
							TargetPort:  intstr.FromInt(15671),
							AppProtocol: pointer.String("https"),
						},
						{
							Name:        "prometheus-tls",
							Protocol:    corev1.ProtocolTCP,
							Port:        15691,
							TargetPort:  intstr.FromInt(15691),
							AppProtocol: pointer.String("prometheus.io/metric-tls"),
						},
					}))
				})
				DescribeTable("only exposes tls ports in the service for enabled plugins",
					func(plugin, servicePortName string, port int, appProtocol *string) {
						instance.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{rabbitmqv1beta1.Plugin(plugin)}
						instance.Spec.TLS.DisableNonTLSListeners = true
						instance.Spec.TLS.CaSecretName = "somecacertname"
						Expect(serviceBuilder.Update(svc)).To(Succeed())
						amqpsPort := corev1.ServicePort{

							Name:        "amqps",
							Protocol:    corev1.ProtocolTCP,
							Port:        5671,
							TargetPort:  intstr.FromInt(5671),
							AppProtocol: pointer.String("amqps"),
						}
						managementTLSPort := corev1.ServicePort{

							Name:        "management-tls",
							Protocol:    corev1.ProtocolTCP,
							Port:        15671,
							TargetPort:  intstr.FromInt(15671),
							AppProtocol: pointer.String("https"),
						}
						prometheusTLSPort := corev1.ServicePort{

							Name:        "prometheus-tls",
							Protocol:    corev1.ProtocolTCP,
							Port:        15691,
							TargetPort:  intstr.FromInt(15691),
							AppProtocol: pointer.String("prometheus.io/metric-tls"),
						}
						expectedPort := corev1.ServicePort{
							Name:        servicePortName,
							Port:        int32(port),
							TargetPort:  intstr.FromInt(port),
							Protocol:    corev1.ProtocolTCP,
							AppProtocol: appProtocol,
						}
						Expect(svc.Spec.Ports).To(ConsistOf(amqpsPort, managementTLSPort, prometheusTLSPort, expectedPort))
					},
					Entry("MQTT", "rabbitmq_mqtt", "mqtts", 8883, pointer.String("mqtts")),
					Entry("MQTT-over-WebSockets", "rabbitmq_web_mqtt", "web-mqtt-tls", 15676, pointer.String("https")),
					Entry("STOMP", "rabbitmq_stomp", "stomps", 61614, pointer.String("stomp.github.io/stomp-tls")),
					Entry("STOMP-over-WebSockets", "rabbitmq_web_stomp", "web-stomp-tls", 15673, pointer.String("https")),
					Entry("Stream", "rabbitmq_stream", "streams", 5551, pointer.String("rabbitmq.com/stream-tls")),
					Entry("OSR", "rabbitmq_multi_dc_replication", "streams", 5551, pointer.String("rabbitmq.com/stream-tls")),
				)
			})

			When("MQTT and Web-MQTT are enabled", func() {
				It("exposes ports for both protocols", func() {
					instance.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{"rabbitmq_mqtt", "rabbitmq_web_mqtt"}
					instance.Spec.TLS.CaSecretName = "my-ca"
					Expect(serviceBuilder.Update(svc)).To(Succeed())
					Expect(svc.Spec.Ports).To(ContainElements([]corev1.ServicePort{
						{
							Name:        "web-mqtt-tls",
							Protocol:    corev1.ProtocolTCP,
							AppProtocol: pointer.String("https"),
							Port:        15676,
							TargetPort:  intstr.FromInt(15676),
						},
						{
							Name:        "mqtts",
							Protocol:    corev1.ProtocolTCP,
							AppProtocol: pointer.String("mqtts"),
							Port:        8883,
							TargetPort:  intstr.FromInt(8883),
						},
					}))
				})
			})

			When("STOMP and Web-STOMP are enabled", func() {
				It("exposes ports for both protocols", func() {
					instance.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{"rabbitmq_stomp", "rabbitmq_web_stomp"}
					instance.Spec.TLS.CaSecretName = "my-ca"
					Expect(serviceBuilder.Update(svc)).To(Succeed())
					Expect(svc.Spec.Ports).To(ContainElements([]corev1.ServicePort{
						{
							Name:        "web-stomp-tls",
							Protocol:    corev1.ProtocolTCP,
							AppProtocol: pointer.String("https"),
							Port:        15673,
							TargetPort:  intstr.FromInt(15673),
						},
						{
							Name:        "stomps",
							Protocol:    corev1.ProtocolTCP,
							AppProtocol: pointer.String("stomp.github.io/stomp-tls"),
							Port:        61614,
							TargetPort:  intstr.FromInt(61614),
						},
					}))
				})
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

					service := updateServiceWithAnnotations(builder, nil, serviceAnno)
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
					service := updateServiceWithAnnotations(builder, instanceAnnotations, serviceAnnotations)
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
					service := updateServiceWithAnnotations(builder, instanceMetadataAnnotations, serviceAnnotations)
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

					service := updateServiceWithAnnotations(builder, instanceAnnotations, serviceAnnotations)

					Expect(service.ObjectMeta.Annotations).To(Equal(expectedAnnotations))
				})
			})
		})

		Context("Labels", func() {
			var (
				serviceBuilder *resource.ServiceBuilder
				svc            *corev1.Service
			)
			BeforeEach(func() {
				serviceBuilder = builder.Service()
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
				serviceBuilder *resource.ServiceBuilder
			)

			BeforeEach(func() {
				serviceBuilder = builder.Service()
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

			It("sets the owner reference", func() {
				err := serviceBuilder.Update(svc)
				Expect(err).NotTo(HaveOccurred())

				Expect(svc.ObjectMeta.OwnerReferences[0].Name).To(Equal("foo"))
			})

			It("exposes the required ports", func() {
				err := serviceBuilder.Update(svc)
				Expect(err).NotTo(HaveOccurred())

				amqpPort := corev1.ServicePort{
					Name:        "amqp",
					Port:        5672,
					TargetPort:  intstr.FromInt(5672),
					Protocol:    corev1.ProtocolTCP,
					AppProtocol: pointer.String("amqp"),
				}
				managementPort := corev1.ServicePort{
					Name:        "management",
					Port:        15672,
					TargetPort:  intstr.FromInt(15672),
					Protocol:    corev1.ProtocolTCP,
					AppProtocol: pointer.String("http"),
				}
				prometheusPort := corev1.ServicePort{
					Name:        "prometheus",
					Port:        15692,
					TargetPort:  intstr.FromInt(15692),
					Protocol:    corev1.ProtocolTCP,
					AppProtocol: pointer.String("prometheus.io/metrics"),
				}
				Expect(svc.Spec.Ports).To(ConsistOf(amqpPort, managementPort, prometheusPort))
			})

			DescribeTable("plugins exposing ports",
				func(plugin, servicePortName string, port int, appProtocol *string) {
					instance.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{rabbitmqv1beta1.Plugin(plugin)}
					Expect(serviceBuilder.Update(svc)).To(Succeed())

					expectedPort := corev1.ServicePort{
						Name:        servicePortName,
						Port:        int32(port),
						TargetPort:  intstr.FromInt(port),
						Protocol:    corev1.ProtocolTCP,
						AppProtocol: appProtocol,
					}
					Expect(svc.Spec.Ports).To(ContainElement(expectedPort))
				},
				Entry("MQTT", "rabbitmq_mqtt", "mqtt", 1883, pointer.String("mqtt")),
				Entry("MQTT-over-WebSockets", "rabbitmq_web_mqtt", "web-mqtt", 15675, pointer.String("http")),
				Entry("STOMP", "rabbitmq_stomp", "stomp", 61613, pointer.String("stomp.github.io/stomp")),
				Entry("STOMP-over-WebSockets", "rabbitmq_web_stomp", "web-stomp", 15674, pointer.String("http")),
				Entry("Stream", "rabbitmq_stream", "stream", 5552, pointer.String("rabbitmq.com/stream")),
				Entry("OSR", "rabbitmq_multi_dc_replication", "stream", 5552, pointer.String("rabbitmq.com/stream")),
			)

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
					{
						Protocol:    corev1.ProtocolTCP,
						Port:        5672,
						TargetPort:  intstr.FromInt(5672),
						Name:        "amqp",
						NodePort:    12345,
						AppProtocol: pointer.String("amqp"),
					},
					{
						Protocol:    corev1.ProtocolTCP,
						Port:        15672,
						Name:        "management",
						TargetPort:  intstr.FromInt(15672),
						NodePort:    1234,
						AppProtocol: nil, // explicitly leaving this nill to test that Update sets the correct value
					},
				}

				serviceBuilder.Instance.Spec.Service.Type = "NodePort"
				err := serviceBuilder.Update(svc)
				Expect(err).NotTo(HaveOccurred())

				expectedAmqpServicePort := corev1.ServicePort{
					Name:        "amqp",
					Protocol:    corev1.ProtocolTCP,
					Port:        5672,
					TargetPort:  intstr.FromInt(5672),
					NodePort:    12345,
					AppProtocol: pointer.String("amqp"),
				}
				expectedManagementServicePort := corev1.ServicePort{
					Protocol:    corev1.ProtocolTCP,
					Port:        15672,
					Name:        "management",
					TargetPort:  intstr.FromInt(15672),
					NodePort:    1234,
					AppProtocol: pointer.String("http"),
				}

				Expect(svc.Spec.Ports).To(ContainElement(expectedAmqpServicePort))
				Expect(svc.Spec.Ports).To(ContainElement(expectedManagementServicePort))
			})

			When("service type is updated from NodePort to ClusterIP", func() {
				It("unsets nodePort field", func() {
					svc.Spec.Type = corev1.ServiceTypeNodePort
					svc.Spec.Ports = []corev1.ServicePort{
						{
							Protocol:   corev1.ProtocolTCP,
							Port:       5672,
							TargetPort: intstr.FromInt(5672),
							Name:       "amqp",
							NodePort:   12345,
						},
					}

					serviceBuilder.Instance.Spec.Service.Type = "ClusterIP"
					err := serviceBuilder.Update(svc)
					Expect(err).NotTo(HaveOccurred())

					// We cant set nodePort to nil because its a primitive
					// For Kubernetes API, setting it to 0 is the same as not setting it at all
					expectedServicePort := corev1.ServicePort{
						Name:        "amqp",
						Protocol:    corev1.ProtocolTCP,
						Port:        5672,
						TargetPort:  intstr.FromInt(5672),
						NodePort:    0,
						AppProtocol: pointer.String("amqp"),
					}

					Expect(svc.Spec.Ports).To(ContainElement(expectedServicePort))
					Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
				})
			})

			When("service type is deleted from the spec", func() {
				It("unsets the service type and node ports", func() {
					svc.Spec.Type = corev1.ServiceTypeNodePort
					svc.Spec.Ports = []corev1.ServicePort{
						{
							Protocol:   corev1.ProtocolTCP,
							Port:       5672,
							TargetPort: intstr.FromInt(5672),
							Name:       "amqp",
							NodePort:   12345,
						},
					}

					serviceBuilder.Instance.Spec.Service.Type = ""
					err := serviceBuilder.Update(svc)
					Expect(err).NotTo(HaveOccurred())

					expectedServicePort := corev1.ServicePort{
						Name:        "amqp",
						Protocol:    corev1.ProtocolTCP,
						Port:        5672,
						TargetPort:  intstr.FromInt(5672),
						NodePort:    0,
						AppProtocol: pointer.String("amqp"),
					}

					Expect(svc.Spec.Ports).To(ContainElement(expectedServicePort))
					Expect(svc.Spec.Type).To(BeEmpty())
				})
			})
		})

		When("Override is provided", func() {
			var (
				svc            *corev1.Service
				serviceBuilder *resource.ServiceBuilder
			)

			BeforeEach(func() {
				serviceBuilder = builder.Service()
				instance = generateRabbitmqCluster()

				svc = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "foo-namespace",
					},
				}
			})

			It("overrides Service.ObjectMeta", func() {
				instance.Spec.Override.Service = &rabbitmqv1beta1.Service{
					EmbeddedLabelsAnnotations: &rabbitmqv1beta1.EmbeddedLabelsAnnotations{
						Labels: map[string]string{
							"new-label-key": "new-label-value",
						},
						Annotations: map[string]string{
							"new-key": "new-value",
						},
					},
				}

				err := serviceBuilder.Update(svc)
				Expect(err).NotTo(HaveOccurred())
				Expect(svc.ObjectMeta.Annotations).To(Equal(map[string]string{"new-key": "new-value"}))
				Expect(svc.ObjectMeta.Labels).To(Equal(map[string]string{
					"app.kubernetes.io/name":      instance.Name,
					"app.kubernetes.io/component": "rabbitmq",
					"app.kubernetes.io/part-of":   "rabbitmq",
					"new-label-key":               "new-label-value",
				}))
			})

			It("overrides ServiceSpec", func() {
				var IPv4 corev1.IPFamily = "IPv4"
				ten := int32(10)
				instance.Spec.Override.Service = &rabbitmqv1beta1.Service{
					Spec: &corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Protocol:    corev1.ProtocolUDP,
								Port:        12345,
								TargetPort:  intstr.FromInt(12345),
								Name:        "my-new-port",
								AppProtocol: pointer.String("my.company.com/protocol"),
							},
						},
						Selector: map[string]string{
							"a-selector": "a-label",
						},
						Type:                     "NodePort",
						SessionAffinity:          "ClientIP",
						LoadBalancerSourceRanges: []string{"1000", "30000"},
						ExternalName:             "my-external-name",
						ExternalTrafficPolicy:    corev1.ServiceExternalTrafficPolicyTypeLocal,
						HealthCheckNodePort:      1234,
						PublishNotReadyAddresses: false,
						SessionAffinityConfig: &corev1.SessionAffinityConfig{
							ClientIP: &corev1.ClientIPConfig{
								TimeoutSeconds: &ten,
							},
						},
						IPFamilies: []corev1.IPFamily{IPv4},
					},
				}

				err := serviceBuilder.Update(svc)
				Expect(err).NotTo(HaveOccurred())
				Expect(svc.Spec.Ports).To(ConsistOf(
					corev1.ServicePort{
						Name:        "amqp",
						Port:        5672,
						TargetPort:  intstr.FromInt(5672),
						Protocol:    corev1.ProtocolTCP,
						AppProtocol: pointer.String("amqp"),
					},
					corev1.ServicePort{
						Name:        "management",
						Port:        15672,
						TargetPort:  intstr.FromInt(15672),
						Protocol:    corev1.ProtocolTCP,
						AppProtocol: pointer.String("http"),
					},
					corev1.ServicePort{
						Name:        "prometheus",
						Port:        15692,
						TargetPort:  intstr.FromInt(15692),
						Protocol:    corev1.ProtocolTCP,
						AppProtocol: pointer.String("prometheus.io/metrics"),
					},
					corev1.ServicePort{
						Protocol:    corev1.ProtocolUDP,
						Port:        12345,
						TargetPort:  intstr.FromInt(12345),
						Name:        "my-new-port",
						AppProtocol: pointer.String("my.company.com/protocol"),
					},
				))
				Expect(svc.Spec.Selector).To(Equal(map[string]string{"a-selector": "a-label", "app.kubernetes.io/name": "foo"}))
				Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))
				Expect(svc.Spec.SessionAffinity).To(Equal(corev1.ServiceAffinityClientIP))
				Expect(svc.Spec.LoadBalancerSourceRanges).To(Equal([]string{"1000", "30000"}))
				Expect(svc.Spec.ExternalName).To(Equal("my-external-name"))
				Expect(svc.Spec.ExternalTrafficPolicy).To(Equal(corev1.ServiceExternalTrafficPolicyTypeLocal))
				Expect(svc.Spec.HealthCheckNodePort).To(Equal(int32(1234)))
				Expect(svc.Spec.PublishNotReadyAddresses).To(BeFalse())
				Expect(*svc.Spec.SessionAffinityConfig.ClientIP.TimeoutSeconds).To(Equal(int32(10)))
				Expect(svc.Spec.IPFamilies).To(ConsistOf(corev1.IPv4Protocol))
			})

			It("ensures override takes precedence when same property is set both at the top level and at the override level", func() {
				instance.Spec.Service.Type = "LoadBalancer"
				instance.Spec.Override.Service = &rabbitmqv1beta1.Service{
					Spec: &corev1.ServiceSpec{
						Type: corev1.ServiceTypeNodePort,
					},
				}

				err := serviceBuilder.Update(svc)
				Expect(err).NotTo(HaveOccurred())
				Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))
			})
		})
	})

	Context("UpdateMayRequireStsRecreate", func() {
		BeforeEach(func() {
			BeforeEach(func() {
				scheme = runtime.NewScheme()
				Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
				Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
				instance = generateRabbitmqCluster()
				builder = resource.RabbitmqResourceBuilder{
					Instance: &instance,
					Scheme:   scheme,
				}

			})
			It("returns false", func() {
				serviceBuilder := builder.Service()
				Expect(serviceBuilder.UpdateMayRequireStsRecreate()).To(BeFalse())
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
	serviceBuilder := rmqBuilder.Service()
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
