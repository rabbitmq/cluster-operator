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
	b64 "encoding/base64"

	"gopkg.in/ini.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("DefaultUserSecret", func() {
	var (
		secret                   *corev1.Secret
		instance                 rabbitmqv1beta1.RabbitmqCluster
		builder                  *resource.RabbitmqResourceBuilder
		defaultUserSecretBuilder *resource.DefaultUserSecretBuilder
		scheme                   *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
		instance = rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: v1.ObjectMeta{
				Name:      "a name",
				Namespace: "a namespace",
			},
		}
		builder = &resource.RabbitmqResourceBuilder{
			Instance: &instance,
			Scheme:   scheme,
		}
		defaultUserSecretBuilder = builder.DefaultUserSecret()
	})

	Context("Build with defaults", func() {
		It("creates the necessary default-user secret", func() {
			var username []byte
			var password []byte
			var host []byte
			var port []byte
			var ok bool

			obj, err := defaultUserSecretBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			secret = obj.(*corev1.Secret)

			By("creating the secret with correct name and namespace", func() {
				Expect(secret.Name).To(Equal(instance.ChildResourceName("default-user")))
				Expect(secret.Namespace).To(Equal("a namespace"))
			})

			By("creating a 'opaque' secret ", func() {
				Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
			})

			By("creating a rabbitmq username that is base64 encoded and 24 characters in length", func() {
				username, ok = secret.Data["username"]
				Expect(ok).NotTo(BeFalse(), "Failed to find a key \"username\" in the generated Secret")
				decodedUsername, err := b64.URLEncoding.DecodeString(string(username))
				Expect(err).NotTo(HaveOccurred())
				Expect(len(decodedUsername)).To(Equal(24))
			})

			By("creating a rabbitmq password that is base64 encoded and 24 characters in length", func() {
				password, ok = secret.Data["password"]
				Expect(ok).NotTo(BeFalse(), "Failed to find a key \"password\" in the generated Secret")
				decodedPassword, err := b64.URLEncoding.DecodeString(string(password))
				Expect(err).NotTo(HaveOccurred())
				Expect(len(decodedPassword)).To(Equal(24))
			})

			By("Setting a host that corresponds to the service address", func() {
				host, ok = secret.Data["host"]
				Expect(ok).NotTo(BeFalse(), "Failed to find a key \"host\" in the generated Secret")
				expectedHost := "a name.a namespace.svc.cluster.local"
				Expect(host).To(BeEquivalentTo(expectedHost))
			})

			By("Setting a port that corresponds to the amqp port", func() {
				port, ok = secret.Data["port"]
				Expect(ok).NotTo(BeFalse(), "Failed to find a key \"port\" in the generated Secret")
				Expect(port).To(BeEquivalentTo("5672"))
			})

			By("creating a default_user.conf file that contains the correct sysctl config format to be parsed by RabbitMQ", func() {
				defaultUserConf, ok := secret.Data["default_user.conf"]
				Expect(ok).NotTo(BeFalse(), "Failed to find a key \"default_user.conf\" in the generated Secret")

				cfg, err := ini.Load(defaultUserConf)
				Expect(err).NotTo(HaveOccurred())

				Expect(cfg.Section("").HasKey("default_user")).To(BeTrue())
				Expect(cfg.Section("").HasKey("default_pass")).To(BeTrue())

				Expect(cfg.Section("").Key("default_user").Value()).To(Equal(string(username)))
				Expect(cfg.Section("").Key("default_pass").Value()).To(Equal(string(password)))
			})

			By("setting 'data.provider' to 'rabbitmq' ", func() {
				provider, ok := secret.Data["provider"]
				Expect(ok).NotTo(BeFalse(), "Failed to find key 'provider' ")
				Expect(string(provider)).To(Equal("rabbitmq"))
			})

			By("setting 'data.type' to 'rabbitmq' ", func() {
				t, ok := secret.Data["type"]
				Expect(ok).NotTo(BeFalse(), "Failed to find key 'type' ")
				Expect(string(t)).To(Equal("rabbitmq"))
			})
		})
	})

	Context("when MQTT, STOMP, streams, WebMQTT, and WebSTOMP are enabled", func() {
		It("adds the MQTT, STOMP, stream, WebMQTT, and WebSTOMP ports to the user secret", func() {
			var port []byte

			instance.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{
				"rabbitmq_mqtt",
				"rabbitmq_stomp",
				"rabbitmq_stream",
				"rabbitmq_web_mqtt",
				"rabbitmq_web_stomp",
			}

			obj, err := defaultUserSecretBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			secret = obj.(*corev1.Secret)

			port, ok := secret.Data["mqtt-port"]
			Expect(ok).NotTo(BeFalse(), "Failed to find key \"mqtt-port\" in the generated Secret")
			Expect(port).To(BeEquivalentTo("1883"))

			port, ok = secret.Data["stomp-port"]
			Expect(ok).NotTo(BeFalse(), "Failed to find key \"stomp-port\" in the generated Secret")
			Expect(port).To(BeEquivalentTo("61613"))

			port, ok = secret.Data["stream-port"]
			Expect(ok).NotTo(BeFalse(), "Failed to find key \"stream-port\" in the generated Secret")
			Expect(port).To(BeEquivalentTo("5552"))

			port, ok = secret.Data["web-mqtt-port"]
			Expect(ok).NotTo(BeFalse(), "Failed to find key \"web-mqtt-port\" in the generated Secret")
			Expect(port).To(BeEquivalentTo("15675"))

			port, ok = secret.Data["web-stomp-port"]
			Expect(ok).NotTo(BeFalse(), "Failed to find key \"web-stomp-port\" in the generated Secret")
			Expect(port).To(BeEquivalentTo("15674"))
		})
	})

	Context("when TLS is enabled", func() {
		It("Uses the AMQPS port in the user secret", func() {
			var port []byte

			instance.Spec.TLS.SecretName = "tls-secret"

			obj, err := defaultUserSecretBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			secret = obj.(*corev1.Secret)

			port, ok := secret.Data["port"]
			Expect(ok).NotTo(BeFalse(), "Failed to find key \"port\" in the generated Secret")
			Expect(port).To(BeEquivalentTo("5671"))
		})

		Context("when MQTT, STOMP, streams, WebMQTT, and WebSTOMP are enabled", func() {
			It("adds the MQTTS, STOMPS, streams, WebMQTTS, and WebSTOMPS ports to the user secret", func() {
				var port []byte

				instance.Spec.TLS.SecretName = "tls-secret"
				instance.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{
					"rabbitmq_mqtt",
					"rabbitmq_stomp",
					"rabbitmq_stream",
					"rabbitmq_web_mqtt",
					"rabbitmq_web_stomp",
				}

				obj, err := defaultUserSecretBuilder.Build()
				Expect(err).NotTo(HaveOccurred())
				secret = obj.(*corev1.Secret)

				port, ok := secret.Data["mqtt-port"]
				Expect(ok).NotTo(BeFalse(), "Failed to find key \"mqtt-port\" in the generated Secret")
				Expect(port).To(BeEquivalentTo("8883"))

				port, ok = secret.Data["stomp-port"]
				Expect(ok).NotTo(BeFalse(), "Failed to find key \"stomp-port\" in the generated Secret")
				Expect(port).To(BeEquivalentTo("61614"))

				port, ok = secret.Data["stream-port"]
				Expect(ok).NotTo(BeFalse(), "Failed to find key \"stream-port\" in the generated Secret")
				Expect(port).To(BeEquivalentTo("5551"))

				port, ok = secret.Data["web-mqtt-port"]
				Expect(ok).NotTo(BeFalse(), "Failed to find key \"web-mqtt-port\" in the generated Secret")
				Expect(port).To(BeEquivalentTo("15676"))

				port, ok = secret.Data["web-stomp-port"]
				Expect(ok).NotTo(BeFalse(), "Failed to find key \"web-stomp-port\" in the generated Secret")
				Expect(port).To(BeEquivalentTo("15673"))
			})
		})
	})

	Context("Update with instance labels", func() {
		It("Updates the secret", func() {
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

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":      instance.Name,
						"app.kubernetes.io/part-of":   "rabbitmq",
						"this-was-the-previous-label": "should-be-deleted",
					},
				},
			}
			err := defaultUserSecretBuilder.Update(secret)
			Expect(err).NotTo(HaveOccurred())

			By("adding new labels from the CR", func() {
				testLabels(secret.Labels)
			})

			By("restoring the default labels", func() {
				labels := secret.Labels
				Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
				Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
				Expect(labels["app.kubernetes.io/part-of"]).To(Equal("rabbitmq"))
			})

			By("deleting the labels that are removed from the CR", func() {
				Expect(secret.Labels).NotTo(HaveKey("this-was-the-previous-label"))
			})
		})
	})

	Context("Update with instance annotations", func() {
		It("updates the secret with the annotations", func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rabbit-labelled",
				},
			}
			instance.Annotations = map[string]string{
				"my-annotation":               "i-like-this",
				"kubernetes.io/name":          "i-do-not-like-this",
				"kubectl.kubernetes.io/name":  "i-do-not-like-this",
				"k8s.io/name":                 "i-do-not-like-this",
				"kubernetes.io/other":         "i-do-not-like-this",
				"kubectl.kubernetes.io/other": "i-do-not-like-this",
				"k8s.io/other":                "i-do-not-like-this",
			}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"i-was-here-already":            "please-dont-delete-me",
						"im-here-to-stay.kubernetes.io": "for-a-while",
						"kubernetes.io/name":            "should-stay",
						"kubectl.kubernetes.io/name":    "should-stay",
						"k8s.io/name":                   "should-stay",
					},
				},
			}
			err := defaultUserSecretBuilder.Update(secret)
			Expect(err).NotTo(HaveOccurred())

			By("updating secret annotations on default-user secret", func() {
				expectedAnnotations := map[string]string{
					"my-annotation":                 "i-like-this",
					"i-was-here-already":            "please-dont-delete-me",
					"im-here-to-stay.kubernetes.io": "for-a-while",
					"kubernetes.io/name":            "should-stay",
					"kubectl.kubernetes.io/name":    "should-stay",
					"k8s.io/name":                   "should-stay",
				}

				Expect(secret.Annotations).To(Equal(expectedAnnotations))
			})
		})
	})

	It("sets owner reference", func() {
		secret = &corev1.Secret{}
		instance = rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "rabbit1",
			},
		}
		Expect(defaultUserSecretBuilder.Update(secret)).NotTo(HaveOccurred())
		Expect(secret.OwnerReferences[0].Name).To(Equal(instance.Name))
	})

	Context("UpdateMayRequireStsRecreate", func() {
		It("returns false", func() {
			Expect(defaultUserSecretBuilder.UpdateMayRequireStsRecreate()).To(BeFalse())
		})
	})
})
