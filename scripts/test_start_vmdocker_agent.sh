#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENTRYPOINT="${ROOT_DIR}/start-vmdocker-agent.sh"
TMPDIR_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/vmdocker-entrypoint.XXXXXX")"
APP_DIR="${TMPDIR_ROOT}/app"
HOOKS_DIR="${TMPDIR_ROOT}/hooks"
TRACE_FILE="${TMPDIR_ROOT}/trace.log"

cleanup() {
  rm -rf "${TMPDIR_ROOT}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

mkdir -p "${APP_DIR}" "${HOOKS_DIR}"

cat > "${APP_DIR}/main" <<'EOF'
#!/bin/sh
set -eu
printf 'main:%s\n' "${RUNTIME_TYPE:-}" >> "${TRACE_FILE}"
EOF
chmod +x "${APP_DIR}/main"

cat > "${HOOKS_DIR}/openclaw.sh" <<'EOF'
#!/bin/sh
bootstrap_openclaw_main() {
    printf 'hook:openclaw\n' >> "${TRACE_FILE}"
}
bootstrap_openclaw_main "$@"
EOF
chmod +x "${HOOKS_DIR}/openclaw.sh"

cat > "${HOOKS_DIR}/claude.sh" <<'EOF'
#!/bin/sh
bootstrap_claude_main() {
    printf 'hook:claude\n' >> "${TRACE_FILE}"
}
bootstrap_claude_main "$@"
EOF
chmod +x "${HOOKS_DIR}/claude.sh"

cat > "${HOOKS_DIR}/telegramcustomer.sh" <<'EOF'
#!/bin/sh
bootstrap_telegramcustomer_main() {
    printf 'hook:telegramcustomer\n' >> "${TRACE_FILE}"
}
bootstrap_telegramcustomer_main "$@"
EOF
chmod +x "${HOOKS_DIR}/telegramcustomer.sh"

cat > "${HOOKS_DIR}/hermes.sh" <<'EOF'
#!/bin/sh
bootstrap_hermes_main() {
    printf 'hook:hermes\n' >> "${TRACE_FILE}"
}
bootstrap_hermes_main "$@"
EOF
chmod +x "${HOOKS_DIR}/hermes.sh"

assert_contains() {
  local file="$1"
  local expected="$2"
  if ! grep -Fq "${expected}" "${file}"; then
    echo "[ERROR] expected ${expected} in ${file}"
    cat "${file}" || true
    exit 1
  fi
}

assert_not_contains() {
  local file="$1"
  local unexpected="$2"
  if grep -Fq "${unexpected}" "${file}"; then
    echo "[ERROR] did not expect ${unexpected} in ${file}"
    cat "${file}" || true
    exit 1
  fi
}

run_entrypoint() {
  local runtime="$1"
  TRACE_FILE="${TRACE_FILE}" \
  VMDOCKER_AGENT_APP_ROOT="${APP_DIR}" \
  VMDOCKER_AGENT_BOOTSTRAP_DIR="${HOOKS_DIR}" \
  RUNTIME_TYPE="${runtime}" \
  sh "${ENTRYPOINT}"
}

: > "${TRACE_FILE}"
run_entrypoint openclaw
assert_contains "${TRACE_FILE}" "hook:openclaw"
assert_contains "${TRACE_FILE}" "main:openclaw"

: > "${TRACE_FILE}"
run_entrypoint claude
assert_contains "${TRACE_FILE}" "hook:claude"
assert_contains "${TRACE_FILE}" "main:claude"
assert_not_contains "${TRACE_FILE}" "hook:openclaw"

: > "${TRACE_FILE}"
run_entrypoint telegramcustomer
assert_contains "${TRACE_FILE}" "hook:telegramcustomer"
assert_contains "${TRACE_FILE}" "main:telegramcustomer"
assert_not_contains "${TRACE_FILE}" "hook:openclaw"
assert_not_contains "${TRACE_FILE}" "hook:claude"

: > "${TRACE_FILE}"
run_entrypoint hermes
assert_contains "${TRACE_FILE}" "hook:hermes"
assert_contains "${TRACE_FILE}" "main:hermes"
assert_not_contains "${TRACE_FILE}" "hook:openclaw"
assert_not_contains "${TRACE_FILE}" "hook:claude"
assert_not_contains "${TRACE_FILE}" "hook:telegramcustomer"

: > "${TRACE_FILE}"
run_entrypoint test
assert_contains "${TRACE_FILE}" "main:test"
assert_not_contains "${TRACE_FILE}" "hook:"

if VMDOCKER_AGENT_APP_ROOT="${APP_DIR}" VMDOCKER_AGENT_BOOTSTRAP_DIR="${HOOKS_DIR}" RUNTIME_TYPE="unknown" sh "${ENTRYPOINT}" >"${TMPDIR_ROOT}/unknown.out" 2>"${TMPDIR_ROOT}/unknown.err"; then
  echo "[ERROR] expected unknown runtime to fail"
  exit 1
fi
assert_contains "${TMPDIR_ROOT}/unknown.err" "unsupported runtime type: unknown"

echo "[OK] entrypoint dispatch test passed"
