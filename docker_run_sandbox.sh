#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-chriswebber/docker-openclaw-sandbox:latest}"
SANDBOX_NAME="${SANDBOX_NAME:-hymatrix-openclaw-sandbox}"
WORKSPACE_DIR="${WORKSPACE_DIR:-$PWD}"
PULL_TEMPLATE="${PULL_TEMPLATE:-missing}"

docker sandbox create \
  --name "${SANDBOX_NAME}" \
  --pull-template "${PULL_TEMPLATE}" \
  -t "${IMAGE_NAME}" \
  shell "${WORKSPACE_DIR}"

docker sandbox run "${SANDBOX_NAME}"
