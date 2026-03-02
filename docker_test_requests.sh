#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-chriswebber/docker-openclaw:latest}"
CONTAINER_NAME="${CONTAINER_NAME:-hymatrix-openclaw-test}"
HOST="${HOST:-127.0.0.1}"
PORT="${PORT:-8080}"
RUNTIME_TYPE="${RUNTIME_TYPE:-test}"
OPENCLAW_GATEWAY_URL="${OPENCLAW_GATEWAY_URL:-http://127.0.0.1:18789}"
WAIT_SECONDS="${WAIT_SECONDS:-30}"
CLEANUP_ON_EXIT="${CLEANUP_ON_EXIT:-false}"

STARTED_BY_SCRIPT="false"

cleanup() {
  if [[ "${CLEANUP_ON_EXIT}" == "true" && "${STARTED_BY_SCRIPT}" == "true" ]]; then
    docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

container_running() {
  docker ps --filter "name=^/${CONTAINER_NAME}$" --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"
}

container_exists() {
  docker ps -a --filter "name=^/${CONTAINER_NAME}$" --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"
}

if container_running; then
  echo "[INFO] container is running: ${CONTAINER_NAME}"
elif container_exists; then
  echo "[INFO] starting existing container: ${CONTAINER_NAME}"
  docker start "${CONTAINER_NAME}" >/dev/null
else
  echo "[INFO] starting new container: ${CONTAINER_NAME}"
  if docker ps --format '{{.Ports}}' | grep -q ":${PORT}->"; then
    echo "[ERROR] host port ${PORT} is already in use by another container"
    echo "[HINT] run with a different port, e.g. PORT=18080 ./docker_test_requests.sh"
    exit 1
  fi

  if [[ "${RUNTIME_TYPE}" == "openclaw" ]]; then
    docker run --name "${CONTAINER_NAME}" -d \
      -p "${PORT}:8080" \
      -p "18789:18789" \
      -e RUNTIME_TYPE=openclaw \
      -e OPENCLAW_GATEWAY_URL="${OPENCLAW_GATEWAY_URL}" \
      "${IMAGE_NAME}" >/dev/null
  else
    docker run --name "${CONTAINER_NAME}" -d \
      -p "${PORT}:8080" \
      -e RUNTIME_TYPE="${RUNTIME_TYPE}" \
      "${IMAGE_NAME}" >/dev/null
  fi
  STARTED_BY_SCRIPT="true"
fi

BASE_URL="http://${HOST}:${PORT}/vmm"

echo "[INFO] waiting for health endpoint: ${BASE_URL}/health"
ready="false"
for _ in $(seq 1 "${WAIT_SECONDS}"); do
  if curl -sS -X POST "${BASE_URL}/health" -H 'Content-Type: application/json' -d '{}' >/tmp/vmdocker_health_resp.json 2>/dev/null; then
    ready="true"
    break
  fi
  sleep 1
done

if [[ "${ready}" != "true" ]]; then
  echo "[ERROR] service not ready in ${WAIT_SECONDS}s"
  docker logs "${CONTAINER_NAME}" --tail 200 | cat
  exit 1
fi

echo "[OK] health response:"
cat /tmp/vmdocker_health_resp.json

echo "\n[INFO] spawn request"
curl -sS -X POST "${BASE_URL}/spawn" \
  -H 'Content-Type: application/json' \
  -d '{"Pid":"pid-e2e-1","Owner":"owner-e2e","CuAddr":"cu-e2e","Evn":{},"Tags":[]}' \
  | tee /tmp/vmdocker_spawn_resp.json

echo "\n[INFO] apply request"
curl -sS -X POST "${BASE_URL}/apply" \
  -H 'Content-Type: application/json' \
  -d '{"From":"target-e2e","Meta":{"Action":"Ping","Sequence":1},"Params":{"Action":"Ping","Reference":"1"}}' \
  | tee /tmp/vmdocker_apply_resp.json

echo "\n[DONE] responses saved to:"
echo "  /tmp/vmdocker_health_resp.json"
echo "  /tmp/vmdocker_spawn_resp.json"
echo "  /tmp/vmdocker_apply_resp.json"
