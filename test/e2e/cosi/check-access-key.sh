#!/usr/bin/env bash
set -euo pipefail

account_id="${1:?Object Storage key ID is required}"
expected_status="${2:?expected HTTP status is required}"
: "${LINODE_TOKEN:?LINODE_TOKEN must match the driver account}"

case "${account_id}" in
  ''|*[!0-9]*) echo 'Expected a numeric Object Storage key ID' >&2; exit 1 ;;
esac

# Match linodego's environment settings instead of a separate CLI profile.
api_url="${LINODE_URL:-https://api.linode.com}"
api_version="${LINODE_API_VERSION:-v4}"
curl_args=(--silent --show-error --connect-timeout 10 --max-time 30)
if [[ -n "${LINODE_CA:-}" ]]; then
  curl_args+=(--cacert "${LINODE_CA}")
fi
status="$(curl "${curl_args[@]}" \
  --output /dev/null --write-out '%{http_code}' \
  --header "Authorization: Bearer ${LINODE_TOKEN}" \
  "${api_url%/}/${api_version}/object-storage/keys/${account_id}")"

# Discard the response body: successful key lookups may contain credentials.
if [[ "${status}" != "${expected_status}" ]]; then
  echo "Object Storage key ${account_id}: expected HTTP ${expected_status}, got ${status}" >&2
  exit 1
fi
