module github.com/pivotal/rabbitmq-for-kubernetes

go 1.12

require (
	github.com/Azure/go-autorest/autorest v0.9.0 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/gophercloud/gophercloud v0.3.0 // indirect
	github.com/onsi/ginkgo v1.6.0
	github.com/onsi/gomega v1.4.2
	golang.org/x/net v0.0.0-20180906233101-161cd47e91fd
	golang.org/x/sys v0.0.0-20190422165155-953cdadca894 // indirect
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	sigs.k8s.io/controller-runtime v0.2.2
)
