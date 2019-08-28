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
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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

	// create the clientset
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

func kubectlExec(namespace, podname, cmd string, args ...string) error {
	kubectlArgs := []string{
		"-n",
		namespace,
		"exec",
		"-it",
		podname,
		"--",
		cmd,
	}

	kubectlArgs = append(kubectlArgs, args...)

	kubectlCmd := exec.Command("kubectl", kubectlArgs...)
	err := kubectlCmd.Run()
	return err
}

func kubectlDelete(namespace, object, objectName string) error {
	kubectlArgs := []string{
		"-n",
		namespace,
		"delete",
		object,
		objectName,
	}

	kubectlCmd := exec.Command("kubectl", kubectlArgs...)
	err := kubectlCmd.Run()
	return err
}

func getExternalIP(clientSet *kubernetes.Clientset, namespace, serviceName string) (string, error) {
	service, err := clientSet.CoreV1().Services(namespace).Get(serviceName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	if len(service.Status.LoadBalancer.Ingress) == 0 {
		return "", nil
	}

	ip := service.Status.LoadBalancer.Ingress[0].IP
	return ip, nil
}

func endpointPoller(clientSet *kubernetes.Clientset, namespace, endpointName string) int {
	endpoints, err := clientSet.CoreV1().Endpoints(namespace).Get(endpointName, metav1.GetOptions{})

	if err != nil {
		fmt.Printf("Failed to Get endpoint %s: %v", endpointName, err)
		return -1
	}

	ret := 0
	for _, endpointSubset := range endpoints.Subsets {
		ret = ret + len(endpointSubset.Addresses)
	}

	return ret
}

func makeRequest(url, httpMethod, rabbitmqUsername, rabbitmqPassword string, body []byte) (responseBody []byte, err error) {

	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest(httpMethod, url, bytes.NewReader(body))
	req.SetBasicAuth(rabbitmqUsername, rabbitmqPassword)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed to run cluster aliveness test: %+v \n", err)
		return responseBody, fmt.Errorf("failed aliveness check: %v with api endpoint: %s", err, url)
	}
	defer resp.Body.Close()
	responseBody, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return responseBody, err
	}

	if resp.StatusCode >= 400 {
		return responseBody, fmt.Errorf("Make request failed with api endpoint: %s and statusCode: %d", url, resp.StatusCode)
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
	json.Unmarshal(response, &messages)

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
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed to read cluster aliveness test: %s \n", err)
		return nil, fmt.Errorf("failed aliveness check: %v with api endpoint: %s", err, url)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Cluster aliveness test failed. Status: %s \n", resp.Status)
		errMessage := fmt.Sprintf("Response code '%d' != '%d'", resp.StatusCode, http.StatusOK)
		return nil, fmt.Errorf("failed aliveness check: %v with api endpoint: %s, error msg: %s", err, url, errMessage)
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

func getRabbitmqUsernameAndPassword(clientset *kubernetes.Clientset, namespace, instanceName, keyName string) (string, string, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(fmt.Sprintf("%s-rabbitmq-secret", instanceName), metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}

	username, ok := secret.Data["rabbitmq-username"]
	if !ok {
		return "", "", fmt.Errorf("cannot find 'rabbitmq-username' in rabbitmq-secret")
	}

	password, ok := secret.Data["rabbitmq-password"]
	if !ok {
		return "", "", fmt.Errorf("cannot find 'rabbitmq-password' in rabbitmq-secret")
	}
	return string(username), string(password), nil
}

func checkPodStatus(clientSet *kubernetes.Clientset, namespace, podName string) (string, error) {
	pod, err := clientSet.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%v", pod.Status.Conditions), nil
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
