#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

IMAGE_NAME="${IMAGE_NAME:-chriswebber/docker-telegramcustomer}"
IMAGE_TAG="${1:-${IMAGE_TAG:-latest}}"
DOCKERFILE_PATH="${DOCKERFILE_PATH:-${SCRIPT_DIR}/Dockerfile.telegramcustomer}"
BUILD_CONTEXT="${BUILD_CONTEXT:-${SCRIPT_DIR}}"
BUILD_PROGRESS="${BUILD_PROGRESS:-plain}"
CLAUDE_GW_PATH="${CLAUDE_GW_PATH:-/Users/sandyzhou/codex-project/claude-gw}"

echo "[build-telegramcustomer] image=${IMAGE_NAME}:${IMAGE_TAG}"
echo "[build-telegramcustomer] dockerfile=${DOCKERFILE_PATH}"
echo "[build-telegramcustomer] context=${BUILD_CONTEXT}"
echo "[build-telegramcustomer] claude_gw=${CLAUDE_GW_PATH}"

if [[ ! -f "${CLAUDE_GW_PATH}/go.mod" ]]; then
    echo "[build-telegramcustomer][error] claude-gw module not found at ${CLAUDE_GW_PATH}"
    exit 1
fi

docker build \
    --progress="${BUILD_PROGRESS}" \
    --build-context "claude_gw=${CLAUDE_GW_PATH}" \
    -f "${DOCKERFILE_PATH}" \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    "${BUILD_CONTEXT}"
