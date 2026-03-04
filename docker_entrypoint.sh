#!/bin/sh
set -eu

PORT="${OPENCLAW_GATEWAY_PORT:-18789}"
BIND="${OPENCLAW_GATEWAY_BIND:-loopback}"
# Resolve a writable state dir for non-root runtimes (some environments set HOME=/).
if [ -n "${OPENCLAW_STATE_DIR:-}" ]; then
    STATE_DIR="${OPENCLAW_STATE_DIR}"
else
    BASE_HOME="${OPENCLAW_HOME:-${HOME:-}}"
    if [ -z "${BASE_HOME}" ] || [ "${BASE_HOME}" = "/" ]; then
        STATE_DIR="/tmp/.openclaw"
    else
        STATE_DIR="${BASE_HOME}/.openclaw"
    fi
fi
RUNTIME_CONFIG_PATH="${STATE_DIR}/openclaw.json"
GATEWAY_READY_WAIT_SECONDS="${OPENCLAW_GATEWAY_READY_WAIT_SECONDS:-60}"

health_probe() {
    node -e '
const url = process.argv[1];
const secret = process.env.OPENCLAW_GATEWAY_TOKEN || process.env.OPENCLAW_GATEWAY_PASSWORD || "";
const headers = secret ? { Authorization: `Bearer ${secret}` } : {};
fetch(url, { headers })
  .then((r) => process.exit(r.ok ? 0 : 1))
  .catch(() => process.exit(1));
' "$1" >/dev/null 2>&1
}

ensure_runtime_gateway_config() {
    if [ "${OPENCLAW_ENABLE_SESSION_SEND_OVER_HTTP:-true}" != "true" ]; then
        return
    fi
    if [ -n "${OPENCLAW_CONFIG_PATH:-}" ]; then
        return
    fi
    if [ -f "${RUNTIME_CONFIG_PATH}" ]; then
        return
    fi

    mkdir -p "${STATE_DIR}"
    cat >"${RUNTIME_CONFIG_PATH}" <<'EOF'
{
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
}

if [ "${RUNTIME_TYPE:-openclaw}" = "openclaw" ]; then
    ensure_runtime_gateway_config
    echo "starting openclaw gateway on ${BIND}:${PORT}"
    set -- openclaw gateway --bind "${BIND}" --port "${PORT}" --allow-unconfigured

    if [ -n "${OPENCLAW_GATEWAY_TOKEN:-}" ]; then
        set -- "$@" --auth token --token "${OPENCLAW_GATEWAY_TOKEN}"
    elif [ -n "${OPENCLAW_GATEWAY_PASSWORD:-}" ]; then
        set -- "$@" --auth password --password "${OPENCLAW_GATEWAY_PASSWORD}"
    fi

    "$@" >/tmp/openclaw-gateway.log 2>&1 &
    GW_PID=$!
    trap 'kill ${GW_PID} 2>/dev/null || true' EXIT INT TERM

    READY_BASE="${OPENCLAW_GATEWAY_URL:-http://127.0.0.1:${PORT}}"
    READY_URL_HEALTHZ="${READY_BASE%/}/healthz"
    READY_URL_HEALTH="${READY_BASE%/}/health"

    i=0
    while [ $i -lt "${GATEWAY_READY_WAIT_SECONDS}" ]; do
        if ! kill -0 "${GW_PID}" 2>/dev/null; then
            echo "openclaw gateway exited unexpectedly"
            echo "see /tmp/openclaw-gateway.log"
            exit 1
        fi

        if health_probe "${READY_URL_HEALTHZ}" || health_probe "${READY_URL_HEALTH}"; then
            echo "openclaw gateway is ready"
            break
        fi
        i=$((i + 1))
        sleep 1
    done

    if [ $i -eq "${GATEWAY_READY_WAIT_SECONDS}" ]; then
        echo "openclaw gateway did not become ready in ${GATEWAY_READY_WAIT_SECONDS}s"
        echo "see /tmp/openclaw-gateway.log"
        exit 1
    fi
fi

exec /app/main
