package rabbitmqclient_test

import (
	"crypto/x509"
	"errors"
	"github.com/rabbitmq/cluster-operator/internal/rabbitmqclient"
	"io/ioutil"
	"net/http"
	"net/url"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ParseReference", func() {
	var (
		existingRabbitMQUsername = "abc123"
		existingRabbitMQPassword = "foo1234"
		existingRabbitMQCluster  *rabbitmqv1beta1.RabbitmqCluster
		existingService          *corev1.Service
		fakeRabbitMQServer       *ghttp.Server
		fakeRabbitMQURL          *url.URL
		fakeRabbitMQPort         int
		certPool                 *x509.CertPool
		creds                    map[string]string
		serverCertPath           string
		serverKeyPath            string
		caCertPath               string
		caCertBytes              []byte
	)
	BeforeEach(func() {
		certPool = x509.NewCertPool()
	})
	AfterEach(func() {
		fakeRabbitMQServer.Close()
	})

	When("RabbitmqCluster is provided", func() {
		JustBeforeEach(func() {
			creds = map[string]string{
				"username": existingRabbitMQUsername,
				"password": existingRabbitMQPassword,
				"uri":      fakeRabbitMQURL.String(),
			}

			fakeRabbitMQServer.RouteToHandler("PUT", "/api/users/example-user", func(w http.ResponseWriter, req *http.Request) {
				user, password, ok := req.BasicAuth()
				if !(ok && user == existingRabbitMQUsername && password == existingRabbitMQPassword) {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			})
		})
		When("the RabbitmqCluster is configured without TLS", func() {
			BeforeEach(func() {
				fakeRabbitMQServer = mockRabbitMQServer()

				var err error
				fakeRabbitMQURL, fakeRabbitMQPort, err = mockRabbitMQURLPort(fakeRabbitMQServer)
				Expect(err).NotTo(HaveOccurred())

				existingRabbitMQCluster = &rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rmq",
						Namespace: "rabbitmq-system",
					},
					Status: rabbitmqv1beta1.RabbitmqClusterStatus{
						Binding: &corev1.LocalObjectReference{
							Name: "rmq-default-user-credentials",
						},
						DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
							ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
								Name:      "rmq-service",
								Namespace: "rabbitmq-system",
							},
						},
					},
				}
				existingService = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rmq-service",
						Namespace: "rabbitmq-system",
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: fakeRabbitMQURL.Hostname(),
						Ports: []corev1.ServicePort{
							{
								Name: "management",
								Port: int32(fakeRabbitMQPort),
							},
						},
					},
				}
			})

			It("generates a rabbithole client which makes successful requests to the RabbitMQ Server", func() {
				generatedClient, err := rabbitmqclient.RabbitholeClientFactory(creds, false, certPool)
				Expect(err).NotTo(HaveOccurred())
				Expect(generatedClient).NotTo(BeNil())

				_, err = generatedClient.PutUser("example-user", rabbithole.UserSettings{})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(fakeRabbitMQServer.ReceivedRequests())).To(Equal(1))
			})
		})

		When("the RabbitmqCluster is configured with TLS", func() {
			BeforeEach(func() {
				fakeRabbitMQServer, serverCertPath, serverKeyPath, caCertPath = mockRabbitMQTLSServer()

				var err error
				fakeRabbitMQURL, fakeRabbitMQPort, err = mockRabbitMQURLPort(fakeRabbitMQServer)
				Expect(err).NotTo(HaveOccurred())

				certBytes, err := ioutil.ReadFile(serverCertPath)
				Expect(err).NotTo(HaveOccurred())
				keyBytes, err := ioutil.ReadFile(serverKeyPath)
				Expect(err).NotTo(HaveOccurred())
				caCertBytes, err = ioutil.ReadFile(caCertPath)
				Expect(err).NotTo(HaveOccurred())
				existingRabbitMQCluster = &rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rmq",
						Namespace: "rabbitmq-system",
					},
					Status: rabbitmqv1beta1.RabbitmqClusterStatus{
						Binding: &corev1.LocalObjectReference{
							Name: "rmq-default-user-credentials",
						},
						DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
							ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
								Name:      "rmq-service",
								Namespace: "rabbitmq-system",
							},
						},
					},
				}
				existingService = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rmq-service",
						Namespace: "rabbitmq-system",
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: fakeRabbitMQURL.Hostname(),
						Ports: []corev1.ServicePort{
							{
								Name: "management",
								Port: int32(fakeRabbitMQPort),
							},
						},
					},
				}
				existingCertSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "tls-certs",
						Namespace: "rabbitmq-system",
					},
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						corev1.TLSCertKey:       certBytes,
						corev1.TLSPrivateKeyKey: keyBytes,
					},
				}
				existingService.Spec.Ports = append(existingService.Spec.Ports, corev1.ServicePort{
					Name: "management-tls",
					Port: int32(fakeRabbitMQPort),
				})
				existingRabbitMQCluster.Spec.TLS = rabbitmqv1beta1.TLSSpec{
					SecretName:             existingCertSecret.Name,
					DisableNonTLSListeners: true,
				}
			})

			When("the CA that signed the certs is not trusted", func() {
				It("generates a rabbithole client which fails to authenticate with the cluster", func() {
					generatedClient, err := rabbitmqclient.RabbitholeClientFactory(creds, true, certPool)
					Expect(err).NotTo(HaveOccurred())
					Expect(generatedClient).NotTo(BeNil())

					_, err = generatedClient.PutUser("example-user", rabbithole.UserSettings{})
					Expect(errors.As(err, &x509.UnknownAuthorityError{})).To(BeTrue())
				})
			})

			When("the CA that signed the certs is trusted", func() {
				JustBeforeEach(func() {
					ok := certPool.AppendCertsFromPEM(caCertBytes)
					Expect(ok).To(BeTrue())
				})
				It("generates a rabbithole client which makes successful requests to the RabbitMQ Server", func() {
					generatedClient, err := rabbitmqclient.RabbitholeClientFactory(creds, true, certPool)
					Expect(err).NotTo(HaveOccurred())
					Expect(generatedClient).NotTo(BeNil())

					_, err = generatedClient.PutUser("example-user", rabbithole.UserSettings{})
					Expect(err).NotTo(HaveOccurred())
					Expect(len(fakeRabbitMQServer.ReceivedRequests())).To(Equal(1))
				})
			})
		})
	})

	When("RabbitmqCluster is not provided", func() {
		BeforeEach(func() {
			fakeRabbitMQServer = mockRabbitMQServer()

			var err error
			fakeRabbitMQURL, fakeRabbitMQPort, err = mockRabbitMQURLPort(fakeRabbitMQServer)
			Expect(err).NotTo(HaveOccurred())

			creds = map[string]string{
				"username": existingRabbitMQUsername,
				"password": existingRabbitMQPassword,
				"uri":      fakeRabbitMQURL.String(),
			}

			fakeRabbitMQServer.RouteToHandler("PUT", "/api/users/example-user", func(w http.ResponseWriter, req *http.Request) {
				user, password, ok := req.BasicAuth()
				if !(ok && user == existingRabbitMQUsername && password == existingRabbitMQPassword) {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			})
		})

		It("generates a rabbithole client which makes successful requests to the RabbitMQ Server", func() {
			generatedClient, err := rabbitmqclient.RabbitholeClientFactory(creds, false, certPool)
			Expect(err).NotTo(HaveOccurred())
			Expect(generatedClient).NotTo(BeNil())

			_, err = generatedClient.PutUser("example-user", rabbithole.UserSettings{})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(fakeRabbitMQServer.ReceivedRequests())).To(Equal(1))
		})
	})
})
