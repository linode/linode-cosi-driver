# Copyright 2023 Linode, LLC
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

ARG TOOLCHAIN_VERSION=1.21

#########################################################################################
# Build
#########################################################################################

# First stage: building the driver executable.
FROM docker.io/library/golang:${TOOLCHAIN_VERSION} as builder

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
COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/
COPY Makefile Makefile

# Explicitly set the version, so the make won't try to get it (and fail).
ENV VERSION="builder"

# Build.
RUN make build

#########################################################################################
# Runtime
#########################################################################################

# Second stage: building final environment for running the executable.
FROM gcr.io/distroless/static-debian11:latest AS runtime

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

# Add labels
LABEL name="linode-cosi-driver"
LABEL description="COSI Driver for Linode Object Storage"
LABEL vendor="Linode, LLC"
LABEL license="Apache-2.0"
LABEL maintainers="Linode COSI Driver Authors"

# Set the entrypoint.
ENTRYPOINT [ "/usr/bin/linode-cosi-driver" ]
CMD []
