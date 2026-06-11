#!/bin/sh

bootstrap_hermes_main() {
    if [ -n "${HOME:-}" ]; then
        PATH="${HOME}/.local/bin:${PATH}"
    fi
    if [ -n "${VMDOCKER_RUNTIME_HOME:-}" ]; then
        PATH="${VMDOCKER_RUNTIME_HOME}/.local/bin:${PATH}"
    fi
    export PATH

    if { [ -n "${BH_TMP_DIR:-}" ] && mkdir -p "${BH_TMP_DIR}" 2>/dev/null && touch "${BH_TMP_DIR}/.write-test" 2>/dev/null && rm -f "${BH_TMP_DIR}/.write-test" 2>/dev/null; } && \
       { [ -n "${BH_RUNTIME_DIR:-}" ] && mkdir -p "${BH_RUNTIME_DIR}" 2>/dev/null && touch "${BH_RUNTIME_DIR}/.write-test" 2>/dev/null && rm -f "${BH_RUNTIME_DIR}/.write-test" 2>/dev/null; }; then
        :
    else
        for candidate in /dev/shm/hermes-bh /run/hermes-bh /tmp/hermes-bh; do
            if mkdir -p "${candidate}" 2>/dev/null && touch "${candidate}/.write-test" 2>/dev/null; then
                rm -f "${candidate}/.write-test" 2>/dev/null || true
                BH_TMP_DIR="${candidate}"
                BH_RUNTIME_DIR="${candidate}"
                export BH_TMP_DIR BH_RUNTIME_DIR
                break
            fi
        done
    fi

    if ! command -v hermes >/dev/null 2>&1; then
        bootstrap_fail "hermes CLI is not available in PATH"
    fi

    bootstrap_info "hermes runtime bootstrap ready"
}

bootstrap_hermes_main "$@"
