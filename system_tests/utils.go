package system_tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	. "github.com/onsi/gomega"
)

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
	client := &http.Client{Timeout: 5 * time.Second}
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

func rabbitmqGetMessageFromQueue(rabbitmqHostName, rabbitmqUsername, rabbitmqPassword string) (*Message, error) {
	getQueuesUrl := fmt.Sprintf("http://%s:15672/api/queues/%%2F/test-queue/get", rabbitmqHostName)
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

	messages := []Message{}
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

func rabbitmqPublishToNewQueue(rabbitmqHostName, rabbitmqUsername, rabbitmqPassword string) error {
	url := fmt.Sprintf("http://%s:15672/api/queues/%%2F/test-queue", rabbitmqHostName)
	_, err := makeRequest(url, http.MethodPut, rabbitmqUsername, rabbitmqPassword, []byte("{\"durable\": true}"))

	if err != nil {
		return err
	}

	url = fmt.Sprintf("http://%s:15672/api/exchanges/%%2F/amq.default/publish", rabbitmqHostName)
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

func rabbitmqAlivenessTest(rabbitmqHostName, rabbitmqUsername, rabbitmqPassword string) (*HealthcheckResponse, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("http://%s:15672/api/aliveness-test/%%2F", rabbitmqHostName)

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

func getRabbitmqUsernameAndPassword(clientset *kubernetes.Clientset, namespace, instanceName string) (string, string, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(fmt.Sprintf("%s-rabbitmq-admin", instanceName), metav1.GetOptions{})
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
	return &rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceName,
			Namespace: namespace,
		},
		Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
			Replicas: 1,
		},
	}
}

func createRabbitmqCluster(client client.Client, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	return client.Create(context.TODO(), rabbitmqCluster)
}

func statefulSetPodName(cluster *rabbitmqv1beta1.RabbitmqCluster, index int) string {
	return cluster.ChildResourceName(strings.Join([]string{statefulSetSuffix, strconv.Itoa(index)}, "-"))
}

func rabbitmqHostname(clientSet *kubernetes.Clientset, cluster *rabbitmqv1beta1.RabbitmqCluster) string {
	service, err := clientSet.CoreV1().Services(cluster.Namespace).Get(cluster.ChildResourceName(ingressServiceSuffix), metav1.GetOptions{})
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, len(service.Status.LoadBalancer.Ingress)).To(BeNumerically(">", 0))

	return service.Status.LoadBalancer.Ingress[0].IP
}

func waitForRabbitmqRunning(cluster *rabbitmqv1beta1.RabbitmqCluster) {
	var err error

	EventuallyWithOffset(1, func() string {
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

	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func waitForLoadBalancer(clientSet *kubernetes.Clientset, cluster *rabbitmqv1beta1.RabbitmqCluster) {
	var err error

	EventuallyWithOffset(1, func() string {
		svc, err := clientSet.CoreV1().Services(cluster.Namespace).Get(cluster.ChildResourceName(ingressServiceSuffix), metav1.GetOptions{})

		if err != nil {
			Expect(err).To(MatchError("not found"))
			return ""
		}

		if len(svc.Status.LoadBalancer.Ingress) == 0 || svc.Status.LoadBalancer.Ingress[0].IP == "" {
			return ""
		}

		endpoints, _ := clientSet.CoreV1().Endpoints(cluster.Namespace).Get(cluster.ChildResourceName(ingressServiceSuffix), metav1.GetOptions{})

		for _, e := range endpoints.Subsets {
			if len(e.NotReadyAddresses) > 0 || int32(len(e.Addresses)) != cluster.Spec.Replicas {
				return ""
			}
		}

		return svc.Status.LoadBalancer.Ingress[0].IP
	}, podCreationTimeout, 1).ShouldNot(BeEmpty())

	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func assertStatefulSetReady(cluster *rabbitmqv1beta1.RabbitmqCluster) {
	numReplicas := cluster.Spec.Replicas

	EventuallyWithOffset(1, func() []byte {
		output, err := statefulSetStatus(cluster)

		if err != nil {
			Expect(string(output)).To(ContainSubstring("not found"))
		}

		return output
	}, podCreationTimeout*time.Duration(numReplicas), 1).Should(ContainSubstring(fmt.Sprintf("%d/%d", numReplicas, numReplicas)))
}

func statefulSetStatus(cluster *rabbitmqv1beta1.RabbitmqCluster) ([]byte, error) {
	return kubectl(
		"-n",
		cluster.Namespace,
		"get",
		"sts",
		cluster.ChildResourceName(statefulSetSuffix),
	)
}

func assertHttpReady(hostname string) {
	EventuallyWithOffset(1, func() int {
		client := &http.Client{Timeout: 5 * time.Second}
		url := fmt.Sprintf("http://%s:15672", hostname)

		req, _ := http.NewRequest(http.MethodGet, url, nil)

		resp, err := client.Do(req)
		if err != nil {
			return 0
		}
		defer resp.Body.Close()

		return resp.StatusCode
	}, podCreationTimeout, 5).Should(Equal(http.StatusOK))
}
