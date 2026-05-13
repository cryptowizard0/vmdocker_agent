#!/bin/sh
set -eu

APP_ROOT="${VMDOCKER_AGENT_APP_ROOT:-/app}"
ASSET_ROOT="${VMDOCKER_AGENT_ASSET_ROOT:-${VMDOCKER_RUNTIME_WORKSPACE:-}/.vmdocker-agent}"
if [ "${ASSET_ROOT}" = "/.vmdocker-agent" ]; then
    ASSET_ROOT=""
fi
BUNDLE_ROOT="${VMDOCKER_AGENT_BUNDLE_ROOT:-/opt/vmdocker-agent-bundle}"
if [ -n "${ASSET_ROOT}" ] && [ -d "${BUNDLE_ROOT}" ] && [ ! -x "${ASSET_ROOT}/bin/start-vmdocker-agent.sh" ]; then
    mkdir -p "${ASSET_ROOT}"
    cp -R "${BUNDLE_ROOT}/." "${ASSET_ROOT}/"
    if [ -f "${ASSET_ROOT}/bin/start-vmdocker-agent.sh" ]; then
        chmod +x "${ASSET_ROOT}/bin/start-vmdocker-agent.sh"
    fi
    if [ -d "${ASSET_ROOT}/bootstrap" ]; then
        chmod +x "${ASSET_ROOT}"/bootstrap/*.sh 2>/dev/null || true
    fi
fi
if [ -n "${VMDOCKER_AGENT_BOOTSTRAP_DIR:-}" ]; then
    BOOTSTRAP_DIR="${VMDOCKER_AGENT_BOOTSTRAP_DIR}"
elif [ -n "${ASSET_ROOT}" ] && [ -d "${ASSET_ROOT}/bootstrap" ]; then
    BOOTSTRAP_DIR="${ASSET_ROOT}/bootstrap"
else
    BOOTSTRAP_DIR="/usr/local/lib/vmdocker-agent/bootstrap"
fi
BACKGROUND_PIDS=""

entry_info() {
    echo "[entrypoint][info] $*" >&2
}

entry_warn() {
    echo "[entrypoint][warn] $*" >&2
}

entry_fail() {
    echo "[entrypoint][fatal] $*" >&2
    exit 1
}

bootstrap_info() {
    runtime="${BOOTSTRAP_RUNTIME:-unknown}"
    echo "[bootstrap][${runtime}][info] $*" >&2
}

bootstrap_warn() {
    runtime="${BOOTSTRAP_RUNTIME:-unknown}"
    echo "[bootstrap][${runtime}][warn] $*" >&2
}

bootstrap_fail() {
    runtime="${BOOTSTRAP_RUNTIME:-unknown}"
    echo "[bootstrap][${runtime}][fatal] $*" >&2
    exit 1
}

register_background_pid() {
    pid="$1"
    if [ -z "${pid}" ]; then
        return 0
    fi
    if [ -z "${BACKGROUND_PIDS}" ]; then
        BACKGROUND_PIDS="${pid}"
    else
        BACKGROUND_PIDS="${BACKGROUND_PIDS} ${pid}"
    fi
}

cleanup_background_pids() {
    for pid in ${BACKGROUND_PIDS}; do
        kill "${pid}" 2>/dev/null || true
    done
}

trap cleanup_background_pids EXIT INT TERM

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

resolve_workspace_root() {
    for candidate in \
        "${WORKSPACE_DIR:-}" \
        "${VMDOCKER_RUNTIME_WORKSPACE:-}" \
        "${VMDOCKER_AGENT_WORKSPACE:-}" \
        "${OPENCLAW_HOME:-}"
    do
        if [ -n "${candidate}" ] && [ -d "${candidate}" ]; then
            printf '%s\n' "${candidate}"
            return 0
        fi
    done
    return 1
}

run_security_audit() {
    entry_info "running startup security audit"

    if command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then
        entry_fail "passwordless sudo is still available for user $(id -un); this is an image misconfiguration"
    fi
    entry_info "sudo escalation check passed"

    if [ -S /var/run/docker.sock ]; then
        entry_warn "docker.sock is exposed inside the sandbox; this comes from the Docker Sandbox platform, not this image"
    fi

    workspace_root="$(resolve_workspace_root || true)"
    if [ -n "${workspace_root}" ] && [ -d "${workspace_root}" ]; then
        workspace_fstype="$(audit_mount_fstype "${workspace_root}" || true)"
        if [ "${workspace_fstype}" = "virtiofs" ]; then
            entry_warn "workspace is mounted via virtiofs; host access scope is controlled by the Docker Sandbox platform"
        fi
    fi

    if [ -r /sys/module/apparmor/parameters/enabled ]; then
        apparmor_enabled="$(tr -d '\n' </sys/module/apparmor/parameters/enabled 2>/dev/null || true)"
        if [ "${apparmor_enabled}" = "Y" ] || [ "${apparmor_enabled}" = "y" ]; then
            entry_info "AppArmor support detected"
        else
            entry_warn "AppArmor support is not enabled; confinement depends on the Docker Sandbox platform"
        fi
    else
        entry_warn "AppArmor visibility is unavailable; confinement depends on the Docker Sandbox platform"
    fi

    if [ -d /sys/fs/selinux ]; then
        if [ -r /sys/fs/selinux/enforce ] && [ "$(cat /sys/fs/selinux/enforce 2>/dev/null || echo 0)" = "1" ]; then
            entry_info "SELinux enforcing mode detected"
        else
            entry_warn "SELinux is present but not enforcing; confinement depends on the Docker Sandbox platform"
        fi
    else
        entry_warn "SELinux is not visible inside the sandbox; confinement depends on the Docker Sandbox platform"
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

validate_runtime_type() {
    case "$1" in
        openclaw|claude|telegramcustomer|test)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

run_bootstrap_hook() {
    runtime="$1"
    hook_path="${BOOTSTRAP_DIR}/${runtime}.sh"

    if [ ! -f "${hook_path}" ]; then
        entry_info "no bootstrap hook configured for runtime ${runtime}"
        return 0
    fi
    if [ ! -x "${hook_path}" ]; then
        entry_fail "bootstrap hook exists but is not executable: ${hook_path}"
    fi

    BOOTSTRAP_RUNTIME="${runtime}"
    export BOOTSTRAP_RUNTIME APP_ROOT
    . "${hook_path}"
    BOOTSTRAP_RUNTIME=""
    unset BOOTSTRAP_RUNTIME || true
}

if [ ! -x "${APP_ROOT}/main" ]; then
    entry_fail "${APP_ROOT}/main is missing or not executable"
fi

runtime_type="${RUNTIME_TYPE:-openclaw}"
if ! validate_runtime_type "${runtime_type}"; then
    entry_fail "unsupported runtime type: ${runtime_type}"
fi

entry_info "runtime type selected: ${runtime_type}"
run_security_audit
run_bootstrap_hook "${runtime_type}"

exec "${APP_ROOT}/main"
