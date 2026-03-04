#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-chriswebber/docker-openclaw:v0.0.1}"
CONTAINER_NAME="${CONTAINER_NAME:-hymatrix-openclaw-test}"
HOST="${HOST:-127.0.0.1}"
PORT="${PORT:-8080}"
RUNTIME_TYPE="openclaw"
OPENCLAW_GATEWAY_URL="${OPENCLAW_GATEWAY_URL:-http://127.0.0.1:18789}"
OPENCLAW_GATEWAY_TOKEN="${OPENCLAW_GATEWAY_TOKEN:-openclaw-test-token}"
WAIT_SECONDS="${WAIT_SECONDS:-90}"
CLEANUP_ON_EXIT="${CLEANUP_ON_EXIT:-false}"
OPENCLAW_GATEWAY_READY_WAIT_SECONDS="${OPENCLAW_GATEWAY_READY_WAIT_SECONDS:-90}"
OPENCLAW_TEST_MODEL="${OPENCLAW_TEST_MODEL:-kimi-coding/k2p5}"

STARTED_BY_SCRIPT="false"
CONFIG_FILE=""

cleanup() {
  if [[ "${CLEANUP_ON_EXIT}" == "true" && "${STARTED_BY_SCRIPT}" == "true" ]]; then
    docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${CONFIG_FILE}" && -f "${CONFIG_FILE}" ]]; then
    rm -f "${CONFIG_FILE}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

if [[ -f ".env" ]]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

require_non_empty_env() {
  local key="$1"
  if [[ -z "${!key:-}" ]]; then
    echo "[ERROR] missing required env: ${key}"
    return 1
  fi
  return 0
}

check_model_credentials() {
  local model="$1"
  local provider="${model%%/*}"
  if [[ "$provider" == "$model" ]]; then
    echo "[WARN] model '${model}' has no provider prefix; skip provider key precheck"
    return 0
  fi

  case "${provider}" in
    openai)
      require_non_empty_env "OPENAI_API_KEY"
      ;;
    anthropic)
      require_non_empty_env "ANTHROPIC_API_KEY"
      ;;
    kimi-code|kimi-coding)
      if [[ -z "${KIMI_API_KEY:-}" && -z "${KIMICODE_API_KEY:-}" && -z "${MOONSHOT_API_KEY:-}" ]]; then
        echo "[ERROR] missing required env: KIMICODE_API_KEY or KIMI_API_KEY or MOONSHOT_API_KEY"
        return 1
      fi
      ;;
    google|gemini)
      if [[ -z "${GEMINI_API_KEY:-}" && -z "${GOOGLE_API_KEY:-}" ]]; then
        echo "[ERROR] missing required env: GEMINI_API_KEY or GOOGLE_API_KEY"
        return 1
      fi
      ;;
    *)
      echo "[WARN] unknown model provider '${provider}'; skip provider key precheck"
      ;;
  esac
}

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

status = data.get("status")
if status != "ok":
    raise SystemExit(f"[ERROR] {label}: expected status=ok, got {status!r} in {path}")
print(f"[OK] {label}: status=ok")
PY
}

assert_apply_action() {
  local file="$1"
  local expected_action="$2"
  local label="$3"
  python - "$file" "$expected_action" "$label" <<'PY'
import json
import sys

path = sys.argv[1]
expected = sys.argv[2]
label = sys.argv[3]
with open(path, "r", encoding="utf-8") as f:
    data = json.load(f)

if data.get("status") != "ok":
    raise SystemExit(f"[ERROR] {label}: non-ok response in {path}: {data!r}")

result = data.get("result")
if isinstance(result, str):
    try:
        result = json.loads(result)
    except json.JSONDecodeError as exc:
        raise SystemExit(f"[ERROR] {label}: result is string but not valid json: {exc}") from exc
if not isinstance(result, dict):
    raise SystemExit(f"[ERROR] {label}: missing object result (or json string) in {path}")

output = result.get("output")
if not isinstance(output, dict):
    output = result.get("Output")
if not isinstance(output, dict):
    raise SystemExit(f"[ERROR] {label}: missing object result.output/result.Output in {path}")

action = output.get("action")
if action != expected:
    raise SystemExit(
        f"[ERROR] {label}: expected result.output.action={expected!r}, got {action!r} in {path}"
    )

print(f"[OK] {label}: action={action}")
PY
}

assert_chat_reply() {
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
    raise SystemExit(f"[ERROR] {label}: non-ok response in {path}: {data!r}")

result = data.get("result")
if isinstance(result, str):
    try:
        result = json.loads(result)
    except json.JSONDecodeError as exc:
        raise SystemExit(f"[ERROR] {label}: result is string but not valid json: {exc}") from exc
if not isinstance(result, dict):
    raise SystemExit(f"[ERROR] {label}: missing object result (or json string) in {path}")

output = result.get("output")
if not isinstance(output, dict):
    output = result.get("Output")
if not isinstance(output, dict):
    raise SystemExit(f"[ERROR] {label}: missing object result.output/result.Output in {path}")

reply = output.get("reply")
if not isinstance(reply, str) or not reply.strip():
    raise SystemExit(f"[ERROR] {label}: expected non-empty output.reply, got {reply!r}")

print(f"[OK] {label}: reply captured")
PY
}

container_exists() {
  docker ps -a --filter "name=^/${CONTAINER_NAME}$" --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"
}

if container_exists; then
  echo "[INFO] removing existing container: ${CONTAINER_NAME}"
  docker rm -f "${CONTAINER_NAME}" >/dev/null
fi

echo "[INFO] starting new container: ${CONTAINER_NAME}"
if docker ps --format '{{.Ports}}' | grep -q ":${PORT}->"; then
  echo "[ERROR] host port ${PORT} is already in use by another container"
  echo "[HINT] run with a different port, e.g. PORT=18080 ./docker_test_requests.sh"
  exit 1
fi

echo "[INFO] testing model: ${OPENCLAW_TEST_MODEL}"
check_model_credentials "${OPENCLAW_TEST_MODEL}"

CONFIG_FILE="$(mktemp /tmp/openclaw-test-config.XXXXXX)"
cat >"${CONFIG_FILE}" <<EOF
{
  "agents": {
    "defaults": {
      "model": { "primary": "${OPENCLAW_TEST_MODEL}" },
      "models": {
        "${OPENCLAW_TEST_MODEL}": { "alias": "Kimi Coding" }
      }
    }
  },
  "gateway": {
    "tools": {
      "allow": ["sessions_send", "gateway"]
    }
  },
  "tools": {
    "sessions": {
      "visibility": "all"
    }
  }
}
EOF

if [[ -z "${KIMICODE_API_KEY:-}" && -n "${KIMI_API_KEY:-}" ]]; then
  KIMICODE_API_KEY="${KIMI_API_KEY}"
fi
if [[ -z "${KIMI_API_KEY:-}" && -n "${KIMICODE_API_KEY:-}" ]]; then
  KIMI_API_KEY="${KIMICODE_API_KEY}"
fi
if [[ -z "${MOONSHOT_API_KEY:-}" && -n "${KIMI_API_KEY:-}" ]]; then
  MOONSHOT_API_KEY="${KIMI_API_KEY}"
fi

if [[ -z "${KIMI_API_KEY:-}" && -n "${MOONSHOT_API_KEY:-}" ]]; then
  KIMI_API_KEY="${MOONSHOT_API_KEY}"
fi

docker run --name "${CONTAINER_NAME}" -d \
  -p "${PORT}:8080" \
  -v "${CONFIG_FILE}:/tmp/openclaw-test-config.json:ro" \
  -e RUNTIME_TYPE="${RUNTIME_TYPE}" \
  -e OPENCLAW_GATEWAY_URL="${OPENCLAW_GATEWAY_URL}" \
  -e OPENCLAW_GATEWAY_TOKEN="${OPENCLAW_GATEWAY_TOKEN}" \
  -e OPENCLAW_GATEWAY_READY_WAIT_SECONDS="${OPENCLAW_GATEWAY_READY_WAIT_SECONDS}" \
  -e OPENCLAW_CONFIG_PATH=/tmp/openclaw-test-config.json \
  -e OPENCLAW_GATEWAY_AUTH_MODE=token \
  -e OPENCLAW_GATEWAY_AUTH_TOKEN="${OPENCLAW_GATEWAY_TOKEN}" \
  -e OPENCLAW_HTTP_TOOLS_INVOKE=true \
  -e OPENCLAW_HTTP_TOOLS_INVOKE_DENY= \
  -e OPENAI_API_KEY="${OPENAI_API_KEY:-}" \
  -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}" \
  -e KIMI_API_KEY="${KIMI_API_KEY:-}" \
  -e KIMICODE_API_KEY="${KIMICODE_API_KEY:-}" \
  -e MOONSHOT_API_KEY="${MOONSHOT_API_KEY:-}" \
  -e GEMINI_API_KEY="${GEMINI_API_KEY:-}" \
  -e GOOGLE_API_KEY="${GOOGLE_API_KEY:-}" \
  "${IMAGE_NAME}" >/dev/null
STARTED_BY_SCRIPT="true"

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
assert_status_ok /tmp/vmdocker_health_resp.json "health"

echo "\n[INFO] spawn request"
curl -sS -X POST "${BASE_URL}/spawn" \
  -H 'Content-Type: application/json' \
  -d '{"Pid":"pid-e2e-1","Owner":"owner-e2e","CuAddr":"cu-e2e","Evn":{},"Tags":[]}' \
  | tee /tmp/vmdocker_spawn_resp.json
assert_status_ok /tmp/vmdocker_spawn_resp.json "spawn"

echo "\n[INFO] apply Execute request"
curl -sS -X POST "${BASE_URL}/apply" \
  -H 'Content-Type: application/json' \
  -d '{"From":"target-e2e","Meta":{"Action":"Execute","Sequence":1},"Params":{"Action":"Execute","Command":"hello openclaw","Reference":"1"}}' \
  | tee /tmp/vmdocker_apply_resp.json
assert_apply_action /tmp/vmdocker_apply_resp.json "Execute" "apply execute"

echo "\n[INFO] apply Chat request"
curl -sS -X POST "${BASE_URL}/apply" \
  -H 'Content-Type: application/json' \
  -d '{"From":"target-e2e","Meta":{"Action":"Chat","Sequence":4},"Params":{"Action":"Chat","Command":"hello agent","Reference":"4"}}' \
  | tee /tmp/vmdocker_apply_chat_resp.json
assert_apply_action /tmp/vmdocker_apply_chat_resp.json "Chat" "apply chat"
assert_chat_reply /tmp/vmdocker_apply_chat_resp.json "apply chat"

echo "\n[INFO] apply ConfigureModel request"
curl -sS -X POST "${BASE_URL}/apply" \
  -H 'Content-Type: application/json' \
  -d "{\"From\":\"target-e2e\",\"Meta\":{\"Action\":\"ConfigureModel\",\"Sequence\":2},\"Params\":{\"Action\":\"ConfigureModel\",\"model\":\"${OPENCLAW_TEST_MODEL}\",\"Reference\":\"2\"}}" \
  | tee /tmp/vmdocker_apply_configure_model_resp.json
assert_apply_action /tmp/vmdocker_apply_configure_model_resp.json "ConfigureModel" "apply configure model"

echo "\n[INFO] apply ConfigureTelegram request"
curl -sS -X POST "${BASE_URL}/apply" \
  -H 'Content-Type: application/json' \
  -d '{"From":"target-e2e","Meta":{"Action":"ConfigureTelegram","Sequence":3},"Params":{"Action":"ConfigureTelegram","dmPolicy":"open","allowFrom":"*","Reference":"3"}}' \
  | tee /tmp/vmdocker_apply_configure_telegram_resp.json
assert_apply_action /tmp/vmdocker_apply_configure_telegram_resp.json "ConfigureTelegram" "apply configure telegram"

echo "\n[DONE] responses saved to:"
echo "  /tmp/vmdocker_health_resp.json"
echo "  /tmp/vmdocker_spawn_resp.json"
echo "  /tmp/vmdocker_apply_resp.json"
echo "  /tmp/vmdocker_apply_chat_resp.json"
echo "  /tmp/vmdocker_apply_configure_model_resp.json"
echo "  /tmp/vmdocker_apply_configure_telegram_resp.json"
