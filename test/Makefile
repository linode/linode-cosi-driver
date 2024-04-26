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

SHELL := /usr/bin/env bash -o errexit -o pipefail -o nounset

ROOT_DIR = "$(shell git rev-parse --show-toplevel)"

export REGISTRY ?= docker.io
export REPOSITORY ?= linode
export IMAGE := linode-cosi-driver
export VERSION ?= $(shell git tag | tail -n 1 | grep '' || echo 'v0.0.0')$(shell git diff --quiet || git rev-parse HEAD | sed 's/\(.\{6\}\).*/-\1/')

TEST_SELECTOR ?= ""

.DEFAULT_GOAL := help

.PHONY: all
all: clean test # Run all targets.

.PHONY: clean
clean: # Cleanup the test environment.

.PHONY: test
test: # Run end-to-end tests.

integration-test:
	cd $(ROOT_DIR); go test -count=1 -tags=integration ./...

e2e-test: clean-test-cluster test-cluster _e2e-test
	$(MAKE) clean-test-cluster

_e2e-test: chainsaw
	$(CHAINSAW) test --parallel 3 --test-dir $(ROOT_DIR)/test/e2e --selector $(TEST_SELECTOR)

test-cluster: ctlptl kind helm
	$(CTLPTL) apply -f $(ROOT_DIR)/test/ctlptl-config.yaml

	kubectl create -k github.com/kubernetes-sigs/container-object-storage-interface-api
	kubectl create -k github.com/kubernetes-sigs/container-object-storage-interface-controller

	$(MAKE) -C $(ROOT_DIR) image
	$(KIND) -n linode-cosi load docker-image ${REGISTRY}/${REPOSITORY}/${IMAGE}:${VERSION}

	$(HELM) install linode-cosi-driver \
        $(ROOT_DIR)/helm/linode-cosi-driver \
        --set=apiToken=$(LINODE_TOKEN) \
        --set=driver.image.tag=$(VERSION) \
        --namespace=linode-cosi-driver \
        --create-namespace \
		--wait
	
	kubectl wait pods --for=condition=Ready=True -l app.kubernetes.io/name=container-object-storage-interface-controller --timeout 1m
	kubectl wait pods -n linode-cosi-driver --for=condition=Ready=True -l app.kubernetes.io/name=linode-cosi-driver --timeout 1m

clean-test-cluster: ctlptl
	$(CTLPTL) delete cluster kind-linode-cosi ||:

.PHONY: help
help: # Show help for each of the Makefile recipes.
	@grep -E '^[a-zA-Z0-9 -]+:.*#'  Makefile | while read -r l; do printf "\033[1;32m$$(echo $$l | cut -f 1 -d':')\033[00m:$$(echo $$l | cut -f 2- -d'#')\n"; done

#####################################################################
# OS / ARCH
#####################################################################
OS=$(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(shell uname -m)
ARCH_SHORT=$(ARCH)
ifeq ($(ARCH_SHORT),x86_64)
ARCH_SHORT := amd64
else ifeq ($(ARCH_SHORT),aarch64)
ARCH_SHORT := arm64
endif

## --------------------------------------
## Build Dependencies
## --------------------------------------

##@ Build Dependencies:

## Location to install dependencies to

# Use CACHE_BIN for tools that cannot use devbox and LOCALBIN for tools that can use either method
CACHE_BIN ?= $(CURDIR)/../bin
LOCALBIN ?= $(CACHE_BIN)

DEVBOX_BIN ?= $(DEVBOX_PACKAGES_DIR)/bin

# if the $DEVBOX_PACKAGES_DIR env variable exists that means we are within a devbox shell and can safely
# use devbox's bin for our tools
ifdef DEVBOX_PACKAGES_DIR
	LOCALBIN = $(DEVBOX_BIN)
endif

export PATH := $(CACHE_BIN):$(PATH)
$(CACHE_BIN):
	mkdir -p $(CACHE_BIN)
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

##@ Tooling Binaries:
CTLPTL ?= $(LOCALBIN)/ctlptl
KIND ?= $(LOCALBIN)/kind
CHAINSAW ?= $(CACHE_BIN)/chainsaw
HELM ?= $(CACHE_BIN)/helm

## Tool Versions
CTLPTL_VERSION ?= v0.8.28
KIND_VERSION ?= v0.22.0
CHAINSAW_VERSION ?= v0.1.9
HELM_VERSION ?= v0.1.9

.PHONY: tools
tools: ctlptl chainsaw kind helm

.PHONY: ctlptl
ctlptl: $(CTLPTL) ## Download ctlptl locally if necessary.
$(CTLPTL): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/tilt-dev/ctlptl/cmd/ctlptl@$(CTLPTL_VERSION)

.PHONY: chainsaw
chainsaw: $(CHAINSAW) ## Download chainsaw locally if necessary.
$(CHAINSAW): $(CACHE_BIN)
	GOBIN=$(CACHE_BIN) go install github.com/kyverno/chainsaw@$(CHAINSAW_VERSION)

.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	curl -fsSL https://github.com/kubernetes-sigs/kind/releases/download/$(KIND_VERSION)/kind-$(OS)-$(ARCH_SHORT) -o $(KIND)
	chmod +x $(KIND)

.PHONY: helm
helm: $(HELM) ## Download helm locally if necessary.
$(HELM): $(CACHE_BIN)
	export HELM_INSTALL_DIR=$(CACHE_BIN); \
		export USE_SUDO=false; \
		export VERIFY_SIGNATURES=true; \
		curl -sf https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 \
		| bash
