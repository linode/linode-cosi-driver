#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
chart_dir="${repo_root}/helm/cosi-crds"
fixtures="${repo_root}/test/charts/cosi-crds/resources.yaml"
work_dir="$(mktemp -d)"
cluster_name="cosi-crds-${RANDOM}-${RANDOM}"
cluster_created=false

cleanup() {
  if [[ "${cluster_created}" == true ]]; then
    kind delete cluster --name "${cluster_name}"
  fi
  rm -rf "${work_dir}"
}
trap cleanup EXIT

# Use a separate kubeconfig throughout; never mutate the caller's current cluster/context.
export KUBECONFIG="${work_dir}/kubeconfig"
cluster_created=true
kind create cluster --name "${cluster_name}" --kubeconfig "${KUBECONFIG}" --wait 120s

crds=(
  bucketaccessclasses.objectstorage.k8s.io
  bucketaccesses.objectstorage.k8s.io
  bucketclaims.objectstorage.k8s.io
  bucketclasses.objectstorage.k8s.io
  buckets.objectstorage.k8s.io
)

# Installing the packaged chart verifies that files/ definitions are in the archive.
helm package "${chart_dir}" --destination "${work_dir}"
chart_version="$(yq -r '.version' "${chart_dir}/Chart.yaml")"
helm install cosi-crds "${work_dir}/cosi-crds-${chart_version}.tgz" \
  --namespace cosi-crds-system --create-namespace --wait --timeout 120s
for crd in "${crds[@]}"; do
  kubectl wait --for=condition=Established --timeout=60s "crd/${crd}"
  policy="$(kubectl get crd "${crd}" -o jsonpath='{.metadata.annotations.helm\.sh/resource-policy}')"
  test "${policy}" = keep
done
kubectl create namespace cosi-crds-test
kubectl apply -f "${fixtures}"
kubectl get -f "${fixtures}" -o jsonpath='{range .items[*]}{.metadata.uid}{"\n"}{end}' \
  | sort > "${work_dir}/original-uids"
test "$(wc -l < "${work_dir}/original-uids" | tr -d ' ')" -eq 5

# Verify a compatible schema change is applied by helm upgrade, not just rendered.
cp -R "${chart_dir}" "${work_dir}/upgrade-chart"
upgrade_crd="${work_dir}/upgrade-chart/files/bucketclaims.objectstorage.k8s.io.yaml"
yq --inplace \
  '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.lifecycleTestField = {"type": "string"}' \
  "${upgrade_crd}"
helm upgrade cosi-crds "${work_dir}/upgrade-chart" \
  --namespace cosi-crds-system --wait --timeout 120s
kubectl -n cosi-crds-test patch bucketclaim lifecycle-claim --type=merge \
  -p '{"spec":{"lifecycleTestField":"retained"}}'
field="$(kubectl -n cosi-crds-test get bucketclaim lifecycle-claim \
  -o jsonpath='{.spec.lifecycleTestField}')"
test "${field}" = retained

# A CRD omitted by a later chart revision must also be retained, along with its instances.
mv "${work_dir}/upgrade-chart/files/bucketaccessclasses.objectstorage.k8s.io.yaml" \
  "${work_dir}/omitted-crd.yaml"
helm upgrade cosi-crds "${work_dir}/upgrade-chart" \
  --namespace cosi-crds-system --wait --timeout 120s
kubectl get bucketaccessclass lifecycle-accessclass -o name
mv "${work_dir}/omitted-crd.yaml" \
  "${work_dir}/upgrade-chart/files/bucketaccessclasses.objectstorage.k8s.io.yaml"

helm uninstall cosi-crds --namespace cosi-crds-system --wait --timeout 120s
for crd in "${crds[@]}"; do
  kubectl get crd "${crd}" -o name
done
kubectl get -f "${fixtures}" -o jsonpath='{range .items[*]}{.metadata.uid}{"\n"}{end}' \
  | sort > "${work_dir}/retained-uids"
diff -u "${work_dir}/original-uids" "${work_dir}/retained-uids"

# Reinstall with the original ownership identity and retain the compatible upgraded schema.
helm install cosi-crds "${work_dir}/upgrade-chart" \
  --namespace cosi-crds-system --wait --timeout 120s
kubectl get -f "${fixtures}" -o jsonpath='{range .items[*]}{.metadata.uid}{"\n"}{end}' \
  | sort > "${work_dir}/reinstalled-uids"
diff -u "${work_dir}/original-uids" "${work_dir}/reinstalled-uids"
field="$(kubectl -n cosi-crds-test get bucketclaim lifecycle-claim \
  -o jsonpath='{.spec.lifecycleTestField}')"
test "${field}" = retained

echo 'COSI CRD chart install, schema upgrade, uninstall retention and reinstall passed'
