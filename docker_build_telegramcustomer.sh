#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

IMAGE_NAME="${IMAGE_NAME:-chriswebber/docker-telegramcustomer}"
IMAGE_TAG="${1:-${IMAGE_TAG:-latest}}"
DOCKERFILE_PATH="${DOCKERFILE_PATH:-${SCRIPT_DIR}/Dockerfile.telegramcustomer}"
BUILD_CONTEXT="${BUILD_CONTEXT:-${SCRIPT_DIR}}"
BUILD_PLATFORM="${BUILD_PLATFORM:-linux/arm64}"
BUILD_PROGRESS="${BUILD_PROGRESS:-plain}"
AGENT_HUB_PATH="${AGENT_HUB_PATH:-/Users/sandyzhou/codex-project/agent-hub}"

echo "[build-telegramcustomer] platform=${BUILD_PLATFORM}"
echo "[build-telegramcustomer] image=${IMAGE_NAME}:${IMAGE_TAG}"
echo "[build-telegramcustomer] dockerfile=${DOCKERFILE_PATH}"
echo "[build-telegramcustomer] context=${BUILD_CONTEXT}"
echo "[build-telegramcustomer] agent_hub=${AGENT_HUB_PATH}"

if [[ ! -f "${AGENT_HUB_PATH}/go.mod" ]]; then
    echo "[build-telegramcustomer][error] agent-hub module not found at ${AGENT_HUB_PATH}"
    exit 1
fi

docker build \
    --platform "${BUILD_PLATFORM}" \
    --progress="${BUILD_PROGRESS}" \
    --build-context "agent_hub=${AGENT_HUB_PATH}" \
    -f "${DOCKERFILE_PATH}" \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    "${BUILD_CONTEXT}"
