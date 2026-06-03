#!/usr/bin/env bash
#
# build-telegramcustomer.sh
#
# Default behavior:
#   Build a single-platform image into the local Docker image store.
#   The default local platform is linux/arm64.
#
# Push behavior:
#   Add --push to build linux/amd64 + linux/arm64 and push directly to Docker Hub.
#
# Usage:
#   ./build-telegramcustomer.sh
#   ./build-telegramcustomer.sh v1.0.0
#   ./build-telegramcustomer.sh latest --platform linux/amd64
#   ./build-telegramcustomer.sh --push
#   ./build-telegramcustomer.sh v1.0.0 --push
#
# Examples:
#   Build local ARM64 image:
#     ./build-telegramcustomer.sh
#
#   Build local AMD64 image:
#     ./build-telegramcustomer.sh latest --platform linux/amd64
#
#   Build and push multi-platform latest image:
#     ./build-telegramcustomer.sh --push
#
#   Build and push multi-platform versioned image:
#     ./build-telegramcustomer.sh v1.0.0 --push
#
# Environment variables:
#   IMAGE_NAME=sandytest456/docker-telegramcustomer
#   IMAGE_TAG=latest
#   LOCAL_PLATFORM=linux/arm64
#   PUSH_PLATFORMS=linux/amd64,linux/arm64
#   DOCKERFILE_PATH=./Dockerfile.telegramcustomer
#   BUILD_CONTEXT=.
#   BUILD_PROGRESS=plain
#   BUILDER_NAME=hermes-multi
#   GITHUB_TOKEN=<token with github.com/xingj404-lab/agent-hub access>
#   GH_TOKEN=<fallback token used when GITHUB_TOKEN is unset>
#
# Notes:
#   --load can only load one platform into the local Docker image store.
#   --push is required for a real multi-platform Docker Hub image.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

IMAGE_NAME="${IMAGE_NAME:-sandytest456/docker-telegramcustomer}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
DOCKERFILE_PATH="${DOCKERFILE_PATH:-${SCRIPT_DIR}/Dockerfile.telegramcustomer}"
BUILD_CONTEXT="${BUILD_CONTEXT:-${SCRIPT_DIR}}"
LOCAL_PLATFORM="${LOCAL_PLATFORM:-linux/arm64}"
PUSH_PLATFORMS="${PUSH_PLATFORMS:-linux/amd64,linux/arm64}"
BUILD_PROGRESS="${BUILD_PROGRESS:-plain}"
BUILDER_NAME="${BUILDER_NAME:-hermes-multi}"
GOPRIVATE="${GOPRIVATE:-github.com/xingj404-lab/agent-hub}"

MODE="local"

usage() {
    cat <<EOF
Usage:
  $0 [tag]
  $0 [tag] --local
  $0 [tag] --platform linux/amd64
  $0 [tag] --push

Examples:
  $0
  $0 v1.0.0
  $0 latest --platform linux/amd64
  $0 v1.0.0 --push

Environment variables:
  IMAGE_NAME=${IMAGE_NAME}
  IMAGE_TAG=${IMAGE_TAG}
  LOCAL_PLATFORM=${LOCAL_PLATFORM}
  PUSH_PLATFORMS=${PUSH_PLATFORMS}
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
                echo "[build-telegramcustomer][error] --platform requires a value"
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
    BUILD_PLATFORM="${PUSH_PLATFORMS}"
    OUTPUT_FLAG="--push"
    OUTPUT_DESC="docker hub"
else
    BUILD_PLATFORM="${LOCAL_PLATFORM}"
    OUTPUT_FLAG="--load"
    OUTPUT_DESC="local docker image store"

    if [[ "${BUILD_PLATFORM}" == *","* ]]; then
        echo "[build-telegramcustomer][error] local build only supports one platform."
        echo "[build-telegramcustomer][error] use --platform linux/arm64 or --platform linux/amd64."
        exit 1
    fi
fi

echo "[build-telegramcustomer] mode=${MODE}"
echo "[build-telegramcustomer] platform=${BUILD_PLATFORM}"
echo "[build-telegramcustomer] image=${IMAGE_NAME}:${IMAGE_TAG}"
echo "[build-telegramcustomer] dockerfile=${DOCKERFILE_PATH}"
echo "[build-telegramcustomer] context=${BUILD_CONTEXT}"
echo "[build-telegramcustomer] builder=${BUILDER_NAME}"
echo "[build-telegramcustomer] output=${OUTPUT_DESC}"

SECRET_ARGS=()
if [[ -z "${GITHUB_TOKEN:-}" && -n "${GH_TOKEN:-}" ]]; then
    export GITHUB_TOKEN="${GH_TOKEN}"
fi
if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    SECRET_ARGS+=(--secret "id=github_token,env=GITHUB_TOKEN")
    echo "[build-telegramcustomer] github_token_secret=enabled"
else
    echo "[build-telegramcustomer] github_token_secret=disabled"
fi

if ! docker buildx inspect "${BUILDER_NAME}" >/dev/null 2>&1; then
    echo "[build-telegramcustomer] creating buildx builder: ${BUILDER_NAME}"
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
    --build-arg "GOPRIVATE=${GOPRIVATE}" \
    "${SECRET_ARGS[@]}" \
    -f "${DOCKERFILE_PATH}" \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    "${OUTPUT_FLAG}" \
    "${BUILD_CONTEXT}"

echo "[build-telegramcustomer] done"
echo "[build-telegramcustomer] image=${IMAGE_NAME}:${IMAGE_TAG}"

if [[ "${MODE}" == "push" ]]; then
    echo "[build-telegramcustomer] verify: docker buildx imagetools inspect ${IMAGE_NAME}:${IMAGE_TAG}"
else
    echo "[build-telegramcustomer] run: docker run -it --rm ${IMAGE_NAME}:${IMAGE_TAG}"
    echo "[build-telegramcustomer] push manually: docker push ${IMAGE_NAME}:${IMAGE_TAG}"
fi
