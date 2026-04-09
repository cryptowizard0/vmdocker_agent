#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

IMAGE_NAME="${IMAGE_NAME:-chriswebber/docker-openclaw}"
IMAGE_TAG="${1:-${IMAGE_TAG:-latest}}"
DOCKERFILE_PATH="${DOCKERFILE_PATH:-${SCRIPT_DIR}/Dockerfile.openclaw}"
BUILD_CONTEXT="${BUILD_CONTEXT:-${SCRIPT_DIR}}"
BUILD_PROGRESS="${BUILD_PROGRESS:-plain}"

usage() {
    cat <<EOF
Usage: $(basename "$0") [tag]

Build the OpenClaw-oriented vmdocker_agent image.

Arguments:
  tag                 Optional image tag. Default: \$IMAGE_TAG or latest

Environment overrides:
  IMAGE_NAME          Docker repository/name. Default: ${IMAGE_NAME}
  DOCKERFILE_PATH     Dockerfile path. Default: ${DOCKERFILE_PATH}
  BUILD_CONTEXT       Docker build context. Default: ${BUILD_CONTEXT}
  BUILD_PROGRESS      docker build progress mode. Default: ${BUILD_PROGRESS}
EOF
}

if [[ "${IMAGE_TAG}" == "-h" || "${IMAGE_TAG}" == "--help" ]]; then
    usage
    exit 0
fi

echo "[build-openclaw] image=${IMAGE_NAME}:${IMAGE_TAG}"
echo "[build-openclaw] dockerfile=${DOCKERFILE_PATH}"
echo "[build-openclaw] context=${BUILD_CONTEXT}"

docker build \
    --progress="${BUILD_PROGRESS}" \
    -f "${DOCKERFILE_PATH}" \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    "${BUILD_CONTEXT}"
