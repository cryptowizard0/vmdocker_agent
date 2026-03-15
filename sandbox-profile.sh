#!/bin/sh

workspace_root="${WORKSPACE_DIR:-}"
if [ -z "${workspace_root}" ]; then
    return 0 2>/dev/null || exit 0
fi

export OPENCLAW_HOME="${OPENCLAW_HOME:-${workspace_root}}"
export OPENCLAW_STATE_DIR="${OPENCLAW_STATE_DIR:-${workspace_root}/.openclaw}"
export OPENCLAW_CONFIG_PATH="${OPENCLAW_CONFIG_PATH:-${OPENCLAW_STATE_DIR}/openclaw.json}"
export OPENCLAW_AGENT_WORKSPACE="${OPENCLAW_AGENT_WORKSPACE:-${OPENCLAW_STATE_DIR}/workspace}"
export HOME="${HOME:-${workspace_root}/.home}"
export TMPDIR="${TMPDIR:-${workspace_root}/.tmp}"
export XDG_CONFIG_HOME="${XDG_CONFIG_HOME:-${workspace_root}/.xdg/config}"
export XDG_CACHE_HOME="${XDG_CACHE_HOME:-${workspace_root}/.xdg/cache}"
export XDG_STATE_HOME="${XDG_STATE_HOME:-${workspace_root}/.xdg/state}"
export NODE_DISABLE_COMPILE_CACHE="${NODE_DISABLE_COMPILE_CACHE:-1}"

mkdir -p \
    "${OPENCLAW_STATE_DIR}" \
    "${OPENCLAW_AGENT_WORKSPACE}" \
    "${HOME}" \
    "${TMPDIR}" \
    "${XDG_CONFIG_HOME}" \
    "${XDG_CACHE_HOME}" \
    "${XDG_STATE_HOME}" \
    2>/dev/null || true
