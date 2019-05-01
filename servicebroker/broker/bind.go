package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/brokerapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func createKubernetesClient() (*kubernetes.Clientset, error) {
	var err error
	var config *rest.Config
	if len(os.Getenv("KUBERNETES_SERVICE_HOST")) > 0 {
		// creates the in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("Failed to create in-cluster config: %s", err)
		}
	} else {
		var kubeconfig string
		if len(os.Getenv("KUBECONFIG")) > 0 {
			kubeconfig = os.Getenv("KUBECONFIG")
		} else {
			kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube/config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("Failed to create out of cluster config: %s", err)
		}
	}

	// creates the clientset
	//TODO: improve this to share common things with provission as well.
	return kubernetes.NewForConfig(config)
}

func (broker RabbitMQServiceBroker) Bind(ctx context.Context, instanceID, bindingID string, details brokerapi.BindDetails, asyncAllowed bool) (brokerapi.Binding, error) {
	vhost := instanceID

	kubernetesClient, err := createKubernetesClient()
	if err != nil {
		return brokerapi.Binding{}, fmt.Errorf("Failed to create kubernetes client: %s", err)
	}

	getOptions := metav1.GetOptions{}
	service, err := kubernetesClient.CoreV1().Services("rabbitmq-for-kubernetes").Get(fmt.Sprintf("p-%s-rabbitmq", vhost), getOptions)
	fmt.Printf("Service object: %+v", service)
	if err != nil {
		return brokerapi.Binding{}, fmt.Errorf("Failed to retrieve service: %s", err)
	}

	var serviceIP string
	if len(service.Status.LoadBalancer.Ingress) > 0 {
		serviceIP = service.Status.LoadBalancer.Ingress[0].IP
	}

	credsBuilder := Builder{
		MgmtDomain:    fmt.Sprintf("%s:%d", serviceIP, 15672),
		Hostnames:     []string{serviceIP},
		VHost:         vhost,
		Username:      broker.Config.RabbitMQ.Administrator.Username,
		Password:      broker.Config.RabbitMQ.Administrator.Password,
		TLS:           bool(broker.Config.RabbitMQ.TLS),
		ProtocolPorts: map[string]int{"amqp": 4567, "clustering": 25672, "http": 15672},
	}

	credentials, err := credsBuilder.Build()
	if err != nil {
		return brokerapi.Binding{}, err
	}

	return brokerapi.Binding{Credentials: credentials}, nil

}

type Builder struct {
	MgmtDomain    string
	Hostnames     []string
	VHost         string
	Username      string
	Password      string
	TLS           bool
	ProtocolPorts map[string]int // key=protocol, value=port, e.g. "amqp": 4567
}

func (b Builder) Build() (output interface{}, err error) {
	bind := binding{
		VHost:        b.VHost,
		Username:     b.Username,
		Password:     b.Password,
		DashboardURL: b.dashboardURL(),
		Hostname:     b.Hostnames[0],
		Hostnames:    b.Hostnames,
		HTTPAPIURI:   b.httpapiuriForBinding(),
		HTTPAPIURIs:  b.httpapiurisForBinding(),
		URI:          b.uriForBinding(b.Hostnames[0]),
		URIs:         b.urisForBinding(),
		TLS:          b.TLS,
		Protocols:    b.protocols(),
	}

	bytes, err := json.Marshal(bind)
	if err != nil {
		return output, err
	}

	err = json.Unmarshal(bytes, &output)
	return output, err
}

func (b Builder) dashboardURL() string {
	return fmt.Sprintf("http://%s/#/login/%s/%s", b.MgmtDomain, b.Username, b.Password)
}

func (b Builder) uriForBinding(hostname string) string {
	return fmt.Sprintf("%s://%s:%s@%s/%s", "amqp", b.Username, b.Password, hostname, b.VHost)
}

func (b Builder) urisForBinding() []string {
	var uris []string
	for _, hostname := range b.Hostnames {
		uris = append(uris, b.uriForBinding(hostname))
	}
	return uris
}

func (b Builder) httpapiuriForBinding() string {
	return fmt.Sprintf("http://%s:%s@%s/api/", b.Username, b.Password, b.MgmtDomain)
}

func (b Builder) httpapiurisForBinding() []string {
	return []string{b.httpapiuriForBinding()}
}

type binding struct {
	DashboardURL string    `json:"dashboard_url"`
	Username     string    `json:"username"`
	Password     string    `json:"password"`
	Hostname     string    `json:"hostname"`
	Hostnames    []string  `json:"hostnames"`
	HTTPAPIURI   string    `json:"http_api_uri"`
	HTTPAPIURIs  []string  `json:"http_api_uris"`
	URI          string    `json:"uri"`
	URIs         []string  `json:"uris"`
	VHost        string    `json:"vhost"`
	TLS          bool      `json:"ssl"`
	Protocols    protocols `json:"protocols"`
}

type protocols map[string]protocol

type protocol struct {
	Username  string   `json:"username"`
	Password  string   `json:"password"`
	VHost     string   `json:"vhost,omitempty"`
	Hostname  string   `json:"host"`
	Hostnames []string `json:"hosts"`
	URI       string   `json:"uri"`
	URIs      []string `json:"uris"`
	Port      int      `json:"port"`
	TLS       bool     `json:"ssl"`
	Path      string   `json:"path,omitempty"`
}

func (b Builder) protocols() protocols {
	ps := make(protocols)
	for protocol, port := range b.ProtocolPorts {
		switch protocol {
		case "amqp":
			ps["amqp"] = b.addAMQPProtocol(port, false)
		}
	}
	ps["management"] = b.addMgmtProtocol()

	return ps
}

func (b Builder) addAMQPProtocol(port int, tls bool) protocol {
	return protocol{
		Username:  b.Username,
		Password:  b.Password,
		VHost:     b.VHost,
		Hostname:  b.Hostnames[0],
		Hostnames: b.Hostnames,
		URI:       b.uriForAMQP(b.Hostnames[0], port),
		URIs:      b.urisForAMQP(port),
		Port:      port,
		TLS:       tls,
	}
}

func (b Builder) uriForAMQP(hostname string, port int) string {
	return fmt.Sprintf("%s://%s:%s@%s:%d/%s", "amqp", b.Username, b.Password, hostname, port, b.VHost)
}

func (b Builder) urisForAMQP(port int) []string {
	var uris []string
	for _, hostname := range b.Hostnames {
		uris = append(uris, b.uriForAMQP(hostname, port))
	}
	return uris
}

func (b Builder) addMgmtProtocol() protocol {
	return protocol{
		Username:  b.Username,
		Password:  b.Password,
		Hostname:  b.Hostnames[0],
		Hostnames: b.Hostnames,
		URI:       b.uriForManagement(b.Hostnames[0], 15672),
		URIs:      b.urisForManagement(15672),
		Port:      15672,
		TLS:       false,
		Path:      "/api/",
	}
}

func (b Builder) uriForManagement(hostname string, port int) string {
	return fmt.Sprintf("http://%s:%s@%s:%d/api/", b.Username, b.Password, hostname, port)
}

func (b Builder) urisForManagement(port int) []string {
	var uris []string
	for _, hostname := range b.Hostnames {
		uris = append(uris, b.uriForManagement(hostname, port))
	}
	return uris
}
