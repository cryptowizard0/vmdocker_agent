#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-chriswebber/docker-openclaw-sandbox:latest}"
SANDBOX_NAME="${SANDBOX_NAME:-hymatrix-openclaw-sandbox-test}"
WORKSPACE_DIR="${WORKSPACE_DIR:-$PWD}"
WAIT_SECONDS="${WAIT_SECONDS:-60}"
RUNTIME_TYPE="${RUNTIME_TYPE:-test}"

cleanup() {
  if command -v docker >/dev/null 2>&1; then
    docker sandbox stop "${SANDBOX_NAME}" >/dev/null 2>&1 || true
    docker sandbox rm "${SANDBOX_NAME}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

assert_status_ok() {
  local payload="$1"
  local label="$2"
  python - "$payload" "$label" <<'PY'
import json
import sys

payload = sys.argv[1]
label = sys.argv[2]
data = json.loads(payload)
if data.get("status") != "ok":
    raise SystemExit(f"[ERROR] {label}: expected status=ok, got {data!r}")
print(f"[OK] {label}: status=ok")
PY
}

assert_apply_ok() {
  local payload="$1"
  python - "$payload" <<'PY'
import json
import sys

data = json.loads(sys.argv[1])
if data.get("status") != "ok":
    raise SystemExit(f"[ERROR] apply: expected status=ok, got {data!r}")
result = data.get("result")
if isinstance(result, str):
    result = json.loads(result)
if result.get("Data") != "test-runtime-ok":
    raise SystemExit(f"[ERROR] apply: unexpected result data {result!r}")
print("[OK] apply: result data=test-runtime-ok")
PY
}

echo "[INFO] removing any existing sandbox with the same name"
docker sandbox stop "${SANDBOX_NAME}" >/dev/null 2>&1 || true
docker sandbox rm "${SANDBOX_NAME}" >/dev/null 2>&1 || true
i=0
until ! docker sandbox ls | grep -q "^${SANDBOX_NAME}[[:space:]]"; do
  i=$((i + 1))
  if [[ "${i}" -ge 30 ]]; then
    echo "[ERROR] sandbox ${SANDBOX_NAME} still exists after waiting for removal"
    exit 1
  fi
  sleep 1
done

echo "[INFO] creating sandbox: ${SANDBOX_NAME}"
docker sandbox create --name "${SANDBOX_NAME}" -t "${IMAGE_NAME}" shell "${WORKSPACE_DIR}"

echo "[INFO] waiting for sandbox exec readiness"
i=0
until docker sandbox exec "${SANDBOX_NAME}" sh -lc "true" >/dev/null 2>&1; do
  i=$((i + 1))
  if [[ "${i}" -ge "${WAIT_SECONDS}" ]]; then
    echo "[ERROR] sandbox exec was not ready after ${WAIT_SECONDS}s"
    exit 1
  fi
  sleep 1
done

echo "[INFO] starting vmdocker_agent inside sandbox"
docker sandbox exec "${SANDBOX_NAME}" sh -lc "RUNTIME_TYPE=${RUNTIME_TYPE} /usr/local/bin/start-vmdocker-agent.sh >/tmp/vmdocker-agent.log 2>&1 &"

echo "[INFO] waiting for /vmm/health"
i=0
until docker sandbox exec "${SANDBOX_NAME}" sh -lc "curl -fsS -X POST http://127.0.0.1:8080/vmm/health >/tmp/vmdocker-agent-health.json"; do
  i=$((i + 1))
  if [[ "${i}" -ge "${WAIT_SECONDS}" ]]; then
    echo "[ERROR] sandbox health check timed out after ${WAIT_SECONDS}s"
    docker sandbox exec "${SANDBOX_NAME}" sh -lc "cat /tmp/vmdocker-agent.log" || true
    exit 1
  fi
  sleep 1
done

echo "[INFO] calling /vmm/spawn"
spawn_payload="$(docker sandbox exec "${SANDBOX_NAME}" sh -lc "curl -fsS -X POST http://127.0.0.1:8080/vmm/spawn \
  -H 'Content-Type: application/json' \
  -d '{\"Pid\":\"sandbox-pid\",\"Owner\":\"owner-1\",\"CuAddr\":\"cu-1\",\"Evn\":{},\"Tags\":[]}'")"
assert_status_ok "${spawn_payload}" "spawn"

echo "[INFO] calling /vmm/apply"
apply_payload="$(docker sandbox exec "${SANDBOX_NAME}" sh -lc "curl -fsS -X POST http://127.0.0.1:8080/vmm/apply \
  -H 'Content-Type: application/json' \
  -d '{\"From\":\"target-1\",\"Meta\":{\"Action\":\"Execute\",\"Sequence\":1},\"Params\":{\"Action\":\"Execute\",\"Command\":\"hello sandbox\",\"Reference\":\"1\"}}'")"
assert_apply_ok "${apply_payload}"

echo "[OK] sandbox smoke test passed"
