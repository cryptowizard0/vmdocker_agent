# Repository Guidelines

## Project Structure & Module Organization
This repository is a Go service that exposes a VMM-compatible HTTP API.

- `main.go`: process entrypoint (`server.New(8080).Run()`).
- `server/`: API routes and server lifecycle (`/vmm/health`, `/vmm/spawn`, `/vmm/apply`).
- `runtime/`: runtime selection and implementations.
- `runtime/testrt/`: in-memory test runtime.
- `runtime/openclaw/`: Openclaw gateway-backed runtime (`openclaw.go`: core logic, `tools.go`: tool definitions, `setup.go`: initialization, `gateway.go`: HTTP client).
- `common/`: shared logging and middleware.
- `utils/`: helper utilities.
- `scripts/`: test and utility scripts.
- `bootstrap/`: runtime-specific startup hooks sourced by the shared entrypoint.
- `docker_build_*.sh`, `Dockerfile.*`: container build helpers.

Keep new runtime implementations under `runtime/` and add package-local tests alongside code.

## Build, Test, and Development Commands
- `go run main.go`: run the API locally on port `8080`.
- `go build -o vmdocker-container`: build local binary.
- `go test ./...`: run all Go tests (currently passes).
- `go test -v -cover ./...`: verbose tests with coverage.
- `./docker_build_openclaw.sh [TAG]`: build the OpenClaw-oriented image from `Dockerfile.openclaw`.
- `./docker_build_claude.sh [TAG]`: build the Claude-oriented image from `Dockerfile.claude`.
- `./scripts/docker_test_requests.sh`: OpenClaw container smoke test.
- `./scripts/docker_test_claude.sh`: Claude container smoke test including checkpoint/restore.
- `./scripts/test_start_vmdocker_agent.sh`: local entrypoint dispatch regression test.

## Coding Style & Naming Conventions
Use standard Go formatting and idioms:

- Format with `gofmt -w <file>` before committing.
- Package names are lowercase (`openclaw`, `testrt`).
- Exported symbols use PascalCase; internal helpers use camelCase.
- Test files use `*_test.go`, with `TestXxx` and clear `t.Fatalf(...)` messages.

Environment-driven config uses explicit names (for example `RUNTIME_TYPE`, `OPENCLAW_GATEWAY_URL`, `OPENCLAW_TIMEOUT_MS`).

## Testing Guidelines
Primary framework is Go’s built-in `testing` package with `httptest` for API/runtime behavior.

- Add unit tests next to changed package code.
- Prefer table-style or subtests (`t.Run(...)`) for action variants.
- Cover success and failure paths for JSON binding, runtime init, and gateway errors.

## Commit & Pull Request Guidelines
Recent commits use short, imperative summaries (for example: `Update vmdocker dependency to v0.0.2`).

- Commit message format: `Verb + scope/outcome` in one concise line.
- PRs should include: purpose, key behavior changes, env var impacts, and test evidence (command + result).
- If API behavior changes, include example request/response for `/vmm/*` endpoints.
