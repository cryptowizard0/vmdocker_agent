#!/bin/sh

bootstrap_hermes_main() {
    if [ -n "${HOME:-}" ]; then
        PATH="${HOME}/.local/bin:${PATH}"
    fi
    if [ -n "${VMDOCKER_RUNTIME_HOME:-}" ]; then
        PATH="${VMDOCKER_RUNTIME_HOME}/.local/bin:${PATH}"
    fi
    export PATH

    if ! command -v hermes >/dev/null 2>&1; then
        bootstrap_fail "hermes CLI is not available in PATH"
    fi

    bootstrap_info "hermes runtime bootstrap ready"
}

bootstrap_hermes_main "$@"
