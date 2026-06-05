#!/bin/sh
set -eu

ASSET_ROOT="${VMDOCKER_AGENT_ASSET_ROOT:-${VMDOCKER_RUNTIME_WORKSPACE:-}/.vmdocker-agent}"
if [ "${ASSET_ROOT}" = "/.vmdocker-agent" ]; then
    ASSET_ROOT=""
fi
BUNDLE_ROOT="${VMDOCKER_AGENT_BUNDLE_ROOT:-/opt/vmdocker-agent-bundle}"

if [ -z "${ASSET_ROOT}" ]; then
    echo "[workspace-entrypoint][fatal] VMDOCKER_RUNTIME_WORKSPACE or VMDOCKER_AGENT_ASSET_ROOT is required" >&2
    exit 1
fi

if [ ! -x "${ASSET_ROOT}/bin/start-vmdocker-agent.sh" ] && [ -d "${BUNDLE_ROOT}" ]; then
    mkdir -p "${ASSET_ROOT}"
    cp -R "${BUNDLE_ROOT}/." "${ASSET_ROOT}/"
    if [ -f "${ASSET_ROOT}/bin/start-vmdocker-agent.sh" ]; then
        chmod +x "${ASSET_ROOT}/bin/start-vmdocker-agent.sh"
    fi
    if [ -d "${ASSET_ROOT}/bootstrap" ]; then
        chmod +x "${ASSET_ROOT}"/bootstrap/*.sh 2>/dev/null || true
    fi
fi

if [ ! -x "${ASSET_ROOT}/bin/start-vmdocker-agent.sh" ]; then
    echo "[workspace-entrypoint][fatal] missing executable ${ASSET_ROOT}/bin/start-vmdocker-agent.sh" >&2
    exit 1
fi

exec "${ASSET_ROOT}/bin/start-vmdocker-agent.sh"
