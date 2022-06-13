// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package system_tests

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	controllerruntime "sigs.k8s.io/controller-runtime"

	"gopkg.in/ini.v1"

	k8sresource "k8s.io/apimachinery/pkg/api/resource"

	mgmtApi "github.com/michaelklishin/rabbit-hole/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-stomp/stomp"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	streamamqp "github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/message"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
	"github.com/streadway/amqp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const podCreationTimeout = 10 * time.Minute
const portReadinessTimeout = 1 * time.Minute
const k8sQueryTimeout = 1 * time.Minute

type featureFlag struct {
	Name  string
	State string
}

func MustHaveEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic(fmt.Sprintf("Environment variable '%s' not found", name))
	}
	return value
}

func createClientSet() (*kubernetes.Clientset, error) {
	config, err := controllerruntime.GetConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("[error] %s \n", err)
	}

	return clientset, err
}

func kubectlExec(namespace, podname, containerName string, args ...string) ([]byte, error) {
	kubectlArgs := append([]string{
		"-n",
		namespace,
		"exec",
		podname,
		"-c",
		containerName,
		"--",
	}, args...)

	return kubectl(kubectlArgs...)
}

func kubectl(args ...string) ([]byte, error) {
	cmd := exec.Command("kubectl", args...)
	return cmd.CombinedOutput()
}

func makeRequest(url, httpMethod, rabbitmqUsername, rabbitmqPassword string, body []byte) (responseBody []byte, err error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest(httpMethod, url, bytes.NewReader(body))
	req.SetBasicAuth(rabbitmqUsername, rabbitmqPassword)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed to make api request to url %s with err: %+v \n", url, err)
		return responseBody, fmt.Errorf("failed with err: %w to api endpoint: %s", err, url)
	}
	defer resp.Body.Close()
	responseBody, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return responseBody, err
	}

	if resp.StatusCode >= 400 {
		return responseBody, fmt.Errorf("make request failed with api endpoint: %s and statusCode: %d", url, resp.StatusCode)
	}

	return
}

func getMessageFromQueue(rabbitmqHostName, rabbitmqPort, rabbitmqUsername, rabbitmqPassword string) (*Message, error) {
	getQueuesUrl := fmt.Sprintf("http://%s:%s/api/queues/%%2F/test-queue/get", rabbitmqHostName, rabbitmqPort)
	data := map[string]interface{}{
		"vhost":    "/",
		"name":     "test-queue",
		"encoding": "auto",
		"ackmode":  "ack_requeue_false",
		"truncate": "50000",
		"count":    "1",
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	response, err := makeRequest(getQueuesUrl, http.MethodPost, rabbitmqUsername, rabbitmqPassword, dataJSON)
	if err != nil {
		return nil, err
	}

	var messages []Message
	err = json.Unmarshal(response, &messages)
	if err != nil {
		return nil, err
	}

	return &messages[0], err
}

type Message struct {
	Payload      string `json:"payload"`
	MessageCount int    `json:"message_count"`
}

func publishToQueue(rabbitmqHostName, rabbitmqPort, rabbitmqUsername, rabbitmqPassword string) error {
	url := fmt.Sprintf("http://%s:%s/api/queues/%%2F/test-queue", rabbitmqHostName, rabbitmqPort)
	_, err := makeRequest(url, http.MethodPut, rabbitmqUsername, rabbitmqPassword, []byte("{\"durable\": true}"))

	if err != nil {
		return err
	}

	url = fmt.Sprintf("http://%s:%s/api/exchanges/%%2F/amq.default/publish", rabbitmqHostName, rabbitmqPort)
	data := map[string]interface{}{
		"vhost": "/",
		"name":  "amq.default",
		"properties": map[string]interface{}{
			"delivery_mode": 2,
			"headers":       map[string]interface{}{},
		},
		"routing_key":      "test-queue",
		"delivery_mode":    "2",
		"payload":          "hello",
		"headers":          map[string]interface{}{},
		"props":            map[string]interface{}{},
		"payload_encoding": "string",
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = makeRequest(url, http.MethodPost, rabbitmqUsername, rabbitmqPassword, dataJSON)
	if err != nil {
		return err
	}

	return nil
}

func connectHTTPS(username, password, hostname, httpsNodePort, caFilePath string) (err error) {
	// create TLS config for https request
	cfg := new(tls.Config)
	cfg.RootCAs = x509.NewCertPool()
	ca, err := ioutil.ReadFile(caFilePath)
	if err != nil {
		return err
	}

	cfg.RootCAs.AppendCertsFromPEM(ca)

	transport := &http.Transport{TLSClientConfig: cfg}
	rmqc, err := mgmtApi.NewTLSClient(fmt.Sprintf("https://%v:%v", hostname, httpsNodePort), username, password, transport)
	if err != nil {
		return err
	}

	_, err = rmqc.Overview()

	return err
}

func connectAMQPS(username, password, hostname, port, caFilePath string) (conn *amqp.Connection, err error) {
	// create TLS config for amqps request
	cfg := new(tls.Config)
	cfg.RootCAs = x509.NewCertPool()
	ca, err := ioutil.ReadFile(caFilePath)
	if err != nil {
		return nil, err
	}
	cfg.RootCAs.AppendCertsFromPEM(ca)

	for retry := 0; retry < 5; retry++ {
		conn, err = amqp.DialTLS(fmt.Sprintf("amqps://%v:%v@%v:%v/", username, password, hostname, port), cfg)
		if err == nil {
			return conn, nil
		}
		time.Sleep(5 * time.Second)
	}
	return nil, err
}

func inspectServerCertificate(username, password, hostname, amqpsPort, caFilePath string) []byte {
	conn, err := connectAMQPS(username, password, hostname, amqpsPort, caFilePath)
	if err != nil {
		return nil
	}
	defer conn.Close()

	state := conn.ConnectionState()
	ExpectWithOffset(1, state.PeerCertificates).To(HaveLen(1))
	return state.PeerCertificates[0].Raw
}

func publishToQueueAMQPS(message, username, password, hostname, amqpsPort, caFilePath string) error {
	// create connection
	conn, err := connectAMQPS(username, password, hostname, amqpsPort, caFilePath)
	if err != nil {
		return err
	}
	defer conn.Close()

	// create channel
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"test-queue", // name
		true,         // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return err
	}

	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(message),
		})
	if err != nil {
		return err
	}

	return nil
}

func getMessageFromQueueAMQPS(username, password, hostname, amqpsPort, caFilePath string) (string, error) {
	// create connection
	conn, err := connectAMQPS(username, password, hostname, amqpsPort, caFilePath)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	// create channel
	ch, err := conn.Channel()
	if err != nil {
		return "", err
	}
	defer ch.Close()

	// declare queue (safety incase the consumer is started before the producer)
	q, err := ch.QueueDeclare(
		"test-queue", // name
		true,         // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return "", err
	}

	// consume from queue
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return "", err
	}

	// return first msg
	for msg := range msgs {
		return string(msg.Body), nil
	}

	return "", nil
}

func alivenessTest(rabbitmqHostName, rabbitmqPort, rabbitmqUsername, rabbitmqPassword string) *HealthcheckResponse {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("http://%s:%s/api/aliveness-test/%%2F", rabbitmqHostName, rabbitmqPort)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	req.SetBasicAuth(rabbitmqUsername, rabbitmqPassword)

	resp, err := client.Do(req)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	Expect(resp).To(HaveHTTPStatus(http.StatusOK))

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	healthcheckResponse := &HealthcheckResponse{}
	ExpectWithOffset(1, json.Unmarshal(b, healthcheckResponse)).To(Succeed())

	return healthcheckResponse
}

type HealthcheckResponse struct {
	Status string `json:"status"`
}

func getUsernameAndPassword(ctx context.Context, clientset *kubernetes.Clientset, namespace, instanceName string) (string, string, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, fmt.Sprintf("%s-default-user", instanceName), metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}

	username, ok := secret.Data["username"]
	if !ok {
		return "", "", fmt.Errorf("cannot find 'username' in %s-default-user", instanceName)
	}

	password, ok := secret.Data["password"]
	if !ok {
		return "", "", fmt.Errorf("cannot find 'password' in %s-default-user", instanceName)
	}
	return string(username), string(password), nil
}

func newRabbitmqCluster(namespace, instanceName string) *rabbitmqv1beta1.RabbitmqCluster {
	cluster := &rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceName,
			Namespace: namespace,
		},
		Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
			Service: rabbitmqv1beta1.RabbitmqClusterServiceSpec{
				Type: "NodePort",
			},
			// run system tests with low resources so that they can run on GitHub actions
			Resources: &corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]k8sresource.Quantity{
					corev1.ResourceCPU:    k8sresource.MustParse("100m"),
					corev1.ResourceMemory: k8sresource.MustParse("500M"),
				},
				Limits: map[corev1.ResourceName]k8sresource.Quantity{
					corev1.ResourceCPU:    k8sresource.MustParse("2000m"),
					corev1.ResourceMemory: k8sresource.MustParse("500M"),
				},
			},
		},
	}

	if os.Getenv("ENVIRONMENT") == "openshift" {
		overrideSecurityContextForOpenshift(cluster)
	}

	if image := os.Getenv("RABBITMQ_IMAGE"); image != "" {
		cluster.Spec.Image = image
	}
	if secret := os.Getenv("RABBITMQ_IMAGE_PULL_SECRET"); secret != "" {
		cluster.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
			{Name: secret},
		}
	}

	return cluster
}

func overrideSecurityContextForOpenshift(cluster *rabbitmqv1beta1.RabbitmqCluster) {

	cluster.Spec.Override = rabbitmqv1beta1.RabbitmqClusterOverrideSpec{
		StatefulSet: &rabbitmqv1beta1.StatefulSet{
			Spec: &rabbitmqv1beta1.StatefulSetSpec{
				Template: &rabbitmqv1beta1.PodTemplateSpec{
					Spec: &corev1.PodSpec{
						SecurityContext: &corev1.PodSecurityContext{},
						Containers: []corev1.Container{
							{
								Name: "rabbitmq",
							},
						},
					},
				},
			},
		},
	}

}

//the updateFn can change properties of the RabbitmqCluster CR
func updateRabbitmqCluster(ctx context.Context, client client.Client, name, namespace string, updateFn func(*rabbitmqv1beta1.RabbitmqCluster)) error {
	var result rabbitmqv1beta1.RabbitmqCluster

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		getErr := client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &result)
		if getErr != nil {
			return getErr
		}

		updateFn(&result)
		updateErr := client.Update(ctx, &result)
		return updateErr
	})

	return retryErr
}

func createRabbitmqCluster(ctx context.Context, client client.Client, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	return client.Create(ctx, rabbitmqCluster)
}

func statefulSetPodName(cluster *rabbitmqv1beta1.RabbitmqCluster, index int) string {
	return cluster.ChildResourceName(strings.Join([]string{"server", strconv.Itoa(index)}, "-"))
}

/*
 * Helper function to fetch a Kubernetes Node IP. Node IPs are necessary
 * to access NodePort type services.
 */
func kubernetesNodeIp(ctx context.Context, clientSet *kubernetes.Clientset) string {
	nodes, err := clientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, nodes).ToNot(BeNil())
	ExpectWithOffset(1, len(nodes.Items)).To(BeNumerically(">", 0))

	var nodeIp string
	for _, address := range nodes.Items[0].Status.Addresses {
		// There are no order guarantees in this array. An Internal IP might come
		// before an external IP or hostname. We want to return an external IP if
		// available, or the internal IP.
		// We don't want to return a hostname because it might not be resolvable by
		// our local DNS
		switch address.Type {
		case corev1.NodeExternalIP:
			return address.Address
		case corev1.NodeInternalIP:
			nodeIp = address.Address
		}
	}
	// we did not find an external IP
	// we might return empty or the internal IP
	return nodeIp
}

func getConfigFileFromPod(namespace string, cluster *rabbitmqv1beta1.RabbitmqCluster, path string) map[string]string {
	output, err := kubectlExec(namespace,
		statefulSetPodName(cluster, 0),
		"rabbitmq",
		"cat",
		path,
	)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	cfg, err := ini.Load(output)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return cfg.Section("").KeysHash()
}

func containsPort(ports []corev1.ServicePort, portName string) bool {
	for _, p := range ports {
		if p.Name == portName {
			return true
		}
	}
	return false
}

func rabbitmqNodePort(ctx context.Context, clientSet *kubernetes.Clientset, cluster *rabbitmqv1beta1.RabbitmqCluster, portName string) string {
	service, err := clientSet.CoreV1().Services(cluster.Namespace).
		Get(ctx, cluster.ChildResourceName(""), metav1.GetOptions{})

	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	for _, port := range service.Spec.Ports {
		if port.Name == portName {
			return strconv.Itoa(int(port.NodePort))
		}
	}

	return ""
}

func waitForRabbitmqUpdate(cluster *rabbitmqv1beta1.RabbitmqCluster) {
	waitForRabbitmqNotRunningWithOffset(cluster, 2)
	waitForRabbitmqRunningWithOffset(cluster, 2)
}

func waitForRabbitmqRunning(cluster *rabbitmqv1beta1.RabbitmqCluster) {
	waitForRabbitmqRunningWithOffset(cluster, 2)
}

func waitForRabbitmqNotRunningWithOffset(cluster *rabbitmqv1beta1.RabbitmqCluster, callStackOffset int) {
	var err error

	EventuallyWithOffset(callStackOffset, func() string {
		output, err := kubectl(
			"-n",
			cluster.Namespace,
			"get",
			"rabbitmqclusters",
			cluster.Name,
			"-ojsonpath='{.status.conditions[?(@.type==\"AllReplicasReady\")].status}'",
		)

		if err != nil {
			ExpectWithOffset(1, string(output)).To(ContainSubstring("not found"))
		}

		return string(output)
	}, podCreationTimeout, 1).Should(Equal("'False'"))

	ExpectWithOffset(callStackOffset, err).NotTo(HaveOccurred())
}

// the callStackOffset makes sure that failures point to the caller of the function
// than the function itself
func waitForRabbitmqRunningWithOffset(cluster *rabbitmqv1beta1.RabbitmqCluster, callStackOffset int) {
	var err error

	EventuallyWithOffset(callStackOffset, func() string {
		output, err := kubectl(
			"-n",
			cluster.Namespace,
			"get",
			"rabbitmqclusters",
			cluster.Name,
			"-ojsonpath='{.status.conditions[?(@.type==\"AllReplicasReady\")].status}'",
		)

		if err != nil {
			ExpectWithOffset(1, string(output)).To(ContainSubstring("not found"))
		}

		return string(output)
	}, podCreationTimeout, 1).Should(Equal("'True'"))

	ExpectWithOffset(callStackOffset, err).NotTo(HaveOccurred())
}

func waitForPortConnectivity(cluster *rabbitmqv1beta1.RabbitmqCluster) {
	waitForPortConnectivityWithOffset(cluster, 2)
}
func waitForPortConnectivityWithOffset(cluster *rabbitmqv1beta1.RabbitmqCluster, callStackOffset int) {
	EventuallyWithOffset(callStackOffset, func() error {
		_, err := kubectlExec(cluster.Namespace, statefulSetPodName(cluster, 0), "rabbitmq",
			"rabbitmq-diagnostics", "check_port_connectivity")
		return err
	}, portReadinessTimeout, 3).Should(Not(HaveOccurred()))
}

func waitForPortReadiness(cluster *rabbitmqv1beta1.RabbitmqCluster, port int) {
	waitForPortReadinessWithOffset(cluster, port, 2)
}
func waitForPortReadinessWithOffset(cluster *rabbitmqv1beta1.RabbitmqCluster, port int, callStackOffset int) {
	EventuallyWithOffset(callStackOffset, func() error {
		_, err := kubectlExec(cluster.Namespace, statefulSetPodName(cluster, 0), "rabbitmq",
			"rabbitmq-diagnostics", "check_port_listener", strconv.Itoa(port))
		return err
	}, portReadinessTimeout, 3).Should(Not(HaveOccurred()))
}

func hasFeatureEnabled(cluster *rabbitmqv1beta1.RabbitmqCluster, featureFlagName string) bool {
	output, err := kubectlExec(cluster.Namespace,
		statefulSetPodName(cluster, 0),
		"rabbitmq",
		"rabbitmqctl",
		"list_feature_flags",
		"--formatter=json",
	)
	Expect(err).NotTo(HaveOccurred())
	var flags []featureFlag
	Expect(json.Unmarshal(output, &flags)).To(Succeed())

	for _, v := range flags {
		if v.Name == featureFlagName && v.State == "enabled" {
			return true
		}
	}
	return false
}

// asserts an event with reason: "TLSError", occurs for the cluster in it's namespace
func assertTLSError(cluster *rabbitmqv1beta1.RabbitmqCluster) {
	var err error

	EventuallyWithOffset(1, func() string {
		output, _ := kubectl(
			"-n",
			cluster.Namespace,
			"get",
			"events",
			"--field-selector",
			fmt.Sprintf("involvedObject.name=%v,involvedObject.namespace=%v,reason=%v", cluster.Name, cluster.Namespace, "TLSError"),
		)

		return string(output)
	}, podCreationTimeout, 1*time.Second).Should(ContainSubstring("TLSError"))

	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func assertHttpReady(hostname, port string) {
	EventuallyWithOffset(1, func() (*http.Response, error) {
		client := &http.Client{Timeout: 10 * time.Second}
		rabbitURL := fmt.Sprintf("http://%s:%s", hostname, port)

		req, err := http.NewRequest(http.MethodGet, rabbitURL, nil)
		ExpectWithOffset(1, err).ToNot(HaveOccurred())

		return client.Do(req)
	}, podCreationTimeout, 5).Should(HaveHTTPStatus(http.StatusOK))
}

func createTLSSecret(secretName, secretNamespace, hostname string) (string, []byte, []byte) {
	// create cert files
	serverCertPath, serverCertFile := createCertFile(2, "server.crt")
	serverKeyPath, serverKeyFile := createCertFile(2, "server.key")
	caCertPath, caCertFile := createCertFile(2, "ca.crt")

	// generate and write cert and key to file
	caCert, caKey, err := createCertificateChain(hostname, caCertFile, serverCertFile, serverKeyFile)
	ExpectWithOffset(1, err).To(Succeed())
	// create k8s tls secret
	ExpectWithOffset(1, k8sCreateTLSSecret(secretName, secretNamespace, serverCertPath, serverKeyPath)).To(Succeed())

	// remove cert files
	ExpectWithOffset(1, os.Remove(serverKeyPath)).To(Succeed())
	ExpectWithOffset(1, os.Remove(serverCertPath)).To(Succeed())
	return caCertPath, caCert, caKey
}

func updateTLSSecret(secretName, secretNamespace, hostname string, caCert, caKey []byte) {
	serverCertPath, serverCertFile := createCertFile(2, "server.crt")
	serverKeyPath, serverKeyFile := createCertFile(2, "server.key")

	ExpectWithOffset(1, generateCertandKey(hostname, caCert, caKey, serverCertFile, serverKeyFile)).To(Succeed())
	ExpectWithOffset(1, k8sCreateTLSSecret(secretName, secretNamespace, serverCertPath, serverKeyPath)).To(Succeed())

	ExpectWithOffset(1, os.Remove(serverKeyPath)).To(Succeed())
	ExpectWithOffset(1, os.Remove(serverCertPath)).To(Succeed())
}

func createCertFile(offset int, fileName string) (string, *os.File) {
	tmpDir, err := ioutil.TempDir("", "certs")
	ExpectWithOffset(offset, err).ToNot(HaveOccurred())
	path := filepath.Join(tmpDir, fileName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0755)
	ExpectWithOffset(offset, err).ToNot(HaveOccurred())
	return path, file
}

func k8sSecretExists(secretName, secretNamespace string) (bool, error) {
	output, err := kubectl(
		"-n",
		secretNamespace,
		"get",
		"secret",
		secretName,
	)

	if err != nil {
		ExpectWithOffset(1, string(output)).To(ContainSubstring("not found"))
		return false, nil
	}

	return true, nil
}

func k8sCreateTLSSecret(secretName, secretNamespace, certPath, keyPath string) error {
	// delete secret if it exists
	secretExists, err := k8sSecretExists(secretName, secretNamespace)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	if secretExists {
		ExpectWithOffset(1, k8sDeleteSecret(secretName, secretNamespace)).To(Succeed())
	}

	// create secret
	output, err := kubectl(
		"-n",
		secretNamespace,
		"create",
		"secret",
		"tls",
		secretName,
		fmt.Sprintf("--cert=%+v", certPath),
		fmt.Sprintf("--key=%+v", keyPath),
	)

	if err != nil {
		return fmt.Errorf("Failed with error: %w\nOutput: %v\n", err, string(output))
	}

	return nil
}

func k8sDeleteSecret(secretName, secretNamespace string) error {
	output, err := kubectl(
		"-n",
		secretNamespace,
		"delete",
		"secret",
		secretName,
	)

	if err != nil {
		return fmt.Errorf("Failed with error: %w\nOutput: %v\n", err, string(output))
	}

	return nil
}

// generate a pair of certificate and key, given a cacert
func generateCertandKey(hostname string, caCert, caKey []byte, certWriter, keyWriter io.Writer) error {
	caPriv, err := helpers.ParsePrivateKeyPEM(caKey)
	if err != nil {
		return err
	}

	caPub, err := helpers.ParseCertificatePEM(caCert)
	if err != nil {
		return err
	}

	s, err := local.NewSigner(caPriv, caPub, signer.DefaultSigAlgo(caPriv), nil)
	if err != nil {
		return err
	}

	// create server cert
	serverReq := &csr.CertificateRequest{
		Names: []csr.Name{
			{
				C:  "UK",
				ST: "London",
				L:  "London",
				O:  "VMWare",
				OU: "RabbitMQ",
			},
		},
		CN:         "tests-server",
		Hosts:      []string{hostname},
		KeyRequest: &csr.KeyRequest{A: "rsa", S: 2048},
	}

	serverCsr, serverKey, err := csr.ParseRequest(serverReq)
	if err != nil {
		return err
	}

	signReq := signer.SignRequest{Hosts: serverReq.Hosts, Request: string(serverCsr)}
	serverCert, err := s.Sign(signReq)
	if err != nil {
		return err
	}

	_, err = certWriter.Write(serverCert)
	if err != nil {
		return err
	}
	_, err = keyWriter.Write(serverKey)
	if err != nil {
		return err
	}
	return nil
}

// creates a CA cert, and uses it to sign another cert
// it returns the generated ca cert and key so they can be reused
func createCertificateChain(hostname string, caCertWriter, certWriter, keyWriter io.Writer) ([]byte, []byte, error) {
	// create a CA cert
	caReq := &csr.CertificateRequest{
		Names: []csr.Name{
			{
				C:  "UK",
				ST: "London",
				L:  "London",
				O:  "VMWare",
				OU: "RabbitMQ",
			},
		},
		CN:         "tests-CA",
		Hosts:      []string{hostname},
		KeyRequest: &csr.KeyRequest{A: "rsa", S: 2048},
	}

	caCert, _, caKey, err := initca.New(caReq)
	if err != nil {
		return nil, nil, err
	}

	_, err = caCertWriter.Write(caCert)
	if err != nil {
		return nil, nil, err
	}

	if err := generateCertandKey(hostname, caCert, caKey, certWriter, keyWriter); err != nil {
		return nil, nil, err
	}

	return caCert, caKey, nil
}

func publishAndConsumeMQTTMsg(hostname, port, username, password string, overWebSocket bool, tlsConfig *tls.Config) {
	url := fmt.Sprintf("tcp://%s:%s", hostname, port)
	if overWebSocket {
		url = fmt.Sprintf("ws://%s:%s/ws", hostname, port)
	}
	opts := mqtt.NewClientOptions().
		AddBroker(url).
		SetUsername(username).
		SetPassword(password).
		SetClientID("system tests MQTT plugin").
		SetProtocolVersion(4) // RabbitMQ MQTT plugin targets MQTT 3.1.1

	if tlsConfig != nil {
		url = fmt.Sprintf("ssl://%s:%s", hostname, port)
		opts = opts.
			AddBroker(url).
			SetTLSConfig(tlsConfig)
	}

	c := mqtt.NewClient(opts)

	var token mqtt.Token
	EventuallyWithOffset(1, func() bool {
		token = c.Connect()
		// Waits for the network request to reach the destination and receive a response
		if !token.WaitTimeout(30 * time.Second) {
			fmt.Printf("Timed out\n")
			return false
		}

		if err := token.Error(); err == nil {
			fmt.Printf("Connected !\n")
			return true
		}
		return false
	}, 30, 20).Should(BeTrue(), "Expected to be able to connect to MQTT port")

	topic := "tests/mqtt"
	msgReceived := false

	handler := func(client mqtt.Client, msg mqtt.Message) {
		defer GinkgoRecover()
		ExpectWithOffset(1, msg.Topic()).To(Equal(topic))
		ExpectWithOffset(1, string(msg.Payload())).To(Equal("test message MQTT"))
		msgReceived = true
	}

	token = c.Subscribe(topic, 0, handler)
	ExpectWithOffset(1, token.Wait()).To(BeTrue(), "Subscribe token should return true")
	ExpectWithOffset(1, token.Error()).ToNot(HaveOccurred(), "Subscribe token received error")

	token = c.Publish(topic, 0, false, "test message MQTT")
	ExpectWithOffset(1, token.Wait()).To(BeTrue(), "Publish token should return true")
	ExpectWithOffset(1, token.Error()).ToNot(HaveOccurred(), "Publish token received error")

	EventuallyWithOffset(1, func() bool {
		return msgReceived
	}, 10*time.Second).Should(BeTrue(), "Expect to receive message")

	token = c.Unsubscribe(topic)
	ExpectWithOffset(1, token.Wait()).To(BeTrue(), "Unsubscribe token should return true")
	ExpectWithOffset(1, token.Error()).ToNot(HaveOccurred(), "Unsubscribe token received error")

	c.Disconnect(250)
}

func publishAndConsumeSTOMPMsg(hostname, port, username, password string, tlsConfig *tls.Config) {
	var conn *stomp.Conn
	var err error

	// Create a secure tls.Conn and pass to stomp.Connect, otherwise use Stomp.Dial
	if tlsConfig != nil {
		secureConn, err := tls.Dial("tcp", fmt.Sprintf("%s:%s", hostname, port), tlsConfig)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		defer secureConn.Close()

		for retry := 0; retry < 5; retry++ {
			fmt.Printf("Attempt #%d to connect using STOMPS\n", retry)
			conn, err = stomp.Connect(secureConn,
				stomp.ConnOpt.Login(username, password),
				stomp.ConnOpt.AcceptVersion(stomp.V12), // RabbitMQ STOMP plugin supports STOMP versions 1.0 through 1.2
				stomp.ConnOpt.Host("/"),                // default virtual host
			)

			if err == nil {
				break
			}

			time.Sleep(2 * time.Second)
		}
	} else {
		for retry := 0; retry < 5; retry++ {
			fmt.Printf("Attempt #%d to connect using STOMP\n", retry)
			conn, err = stomp.Dial("tcp",
				fmt.Sprintf("%s:%s", hostname, port),
				stomp.ConnOpt.Login(username, password),
				stomp.ConnOpt.AcceptVersion(stomp.V12),
				stomp.ConnOpt.Host("/"),
			)

			if err == nil {
				break
			}

			time.Sleep(2 * time.Second)
		}
	}

	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	queue := "/queue/system-tests-stomp"
	sub, err := conn.Subscribe(queue, stomp.AckAuto)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	msgReceived := false
	go func() {
		defer GinkgoRecover()
		var msg *stomp.Message
		EventuallyWithOffset(1, sub.C, 5*time.Second).Should(Receive(&msg))
		ExpectWithOffset(1, msg.Err).ToNot(HaveOccurred())
		ExpectWithOffset(1, string(msg.Body)).To(Equal("test message STOMP"))
		msgReceived = true
	}()

	ExpectWithOffset(1, conn.Send(queue, "text/plain", []byte("test message STOMP"), nil)).To(Succeed())
	EventuallyWithOffset(1, func() bool {
		return msgReceived
	}, 5*time.Second).Should(BeTrue())
	ExpectWithOffset(1, sub.Unsubscribe()).To(Succeed())
	ExpectWithOffset(1, conn.Disconnect()).To(Succeed())
}

func publishAndConsumeStreamMsg(host, port, username, password string) {
	portInt, err := strconv.Atoi(port)
	Expect(err).ToNot(HaveOccurred())

	var env *stream.Environment
	Eventually(func() error {
		fmt.Println("connecting to stream endpoint ...")
		env, err = stream.NewEnvironment(stream.NewEnvironmentOptions().
			SetHost(host).
			SetPort(portInt).
			SetPassword(password).
			SetUser(username).
			SetAddressResolver(stream.AddressResolver{
				Host: host,
				Port: portInt,
			}))
		if err == nil {
			fmt.Println("connected to stream endpoint")
			return nil
		} else {
			fmt.Printf("failed to connect to stream endpoint (%s:%d) due to %g\n", host, portInt, err)
		}
		return err
	}, portReadinessTimeout*5, portReadinessTimeout).ShouldNot(HaveOccurred())

	const streamName = "system-test-stream"
	Expect(env.DeclareStream(
		streamName,
		&stream.StreamOptions{
			MaxLengthBytes: stream.ByteCapacity{}.KB(1),
		},
	)).To(Succeed())

	producer, err := env.NewProducer(streamName, nil)
	Expect(err).ToNot(HaveOccurred())
	chPublishConfirm := producer.NotifyPublishConfirmation()
	const msgSent = "test message"
	Expect(producer.BatchSend(
		[]message.StreamMessage{streamamqp.NewMessage([]byte(msgSent))})).To(Succeed())
	Eventually(chPublishConfirm).Should(Receive())
	Expect(producer.Close()).To(Succeed())

	var msgReceived string
	handleMessages := func(consumerContext stream.ConsumerContext, message *streamamqp.Message) {
		Expect(message.Data).To(HaveLen(1))
		msgReceived = string(message.Data[0][:])
	}
	consumer, err := env.NewConsumer(
		streamName,
		handleMessages,
		stream.NewConsumerOptions().
			SetOffset(stream.OffsetSpecification{}.First()))
	Expect(err).ToNot(HaveOccurred())
	Eventually(func() string {
		return msgReceived
	}).Should(Equal(msgSent), "consumer should receive message")
	Expect(consumer.Close()).To(Succeed())
	Expect(env.DeleteStream(streamName)).To(Succeed())
	Expect(env.Close()).To(Succeed())
}

func pod(ctx context.Context, clientSet *kubernetes.Clientset, r *rabbitmqv1beta1.RabbitmqCluster, i int) *corev1.Pod {
	podName := statefulSetPodName(r, i)
	var pod *corev1.Pod
	EventuallyWithOffset(1, func() error {
		var err error
		pod, err = clientSet.CoreV1().Pods(r.Namespace).Get(ctx, podName, metav1.GetOptions{})
		return err
	}, 10).Should(Succeed())
	return pod
}
