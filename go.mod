module github.com/rabbitmq/cluster-operator

go 1.15

require (
	github.com/Azure/go-autorest/autorest v0.9.2 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.8.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/cloudflare/cfssl v1.5.0
	github.com/eclipse/paho.mqtt.golang v1.3.0
	github.com/go-delve/delve v1.5.1
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/go-stomp/stomp v2.1.0+incompatible
	github.com/gophercloud/gophercloud v0.5.0 // indirect
	github.com/michaelklishin/rabbit-hole/v2 v2.6.0
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.4
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/prometheus/client_golang v1.2.1 // indirect
	github.com/sclevine/yj v0.0.0-20200815061347-554173e71934
	github.com/smartystreets/goconvey v1.6.4 // indirect
	github.com/streadway/amqp v0.0.0-20200108173154-1c71cc93ed71
	go.uber.org/multierr v1.2.0 // indirect
	golang.org/x/net v0.0.0-20201202161906-c7110b5ffcbb
	gopkg.in/ini.v1 v1.62.0
	k8s.io/api v0.18.10
	k8s.io/apimachinery v0.18.10
	k8s.io/client-go v0.18.10
	k8s.io/utils v0.0.0-20200603063816-c1c6865ac451
	sigs.k8s.io/controller-runtime v0.6.4
	sigs.k8s.io/controller-tools v0.4.1
	sigs.k8s.io/kind v0.9.0
	sigs.k8s.io/kustomize/kustomize/v3 v3.8.8
)
