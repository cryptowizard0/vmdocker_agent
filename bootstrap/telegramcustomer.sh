#!/bin/sh

bootstrap_telegramcustomer_main() {
    if ! command -v hermes >/dev/null 2>&1; then
        bootstrap_fail "hermes CLI is not available in PATH"
    fi

    bootstrap_info "telegramcustomer runtime bootstrap ready"
}

bootstrap_telegramcustomer_main "$@"
