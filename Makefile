# Copyright 2023 Akamai Technologies, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Image URL to use all building/pushing image targets
IMG ?= docker.io/linode/linode-cosi-driver
TAG ?= dev-v$(shell date +%y%m%d-%H%M%S)-$(shell git rev-parse HEAD | cut -c1-6)
PLATFORM ?= linux/$(shell go env GOARCH)
CHAINSAW_ARGS ?=

# Versions of COSI dependencies
CRD_VERSION := v0.1.0
CONTROLLER_VERSION := v0.1.2-alpha1

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# flags for
GOFLAGS += -trimpath
LDFLAGS += -X ${MODULE_NAME}/pkg/version.Version=${VERSION} -s -w -extldflags "-static"
GO_SETTINGS += CGO_ENABLED=0

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: generate

.PHONY: clean
clean:
	-rm -r bin/

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

export PATH := $(LOCALBIN):$(PATH)

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: generate
generate: gowrap # Generate code.
	go generate ./...

.PHONY: build
build: generate # Build the binary.
	${GO_SETTINGS} go build \
		${GOFLAGS} \
		-ldflags="${LDFLAGS}" \
		-o ./bin/linode-cosi-driver \
		./cmd/linode-cosi-driver

.PHONY: generate-docs
generate-docs: helm-docs ## Run kube-linter on Kubernetes manifests.
	$(HELM_DOCS) --badge-style=flat

.PHONY: generate-schemas
generate-schemas: helm-values-schema-json ## Run generate schema for Helm Chart values.
	$(HELM_VALUES_SCHEMA_JSON) \
		-draft=7 \
		-indent=2 \
		-input=helm/linode-cosi-driver/values.yaml \
		-output=helm/linode-cosi-driver/values.schema.json \

.PHONY: test
test: generate ## Run tests.
	go test \
		-race \
		-cover -covermode=atomic -coverprofile=coverage.out \
		./...

.PHONY: test-integration
test-integration: generate ## Run integration tests.
	go test \
		-tags=integration \
		-race \
		-cover -covermode=atomic -coverprofile=coverage.out \
		./...

.PHONY: test-e2e
test-e2e: chainsaw ## Run the e2e tests against a k8s instance using Kyverno Chainsaw.
	$(CHAINSAW) test ${CHAINSAW_ARGS}

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter.
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes.
	$(GOLANGCI_LINT) run --fix

.PHONY: lint-manifests
lint-manifests: kube-linter ## Run kube-linter on Kubernetes manifests.
	$(KUBE_LINTER) lint --config=helm/.kube-linter.yaml ./helm/**

.PHONY: hadolint
hadolint: ## Run hadolint on Dockerfile
	$(CONTAINER_TOOL) run --rm -i hadolint/hadolint < Dockerfile

##@ CI

.PHONY: diff
diff: ## Run git diff-index to check if any changes are made.
	git --no-pager diff HEAD --

##@ Build

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build \
		--platform=${PLATFORM} \
		--tag=${IMG}:${TAG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}:${TAG}

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: cluster
cluster: kind ctlptl
	$(CTLPTL) apply -f hack/kind.yaml

.PHONY: cluster-reset
cluster-reset: kind ctlptl
	$(CTLPTL) delete -f hack/kind.yaml

.PHONY: deploy-deps
deploy-deps: ## Deploy all dependencies of Linode COSI Driver. This step installs CRDs and Controller.
	kubectl apply -k github.com/kubernetes-sigs/container-object-storage-interface-api/?ref=${CRD_VERSION}
	kubectl apply -k github.com/kubernetes-sigs/container-object-storage-interface-controller/?ref=${CONTROLLER_VERSION}

.PHONY: undeploy-deps
undeploy-deps: ## Deploy all dependencies of Linode COSI Driver. This step installs CRDs and Controller.
	kubectl delete -k github.com/kubernetes-sigs/container-object-storage-interface-controller/?ref=${CONTROLLER_VERSION}
	kubectl delete -k github.com/kubernetes-sigs/container-object-storage-interface-api/?ref=${CRD_VERSION}

.PHONY: deploy
deploy: helm ## Deploy driver to the K8s cluster specified in ~/.kube/config.
	$(HELM) upgrade --install \
		linode-cosi-driver \
		./helm/linode-cosi-driver \
			--set=apiToken=$$LINODE_TOKEN \
			--set=driver.image.repository=${IMG} \
			--set=driver.image.tag=${TAG}

.PHONY: undeploy
undeploy: helm ## Undeploy driver from the K8s cluster specified in ~/.kube/config.
	$(HELM) uninstall linode-cosi-driver

##@ Dependencies

## Tool Binaries
KUBECTL ?= kubectl
CHAINSAW ?= $(LOCALBIN)/chainsaw
CTLPTL ?= $(LOCALBIN)/ctlptl
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
GOWRAP ?= $(LOCALBIN)/gowrap
HELM ?= $(LOCALBIN)/helm
HELM_DOCS ?= $(LOCALBIN)/helm-docs
HELM_VALUES_SCHEMA_JSON ?= $(LOCALBIN)/helm-values-schema-json
KIND ?= $(LOCALBIN)/kind
KUBE_LINTER ?= $(LOCALBIN)/kube-linter

## Tool Versions
CHAINSAW_VERSION ?= $(shell grep 'github.com/kyverno/chainsaw ' ./go.mod | cut -d ' ' -f 2)
CTLPTL_VERSION ?= $(shell grep 'github.com/tilt-dev/ctlptl ' ./go.mod | cut -d ' ' -f 2)
GOLANGCI_LINT_VERSION ?= $(shell grep 'github.com/golangci/golangci-lint ' ./go.mod | cut -d ' ' -f 2)
GOWRAP_VERSION ?= $(shell grep 'github.com/hexdigest/gowrap ' ./go.mod | cut -d ' ' -f 2)
HELM_VERSION ?= $(shell grep 'helm.sh/helm/v3 ' ./go.mod | cut -d ' ' -f 2)
HELM_DOCS_VERSION ?= $(shell grep 'github.com/norwoodj/helm-docs ' ./go.mod | cut -d ' ' -f 2)
HELM_VALUES_SCHEMA_JSON_VERSION ?= $(shell grep 'github.com/losisin/helm-values-schema-json ' ./go.mod | cut -d ' ' -f 2)
KIND_VERSION ?= $(shell grep 'sigs.k8s.io/kind ' ./go.mod | cut -d ' ' -f 2)
KUBE_LINTER_VERSION ?= $(shell grep 'golang.stackrox.io/kube-linter ' ./go.mod | cut -d ' ' -f 2)

.PHONY: chainsaw
chainsaw: $(CHAINSAW)$(CHAINSAW_VERSION) ## Download chainsaw locally if necessary.
$(CHAINSAW)$(CHAINSAW_VERSION): $(LOCALBIN)
	$(call go-install-tool,$(CHAINSAW),github.com/kyverno/chainsaw,$(CHAINSAW_VERSION))

.PHONY: ctlptl
ctlptl: $(CTLPTL)$(CTLPTL_VERSION) ## Download ctlptl locally if necessary.
$(CTLPTL)$(CTLPTL_VERSION): $(LOCALBIN)
	$(call go-install-tool,$(CTLPTL),github.com/tilt-dev/ctlptl/cmd/ctlptl,$(CTLPTL_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT)$(GOLANGCI_LINT_VERSION) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT)$(GOLANGCI_LINT_VERSION): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

.PHONY: gowrap
gowrap: $(GOWRAP)$(GOWRAP_VERSION) ## Download gowrap locally if necessary.
$(GOWRAP)$(GOWRAP_VERSION): $(LOCALBIN)
	$(call go-install-tool,$(GOWRAP),github.com/hexdigest/gowrap/cmd/gowrap,$(GOWRAP_VERSION))

.PHONY: helm
helm: $(HELM)$(HELM_VERSION) ## Download helm locally if necessary.
$(HELM)$(HELM_VERSION): $(LOCALBIN)
	$(call go-install-tool,$(HELM),helm.sh/helm/v3/cmd/helm,$(HELM_VERSION))

.PHONY: helm-docs
helm-docs: $(HELM_DOCS)$(HELM_DOCS_VERSION) ## Download helm-docs locally if necessary.
$(HELM_DOCS)$(HELM_DOCS_VERSION): $(LOCALBIN)
	$(call go-install-tool,$(HELM_DOCS),github.com/norwoodj/helm-docs/cmd/helm-docs,$(HELM_DOCS_VERSION))

.PHONY: helm-values-schema-json
helm-values-schema-json: $(HELM_VALUES_SCHEMA_JSON)$(HELM_VALUES_SCHEMA_JSON_VERSION) ## Download helm-values-schema-json locally if necessary.
$(HELM_VALUES_SCHEMA_JSON)$(HELM_VALUES_SCHEMA_JSON_VERSION): $(LOCALBIN)
	$(call go-install-tool,$(HELM_VALUES_SCHEMA_JSON),github.com/losisin/helm-values-schema-json,$(HELM_VALUES_SCHEMA_JSON_VERSION))

.PHONY: kind
kind: $(KIND)$(KIND_VERSION) ## Download kind locally if necessary.
$(KIND)$(KIND_VERSION): $(LOCALBIN)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind,$(KIND_VERSION))

.PHONY: kube-linter
kube-linter: $(KUBE_LINTER)$(KUBE_LINTER_VERSION) ## Download kube-linter locally if necessary.
$(KUBE_LINTER)$(KUBE_LINTER_VERSION): $(LOCALBIN)
	$(call go-install-tool,$(KUBE_LINTER),golang.stackrox.io/kube-linter/cmd/kube-linter,$(KUBE_LINTER_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef
