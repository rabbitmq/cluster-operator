module github.com/rabbitmq/cluster-operator

go 1.16

require (
	github.com/cloudflare/cfssl v1.6.0
	github.com/eclipse/paho.mqtt.golang v1.3.5
	github.com/elastic/crd-ref-docs v0.0.7
	github.com/go-delve/delve v1.7.0
	github.com/go-logr/logr v0.4.0
	github.com/go-stomp/stomp v2.1.4+incompatible
	github.com/michaelklishin/rabbit-hole/v2 v2.10.0
	github.com/mikefarah/yq/v4 v4.11.2
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/rabbitmq/rabbitmq-stream-go-client v0.0.0-20210422170636-520637be5dde
	github.com/sclevine/yj v0.0.0-20200815061347-554173e71934
	github.com/streadway/amqp v1.0.0
	golang.org/x/net v0.0.0-20210520170846-37e1c6afe023
	gopkg.in/ini.v1 v1.62.0
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.22.0
	k8s.io/client-go v0.21.3
	k8s.io/utils v0.0.0-20210722164352-7f3ee0f31471
	sigs.k8s.io/controller-runtime v0.9.6
	sigs.k8s.io/controller-runtime/tools/setup-envtest v0.0.0-20210623192810-985e819db7af
	sigs.k8s.io/controller-tools v0.6.2
	sigs.k8s.io/kind v0.11.1
	sigs.k8s.io/kustomize/kustomize/v3 v3.10.0
)
