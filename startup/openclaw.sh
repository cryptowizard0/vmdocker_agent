#!/bin/sh
# Default openclaw start.sh (platform template).
#
# The adapter has already run PrepareOpenclawRuntime and exported
# OPENCLAW_STATE_DIR / OPENCLAW_CONFIG_PATH / OPENCLAW_GATEWAY_LOG_PATH before
# invoking this script. This template starts the gateway in the background and
# returns; the adapter (PID 1) supervises it and gates /vmm/health on it.
#
# Copy and edit this file in your module to customize engine startup.
set -eu

PORT="${OPENCLAW_GATEWAY_PORT:-18789}"
BIND="${OPENCLAW_GATEWAY_BIND:-loopback}"
LOG="${OPENCLAW_GATEWAY_LOG_PATH:-/tmp/openclaw-gateway.log}"

set -- openclaw gateway --bind "${BIND}" --port "${PORT}" --allow-unconfigured
if [ -n "${OPENCLAW_GATEWAY_TOKEN:-}" ]; then
    set -- "$@" --auth token --token "${OPENCLAW_GATEWAY_TOKEN}"
elif [ -n "${OPENCLAW_GATEWAY_PASSWORD:-}" ]; then
    set -- "$@" --auth password --password "${OPENCLAW_GATEWAY_PASSWORD}"
fi

"$@" >"${LOG}" 2>&1 &

# Add module-specific initialization / workspace seeding below this line.
