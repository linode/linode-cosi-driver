#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
chart_dir="${repo_root}/helm/cosi-crds"
upstream_ref="$(yq -r '.appVersion' "${chart_dir}/Chart.yaml")"
if [[ -z "${upstream_ref}" || "${upstream_ref}" == null ]]; then
  echo 'The CRD chart must pin an upstream appVersion' >&2
  exit 1
fi

work_dir="$(mktemp -d)"
trap 'rm -rf "${work_dir}"' EXIT
mkdir -p "${work_dir}/files"

# The upstream kustomization supplies the required api-approved.kubernetes.io annotation.
kubectl kustomize \
  "github.com/kubernetes-sigs/container-object-storage-interface/?ref=${upstream_ref}" \
  > "${work_dir}/upstream.yaml"

for resource in bucketaccessclasses bucketaccesses bucketclaims bucketclasses buckets; do
  name="${resource}.objectstorage.k8s.io"
  CRD_NAME="${name}" yq eval --exit-status \
    'select(.kind == "CustomResourceDefinition" and .metadata.name == strenv(CRD_NAME))' \
    "${work_dir}/upstream.yaml" > "${work_dir}/files/${name}.yaml"
done

# Replace files only after every expected upstream definition was generated successfully.
mkdir -p "${chart_dir}/files"
rm -f "${chart_dir}/files/"*.yaml
cp "${work_dir}/files/"*.yaml "${chart_dir}/files/"
