#!/bin/sh
# Verifies the user startup hook runs in isolation and never prevents the
# wrapper from reaching the adapter launch.
set -eu

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
WRAPPER="${REPO_DIR}/start-vmdocker-agent.sh"
TMP="$(mktemp -d)"

cleanup() {
    rm -rf "${TMP}"
}
trap cleanup EXIT

VMDOCKER_WRAPPER_LIB=1
USER_STARTUP_HOOK="${TMP}/hook.sh"
VMDOCKER_STARTUP_HOOK_TIMEOUT=2
export VMDOCKER_WRAPPER_LIB USER_STARTUP_HOOK VMDOCKER_STARTUP_HOOK_TIMEOUT
. "${WRAPPER}"
trap cleanup EXIT

run_case() {
    name="$1"
    hook_body="$2"
    printf '%s\n' "${hook_body}" > "${TMP}/hook.sh"
    chmod +x "${TMP}/hook.sh"

    start="$(date +%s)"
    run_user_startup_hook
    rc=$?
    end="$(date +%s)"
    elapsed=$((end - start))

    if [ "${rc}" -ne 0 ]; then
        echo "FAIL [${name}]: run_user_startup_hook returned ${rc}, want 0"
        exit 1
    fi
    if [ "${elapsed}" -gt 5 ]; then
        echo "FAIL [${name}]: took ${elapsed}s, hook was not bounded"
        exit 1
    fi
    echo "ok   [${name}] (${elapsed}s)"
}

run_case "blocking" '#!/bin/sh
while true; do sleep 1; done'

run_case "exit-nonzero" '#!/bin/sh
exit 7'

run_case "exec-user-bin" '#!/bin/sh
exec sleep 10'

if [ "${RUN_E2E:-1}" = "1" ]; then
    E2E_DIR="$(mktemp -d)"
    cat > "${E2E_DIR}/adapter.sh" <<'EOF'
#!/bin/sh
echo "adapter-started" > "${E2E_MARKER}"
EOF
    chmod +x "${E2E_DIR}/adapter.sh"
    printf '#!/bin/sh\nwhile true; do sleep 1; done\n' > "${E2E_DIR}/hook.sh"
    chmod +x "${E2E_DIR}/hook.sh"

    E2E_MARKER="${E2E_DIR}/marker"
    export E2E_MARKER
    env RUNTIME_TYPE=openclaw \
        VMDOCKER_WRAPPER_LIB=0 \
        VMDOCKER_ADAPTER_BIN="${E2E_DIR}/adapter.sh" \
        USER_STARTUP_HOOK="${E2E_DIR}/hook.sh" \
        VMDOCKER_STARTUP_HOOK_TIMEOUT=2 \
        VMDOCKER_AGENT_APP_ROOT="${E2E_DIR}" \
        sh "${WRAPPER}" || true

    if [ ! -f "${E2E_MARKER}" ]; then
        echo "FAIL [e2e]: adapter was never reached despite blocking hook"
        rm -rf "${E2E_DIR}"
        exit 1
    fi
    echo "ok   [e2e] adapter reached after bounded blocking hook"
    rm -rf "${E2E_DIR}"
fi

echo "ALL PASS: user startup hook is isolated; wrapper always reaches adapter"
