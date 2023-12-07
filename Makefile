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
.DEFAULT_GOAL := help

GO ?= go
ENGINE ?= docker

VERSION ?= $(shell git tag | tail -n 1 | grep '' || echo 'v0.0.0')$(shell git diff --quiet || git rev-parse HEAD | sed 's/\(.\{6\}\).*/-\1/')
TOOLCHAIN_VERSION := $(shell sed -En 's/^go (.*)$$/\1/p' go.mod)
MODULE_NAME := $(shell sed -En 's/^module (.*)$$/\1/p' go.mod)

REGISTRY := docker.io
IMAGE := linode/linode-cosi-driver

CONTAINERFILE ?= Dockerfile
OCI_TAGS += --tag=${REGISTRY}/${IMAGE}:${VERSION}
OCI_BUILDARGS += --build-arg=VERSION=${VERSION}

GOFLAGS ?=
LDFLAGS += -X ${MODULE_NAME}/pkg/version.Version=${VERSION}
GO_SETTINGS += CGO_ENABLED=0

_GOBIN := $(shell go env GOPATH)/bin

.PHONY: all
all: test build image # Run all targets.

.PHONY: generate
generate: ${_GOBIN}/gowrap # Generate additional code.
	${GO} generate ./...

.PHONY: build
build: clean generate # Build the binary.
	${GO_SETTINGS} ${GO} build \
		${GOFLAGS} \
		-ldflags="${LDFLAGS}" \
		-o ./bin/${NAME} \
		./cmd/linode-cosi-driver

.PHONY: image
image: clean-image # Build container image.
	${ENGINE} build \
		${OCI_TAGS} ${OCI_BUILDARGS} \
		--file=${CONTAINERFILE} \
		--target=runtime \
		.

.PHONY: test
test: # Run unit tests.
	${GO} test ${GOFLAGS} \
		-race \
		-cover -covermode=atomic -coverprofile=coverage.out \
		./...

.PHONY: e2e
e2e: # Run end to end tests. (FIXME: this is placeholder)
	@-echo "this is placeholder"

.PHONY: clean
clean: # Clean the previous build files.
	@rm -rf ./bin

.PHONY: clean-image
clean-image: # Attempt to remove the old container image builds.
	@-${ENGINE} image rm -f $(shell ${ENGINE} image ls -aq ${REGISTRY}/${REPOSITORY}/${NAME}:${VERSION} | xargs -n1 | sort -u | xargs)

.PHONY: help
help: # Show help for each of the Makefile recipes.
	@grep -E '^[a-zA-Z0-9 -]+:.*#'  Makefile | while read -r l; do printf "\033[1;32m$$(echo $$l | cut -f 1 -d':')\033[00m:$$(echo $$l | cut -f 2- -d'#')\n"; done

${_GOBIN}/gowrap:
	${GO} install github.com/hexdigest/gowrap/cmd/gowrap@latest
