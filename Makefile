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
	helm-docs --badge-style=flat

.PHONY: generate-schemas
generate-schemas: ## Generate the Helm chart values schema.
	helm-values-schema-json \
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
test-e2e: local-deploy ## Run the e2e tests against a k8s instance using Kyverno Chainsaw.
	chainsaw test ${CHAINSAW_ARGS}

.PHONY: lint
lint: ## Run golangci-lint linter.
	golangci-lint run

.PHONY: lint-fix
lint-fix: ## Run golangci-lint linter and perform fixes.
	golangci-lint run --fix

.PHONY: lint-manifests
lint-manifests: ## Run kube-linter on Kubernetes manifests.
	kube-linter lint --config=helm/.kube-linter.yaml ./helm/**

.PHONY: hadolint
hadolint: ## Run hadolint on Dockerfile
	$(CONTAINER_TOOL) run --rm -i hadolint/hadolint < Dockerfile

.PHONY: generate-mocks
generate-mocks:
	mockgen -source=./pkg/s3/s3.go -destination=./testing/mock/s3.gen.go -package=mock -typed -mock_names=Client=MockS3Client
	mockgen -source=./pkg/linodeclient/linodeclient.go -destination=./testing/mock/linodeclient.gen.go -package=mock -typed -mock_names=Client=MockLinodeClient

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
cluster:
	ctlptl apply -f hack/kind.yaml

.PHONY: cluster-reset
cluster-reset:
	ctlptl delete -f hack/kind.yaml

.PHONY: deploy-deps
deploy-deps: ## Deploy all dependencies of Linode COSI Driver. This step installs CRDs and Controller.
	kubectl apply -k github.com/kubernetes-sigs/container-object-storage-interface/?ref=${COSI_VERSION}

.PHONY: undeploy-deps
undeploy-deps: ## Deploy all dependencies of Linode COSI Driver. This step installs CRDs and Controller.
	kubectl delete -k github.com/kubernetes-sigs/container-object-storage-interface/?ref=${COSI_VERSION}

.PHONY: deploy
deploy: ## Deploy driver to the K8s cluster specified in ~/.kube/config.
	helm upgrade --install \
		linode-cosi-driver \
		./helm/linode-cosi-driver \
			--set=apiToken=$$LINODE_TOKEN \
			--set=driver.image.repository=${IMG} \
			--set=driver.image.tag=${TAG}

.PHONY: local-deploy
local-deploy: cluster
	tilt ci -f Tiltfile

.PHONY: undeploy
undeploy: ## Undeploy driver from the K8s cluster specified in ~/.kube/config.
	helm uninstall linode-cosi-driver

##@ Dependencies
