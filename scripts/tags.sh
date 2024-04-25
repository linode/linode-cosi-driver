#!/usr/bin/env bash

# ./scripts/tags.sh "docker.io/example/example" "$(git rev-parse HEAD)" 'main' 'v0.1.0'
# This script generates version information and tags for a Docker image based on input parameters.

set -e
set -u
set -o pipefail

IMAGE="${1}"
SHA="$(echo "${2}" | cut -c1-8)"
REF_NAME="${3}"
LATEST_REF="${4}"
GITHUB_OUTPUT="${5}"
DATE="v$(date +'%Y%m%d')"

if [[ "${REF_NAME}" =~ ([v][0-9]+\.[0-9]+\.[0-9]+.*) ]]; then
  REF_NAME="${BASH_REMATCH[1]}"
fi

if [[ "${LATEST_REF}" =~ ([v][0-9]+\.[0-9]+\.[0-9]+.*) ]]; then
  LATEST_REF="${BASH_REMATCH[1]}"
fi

# Determine the version and tags based on the reference name
if [[ "${REF_NAME}" == v* ]]; then
  VERSION="${DATE}-${REF_NAME}-${SHA}"
  TAGS=("${VERSION}" "${REF_NAME}" "latest")
  CHART="${REF_NAME}"
elif [[ "${REF_NAME}" == "main" ]]; then
  VERSION="${DATE}-${LATEST_REF}-${SHA}"
  TAGS=("${VERSION}" "canary")
  CHART="${LATEST_REF}-${SHA}"
else
  VERSION="${DATE}-${SHA}"
  TAGS=("${VERSION}")
  CHART="${LATEST_REF}-${SHA}"
fi

# Create a comma-separated list of tags
length="${#TAGS[@]}"
TAGS_CSV=()
for ((i=0; i<length-1; i++)); do
  TAGS_CSV+=("${TAGS[i]},")
done
TAGS_CSV+=("${TAGS[$length-1]}")

{
echo "all=$(printf "${IMAGE}:%s" "${TAGS_CSV[@]}")";
echo "version=${VERSION}";
echo "full_version=${IMAGE}:${VERSION}";
echo "chart=${CHART}";
} >> "${GITHUB_OUTPUT}"
