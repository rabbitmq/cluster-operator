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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-stomp/stomp"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/streadway/amqp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const podCreationTimeout = 600 * time.Second

func MustHaveEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic(fmt.Sprintf("Value '%s' not found", name))
	}
	return value
}

func createClientSet() (*kubernetes.Clientset, error) {
	config, err := createRestConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("[error] %s \n", err)
	}

	return clientset, err
}

func createRestConfig() (*rest.Config, error) {
	var config *rest.Config
	var err error
	var kubeconfig string

	if len(os.Getenv("KUBECONFIG")) > 0 {
		kubeconfig = os.Getenv("KUBECONFIG")
	} else {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube/config")
	}
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func kubectlExec(namespace, podname string, args ...string) ([]byte, error) {
	kubectlArgs := append([]string{
		"-n",
		namespace,
		"exec",
		podname,
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
		return responseBody, fmt.Errorf("failed with err: %v to api endpoint: %s", err, url)
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

func alivenessTest(rabbitmqHostName, rabbitmqPort, rabbitmqUsername, rabbitmqPassword string) (*HealthcheckResponse, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("http://%s:%s/api/aliveness-test/%%2F", rabbitmqHostName, rabbitmqPort)

	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.SetBasicAuth(rabbitmqUsername, rabbitmqPassword)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed to run cluster aliveness test: %+v \n", err)
		return nil, fmt.Errorf("failed aliveness check: %v with api endpoint: %s", err, url)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Cluster aliveness test failed. Status: %s \n", resp.Status)
		errMessage := fmt.Sprintf("Response code '%d' != '%d'", resp.StatusCode, http.StatusOK)
		return nil, fmt.Errorf("failed aliveness check: %v with api endpoint: %s, error msg: %s", err, url, errMessage)
	}

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed to read cluster aliveness test: %s \n", err)
		return nil, fmt.Errorf("failed aliveness check: %v with api endpoint: %s", err, url)
	}

	healthcheckResponse := &HealthcheckResponse{}
	err = json.Unmarshal(b, healthcheckResponse)
	if err != nil {
		fmt.Printf("Failed to umarshal cluster aliveness test result: %s \n", err)
		return nil, err
	}

	return healthcheckResponse, nil
}

type HealthcheckResponse struct {
	Status string `json:"status"`
}

func getUsernameAndPassword(ctx context.Context, clientset *kubernetes.Clientset, namespace, instanceName string) (string, string, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, fmt.Sprintf("%s-rabbitmq-admin", instanceName), metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}

	username, ok := secret.Data["username"]
	if !ok {
		return "", "", fmt.Errorf("cannot find 'username' in %s-rabbitmq-admin", instanceName)
	}

	password, ok := secret.Data["password"]
	if !ok {
		return "", "", fmt.Errorf("cannot find 'password' in %s-rabbitmq-admin", instanceName)
	}
	return string(username), string(password), nil
}

func generateRabbitmqCluster(namespace, instanceName string) *rabbitmqv1beta1.RabbitmqCluster {
	one := int32(1)
	return &rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceName,
			Namespace: namespace,
		},
		Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
			Replicas: &one,
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
	return cluster.ChildResourceName(strings.Join([]string{statefulSetSuffix, strconv.Itoa(index)}, "-"))
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

func rabbitmqNodePort(ctx context.Context, clientSet *kubernetes.Clientset, cluster *rabbitmqv1beta1.RabbitmqCluster, portName string) string {
	service, err := clientSet.CoreV1().Services(cluster.Namespace).
		Get(ctx, cluster.ChildResourceName("client"), metav1.GetOptions{})

	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	for _, port := range service.Spec.Ports {
		if port.Name == portName {
			return strconv.Itoa(int(port.NodePort))
		}
	}

	return ""
}

func waitForTLSUpdate(cluster *rabbitmqv1beta1.RabbitmqCluster) {
	waitForRabbitmqNotRunningWithOffset(cluster, 2)
	waitForRabbitmqRunning(cluster)
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
			Expect(string(output)).To(ContainSubstring("not found"))
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
			Expect(string(output)).To(ContainSubstring("not found"))
		}

		return string(output)
	}, podCreationTimeout, 1).Should(Equal("'True'"))

	ExpectWithOffset(callStackOffset, err).NotTo(HaveOccurred())
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
	EventuallyWithOffset(1, func() int {
		client := &http.Client{Timeout: 10 * time.Second}
		url := fmt.Sprintf("http://%s:%s", hostname, port)

		req, _ := http.NewRequest(http.MethodGet, url, nil)

		resp, err := client.Do(req)
		if err != nil {
			return 0
		}
		defer resp.Body.Close()

		return resp.StatusCode
	}, podCreationTimeout, 5).Should(Equal(http.StatusOK))
}

func createTLSSecret(secretName, secretNamespace, hostname string) string {
	// create key and crt files
	tmpDir := os.TempDir()
	serverCertPath := filepath.Join(tmpDir, "server.crt")
	serverCertFile, err := os.OpenFile(serverCertPath, os.O_CREATE|os.O_RDWR, 0755)
	Expect(err).ToNot(HaveOccurred())

	serverKeyPath := filepath.Join(tmpDir, "server.key")
	serverKeyFile, err := os.OpenFile(serverKeyPath, os.O_CREATE|os.O_RDWR, 0755)
	Expect(err).ToNot(HaveOccurred())

	caCertPath := filepath.Join(tmpDir, "ca.crt")
	caCertFile, err := os.OpenFile(caCertPath, os.O_CREATE|os.O_RDWR, 0755)
	Expect(err).ToNot(HaveOccurred())

	// generate and write cert and key to file
	Expect(createCertificateChain(hostname, caCertFile, serverCertFile, serverKeyFile)).To(Succeed())
	// create k8s tls secret
	Expect(k8sCreateSecretTLS(secretName, secretNamespace, serverCertPath, serverKeyPath)).To(Succeed())

	// remove server files
	Expect(os.Remove(serverKeyPath)).To(Succeed())
	Expect(os.Remove(serverCertPath)).To(Succeed())
	return caCertPath
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
		Expect(string(output)).To(ContainSubstring("not found"))
		return false, nil
	}

	return true, nil
}

func k8sCreateSecretTLS(secretName, secretNamespace, certPath, keyPath string) error {
	// delete secret if it exists
	secretExists, err := k8sSecretExists(secretName, secretNamespace)
	Expect(err).NotTo(HaveOccurred())
	if secretExists {
		Expect(k8sDeleteSecret(secretName, secretNamespace)).To(Succeed())
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
		return fmt.Errorf("Failed with error: %v\nOutput: %v\n", err.Error(), string(output))
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
		return fmt.Errorf("Failed with error: %v\nOutput: %v\n", err.Error(), string(output))
	}

	return nil
}

// creates a CA cert, and uses it to sign another cert
func createCertificateChain(hostname string, caCertWriter, certWriter, keyWriter io.Writer) error {
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
		return err
	}

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

	_, err = caCertWriter.Write(caCert)
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

func publishAndConsumeMQTTMsg(hostname, nodePort, username, password string, overWebSocket bool) {
	url := fmt.Sprintf("tcp://%s:%s", hostname, nodePort)
	if overWebSocket {
		url = fmt.Sprintf("ws://%s:%s/ws", hostname, nodePort)
	}
	opts := mqtt.NewClientOptions().
		AddBroker(url).
		SetUsername(username).
		SetPassword(password).
		SetClientID("system tests MQTT plugin").
		SetProtocolVersion(4) // RabbitMQ MQTT plugin targets MQTT 3.1.1

	c := mqtt.NewClient(opts)

	var token mqtt.Token
	for retry := 0; retry < 5; retry++ {
		fmt.Printf("Attempt #%d to connect using MQTT\n", retry)
		token = c.Connect()
		// Waits for the network request to reach the destination and receive a response
		Expect(token.WaitTimeout(3 * time.Second)).To(BeTrue())

		if err := token.Error(); err == nil {
			break
		}

		time.Sleep(2 * time.Second)
	}

	Expect(token.Error()).ToNot(HaveOccurred())

	topic := "tests/mqtt"
	msgReceived := false

	handler := func(client mqtt.Client, msg mqtt.Message) {
		defer GinkgoRecover()
		Expect(msg.Topic()).To(Equal(topic))
		Expect(string(msg.Payload())).To(Equal("test message MQTT"))
		msgReceived = true
	}

	token = c.Subscribe(topic, 0, handler)
	Expect(token.Wait()).To(BeTrue())
	Expect(token.Error()).ToNot(HaveOccurred())

	token = c.Publish(topic, 0, false, "test message MQTT")
	Expect(token.Wait()).To(BeTrue())
	Expect(token.Error()).ToNot(HaveOccurred())

	Eventually(func() bool {
		return msgReceived
	}, 5*time.Second).Should(BeTrue())

	token = c.Unsubscribe(topic)
	Expect(token.Wait()).To(BeTrue())
	Expect(token.Error()).ToNot(HaveOccurred())

	c.Disconnect(250)
}

func publishAndConsumeSTOMPMsg(hostname, stompNodePort, username, password string) {
	var conn *stomp.Conn
	var err error
	for retry := 0; retry < 5; retry++ {
		fmt.Printf("Attempt #%d to connect using STOMP\n", retry)
		conn, err = stomp.Dial("tcp",
			fmt.Sprintf("%s:%s", hostname, stompNodePort),
			stomp.ConnOpt.Login(username, password),
			stomp.ConnOpt.AcceptVersion(stomp.V12), // RabbitMQ STOMP plugin supports STOMP versions 1.0 through 1.2
			stomp.ConnOpt.Host("/"),                // default virtual host
		)

		if err == nil {
			break
		}

		time.Sleep(2 * time.Second)
	}
	Expect(err).ToNot(HaveOccurred())

	queue := "/queue/system-tests-stomp"
	sub, err := conn.Subscribe(queue, stomp.AckAuto)
	Expect(err).ToNot(HaveOccurred())

	msgReceived := false
	go func() {
		defer GinkgoRecover()
		var msg *stomp.Message
		Eventually(sub.C, 5*time.Second).Should(Receive(&msg))
		Expect(msg.Err).ToNot(HaveOccurred())
		Expect(string(msg.Body)).To(Equal("test message STOMP"))
		msgReceived = true
	}()

	Expect(conn.Send(queue, "text/plain", []byte("test message STOMP"), nil)).To(Succeed())
	Eventually(func() bool {
		return msgReceived
	}, 5*time.Second).Should(BeTrue())
	Expect(sub.Unsubscribe()).To(Succeed())
	Expect(conn.Disconnect()).To(Succeed())
}
