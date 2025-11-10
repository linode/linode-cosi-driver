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

#########################################################################################
# Build
#########################################################################################

# First stage: building the driver executable.
FROM docker.io/library/golang:1.25.4 AS builder

# Set the working directory.
WORKDIR /work

# Prepare dir so it can be copied over to runtime layer.
RUN mkdir -p /var/lib/cosi

# Copy the Go Modules manifests.
COPY go.mod go.mod
COPY go.sum go.sum

# Cache dep before building and copying source so that we don't need to re-download as
# much and so that source changes don't invalidate our downloaded layer.
RUN go mod download

# Copy the go source.
COPY Makefile Makefile
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build.
ARG VERSION="unknown"
RUN make build VERSION=${VERSION}

#########################################################################################
# Runtime
#########################################################################################

# Second stage: building final environment for running the executable.
FROM gcr.io/distroless/static:nonroot AS runtime

# Copy the executable.
COPY --from=builder --chown=65532:65532 /work/bin/linode-cosi-driver /usr/bin/linode-cosi-driver

# Copy the volume directory with correct permissions, so driver can bind a socket there.
COPY --from=builder --chown=65532:65532 /var/lib/cosi /var/lib/cosi

# Set volume mount point for app socket.
VOLUME [ "/var/lib/cosi" ]

# Set the final UID:GID to non-root user.
USER 65532:65532

# Disable healthcheck.
HEALTHCHECK NONE

# Args for dynamically setting labels.
ARG VERSION="unknown"

# Add labels
LABEL org.opencontainers.image.title="linode-cosi-driver"
LABEL org.opencontainers.image.description="COSI Driver for Linode Object Storage"
LABEL org.opencontainers.image.authors="Linode COSI Driver Authors"
LABEL org.opencontainers.image.vendor="Akamai Technologies, Inc."
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.license="Apache-2.0"
LABEL org.opencontainers.image.source="https://github.com/linode/linode-cosi-driver"
LABEL org.opencontainers.image.documentation="https://github.com/linode/linode-cosi-driver"
LABEL org.opencontainers.image.base.name="gcr.io/distroless/static:nonroot"

# Set the entrypoint.
ENTRYPOINT [ "/usr/bin/linode-cosi-driver" ]
CMD []
