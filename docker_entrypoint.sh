#!/bin/sh
set -eu

PORT="${OPENCLAW_GATEWAY_PORT:-18789}"

if [ "${RUNTIME_TYPE:-openclaw}" = "openclaw" ]; then
    echo "starting openclaw gateway on port ${PORT}"
    openclaw gateway --port "${PORT}" --force >/tmp/openclaw-gateway.log 2>&1 &
    GW_PID=$!
    trap 'kill ${GW_PID} 2>/dev/null || true' EXIT INT TERM

    READY_BASE="${OPENCLAW_GATEWAY_URL:-http://127.0.0.1:${PORT}}"
    READY_URL="${READY_BASE%/}/health"

    i=0
    while [ $i -lt 30 ]; do
        if curl -fsS "${READY_URL}" >/dev/null 2>&1; then
            echo "openclaw gateway is ready"
            break
        fi
        i=$((i + 1))
        sleep 1
    done

    if [ $i -eq 30 ]; then
        echo "openclaw gateway did not become ready in time"
        echo "see /tmp/openclaw-gateway.log"
        exit 1
    fi
fi

exec /app/main
