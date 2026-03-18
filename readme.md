# VMDocker Container

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://golang.org/)
[![Docker](https://img.shields.io/badge/docker-supported-blue.svg)](https://www.docker.com/)

VMDocker Container is a Docker-based runtime environment designed to execute computational tasks for `HyMatrix`, working seamlessly with `Vmdocker` for distributed computing scenarios.

More about HyMatrix & Vmdocker:
> - [Vmdocker](https://github.com/cryptowizard0/vmdocker)
> - [HyMatrix Website](https://hymatrix.com/)
> - [HyMatrix Documentation](https://docs.hymatrix.com/)

## 🚀 Features

- **Runtime Modes**: Supports `openclaw` and in-memory `test` runtimes via `RUNTIME_TYPE`
- **Docker Integration**: Containerized deployment for consistency
- **RESTful API**: `/vmm/health`, `/vmm/spawn`, `/vmm/apply`

## ⚙️ Configuration

The runtime behavior can be customized via environment variables.

### General
- `RUNTIME_TYPE`: Runtime implementation selector (e.g., `openclaw`, `test`).
- `OPENCLAW_GATEWAY_URL`: Base URL for the Openclaw gateway (default: `http://127.0.0.1:18789`).
- `OPENCLAW_GATEWAY_TOKEN`: Authentication token for the gateway (optional).
- `OPENCLAW_TIMEOUT_MS`: Request timeout in milliseconds (default: `30000`).
- `OPENCLAW_CONFIG_TEMPLATE_PATH`: Read-only template copied on first startup (default: `/app/openclaw.default.json`).

### Runtime Bootstrap
- `OPENCLAW_STATE_DIR`: Explicit writable state directory. Highest priority.
- In Docker Sandbox mode, `vmdocker` should pass `OPENCLAW_STATE_DIR` using the resolved sandbox workspace path so OpenClaw state persists in the mounted host workspace.
- `OPENCLAW_AGENT_WORKSPACE`: Optional override for OpenClaw's agent workspace. In Docker Sandbox mode this should be set to a path inside the mapped workspace, typically `<mapped>/.openclaw/workspace`.
- `OPENCLAW_HOME`: Alternate home base. Runtime will use `<OPENCLAW_HOME>/.openclaw` when `OPENCLAW_STATE_DIR` is unset.
- `HOME`: Fallback home base. Runtime will use `<HOME>/.openclaw` when `OPENCLAW_STATE_DIR` and `OPENCLAW_HOME` are unset.
- Runtime only falls back to `/tmp/.openclaw` when no usable home directory is available.
- `OPENCLAW_CONFIG_PATH`: Optional override for the effective runtime config path. If unset, runtime uses `<state-dir>/openclaw.json`.
- `OPENCLAW_GATEWAY_MODE`: Optional gateway mode written into the effective config only when the config file is first materialized.
- `OPENCLAW_GATEWAY_READY_WAIT_SECONDS`: Startup health-check timeout for the embedded gateway.
- `NODE_DISABLE_COMPILE_CACHE`: Defaults to `1` in the sandbox image to avoid Node 22 compile-cache crashes during OpenClaw hot restarts after config changes such as Telegram setup.

### Session Management
- `OPENCLAW_SESSION_KEY`: Fallback session key if session creation fails (default: `main`).
- `OPENCLAW_SESSION_TITLE`: Title for the created session (optional).
- `OPENCLAW_SESSION_METADATA_JSON`: JSON string containing initial session metadata (optional).

### Tool Names
Customize the tool names invoked on the gateway:
- `OPENCLAW_TOOL_CREATE_SESSION`: Tool for creating sessions (default: `sessions_create`).
- `OPENCLAW_TOOL_CLOSE_SESSION`: Tool for closing sessions (default: `sessions_delete`).
- `OPENCLAW_TOOL_SEND_SESSION`: Default tool for sending messages (default: `sessions_send`).
- `OPENCLAW_TOOL_QUERY`: Tool for `Query` action (default: `sessions_send`).
- `OPENCLAW_TOOL_EXECUTE`: Tool for `Execute` action (default: `sessions_send`).
- `OPENCLAW_TOOL_CHAT`: Tool for `Chat` action (default: `sessions_send`).
- `OPENCLAW_TOOL_SET_MODEL`: Tool for configuring models (default: `session_status`).
- `OPENCLAW_TOOL_GATEWAY`: Tool for gateway configuration (default: `gateway`).

### Endpoints
Customize the API paths appended to the gateway base URL:
- `OPENCLAW_ENDPOINT_PING`: Path for ping action (default: `/health`).
- `OPENCLAW_ENDPOINT_QUERY`: Path for query action (default: `/tools/invoke`).
- `OPENCLAW_ENDPOINT_EXECUTE`: Path for execute action (default: `/tools/invoke`).
- `OPENCLAW_ENDPOINT_CHAT`: Path for chat action (default: `/tools/invoke`).
- `OPENCLAW_ENDPOINT_CREATE_SESSION`: Path for create session action (default: `/tools/invoke`).
- `OPENCLAW_ENDPOINT_CLOSE_SESSION`: Path for close session action (default: `/tools/invoke`).
- `OPENCLAW_ENDPOINT_CONFIGURE_MODEL`: Path for configure model action (default: `/tools/invoke`).
- `OPENCLAW_ENDPOINT_CONFIGURE_TELEGRAM`: Path for configure telegram action (default: `/tools/invoke`).

## 🐳 OCI Image Workflow

### Prerequisites

- Docker installed and running
- Go 1.24+ (for local development)

### Build OCI Image

```bash
./docker_build.sh <VERSION>
```

- `<VERSION>`: Image version tag (e.g., v1.0.0, latest, dev)

```bash
./docker_build.sh v1.0.0
./docker_build.sh latest
```

### Run OCI Container

```bash
./docker_run.sh
```

The OCI image still exposes `/vmm/health`, `/vmm/spawn`, and `/vmm/apply` on port `8080`.

## 🧪 Docker Sandbox Template Workflow

This repository also ships a Docker Sandboxes template image for Docker Desktop 4.58+.

Docker Sandbox exposes the primary workspace inside the sandbox at the same absolute path as on the host. Persistent OpenClaw state should therefore be configured through `OPENCLAW_STATE_DIR` based on that resolved host workspace path, rather than assuming a fixed in-sandbox mount like `/workspace`.

For tighter workspace confinement, set `OPENCLAW_AGENT_WORKSPACE` inside the mapped directory as well. The runtime will then force `agents.defaults.workspace` to that path and enable `tools.fs.workspaceOnly=true`, so OpenClaw's own workspace files no longer fall back to `~/.openclaw/workspace`.

Sandbox startup now runs a security audit before launching OpenClaw. The image treats passwordless `sudo` for `agent` as a fatal misconfiguration and refuses to start if it is still present. Platform-level exposures such as `docker.sock`, `virtiofs`, and missing AppArmor/SELinux visibility are logged as high-priority warnings but do not block startup, because they are controlled by Docker Sandbox rather than this image.

Recommended deployment posture:
- Do not treat Docker Sandbox alone as a hard trust boundary.
- If your platform allows it, avoid exposing `/var/run/docker.sock` to untrusted sandboxes.
- Limit the shared workspace path to the smallest host directory you actually need.

### Build Sandbox Template

```bash
./docker_build_sandbox.sh <VERSION>
```

Example:

```bash
./docker_build_sandbox.sh latest
```

This step only builds the sandbox template image from [`Dockerfile.sandbox`](/Users/webbergao/work/src/HymxWorkspace/vmdocker_agent/Dockerfile.sandbox). It does not create or run a sandbox yet.

### Create And Run Sandbox

Use the helper script:

```bash
./docker_run_sandbox.sh
```

Or run the Docker Sandbox commands directly:

```bash
docker sandbox create --name hymatrix-openclaw-sandbox -t chriswebber/docker-openclaw-sandbox:latest shell /path/to/workspace
docker sandbox run hymatrix-openclaw-sandbox
```

If you want Docker to create and run in one go, the CLI also supports:

```bash
docker sandbox run --name hymatrix-openclaw-sandbox -t chriswebber/docker-openclaw-sandbox:latest shell /path/to/workspace
```

After the sandbox is running, start the service inside it with:

```bash
docker sandbox exec hymatrix-openclaw-sandbox sh -lc 'start-vmdocker-agent.sh'
```

Cloud/provider usage is the only supported sandbox model path in this release. Docker Model Runner localhost bridging is intentionally out of scope for now.

## 🛠️ Local Development

### Running Locally

```bash
# Run directly with Go
go run main.go

# Or build and run binary
go build -o vmdocker-container
./vmdocker-container
```

### Generating A VMDocker Module

`vmdocker_agent` now owns the module-generation flow for sandbox modules.

```bash
go run ./cmd/module
```

The command automatically reads `/Users/webbergao/work/src/HymxWorkspace/vmdocker_agent/.env`.

Fill the common entries there first:

```dotenv
VMDOCKER_URL=http://127.0.0.1:8080
VMDOCKER_PRIVATE_KEY=
```

Then enable one generation mode in the same file:

- Pull mode: `VMDOCKER_SANDBOX_IMAGE_NAME`, optional `VMDOCKER_SANDBOX_IMAGE_ID`
- Build mode: `VMDOCKER_BUILD_DOCKERFILE`, `VMDOCKER_BUILD_CONTEXT_DIR`, `VMDOCKER_BUILD_TAG`

The repository already includes a ready-to-fill `.env` template with all of these entries commented by mode.

What `go run ./cmd/module` does now:

1. Resolve the final local image using Pull mode or Build mode
2. Export that image with `docker save`
3. Compress the archive with `gzip`
4. Store the compressed bytes in the generated module bundle `data`
5. Write a local file `mod-<module-id>.json`
6. Print the generated module id and local file path

The generated module is therefore self-contained for cold starts. If a VMDocker node later does not have the image locally, it can restore it from the module file instead of rebuilding it from a Dockerfile.

### Testing

```bash
# Run all tests
go test -v ./...

# Run tests with coverage
go test -v -cover ./...

# OCI smoke test
./scripts/docker_test_requests.sh

# Sandbox smoke test
./scripts/docker_test_sandbox.sh
```

## 📡 API Reference

Base path: `/vmm`

### POST `/vmm/health`

Simple liveness endpoint.

Request:
```json
{}
```

Response:
```json
{"status":"ok"}
```

### POST `/vmm/spawn`

Initialize runtime instance (must be called before `/vmm/apply`).

Request fields:
- `Pid` (string): process id from caller
- `Owner` (string): owner id
- `CuAddr` (string): compute unit address
- `Evn` (object): runtime env map
- `Tags` (array): tags passed to runtime

Openclaw setup keys are now read from `Tags` (`Tag.Name` => key, `Tag.Value` => value).

Supported spawn tag keys (Openclaw):
- `model` / `Model` / `modelName` / `ModelName`: initial model
- `provider` / `Provider`: provider prefix helper for model composition
- `apiKey` / `ApiKey` / `APIKey` / `modelApiKey` / `ModelApiKey`: provider API key; runtime writes it into OpenClaw auth store (`auth-profiles.json`) as `<provider>:default`
- `botToken`, `defaultAccount`, `dmPolicy`, `allowFrom`: initial Telegram patch fields

If `apiKey` is omitted, the runtime does not fail fast. This is intentional for sandbox deployments that rely on externally injected provider credentials instead of local auth-store writes.

Example:
```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/spawn \
  -H 'Content-Type: application/json' \
  -d '{
    "Pid":"pid-1",
    "Owner":"owner-1",
    "CuAddr":"cu-1",
    "Evn":{},
    "Tags":[
      {"name":"model","value":"kimi-coding/k2p5"},
      {"name":"apiKey","value":"<YOUR_MODEL_API_KEY>"}
    ]
  }'
```

### POST `/vmm/apply`

Run one runtime action.

Request fields:
- `From` (string): message target fallback
- `Meta.Action` (string): action name
- `Meta.Sequence` (number): request id fallback
- `Meta.Data` (string, optional): fallback command/model source
- `Params` (object): action parameters

Common `Params` fields:
- `Action` / `action`: action override if `Meta.Action` is empty
- `Reference` / `reference`: explicit request id

#### Supported Actions

1. `Ping`
2. `Query`
3. `Execute`
4. `Chat`
5. `CreateSession`
6. `CloseSession`
7. `ConfigureModel` (alias: `SetModel`)
8. `ConfigureTelegram` (aliases: `TelegramConfig`, `SetTelegram`)

Action resolution notes:
- Action name is case-insensitive.
- If missing, defaults to `Query`.

#### Action Parameters

`Query` / `Execute` / `Chat`:
- command text (first non-empty wins):
  - `Params.command`, `Params.Command`
  - `Params.prompt`, `Params.Prompt`
  - `Params.input`, `Params.Input`
  - `Params.data`, `Params.Data`
  - fallback: `Meta.Data`
- `Params.timeoutSeconds` / `Params.TimeoutSeconds` (int, optional)

`CreateSession`:
- no required apply params (session is created via gateway tool)

`CloseSession`:
- no required apply params (closes current runtime session)

`ConfigureModel`:
- model value (first non-empty wins):
  - `Params.model`, `Params.Model`
  - `Params.modelName`, `Params.ModelName`
  - fallback: `Meta.Data`
- optional provider composition:
  - `Params.provider` / `Params.Provider`
  - when both provider + model provided and model has no `/`, runtime sends `provider/model`

`ConfigureTelegram`:
- patch fields:
  - `Params.botToken` (string)
  - `Params.defaultAccount` (string)
  - `Params.dmPolicy` (string)
  - `Params.allowFrom` (string)
- `allowFrom` supports comma-separated (`"@alice,+1555"`) or JSON array string (`"[\"@alice\",\"+1555\"]"`)

#### Apply Examples

`Execute`:
```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/apply \
  -H 'Content-Type: application/json' \
  -d '{
    "From":"target-1",
    "Meta":{"Action":"Execute","Sequence":1},
    "Params":{"Action":"Execute","Command":"hello openclaw","Reference":"1","timeoutSeconds":"30"}
  }'
```

`Query`:
```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/apply \
  -H 'Content-Type: application/json' \
  -d '{
    "From":"target-1",
    "Meta":{"Action":"Query","Sequence":11},
    "Params":{"Action":"Query","Prompt":"summarize latest status","Reference":"11"}
  }'
```

`Chat`:
```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/apply \
  -H 'Content-Type: application/json' \
  -d '{
    "From":"target-1",
    "Meta":{"Action":"Chat","Sequence":2},
    "Params":{"Action":"Chat","Command":"你好","Reference":"2"}
  }'
```

`Ping`:
```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/apply \
  -H 'Content-Type: application/json' \
  -d '{
    "From":"target-1",
    "Meta":{"Action":"Ping","Sequence":12},
    "Params":{"Action":"Ping","Reference":"12"}
  }'
```

`CreateSession`:
```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/apply \
  -H 'Content-Type: application/json' \
  -d '{
    "From":"target-1",
    "Meta":{"Action":"CreateSession","Sequence":13},
    "Params":{"Action":"CreateSession","Reference":"13"}
  }'
```

`CloseSession`:
```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/apply \
  -H 'Content-Type: application/json' \
  -d '{
    "From":"target-1",
    "Meta":{"Action":"CloseSession","Sequence":14},
    "Params":{"Action":"CloseSession","Reference":"14"}
  }'
```

`ConfigureModel`:
```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/apply \
  -H 'Content-Type: application/json' \
  -d '{
    "From":"target-1",
    "Meta":{"Action":"ConfigureModel","Sequence":3},
    "Params":{"Action":"ConfigureModel","model":"kimi-coding/k2p5","Reference":"3"}
  }'
```

`ConfigureTelegram`:
```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/apply \
  -H 'Content-Type: application/json' \
  -d '{
    "From":"target-1",
    "Meta":{"Action":"ConfigureTelegram","Sequence":4},
    "Params":{
      "Action":"ConfigureTelegram",
      "dmPolicy":"open",
      "allowFrom":"*",
      "Reference":"4"
    }
  }'
```

#### Apply Response Notes

Success shape:
```json
{
  "status":"ok",
  "result":{
    "Messages":[...],
    "Output":{
      "runtime":"openclaw",
      "action":"Chat",
      "requestId":"2",
      "sessionId":"main",
      "gatewayStatus":"200 OK",
      "statusCode":200,
      "gateway":{},
      "reply":"... (Chat only)"
    }
  }
}
```

For `Chat`, runtime additionally writes reply text to:
- `result.Output.reply`


## 🏗️ Project Structure

```
.
├── common/             # Shared utilities
├── runtime/            # Runtime implementations
│   ├── openclaw/        # Openclaw runtime
│   └── testrt/          # In-memory test runtime
├── server/             # HTTP server implementation
├── utils/              # Helper utilities
├── Dockerfile          # Docker build file
├── Dockerfile.sandbox  # Docker Sandboxes template build file
├── docker_build.sh     # Build script
├── docker_build_sandbox.sh # Sandbox template build script
├── docker_run_sandbox.sh # Sandbox create/run helper
├── start-vmdocker-agent.sh # Shared bootstrap script for OCI/sandbox
├── docker_run.sh       # Run script
└── main.go            # Application entry point
```

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🔗 Related Projects

- [Hymx](https://github.com/cryptowizard0/hymx) - The main computation framework
- [Vmdocker](https://github.com/cryptowizard0/vmdocker) - Container orchestration system
- [AOS](https://github.com/cryptowizard0/aos) - Actor Oriented System
