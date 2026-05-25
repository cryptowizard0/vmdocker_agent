# Claude Runtime Developer Guide

This document describes the `claude` runtime in `vmdocker_agent` from a developer point of view. It is based on the current implementation in:

- `runtime/claudecode/claudecode.go`
- `server/api.go`
- `Dockerfile.claude`
- `bootstrap/claude.sh`
- `scripts/docker_test_claude.sh`

Use this guide when you need to build, run, integrate, or debug the Claude-backed runtime exposed through the VMM-compatible HTTP API.

## 1. What The Claude Runtime Is

The Claude runtime is a `vmdocker_agent` backend that turns VMM `spawn/apply/checkpoint/restore` requests into Claude Code CLI calls.

At a high level:

1. `vmdocker_agent` exposes HTTP endpoints under `/vmm/*`.
2. When `RUNTIME_TYPE=claude`, the server instantiates `runtime/claudecode.Runtime`.
3. `Apply` requests are translated into a Claude CLI invocation.
4. The runtime stores the Claude session ID returned by the CLI.
5. `Checkpoint` serializes that session ID and runtime config into JSON.
6. `Restore` rehydrates the runtime and resumes the same Claude session on the next `Apply`.

This runtime is intentionally simple. It does not implement an intermediate gateway like OpenClaw. The Claude CLI is the execution engine.

## 2. Runtime Architecture

### 2.1 Components

- `server/api.go`
  Accepts `/vmm/spawn`, `/vmm/apply`, `/vmm/checkpoint`, `/vmm/restore`, and `/vmm/health`.
- `runtime/runtime.go`
  Selects the backend from `RUNTIME_TYPE`.
- `runtime/claudecode/claudecode.go`
  Implements the Claude-specific runtime logic.
- `start-vmdocker-agent.sh`
  Shared entrypoint. Runs startup audit, dispatches runtime bootstrap, then launches `/app/main`.
- `bootstrap/claude.sh`
  Claude runtime bootstrap hook. It currently verifies that `claude` is available in `PATH`.
- `Dockerfile.claude`
  Builds the Claude-oriented image on top of `docker/sandbox-templates:claude-code`.

### 2.2 Request Flow

```text
Caller
  -> POST /vmm/spawn
  -> POST /vmm/apply
       -> runtime/claudecode.Apply()
       -> exec claude [--resume <session>] -p <prompt> --output-format json --dangerously-skip-permissions ...
       -> parse JSON result
       -> store session_id in runtime state
       -> return VMM result JSON
  -> POST /vmm/checkpoint
       -> serialize runtime state
  -> POST /vmm/restore
       -> restore runtime state
  -> POST /vmm/apply
       -> claude --resume <restored-session-id> ...
```

### 2.3 Image / Sandbox Model

The Claude image uses `Dockerfile.claude`, which:

- compiles `vmdocker_agent`
- starts from `docker/sandbox-templates:claude-code`
- installs basic shell tools such as `curl` and `bash`
- copies the shared entrypoint and Claude bootstrap hook
- removes passwordless `sudo` for the `agent` user
- sets `RUNTIME_TYPE=claude`

At container startup:

1. `start-vmdocker-agent.sh` runs a basic security audit.
2. The script loads `bootstrap/claude.sh`.
3. The Claude bootstrap checks that the `claude` binary exists.
4. The entrypoint `exec`s `/app/main`.

Unlike the OpenClaw runtime, no separate gateway process is started.

## 3. Lifecycle

### 3.1 Spawn

`/vmm/spawn` creates the runtime instance, but it does not create a Claude session by itself.

Important implications:

- `spawn` loads config and validates the Claude binary path.
- the first real Claude session is established lazily by the first successful `/vmm/apply`
- a checkpoint taken immediately after `spawn` can be valid even if no `sessionId` exists yet

### 3.2 Apply

`/vmm/apply` is the main execution path.

For each request, the runtime:

1. resolves the action
2. extracts the prompt text
3. loads the current config snapshot
4. resumes the previous Claude session when one exists
5. invokes the Claude CLI
6. parses the JSON result
7. updates in-memory checkpoint state
8. returns a VMM-compatible result envelope

### 3.3 Checkpoint

`/vmm/checkpoint` returns a JSON string that contains Claude runtime state.

Current format:

```json
{
  "format": "claudecode.runtime.v1",
  "sessionId": "sess-123",
  "cwd": "/runtime/workspace",
  "model": "claude-sonnet-4-5",
  "baseURL": "https://your-proxy.example.com"
}
```

Notes:

- `format` is always `claudecode.runtime.v1`
- `sessionId` may be empty if no successful apply happened yet
- `cwd`, `model`, and `baseURL` are carried so the runtime can resume consistently

### 3.4 Restore

`/vmm/restore` creates a new runtime from a previous checkpoint string.

Behavior details:

- `State` must be a non-empty JSON string
- `format` must be empty or `claudecode.runtime.v1`
- the restored `sessionId` is reused on the next `apply`
- environment configuration still has priority over checkpoint values for:
  - workspace (`cwd`)
  - model
  - base URL

That last point matters in deployment. If you restore a checkpoint in a new container and set `ANTHROPIC_BASE_URL` or `ANTHROPIC_MODEL`, the runtime uses the new environment values while still resuming the saved Claude session ID.

## 4. Configuration

### 4.1 Runtime Selection

Set:

```bash
RUNTIME_TYPE=claude
```

### 4.2 Claude-Specific Environment Variables

- `CLAUDE_CODE_BIN`
  Optional path override for the Claude CLI. Default: `claude`.
- `CLAUDE_CODE_TIMEOUT_MS`
  Request timeout in milliseconds. Default: `600000` (10 minutes).
- `CLAUDE_CODE_FLAGS`
  Extra CLI flags appended after the built-in arguments. Supports shell-like quoting.
- `ANTHROPIC_API_KEY`
  Claude API key injected into the Claude CLI environment.
- `ANTHROPIC_BASE_URL`
  Optional Anthropic-compatible base URL for a proxy or gateway.
- `ANTHROPIC_MODEL`
  Preferred model from environment.
- `CLAUDE_MODEL`
  Fallback model if `ANTHROPIC_MODEL` is unset.

### 4.3 Workspace Resolution

The Claude runtime chooses its working directory in this order:

1. `VMDOCKER_AGENT_WORKSPACE`
2. `VMDOCKER_RUNTIME_WORKSPACE`
3. `OPENCLAW_AGENT_WORKSPACE`
4. current process working directory

The resolved path is converted to an absolute path and used as `cmd.Dir` for the Claude CLI.

In practice, for Claude deployments you should set:

```bash
VMDOCKER_RUNTIME_WORKSPACE=/runtime
VMDOCKER_AGENT_WORKSPACE=/runtime/workspace
```

This is also what the repository smoke test does.

### 4.4 Model Resolution

The Claude runtime resolves the model in this order:

1. `ANTHROPIC_MODEL`
2. `CLAUDE_MODEL`
3. spawn tag keys:
   - `model`
   - `Model`
   - `modelName`
   - `ModelName`

If no model is resolved, the runtime simply omits `--model` and lets the Claude CLI use its own default.

## 5. API Contract

### 5.1 POST `/vmm/health`

Simple health probe:

```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/health
```

Expected response:

```json
{"status":"ok"}
```

### 5.2 POST `/vmm/spawn`

Creates the runtime instance.

Example:

```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/spawn \
  -H 'Content-Type: application/json' \
  -d '{
    "Pid":"claude-pid",
    "Owner":"owner-1",
    "CuAddr":"cu-1",
    "Evn":{},
    "Tags":[
      {"name":"model","value":"claude-sonnet-4-5"}
    ]
  }'
```

Expected response:

```json
{"status":"ok"}
```

Notes:

- `Tags` are converted into a `map[string]string`
- for the Claude runtime, only model-related spawn tags are consumed
- `spawn` must be called before `apply`

### 5.3 POST `/vmm/apply`

Runs one Claude request.

Example:

```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/apply \
  -H 'Content-Type: application/json' \
  -d '{
    "From":"target-1",
    "Meta":{"Action":"Chat","Sequence":1},
    "Params":{
      "Action":"Chat",
      "Command":"Explain how the current workspace is structured.",
      "Reference":"1"
    }
  }'
```

Server response shape:

```json
{
  "status": "ok",
  "result": "{\"Messages\":[...],\"Output\":...,\"Data\":\"...\",\"Error\":null}"
}
```

Important:

- `result` is a JSON string, not a nested object
- callers must parse `result` again if they need the VMM payload

### 5.4 POST `/vmm/checkpoint`

Example:

```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/checkpoint
```

Response:

```json
{
  "status":"ok",
  "state":"{\"format\":\"claudecode.runtime.v1\",\"sessionId\":\"sess-123\",\"cwd\":\"/runtime/workspace\"}"
}
```

### 5.5 POST `/vmm/restore`

Example:

```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/restore \
  -H 'Content-Type: application/json' \
  -d '{
    "Env":{},
    "Tags":[],
    "State":"{\"format\":\"claudecode.runtime.v1\",\"sessionId\":\"sess-123\",\"cwd\":\"/runtime/workspace\"}"
  }'
```

Response:

```json
{"status":"ok"}
```

## 6. Supported Actions

The Claude runtime recognizes action names using this order:

1. `Meta.Action`
2. `Params.action`
3. `Params.Action`
4. default: `Query`

Action matching is case-insensitive for these built-in names:

- `Query`
- `Execute`
- `Chat`

### 6.1 `Query`

Use `Query` for plain prompt-response interactions when you do not need a chat-shaped output payload.

Example:

```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/apply \
  -H 'Content-Type: application/json' \
  -d '{
    "From":"target-1",
    "Meta":{"Action":"Query","Sequence":11},
    "Params":{
      "Action":"Query",
      "Prompt":"Summarize the current repository layout in 5 bullets.",
      "Reference":"11"
    }
  }'
```

Behavior:

- prompt is sent to Claude
- `result.Data` is the reply text
- `result.Output` is also the reply text

### 6.2 `Execute`

Use `Execute` when the caller semantically wants an execution-style action, but note that the Claude runtime currently treats it the same as `Query` at the transport level.

Example:

```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/apply \
  -H 'Content-Type: application/json' \
  -d '{
    "From":"target-1",
    "Meta":{"Action":"Execute","Sequence":12},
    "Params":{
      "Action":"Execute",
      "Command":"Inspect the current directory and describe the top-level files.",
      "Reference":"12"
    }
  }'
```

Behavior:

- prompt is sent to Claude
- `result.Data` is the reply text
- `result.Output` is the reply text

### 6.3 `Chat`

Use `Chat` when the caller expects a chat-style output object.

Example:

```bash
curl -sS -X POST http://127.0.0.1:8080/vmm/apply \
  -H 'Content-Type: application/json' \
  -d '{
    "From":"target-1",
    "Meta":{"Action":"Chat","Sequence":13},
    "Params":{
      "Action":"Chat",
      "Command":"Reply with one sentence describing this runtime.",
      "Reference":"13"
    }
  }'
```

Behavior:

- prompt is sent to Claude
- `result.Data` is the reply text
- `result.Output` is:

```json
{
  "action": "Chat",
  "reply": "<reply text>"
}
```

This is the main behavioral difference between `Chat` and `Query` / `Execute`.

### 6.4 Unsupported Or Non-Special Actions

The Claude runtime does not implement OpenClaw-style management actions such as:

- `Ping`
- `CreateSession`
- `CloseSession`
- `ConfigureModel`
- `ConfigureTelegram`

If you send another action string, the runtime does not map it to any special Claude operation. It still tries to extract a prompt and run Claude. In other words:

- unknown actions are not rejected early
- unknown actions do not gain special behavior
- if no command-like field is present, the request fails with `claude apply failed: command is empty`

For Claude integrations, treat `Query`, `Execute`, and `Chat` as the supported action set.

## 7. Prompt Extraction Rules

The runtime extracts the prompt from the first non-empty field in this order:

1. `Params.command`
2. `Params.Command`
3. `Params.prompt`
4. `Params.Prompt`
5. `Params.input`
6. `Params.Input`
7. `Params.data`
8. `Params.Data`
9. `Meta.Data`

If every source is empty, the request fails.

Recommended practice:

- use `Params.Command` for consistency with existing VMM callers
- provide `Meta.Action` explicitly
- provide `Params.Reference` explicitly

## 8. How CLI Invocation Works

For each apply, the runtime builds a command like:

```bash
claude [--resume <session-id>] \
  -p "<prompt>" \
  --output-format json \
  --dangerously-skip-permissions \
  [--model "<model>"] \
  [extra flags from CLAUDE_CODE_FLAGS]
```

Implementation details:

- `--resume` is added only when a session ID already exists
- `--model` is added only when a model is configured
- `ANTHROPIC_API_KEY` and `ANTHROPIC_BASE_URL` are injected into the Claude CLI environment
- the process working directory is the resolved runtime workspace

The CLI output must be valid JSON matching the expected shape:

```json
{
  "type": "result",
  "subtype": "success",
  "is_error": false,
  "result": "reply text",
  "session_id": "sess-123"
}
```

If `is_error=true`, the runtime returns an error.

If the process exits non-zero, stderr is surfaced in the API error.

If the CLI returns success with an empty `result`, the runtime retries once before failing.

## 9. Response Semantics

The Claude runtime returns a VMM result with:

- one outbound message
- `Data` set to the Claude reply text
- `Output` set to either:
  - the reply text for `Query` / `Execute`
  - `{ "action": "Chat", "reply": "<text>" }` for `Chat`

Each response message includes tags:

- `Runtime=claude`
- `SessionID=<current-session-id>`
- `Reference=<request-id>`
- `Reply=<reply-text>`
- `Action=<resolved-action>` when present

Additionally, any request param whose key starts with `X-` is copied into the response message tags unchanged.

Target resolution order:

1. request `From`
2. `Params.From`
3. `Meta.FromProcess`

Request ID resolution order:

1. `Params.reference`
2. `Params.Reference`
3. `Meta.Sequence`
4. generated `time.Now().UnixNano()`

## 10. Build And Run

### 10.1 Build The Claude Image

```bash
cd /Users/webbergao/work/src/HymxWorkspace/vmdocker_agent
./docker_build_claude.sh latest
```

### 10.2 Run The Container Locally

```bash
docker run --rm -p 8080:8080 \
  -e RUNTIME_TYPE=claude \
  -e ANTHROPIC_API_KEY=your_key \
  -e ANTHROPIC_MODEL=claude-sonnet-4-5 \
  -e VMDOCKER_RUNTIME_WORKSPACE=/runtime \
  -e VMDOCKER_AGENT_WORKSPACE=/runtime/workspace \
  -e VMDOCKER_RUNTIME_HOME=/runtime/.home \
  -e HOME=/runtime/.home \
  -v "$(pwd)/tmp-runtime:/runtime" \
  chriswebber/docker-claude:latest
```

Then call:

1. `/vmm/health`
2. `/vmm/spawn`
3. `/vmm/apply`

### 10.3 Run The Repo Smoke Test

```bash
cd /Users/webbergao/work/src/HymxWorkspace/vmdocker_agent
ANTHROPIC_API_KEY=your_key ./scripts/docker_test_claude.sh
```

The smoke test verifies:

- container startup
- Claude bootstrap path
- spawn/apply success
- checkpoint generation
- restore into a fresh container
- session continuity after restore

## 11. Troubleshooting

### `find claude binary ... failed`

Cause:

- `claude` is not installed or not in `PATH`
- `CLAUDE_CODE_BIN` points to the wrong binary

Fix:

- verify the image contains the Claude CLI
- verify `CLAUDE_CODE_BIN`

### `claude apply failed: command is empty`

Cause:

- no prompt-like field was provided

Fix:

- send one of `Command`, `Prompt`, `Input`, `Data`, or `Meta.Data`

### `claude apply failed: timeout after ...`

Cause:

- Claude request exceeded `CLAUDE_CODE_TIMEOUT_MS`

Fix:

- increase `CLAUDE_CODE_TIMEOUT_MS`
- reduce task scope

### Restore succeeds but behavior changes

Cause:

- environment config overrides checkpoint config for model or base URL

Fix:

- ensure the restore container uses the same model and base URL if exact continuity is required

## 12. Developer Notes

- The Claude runtime is session-oriented, but session creation is lazy.
- The runtime currently supports a narrow action surface by design.
- OpenClaw management actions are not part of the Claude runtime contract.
- If you extend the action surface later, update this document together with:
  - `runtime/claudecode/claudecode.go`
  - `runtime/claudecode/claudecode_test.go`
  - `scripts/docker_test_claude.sh`
