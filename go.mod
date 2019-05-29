module github.com/pivotal/rabbitmq-for-kubernetes

go 1.12

require (
	github.com/alecthomas/template v0.0.0-20160405071501-a0175ee3bccc // indirect
	github.com/alecthomas/units v0.0.0-20151022065526-2efee857e7cf // indirect
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.6.0
	github.com/onsi/gomega v1.4.2
	github.com/prometheus/common v0.0.0-20180801064454-c7de2306084e
	github.com/sirupsen/logrus v1.4.2 // indirect
	golang.org/x/net v0.0.0-20180906233101-161cd47e91fd
	gopkg.in/alecthomas/kingpin.v2 v2.2.6 // indirect
	k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	sigs.k8s.io/controller-runtime v0.2.0-beta.1
)
