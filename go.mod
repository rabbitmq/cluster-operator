module github.com/rabbitmq/cluster-operator

go 1.15

require (
	github.com/cloudflare/cfssl v1.5.0
	github.com/eclipse/paho.mqtt.golang v1.3.2
	github.com/elastic/crd-ref-docs v0.0.7
	github.com/go-delve/delve v1.6.0
	github.com/go-logr/logr v0.3.0
	github.com/go-stomp/stomp v2.1.2+incompatible
	github.com/michaelklishin/rabbit-hole/v2 v2.6.0
	github.com/onsi/ginkgo v1.15.1
	github.com/onsi/gomega v1.10.5
	github.com/sclevine/yj v0.0.0-20200815061347-554173e71934
	github.com/streadway/amqp v0.0.0-20200108173154-1c71cc93ed71
	golang.org/x/net v0.0.0-20201202161906-c7110b5ffcbb
	gopkg.in/ini.v1 v1.62.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v0.20.2
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/controller-runtime v0.8.2
	sigs.k8s.io/controller-tools v0.5.0
	sigs.k8s.io/kind v0.10.0
	sigs.k8s.io/kustomize/kustomize/v3 v3.10.0
)
