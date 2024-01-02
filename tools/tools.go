//go:build tools
// +build tools

package tools

import (
	_ "github.com/elastic/crd-ref-docs"
	_ "github.com/onsi/ginkgo/v2/ginkgo"
	_ "github.com/sclevine/yj"
	_ "golang.org/x/vuln/cmd/govulncheck"
	_ "sigs.k8s.io/controller-runtime/tools/setup-envtest"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
	_ "sigs.k8s.io/kind"
	_ "sigs.k8s.io/kustomize/kustomize/v5"
)
