#!/bin/sh
set -eu

APP_ROOT="${VMDOCKER_AGENT_APP_ROOT:-/app}"

audit_info() {
    echo "[security][info] $*" >&2
}

audit_warn() {
    echo "[security][warn] $*" >&2
}

audit_fail() {
    echo "[security][fatal] $*" >&2
    exit 1
}

audit_mount_fstype() {
    target="$1"
    if command -v stat >/dev/null 2>&1; then
        stat -f -c %T "${target}" 2>/dev/null && return 0
        stat -f %T "${target}" 2>/dev/null && return 0
    fi
    if command -v mount >/dev/null 2>&1; then
        mount 2>/dev/null | awk -v path="${target}" '$3 == path { print $5; exit }'
    fi
}

run_security_audit() {
    audit_info "running startup security audit"

    if command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then
        audit_fail "passwordless sudo is still available for user $(id -un); this is an image misconfiguration"
    fi
    audit_info "sudo escalation check passed"

    if [ -S /var/run/docker.sock ]; then
        audit_warn "docker.sock is exposed inside the sandbox; this comes from the Docker Sandbox platform, not this image"
    fi

    workspace_root="${WORKSPACE_DIR:-${OPENCLAW_HOME:-}}"
    if [ -n "${workspace_root}" ] && [ -d "${workspace_root}" ]; then
        workspace_fstype="$(audit_mount_fstype "${workspace_root}" || true)"
        if [ "${workspace_fstype}" = "virtiofs" ]; then
            audit_warn "workspace is mounted via virtiofs; host access scope is controlled by the Docker Sandbox platform"
        fi
    fi

    if [ -r /sys/module/apparmor/parameters/enabled ]; then
        apparmor_enabled="$(tr -d '\n' </sys/module/apparmor/parameters/enabled 2>/dev/null || true)"
        if [ "${apparmor_enabled}" = "Y" ] || [ "${apparmor_enabled}" = "y" ]; then
            audit_info "AppArmor support detected"
        else
            audit_warn "AppArmor support is not enabled; confinement depends on the Docker Sandbox platform"
        fi
    else
        audit_warn "AppArmor visibility is unavailable; confinement depends on the Docker Sandbox platform"
    fi

    if [ -d /sys/fs/selinux ]; then
        if [ -r /sys/fs/selinux/enforce ] && [ "$(cat /sys/fs/selinux/enforce 2>/dev/null || echo 0)" = "1" ]; then
            audit_info "SELinux enforcing mode detected"
        else
            audit_warn "SELinux is present but not enforcing; confinement depends on the Docker Sandbox platform"
        fi
    else
        audit_warn "SELinux is not visible inside the sandbox; confinement depends on the Docker Sandbox platform"
    fi
}

health_probe() {
    url="$1"
    auth_token="${OPENCLAW_GATEWAY_TOKEN:-${OPENCLAW_GATEWAY_PASSWORD:-}}"

    if command -v curl >/dev/null 2>&1; then
        if [ -n "${auth_token}" ]; then
            curl -fsS -H "Authorization: Bearer ${auth_token}" "${url}" >/dev/null
        else
            curl -fsS "${url}" >/dev/null
        fi
        return $?
    fi

    if command -v wget >/dev/null 2>&1; then
        if [ -n "${auth_token}" ]; then
            wget -qO- --header="Authorization: Bearer ${auth_token}" "${url}" >/dev/null
        else
            wget -qO- "${url}" >/dev/null
        fi
        return $?
    fi

    echo "neither curl nor wget is available for gateway health checks" >&2
    return 1
}

prepare_openclaw_runtime() {
    if [ ! -x "${APP_ROOT}/bootstrap" ]; then
        echo "${APP_ROOT}/bootstrap is missing or not executable" >&2
        exit 1
    fi

    eval "$("${APP_ROOT}/bootstrap" prepare --shell)"
    export OPENCLAW_STATE_DIR
    export OPENCLAW_CONFIG_PATH
    export OPENCLAW_GATEWAY_LOG_PATH
}

start_openclaw_gateway() {
    port="${OPENCLAW_GATEWAY_PORT:-18789}"
    bind="${OPENCLAW_GATEWAY_BIND:-loopback}"
    wait_seconds="${OPENCLAW_GATEWAY_READY_WAIT_SECONDS:-60}"

    prepare_openclaw_runtime

    echo "starting openclaw gateway on ${bind}:${port}"
    set -- openclaw gateway --bind "${bind}" --port "${port}" --allow-unconfigured

    if [ -n "${OPENCLAW_GATEWAY_TOKEN:-}" ]; then
        set -- "$@" --auth token --token "${OPENCLAW_GATEWAY_TOKEN}"
    elif [ -n "${OPENCLAW_GATEWAY_PASSWORD:-}" ]; then
        set -- "$@" --auth password --password "${OPENCLAW_GATEWAY_PASSWORD}"
    fi

    "$@" >"${OPENCLAW_GATEWAY_LOG_PATH}" 2>&1 &
    gw_pid=$!
    trap 'kill ${gw_pid} 2>/dev/null || true' EXIT INT TERM

    ready_base="${OPENCLAW_GATEWAY_URL:-http://127.0.0.1:${port}}"
    ready_healthz="${ready_base%/}/healthz"
    ready_health="${ready_base%/}/health"

    i=0
    while [ "${i}" -lt "${wait_seconds}" ]; do
        if ! kill -0 "${gw_pid}" 2>/dev/null; then
            echo "openclaw gateway exited unexpectedly" >&2
            echo "see ${OPENCLAW_GATEWAY_LOG_PATH}" >&2
            exit 1
        fi

        if health_probe "${ready_healthz}" || health_probe "${ready_health}"; then
            echo "openclaw gateway is ready"
            return 0
        fi

        i=$((i + 1))
        sleep 1
    done

    echo "openclaw gateway did not become ready in ${wait_seconds}s" >&2
    echo "see ${OPENCLAW_GATEWAY_LOG_PATH}" >&2
    exit 1
}

if [ "${RUNTIME_TYPE:-openclaw}" = "openclaw" ]; then
    run_security_audit
    start_openclaw_gateway
fi

"${APP_ROOT}/main"
