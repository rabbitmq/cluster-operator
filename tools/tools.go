// +build tools

package tools

import (
	_ "github.com/elastic/crd-ref-docs"
	_ "github.com/go-delve/delve/cmd/dlv"
	_ "github.com/onsi/ginkgo/ginkgo"
	_ "github.com/sclevine/yj"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
	_ "sigs.k8s.io/kind"
	_ "sigs.k8s.io/kustomize/kustomize/v3"
)
