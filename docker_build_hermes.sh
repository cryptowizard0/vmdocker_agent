#!/usr/bin/env bash
#
# build-hermes.sh
#
# Default behavior:
#   Build a single-platform image into the local Docker image store.
#   The default local platform is linux/arm64.
#
# Push behavior:
#   Add --push to build linux/amd64 + linux/arm64 and push directly to Docker Hub.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

IMAGE_NAME="${IMAGE_NAME:-sandytest456/docker-hermes}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
DOCKERFILE_PATH="${DOCKERFILE_PATH:-${SCRIPT_DIR}/Dockerfile.hermes}"
BUILD_CONTEXT="${BUILD_CONTEXT:-${SCRIPT_DIR}}"
LOCAL_PLATFORM="${LOCAL_PLATFORM:-linux/arm64}"
PUSH_PLATFORMS="${PUSH_PLATFORMS:-linux/amd64,linux/arm64}"
BUILD_PROGRESS="${BUILD_PROGRESS:-plain}"
AGENT_HUB_PATH="${AGENT_HUB_PATH:-/Users/sandyzhou/codex-project/agent-hub}"

MODE="local"

usage() {
    cat <<EOF
Usage:
  $0 [tag]
  $0 [tag] --local
  $0 [tag] --platform linux/amd64
  $0 [tag] --push

Environment variables:
  IMAGE_NAME=${IMAGE_NAME}
  IMAGE_TAG=${IMAGE_TAG}
  LOCAL_PLATFORM=${LOCAL_PLATFORM}
  PUSH_PLATFORMS=${PUSH_PLATFORMS}
  AGENT_HUB_PATH=${AGENT_HUB_PATH}
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --push)
            MODE="push"
            shift
            ;;
        --local)
            MODE="local"
            shift
            ;;
        --platform)
            if [[ $# -lt 2 ]]; then
                echo "[build-hermes][error] --platform requires a value"
                exit 1
            fi
            LOCAL_PLATFORM="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            IMAGE_TAG="$1"
            shift
            ;;
    esac
done

if [[ "${MODE}" == "push" ]]; then
    BUILDER_NAME="${BUILDER_NAME:-hermes-multi}"
    BUILD_PLATFORM="${PUSH_PLATFORMS}"
    OUTPUT_FLAG="--push"
    OUTPUT_DESC="docker hub"
else
    BUILDER_NAME="${BUILDER_NAME:-desktop-linux}"
    BUILD_PLATFORM="${LOCAL_PLATFORM}"
    OUTPUT_FLAG="--load"
    OUTPUT_DESC="local docker image store"

    if [[ "${BUILD_PLATFORM}" == *","* ]]; then
        echo "[build-hermes][error] local build only supports one platform."
        echo "[build-hermes][error] use --platform linux/arm64 or --platform linux/amd64."
        exit 1
    fi
fi

echo "[build-hermes] mode=${MODE}"
echo "[build-hermes] platform=${BUILD_PLATFORM}"
echo "[build-hermes] image=${IMAGE_NAME}:${IMAGE_TAG}"
echo "[build-hermes] dockerfile=${DOCKERFILE_PATH}"
echo "[build-hermes] context=${BUILD_CONTEXT}"
echo "[build-hermes] agent_hub=${AGENT_HUB_PATH}"
echo "[build-hermes] builder=${BUILDER_NAME}"
echo "[build-hermes] output=${OUTPUT_DESC}"

if [[ ! -f "${AGENT_HUB_PATH}/go.mod" ]]; then
    echo "[build-hermes][error] agent-hub module not found at ${AGENT_HUB_PATH}"
    exit 1
fi

if ! docker buildx inspect "${BUILDER_NAME}" >/dev/null 2>&1; then
    echo "[build-hermes] creating buildx builder: ${BUILDER_NAME}"
    docker buildx create \
        --name "${BUILDER_NAME}" \
        --driver docker-container \
        --use
else
    docker buildx use "${BUILDER_NAME}"
fi

docker buildx inspect --bootstrap

docker buildx build \
    --builder "${BUILDER_NAME}" \
    --platform "${BUILD_PLATFORM}" \
    --progress="${BUILD_PROGRESS}" \
    --build-context "agent_hub=${AGENT_HUB_PATH}" \
    -f "${DOCKERFILE_PATH}" \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    "${OUTPUT_FLAG}" \
    "${BUILD_CONTEXT}"

echo "[build-hermes] done"
echo "[build-hermes] image=${IMAGE_NAME}:${IMAGE_TAG}"

if [[ "${MODE}" == "push" ]]; then
    echo "[build-hermes] verify: docker buildx imagetools inspect ${IMAGE_NAME}:${IMAGE_TAG}"
else
    echo "[build-hermes] run: docker run -it --rm ${IMAGE_NAME}:${IMAGE_TAG}"
    echo "[build-hermes] push manually: docker push ${IMAGE_NAME}:${IMAGE_TAG}"
fi
