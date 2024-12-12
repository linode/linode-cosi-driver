//go:build never
// +build never

package hack

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/kyverno/chainsaw"
	_ "github.com/losisin/helm-values-schema-json"
	_ "github.com/norwoodj/helm-docs/cmd/helm-docs"
	_ "github.com/tilt-dev/ctlptl/cmd/ctlptl"
	_ "golang.stackrox.io/kube-linter/cmd/kube-linter"
	_ "helm.sh/helm/v3/cmd/helm"
	_ "sigs.k8s.io/kind"
)
