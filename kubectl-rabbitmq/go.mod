module github.com/rabbitmq/cluster-operator/kubectl-rabbitmq

go 1.27.4

require (
	github.com/rabbitmq/cluster-operator/v2 v2.23.2
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.12.1
	k8s.io/api v0.37.2
	k8s.io/apimachinery v0.37.2
	sigs.k8s.io/yaml v1.6.0
)

replace github.com/rabbitmq/cluster-operator/v2 => ../

require (
	github.com/fxamacker/cbor/v2 v2.10.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/spf13/pflag v1.1.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.yaml.in/yaml/v2 v2.4.5 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.0.0-20261104182252-5bee61fc5225 // indirect
	k8s.io/utils v0.0.0-20261023123148-ca04ba58ecab // indirect
	sigs.k8s.io/controller-runtime v0.25.2 // indirect
	sigs.k8s.io/json v0.0.0-20260909141634-11ed52e25bc5 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.4.2 // indirect
)
