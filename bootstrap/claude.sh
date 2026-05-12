#!/bin/sh

bootstrap_claude_main() {
    if ! command -v claude >/dev/null 2>&1; then
        bootstrap_fail "claude CLI is not available in PATH"
    fi

    bootstrap_info "claude runtime bootstrap ready"
}

bootstrap_claude_main "$@"
