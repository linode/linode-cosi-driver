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
IMG ?= localhost:5005/linode-cosi-driver
TAG ?= dev-$(shell git describe --match='' --always --abbrev=6 --dirty)
PLATFORM ?= linux/$(shell go env GOARCH)
CHAINSAW_ARGS ?=

# Versions of COSI dependencies
COSI_VERSION := 7ddc93baaa3f08c9c8990a17c7b958955d93c044

OS=$(shell uname -s | tr '[:upper:]' '[:lower:]')
TILT_OS=$(OS)
ifeq ($(TILT_OS),darwin)
TILT_OS=mac
endif
ARCH=$(shell uname -m)
ARCH_SHORT=$(ARCH)
ifeq ($(ARCH_SHORT),x86_64)
ARCH_SHORT := amd64
else ifeq ($(ARCH_SHORT),aarch64)
ARCH_SHORT := arm64
endif
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
all: build

.PHONY: clean
clean:
	-rm -r bin/

## Temporary compatibility path for pull_request_target workflows running from
## a base branch that predates the mise migration.
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

.PHONY: build
build: # Build the binary.
	${GO_SETTINGS} go build \
		${GOFLAGS} \
		-ldflags="${LDFLAGS}" \
		-o ./bin/linode-cosi-driver \
		./cmd/linode-cosi-driver

.PHONY: generate-docs
generate-docs: ## Generate Helm chart documentation.
	$(HELM_DOCS) --badge-style=flat

.PHONY: generate-schemas
generate-schemas: ## Generate the Helm chart values schema.
	$(HELM_VALUES_SCHEMA_JSON) \
		--draft=7 \
		--indent=2 \
		--values=helm/linode-cosi-driver/values.yaml \
		--output=helm/linode-cosi-driver/values.schema.json \

.PHONY: test
test: generate-mocks
	go test \
		-race \
		-cover -coverprofile=coverage.out \
		./...
	@if [ -f coverage.out ]; then \
		grep -v ".gen.go" coverage.out > coverage.filtered.out && mv coverage.filtered.out coverage.out; \
	fi

.PHONY: test-integration
test-integration: ## Run integration tests.
	go test \
		-tags=integration \
		-race \
		-cover -coverprofile=integration-coverage.out \
		./...
	@if [ -f integration-coverage.out ]; then \
		grep -v ".gen.go" integration-coverage.out > integration-coverage.filtered.out && mv integration-coverage.filtered.out integration-coverage.out; \
	fi

.PHONY: test-e2e
test-e2e: local-deploy chainsaw ## Run the e2e tests against a k8s instance using Kyverno Chainsaw.
	$(CHAINSAW) test ${CHAINSAW_ARGS}

.PHONY: lint
lint: ## Run golangci-lint linter.
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: ## Run golangci-lint linter and perform fixes.
	$(GOLANGCI_LINT) run --fix

.PHONY: lint-manifests
lint-manifests: ## Run kube-linter on Kubernetes manifests.
	$(KUBE_LINTER) lint --config=helm/.kube-linter.yaml ./helm/**

.PHONY: hadolint
hadolint: ## Run hadolint on Dockerfile
	$(CONTAINER_TOOL) run --rm -i hadolint/hadolint < Dockerfile

.PHONY: generate-mocks
generate-mocks:
	$(MOCKGEN) -source=./pkg/s3/s3.go -destination=./testing/mock/s3.gen.go -package=mock -typed -mock_names=Client=MockS3Client
	$(MOCKGEN) -source=./pkg/linodeclient/linodeclient.go -destination=./testing/mock/linodeclient.gen.go -package=mock -typed -mock_names=Client=MockLinodeClient

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
	kubectl apply -k github.com/kubernetes-sigs/container-object-storage-interface/?ref=${COSI_VERSION}

.PHONY: undeploy-deps
undeploy-deps: ## Deploy all dependencies of Linode COSI Driver. This step installs CRDs and Controller.
	kubectl delete -k github.com/kubernetes-sigs/container-object-storage-interface/?ref=${COSI_VERSION}

.PHONY: deploy
deploy: ## Deploy driver to the K8s cluster specified in ~/.kube/config.
	$(HELM) upgrade --install \
		linode-cosi-driver \
		./helm/linode-cosi-driver \
			--set=apiToken=$$LINODE_TOKEN \
			--set=driver.image.repository=${IMG} \
			--set=driver.image.tag=${TAG}

.PHONY: local-deploy
local-deploy: cluster tilt
	$(TILT) ci -f Tiltfile

.PHONY: undeploy
undeploy: ## Undeploy driver from the K8s cluster specified in ~/.kube/config.
	$(HELM) uninstall linode-cosi-driver

##@ Dependencies

## Tool Binaries
KUBECTL ?= kubectl
CHAINSAW                ?= chainsaw
CTLPTL                  ?= ctlptl
GOLANGCI_LINT           ?= golangci-lint
HELM                    ?= helm
HELM_DOCS               ?= helm-docs
HELM_VALUES_SCHEMA_JSON ?= helm-values-schema-json
KIND                    ?= kind
KUBE_LINTER             ?= kube-linter
MOCKGEN                 ?= mockgen
TILT                    ?= tilt

## Tool versions used only by the temporary pull_request_target fallback.
CHAINSAW_VERSION ?= v0.2.15
CTLPTL_VERSION   ?= v0.9.4
KIND_VERSION     ?= v0.29.0
TILT_VERSION     ?= 0.37.5

.PHONY: chainsaw
chainsaw: $(LOCALBIN)
	@if ! command -v "$(CHAINSAW)" >/dev/null 2>&1; then \
		echo "Installing github.com/kyverno/chainsaw@$(CHAINSAW_VERSION)"; \
		GOBIN=$(LOCALBIN) go install github.com/kyverno/chainsaw@$(CHAINSAW_VERSION); \
	fi

.PHONY: ctlptl
ctlptl: $(LOCALBIN)
	@if ! command -v "$(CTLPTL)" >/dev/null 2>&1; then \
		echo "Installing github.com/tilt-dev/ctlptl/cmd/ctlptl@$(CTLPTL_VERSION)"; \
		GOBIN=$(LOCALBIN) go install github.com/tilt-dev/ctlptl/cmd/ctlptl@$(CTLPTL_VERSION); \
	fi

.PHONY: kind
kind: $(LOCALBIN)
	@if ! command -v "$(KIND)" >/dev/null 2>&1; then \
		echo "Installing sigs.k8s.io/kind@$(KIND_VERSION)"; \
		GOBIN=$(LOCALBIN) go install sigs.k8s.io/kind@$(KIND_VERSION); \
	fi

.PHONY: tilt
tilt: $(LOCALBIN)
	@if ! command -v "$(TILT)" >/dev/null 2>&1; then \
		echo "Installing tilt v$(TILT_VERSION)"; \
		curl -fsSL "https://github.com/tilt-dev/tilt/releases/download/v$(TILT_VERSION)/tilt.$(TILT_VERSION).$(TILT_OS).$(ARCH).tar.gz" | tar -xzm -C $(LOCALBIN) tilt; \
	fi
