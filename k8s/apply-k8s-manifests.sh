#!/usr/bin/env bash
set -euo pipefail

# Applies Kubernetes manifests under this k8s/ directory in the correct order.
# Usage:
#   DOCKERHUB_USERNAME=... IMAGE_TAG=... ./apply-k8s-manifests.sh

K8S_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${K8S_DIR}/.." && pwd)"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "ERROR: kubectl not found in PATH" >&2
  exit 1
fi

if [[ -z "${DOCKERHUB_USERNAME:-}" ]]; then
  echo "ERROR: DOCKERHUB_USERNAME is not set. Export it before running this script." >&2
  exit 1
fi

: "${IMAGE_TAG:=latest}"

HAS_ENVSUBST=0
if command -v envsubst >/dev/null 2>&1; then
  HAS_ENVSUBST=1
fi

declare -a FILES=(
  "namespace.yaml"
  "pvc.yaml"
  "rbac-cli.yaml"
  "deployment-ui.yaml"
  "service-ui-nodeport.yaml"
)

echo "Applying Kubernetes manifests from ${K8S_DIR}"
for f in "${FILES[@]}"; do
  path="${K8S_DIR}/${f}"
  if [[ -f "${path}" ]]; then
    if [[ ${HAS_ENVSUBST} -eq 1 ]]; then
      echo "kubectl apply -f <(envsubst '\${DOCKERHUB_USERNAME} \${IMAGE_TAG}' < ${path})"
      # shellcheck disable=SC2016
      envsubst '\${DOCKERHUB_USERNAME} \${IMAGE_TAG}' < "${path}" | kubectl apply -f -
    else
      echo "INFO: envsubst not found, using sed fallback for variable substitution in ${f}" >&2
      sed \
        -e "s#\${DOCKERHUB_USERNAME}#${DOCKERHUB_USERNAME}#g" \
        -e "s#\${IMAGE_TAG}#${IMAGE_TAG}#g" "${path}" | kubectl apply -f -
    fi
  else
    echo "WARN: Skipping missing file: ${path}" >&2
  fi
done

echo "Done. Verify resources with: kubectl -n k8s-diagnostic get all"

