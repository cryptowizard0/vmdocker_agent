#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENTRYPOINT="${ROOT_DIR}/start-vmdocker-agent.sh"
WORKSPACE_ENTRYPOINT="${ROOT_DIR}/start-vmdocker-agent-workspace.sh"
TMPDIR_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/vmdocker-entrypoint.XXXXXX")"
APP_DIR="${TMPDIR_ROOT}/app"
HOOKS_DIR="${TMPDIR_ROOT}/bootstrap"
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

run_entrypoint_from_workspace_asset() {
  local runtime="$1"
  local asset_root="$2"
  TRACE_FILE="${TRACE_FILE}" \
  VMDOCKER_AGENT_APP_ROOT="${APP_DIR}" \
  VMDOCKER_RUNTIME_WORKSPACE="${TMPDIR_ROOT}/runtime" \
  VMDOCKER_AGENT_ASSET_ROOT="${asset_root}" \
  RUNTIME_TYPE="${runtime}" \
  sh "${ENTRYPOINT}"
}

run_entrypoint_with_bundle() {
  local runtime="$1"
  local asset_root="$2"
  local bundle_root="$3"
  TRACE_FILE="${TRACE_FILE}" \
  VMDOCKER_AGENT_APP_ROOT="${APP_DIR}" \
  VMDOCKER_RUNTIME_WORKSPACE="${TMPDIR_ROOT}/runtime" \
  VMDOCKER_AGENT_ASSET_ROOT="${asset_root}" \
  VMDOCKER_AGENT_BUNDLE_ROOT="${bundle_root}" \
  RUNTIME_TYPE="${runtime}" \
  sh "${ENTRYPOINT}"
}

run_workspace_entrypoint_with_bundle() {
  local asset_root="$1"
  local bundle_root="$2"
  TRACE_FILE="${TRACE_FILE}" \
  VMDOCKER_RUNTIME_WORKSPACE="${TMPDIR_ROOT}/runtime" \
  VMDOCKER_AGENT_ASSET_ROOT="${asset_root}" \
  VMDOCKER_AGENT_BUNDLE_ROOT="${bundle_root}" \
  sh "${WORKSPACE_ENTRYPOINT}"
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
run_entrypoint test
assert_contains "${TRACE_FILE}" "main:test"
assert_not_contains "${TRACE_FILE}" "hook:"

ASSET_ROOT="${TMPDIR_ROOT}/runtime/.vmdocker-agent"
mkdir -p "${ASSET_ROOT}/bootstrap"
cp "${HOOKS_DIR}/claude.sh" "${ASSET_ROOT}/bootstrap/claude.sh"
chmod +x "${ASSET_ROOT}/bootstrap/claude.sh"

: > "${TRACE_FILE}"
run_entrypoint_from_workspace_asset claude "${ASSET_ROOT}"
assert_contains "${TRACE_FILE}" "hook:claude"
assert_contains "${TRACE_FILE}" "main:claude"

BUNDLE_ROOT="${TMPDIR_ROOT}/bundle"
MATERIALIZED_ASSET_ROOT="${TMPDIR_ROOT}/materialized/.vmdocker-agent"
mkdir -p "${BUNDLE_ROOT}/bootstrap"
cp "${HOOKS_DIR}/telegramcustomer.sh" "${BUNDLE_ROOT}/bootstrap/telegramcustomer.sh"
chmod +x "${BUNDLE_ROOT}/bootstrap/telegramcustomer.sh"

: > "${TRACE_FILE}"
run_entrypoint_with_bundle telegramcustomer "${MATERIALIZED_ASSET_ROOT}" "${BUNDLE_ROOT}"
assert_contains "${TRACE_FILE}" "hook:telegramcustomer"
assert_contains "${TRACE_FILE}" "main:telegramcustomer"
test -x "${MATERIALIZED_ASSET_ROOT}/bootstrap/telegramcustomer.sh"

mkdir -p "${ASSET_ROOT}/bin"
cat > "${ASSET_ROOT}/bin/start-vmdocker-agent.sh" <<'EOF'
#!/bin/sh
EOF
chmod +x "${ASSET_ROOT}/bin/start-vmdocker-agent.sh"
cat > "${BUNDLE_ROOT}/bootstrap/claude.sh" <<'EOF'
#!/bin/sh
bootstrap_claude_main() {
    printf 'hook:bundle-claude\n' >> "${TRACE_FILE}"
}
bootstrap_claude_main "$@"
EOF
chmod +x "${BUNDLE_ROOT}/bootstrap/claude.sh"

: > "${TRACE_FILE}"
run_entrypoint_with_bundle claude "${ASSET_ROOT}" "${BUNDLE_ROOT}"
assert_contains "${TRACE_FILE}" "hook:claude"
assert_not_contains "${TRACE_FILE}" "hook:bundle-claude"

WRAPPER_BUNDLE_ROOT="${TMPDIR_ROOT}/wrapper-bundle"
WRAPPER_ASSET_ROOT="${TMPDIR_ROOT}/wrapper-runtime/.vmdocker-agent"
mkdir -p "${WRAPPER_BUNDLE_ROOT}/bin"
cat > "${WRAPPER_BUNDLE_ROOT}/bin/start-vmdocker-agent.sh" <<'EOF'
#!/bin/sh
set -eu
printf 'wrapper:workspace-entrypoint\n' >> "${TRACE_FILE}"
EOF
chmod +x "${WRAPPER_BUNDLE_ROOT}/bin/start-vmdocker-agent.sh"

: > "${TRACE_FILE}"
run_workspace_entrypoint_with_bundle "${WRAPPER_ASSET_ROOT}" "${WRAPPER_BUNDLE_ROOT}"
assert_contains "${TRACE_FILE}" "wrapper:workspace-entrypoint"
test -x "${WRAPPER_ASSET_ROOT}/bin/start-vmdocker-agent.sh"

if VMDOCKER_AGENT_APP_ROOT="${APP_DIR}" VMDOCKER_AGENT_BOOTSTRAP_DIR="${HOOKS_DIR}" RUNTIME_TYPE="unknown" sh "${ENTRYPOINT}" >"${TMPDIR_ROOT}/unknown.out" 2>"${TMPDIR_ROOT}/unknown.err"; then
  echo "[ERROR] expected unknown runtime to fail"
  exit 1
fi
assert_contains "${TMPDIR_ROOT}/unknown.err" "unsupported runtime type: unknown"

echo "[OK] entrypoint dispatch test passed"
