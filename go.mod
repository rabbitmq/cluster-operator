module github.com/rabbitmq/cluster-operator

go 1.16

require (
	github.com/cloudflare/cfssl v1.6.0
	github.com/eclipse/paho.mqtt.golang v1.3.5
	github.com/elastic/crd-ref-docs v0.0.7
	github.com/go-delve/delve v1.6.1
	github.com/go-logr/logr v0.4.0
	github.com/go-stomp/stomp v2.1.4+incompatible
	github.com/michaelklishin/rabbit-hole/v2 v2.10.0
	github.com/mikefarah/yq/v4 v4.9.6
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/rabbitmq/rabbitmq-stream-go-client v0.0.0-20210422170636-520637be5dde
	github.com/sclevine/yj v0.0.0-20200815061347-554173e71934
	github.com/streadway/amqp v1.0.0
	golang.org/x/net v0.0.0-20210428140749-89ef3d95e781
	gopkg.in/ini.v1 v1.62.0
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/controller-runtime v0.9.1
	sigs.k8s.io/controller-tools v0.6.1
	sigs.k8s.io/kind v0.11.1
	sigs.k8s.io/kustomize/kustomize/v3 v3.10.0
)
