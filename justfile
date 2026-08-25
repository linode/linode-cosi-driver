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

# Recipes run under bash, exiting when a command exits non-zero or a piped
# command fails.
set shell := ["bash", "-eo", "pipefail", "-c"]

# Every variable below reads an environment variable of the same name (upper
# cased) first, so `IMG=... just ko-build` still works. `just VAR=value recipe`
# overrides them too.

# Image URL to use all building/pushing image targets
img := env('IMG', 'localhost:5005/linode-cosi-driver')
tag := env('TAG', 'dev-' + `git describe --match='' --always --abbrev=6 --dirty`)

# Platform for local image builds. Releases build every platform in .ko.yaml
# instead; a local build only ever runs on this machine.
platform := env('PLATFORM', 'linux/' + `go env GOARCH`)
chainsaw_args := env('CHAINSAW_ARGS', '')

# Go module path, used to build the -X ldflag for version injection.
module_name := env('MODULE_NAME', 'github.com/linode/linode-cosi-driver')

# Version stamped into the binary and the image.
version := env('VERSION', tag)

# ko settings. ko_docker_repo selects the destination registry, image_version is
# read by .ko.yaml to stamp the version, image_tags are the tags to apply.
ko_docker_repo := env('KO_DOCKER_REPO', img)
image_version := env('IMAGE_VERSION', version)
image_tags := env('IMAGE_TAGS', tag)

# OCI labels applied to the published image.
# --image-label is comma-separated, so each label is passed as its own flag and the
# values must not contain commas.
ko_labels := "--image-label org.opencontainers.image.title=linode-cosi-driver" + \
    " --image-label 'org.opencontainers.image.description=COSI Driver for Linode Object Storage'" + \
    " --image-label 'org.opencontainers.image.authors=Linode COSI Driver Authors'" + \
    " --image-label 'org.opencontainers.image.vendor=Akamai Technologies Inc.'" + \
    " --image-label org.opencontainers.image.version=" + image_version + \
    " --image-label org.opencontainers.image.licenses=Apache-2.0" + \
    " --image-label org.opencontainers.image.source=https://github.com/linode/linode-cosi-driver" + \
    " --image-label org.opencontainers.image.documentation=https://github.com/linode/linode-cosi-driver"

# Versions of COSI dependencies
cosi_version := "7ddc93baaa3f08c9c8990a17c7b958955d93c044"

# Documentation site preview; the pages tag carries the same gem set GitHub
# Pages builds with, so a local build matches what the site serves.
docs_image := env('DOCS_IMAGE', 'jekyll/jekyll:pages')
docs_container := env('DOCS_CONTAINER', 'cosi-docs')
docs_port := env('DOCS_PORT', '4000')
docs_livereload_port := env('DOCS_LIVERELOAD_PORT', '35729')

# flags for the Go build
goflags := trim(env('GOFLAGS', '') + " -trimpath")
ldflags := trim(env('LDFLAGS', '') + " -X " + module_name + "/pkg/version.Version=" + version + " -s -w -extldflags \"-static\"")
go_settings := env('GO_SETTINGS', 'CGO_ENABLED=0')

alias help := default

# Display this help.
[group('General')]
default:
    @just --list --list-heading $'\nUsage:\n  just <recipe>\n\n'

# Remove build artifacts.
[group('General')]
clean:
    -rm -r bin/

# Build the binary.
[group('Development')]
build:
    {{ go_settings }} go build \
        {{ goflags }} \
        -ldflags="{{ ldflags }}" \
        -o ./bin/linode-cosi-driver \
        ./cmd/linode-cosi-driver

# Generate Helm chart documentation and the values schema.
[group('Development')]
generate: generate-docs generate-schemas

# Generate Helm chart documentation.
[group('Development')]
generate-docs:
    helm-docs --badge-style=flat

# Generate the Helm chart values schema.
[group('Development')]
generate-schemas:
    helm-values-schema-json \
        --draft=7 \
        --indent=2 \
        --values=helm/linode-cosi-driver/values.yaml \
        --output=helm/linode-cosi-driver/values.schema.json

# Generate the gomock doubles used by the tests.
[group('Development')]
generate-mocks:
    mockgen -source=./pkg/s3/s3.go -destination=./testing/mock/s3.gen.go -package=mock -typed -mock_names=Client=MockS3Client
    mockgen -source=./pkg/linodeclient/linodeclient.go -destination=./testing/mock/linodeclient.gen.go -package=mock -typed -mock_names=Client=MockLinodeClient

# Run unit tests.
[group('Development')]
test: generate-mocks
    #!/usr/bin/env bash
    set -eo pipefail
    go test \
        -race \
        -cover -coverprofile=coverage.out \
        ./...
    if [ -f coverage.out ]; then
        grep -v ".gen.go" coverage.out > coverage.filtered.out && mv coverage.filtered.out coverage.out
    fi

# Run integration tests.
[group('Development')]
test-integration:
    #!/usr/bin/env bash
    set -eo pipefail
    go test \
        -tags=integration \
        -race \
        -cover -coverprofile=integration-coverage.out \
        ./...
    if [ -f integration-coverage.out ]; then
        grep -v ".gen.go" integration-coverage.out > integration-coverage.filtered.out && mv integration-coverage.filtered.out integration-coverage.out
    fi

# Run the e2e tests against a k8s instance using Kyverno Chainsaw.
[group('Development')]
test-e2e: local-deploy
    chainsaw test {{ chainsaw_args }}

# Run golangci-lint linter.
[group('Development')]
lint:
    golangci-lint run

# Run golangci-lint linter and perform fixes.
[group('Development')]
lint-fix:
    golangci-lint run --fix

# Run kube-linter on Kubernetes manifests.
[group('Development')]
lint-manifests:
    kube-linter lint --config=helm/.kube-linter.yaml ./helm/**

# The schema and yamllint configs are vendored under helm/.ct because ct only
# looks for them in the working directory, $HOME/.ct and /etc/ct. The in-tree
# chart version is rewritten at release time, so the version-increment check
# can never be satisfied here.

# Run chart-testing lint on the Helm charts.
[group('Development')]
lint-chart target_branch='main':
    ct lint \
        --chart-dirs=helm \
        --chart-yaml-schema=helm/.ct/chart_schema.yaml \
        --lint-conf=helm/.ct/lintconf.yaml \
        --check-version-increment=false \
        --target-branch={{ target_branch }}

# Serve the documentation site locally with live reload.
[group('Documentation')]
serve-docs:
    @echo "Serving the docs on http://localhost:{{ docs_port }}, press ctrl-c to stop"
    docker run --rm --interactive --tty --name {{ docs_container }} \
        --publish {{ docs_port }}:4000 \
        --publish {{ docs_livereload_port }}:35729 \
        --volume "{{ justfile_directory() }}:/srv/jekyll" \
        {{ docs_image }} \
        jekyll serve --host 0.0.0.0 --livereload --force-polling

# Build the documentation site the way GitHub Pages does.
[group('Documentation')]
build-docs:
    docker run --rm \
        --volume "{{ justfile_directory() }}:/srv/jekyll" \
        {{ docs_image }} \
        jekyll build

# Run git diff to check if any changes are made.
[group('CI')]
diff:
    git --no-pager diff HEAD --

# Stamp version and appVersion into Chart.yaml, defaulting to the newest helm-v* tag.
[group('CI')]
set-chart-version chart_version='':
    #!/usr/bin/env bash
    set -eo pipefail
    chart_version='{{ chart_version }}'
    if [ -z "$chart_version" ]; then
        tag=$(git describe --tags --abbrev=0 --match 'helm-v*')
        chart_version=${tag#helm-}
    fi
    # Assign rather than substitute a placeholder, so this cannot silently
    # no-op when the committed Chart.yaml already carries a real version.
    yq --inplace \
        ".version = \"${chart_version}\" | .appVersion = \"${chart_version}\"" \
        helm/linode-cosi-driver/Chart.yaml
    yq '{"version": .version, "appVersion": .appVersion}' helm/linode-cosi-driver/Chart.yaml

# Build the image with ko for this machine, loading it into the local daemon.
[group('Build')]
ko-build:
    IMAGE_VERSION={{ image_version }} \
    KO_DOCKER_REPO={{ ko_docker_repo }} \
        ko build --local --bare --platform={{ platform }} --tags={{ image_tags }} {{ ko_labels }} ./cmd/linode-cosi-driver

# Build and push the multi-arch image with ko.
[group('Build')]
ko-publish:
    IMAGE_VERSION={{ image_version }} \
    KO_DOCKER_REPO={{ ko_docker_repo }} \
        ko build --bare --tags={{ image_tags }} {{ ko_labels }} ./cmd/linode-cosi-driver

# Create the local kind cluster.
[group('Deployment')]
cluster:
    ctlptl apply -f hack/kind.yaml

# Delete the local kind cluster.
[group('Deployment')]
cluster-reset:
    ctlptl delete -f hack/kind.yaml

# Deploy all dependencies of Linode COSI Driver. This step installs CRDs and Controller.
[group('Deployment')]
deploy-deps:
    kubectl apply -k github.com/kubernetes-sigs/container-object-storage-interface/?ref={{ cosi_version }}

# Undeploy all dependencies of Linode COSI Driver. This step removes CRDs and Controller.
[group('Deployment')]
undeploy-deps:
    kubectl delete -k github.com/kubernetes-sigs/container-object-storage-interface/?ref={{ cosi_version }}

# Deploy driver to the K8s cluster specified in ~/.kube/config.
[group('Deployment')]
deploy:
    helm upgrade --install \
        linode-cosi-driver \
        ./helm/linode-cosi-driver \
            --set=apiToken=$LINODE_TOKEN \
            --set=driver.image.repository={{ img }} \
            --set=driver.image.tag={{ tag }}

# Tilt builds the image with ko, which takes no build flags from the Tiltfile:
# IMAGE_VERSION is interpolated into .ko.yaml's ldflags, and KO_DEFAULTPLATFORMS
# narrows the release platform list down to this machine, since ko's --local
# publisher cannot load a multi-arch index into the Docker daemon.

# Build the image and deploy the driver to the local kind cluster with Tilt.
[group('Deployment')]
local-deploy: cluster
    IMAGE_VERSION={{ image_version }} \
    KO_DEFAULTPLATFORMS={{ platform }} \
        tilt ci -f Tiltfile

# Undeploy driver from the K8s cluster specified in ~/.kube/config.
[group('Deployment')]
undeploy:
    helm uninstall linode-cosi-driver
