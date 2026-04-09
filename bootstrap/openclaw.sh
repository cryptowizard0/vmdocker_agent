#!/bin/sh

bootstrap_openclaw_main() {
    if [ ! -x "${APP_ROOT}/bootstrap" ]; then
        bootstrap_fail "${APP_ROOT}/bootstrap is missing or not executable"
    fi

    eval "$("${APP_ROOT}/bootstrap" prepare --shell)"
    export OPENCLAW_STATE_DIR
    export OPENCLAW_CONFIG_PATH
    export OPENCLAW_GATEWAY_LOG_PATH

    port="${OPENCLAW_GATEWAY_PORT:-18789}"
    bind="${OPENCLAW_GATEWAY_BIND:-loopback}"
    wait_seconds="${OPENCLAW_GATEWAY_READY_WAIT_SECONDS:-60}"

    bootstrap_info "starting openclaw gateway on ${bind}:${port}"
    set -- openclaw gateway --bind "${bind}" --port "${port}" --allow-unconfigured

    if [ -n "${OPENCLAW_GATEWAY_TOKEN:-}" ]; then
        set -- "$@" --auth token --token "${OPENCLAW_GATEWAY_TOKEN}"
    elif [ -n "${OPENCLAW_GATEWAY_PASSWORD:-}" ]; then
        set -- "$@" --auth password --password "${OPENCLAW_GATEWAY_PASSWORD}"
    fi

    "$@" >"${OPENCLAW_GATEWAY_LOG_PATH}" 2>&1 &
    gw_pid=$!
    register_background_pid "${gw_pid}"

    ready_base="${OPENCLAW_GATEWAY_URL:-http://127.0.0.1:${port}}"
    ready_healthz="${ready_base%/}/healthz"
    ready_health="${ready_base%/}/health"

    i=0
    while [ "${i}" -lt "${wait_seconds}" ]; do
        if ! kill -0 "${gw_pid}" 2>/dev/null; then
            bootstrap_fail "openclaw gateway exited unexpectedly; see ${OPENCLAW_GATEWAY_LOG_PATH}"
        fi

        if health_probe "${ready_healthz}" || health_probe "${ready_health}"; then
            bootstrap_info "openclaw gateway is ready"
            return 0
        fi

        i=$((i + 1))
        sleep 1
    done

    bootstrap_fail "openclaw gateway did not become ready in ${wait_seconds}s; see ${OPENCLAW_GATEWAY_LOG_PATH}"
}

bootstrap_openclaw_main "$@"
