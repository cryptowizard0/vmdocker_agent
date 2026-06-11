#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-sandytest456/docker-hermes:latest}"
CONTAINER_NAME="${CONTAINER_NAME:-hymatrix-hermes-test}"
RESTORE_CONTAINER_NAME="${RESTORE_CONTAINER_NAME:-hymatrix-hermes-test-restore}"
HOST="${HOST:-127.0.0.1}"
PORT="${PORT:-8080}"
WAIT_SECONDS="${WAIT_SECONDS:-90}"
CLEANUP_ON_EXIT="${CLEANUP_ON_EXIT:-true}"
TEST_RESTORE="${TEST_RESTORE:-true}"

WORKSPACE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/vmdocker-hermes-smoke.XXXXXX")"
CHECKPOINT_FILE="${WORKSPACE_DIR}/checkpoint.json"
RESTORE_PAYLOAD_FILE="${WORKSPACE_DIR}/restore.json"

cleanup() {
  if [[ "${CLEANUP_ON_EXIT}" == "true" ]]; then
    docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
    docker rm -f "${RESTORE_CONTAINER_NAME}" >/dev/null 2>&1 || true
    rm -rf "${WORKSPACE_DIR}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

assert_status_ok() {
  local file="$1"
  local label="$2"
  python - "$file" "$label" <<'PY'
import json
import sys

path = sys.argv[1]
label = sys.argv[2]
with open(path, "r", encoding="utf-8") as f:
    data = json.load(f)
if data.get("status") != "ok":
    raise SystemExit(f"[ERROR] {label}: expected status=ok, got {data!r}")
print(f"[OK] {label}: status=ok")
PY
}

assert_checkpoint_state() {
  local file="$1"
  python - "$file" <<'PY'
import json
import sys

path = sys.argv[1]
with open(path, "r", encoding="utf-8") as f:
    data = json.load(f)
if data.get("status") != "ok":
    raise SystemExit(f"[ERROR] checkpoint: expected status=ok, got {data!r}")
state = data.get("state")
if not isinstance(state, str) or not state:
    raise SystemExit(f"[ERROR] checkpoint: missing state string in {data!r}")
payload = json.loads(state)
if payload.get("format") != "hermes.runtime.v1":
    raise SystemExit(f"[ERROR] checkpoint: unexpected format {payload!r}")
print(f"[OK] checkpoint: format={payload['format']}")
PY
}

assert_container_logs_contain() {
  local container_name="$1"
  local expected="$2"
  local label="$3"
  local logs
  logs="$(docker logs "${container_name}" 2>&1 || true)"
  if [[ "${logs}" != *"${expected}"* ]]; then
    echo "[ERROR] ${label}: expected container logs to contain ${expected}"
    printf '%s\n' "${logs}"
    exit 1
  fi
  echo "[OK] ${label}: container logs contain ${expected}"
}

assert_container_logs_not_contains() {
  local container_name="$1"
  local unexpected="$2"
  local label="$3"
  local logs
  logs="$(docker logs "${container_name}" 2>&1 || true)"
  if [[ "${logs}" == *"${unexpected}"* ]]; then
    echo "[ERROR] ${label}: did not expect container logs to contain ${unexpected}"
    printf '%s\n' "${logs}"
    exit 1
  fi
  echo "[OK] ${label}: container logs do not contain ${unexpected}"
}

write_restore_payload() {
  local checkpoint_file="$1"
  local output_file="$2"
  python - "$checkpoint_file" "$output_file" <<'PY'
import json
import sys

checkpoint_path = sys.argv[1]
output_path = sys.argv[2]
with open(checkpoint_path, "r", encoding="utf-8") as f:
    checkpoint = json.load(f)
payload = {
    "Env": {},
    "Tags": [],
    "State": checkpoint["state"],
}
with open(output_path, "w", encoding="utf-8") as f:
    json.dump(payload, f)
PY
}

wait_for_health() {
  local container_name="$1"
  local url="http://${HOST}:${PORT}/vmm/health"
  local i=0
  until curl -fsS -X POST "${url}" >/dev/null 2>&1; do
    i=$((i + 1))
    if [[ "${i}" -ge "${WAIT_SECONDS}" ]]; then
      echo "[ERROR] health check timed out after ${WAIT_SECONDS}s for ${container_name}"
      docker logs --tail 200 "${container_name}" || true
      exit 1
    fi
    sleep 1
  done
}

start_container() {
  local container_name="$1"
  docker rm -f "${container_name}" >/dev/null 2>&1 || true
  docker run --name "${container_name}" -d \
    -p "${PORT}:8080" \
    -v "${WORKSPACE_DIR}:/runtime" \
    -e RUNTIME_TYPE=hermes \
    -e VMDOCKER_RUNTIME_WORKSPACE=/runtime \
    -e VMDOCKER_AGENT_WORKSPACE=/runtime/workspace \
    -e VMDOCKER_RUNTIME_HOME=/runtime/.home \
    -e HOME=/runtime/.home \
    "${IMAGE_NAME}" >/dev/null
  wait_for_health "${container_name}"
  docker exec "${container_name}" test -x /usr/local/lib/vmdocker-agent/bootstrap/hermes.sh
  docker exec "${container_name}" test ! -e /usr/local/lib/vmdocker-agent/bootstrap/openclaw.sh
  assert_container_logs_contain "${container_name}" "[bootstrap][hermes][info] hermes runtime bootstrap ready" "hermes bootstrap"
  assert_container_logs_not_contains "${container_name}" "[bootstrap][openclaw][info] starting openclaw gateway" "hermes bootstrap isolation"
}

echo "[INFO] starting hermes container: ${CONTAINER_NAME}"
start_container "${CONTAINER_NAME}"

echo "[INFO] calling /vmm/spawn"
curl -fsS -X POST "http://${HOST}:${PORT}/vmm/spawn" \
  -H 'Content-Type: application/json' \
  -d '{"Pid":"hermes-pid","Owner":"owner-1","CuAddr":"cu-1","Evn":{},"Tags":[]}' > "${WORKSPACE_DIR}/spawn.json"
assert_status_ok "${WORKSPACE_DIR}/spawn.json" "spawn"

echo "[INFO] calling /vmm/checkpoint"
curl -fsS -X POST "http://${HOST}:${PORT}/vmm/checkpoint" > "${CHECKPOINT_FILE}"
assert_checkpoint_state "${CHECKPOINT_FILE}"

if [[ "${TEST_RESTORE}" == "true" ]]; then
  echo "[INFO] restarting container to verify restore"
  docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true

  start_container "${RESTORE_CONTAINER_NAME}"
  write_restore_payload "${CHECKPOINT_FILE}" "${RESTORE_PAYLOAD_FILE}"

  echo "[INFO] calling /vmm/restore"
  curl -fsS -X POST "http://${HOST}:${PORT}/vmm/restore" \
    -H 'Content-Type: application/json' \
    --data @"${RESTORE_PAYLOAD_FILE}" > "${WORKSPACE_DIR}/restore-response.json"
  assert_status_ok "${WORKSPACE_DIR}/restore-response.json" "restore"
fi

echo "[OK] hermes smoke test passed"
