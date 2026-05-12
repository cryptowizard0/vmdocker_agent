#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

IMAGE_NAME="${IMAGE_NAME:-chriswebber/docker-claude}"
IMAGE_TAG="${1:-${IMAGE_TAG:-latest}}"
DOCKERFILE_PATH="${DOCKERFILE_PATH:-${SCRIPT_DIR}/Dockerfile.claude}"
BUILD_CONTEXT="${BUILD_CONTEXT:-${SCRIPT_DIR}}"
BUILD_PROGRESS="${BUILD_PROGRESS:-plain}"

echo "[build-claude] image=${IMAGE_NAME}:${IMAGE_TAG}"
echo "[build-claude] dockerfile=${DOCKERFILE_PATH}"
echo "[build-claude] context=${BUILD_CONTEXT}"

docker build \
    --progress="${BUILD_PROGRESS}" \
    -f "${DOCKERFILE_PATH}" \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    "${BUILD_CONTEXT}"
