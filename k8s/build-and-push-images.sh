#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   DOCKERHUB_USERNAME=parlakisik DOCKERHUB_TOKEN=... ./build-and-push-images.sh [tag]

TAG="${1:-latest}"

if [[ -z "${DOCKERHUB_USERNAME:-}" || -z "${DOCKERHUB_TOKEN:-}" ]]; then
  echo "ERROR: Please export DOCKERHUB_USERNAME and DOCKERHUB_TOKEN" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

UI_IMAGE="${DOCKERHUB_USERNAME}/k8s-diagnostic-ui:${TAG}"
CLI_IMAGE="${DOCKERHUB_USERNAME}/k8s-diagnostic-cli:${TAG}"

echo "Logging in to Docker Hub as ${DOCKERHUB_USERNAME}"
echo "${DOCKERHUB_TOKEN}" | docker login -u "${DOCKERHUB_USERNAME}" --password-stdin

echo "Building UI image: ${UI_IMAGE}"
docker build -f docker/Dockerfile.ui -t "${UI_IMAGE}" .

echo "Building CLI image: ${CLI_IMAGE}"
docker build -f docker/Dockerfile.cli -t "${CLI_IMAGE}" .

echo "Pushing images"
docker push "${UI_IMAGE}"
docker push "${CLI_IMAGE}"

echo "Done. Pushed:"
echo "  - ${UI_IMAGE}"
echo "  - ${CLI_IMAGE}"
