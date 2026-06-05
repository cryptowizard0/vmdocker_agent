# Build Profiles 配置手册

`build/profiles/<name>.toml` 是 `vmdocker_agent` 生成 portable VMDocker module 的声明式入口。它描述三件事：

1. 用哪个 Dockerfile 和 image name 构建或复用镜像。
2. 运行时应该选择哪个 harness profile。
3. 生成 module 时需要携带哪些 image/runtime metadata 和容器环境变量。

生成入口：

```bash
go run ./cmd/module -profile claude
go run ./cmd/module -profile build/profiles/claude.toml
```

只校验并打印 build profile：

```bash
go run ./cmd/build -profile build/profiles/claude.toml
```

## 最小示例

```toml
name = "claude"
runtime_profile = "harness/profiles/claude/profile.toml"
dockerfile = "Dockerfile.claude"
image_name = "chriswebber/docker-claude"

[assets]
bootstrap = ["claude.sh"]

[env]
RUNTIME_TYPE = "claude"
```

默认情况下不用写：

- `context = "."`
- `start_command = "/usr/local/bin/start-vmdocker-agent-workspace.sh"`
- `[module_tags] Sandbox-Agent = "shell"`
- `[module_tags] Start-Command = ...`
- `[env] VMDOCKER_AGENT_PROFILE = ...`

## 顶层字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `name` | 是 | build profile 名称，用于日志和人工识别。通常与文件名一致。 |
| `runtime_profile` | 是 | harness profile 文件路径，必须是相对路径并以 `profile.toml` 结尾，例如 `harness/profiles/claude/profile.toml`。 |
| `dockerfile` | 是 | Dockerfile 路径，相对当前执行目录解析。 |
| `image_name` | 是 | 最终镜像名。生成 module 前会先检查本地是否已有该镜像；没有则构建。 |
| `context` | 否 | Docker build context，默认 `.`。 |
| `start_command` | 否 | 写入 module tag `Start-Command` 的容器启动命令，默认 `/usr/local/bin/start-vmdocker-agent-workspace.sh`。 |

`runtime_profile` 不再写成裸名字，例如 `claude`。它应直接指向文件：

```toml
runtime_profile = "harness/profiles/tg-customer/profile.toml"
```

modulegen 会从父目录名派生：

```text
Container-Env-VMDOCKER_AGENT_PROFILE=tg-customer
```

## start_command

默认值：

```toml
start_command = "/usr/local/bin/start-vmdocker-agent-workspace.sh"
```

通常不要在 build profile 中写这个字段。默认 wrapper 会：

1. 解析 asset root：
   - 优先 `VMDOCKER_AGENT_ASSET_ROOT`
   - 否则使用 `$VMDOCKER_RUNTIME_WORKSPACE/.vmdocker-agent`
2. 从 image bundle 复制 `/opt/vmdocker-agent-bundle` 到 asset root。
3. 执行 `$asset_root/bin/start-vmdocker-agent.sh`。

只有当 image 使用完全不同的 workspace 启动契约时，才需要自定义 `start_command`。自定义值必须显式包含 `VMDOCKER_RUNTIME_WORKSPACE`，避免回到固定系统路径。

不要在 `[module_tags]` 里手写 `Start-Command`；它由顶层 `start_command` 自动派生。

## `[env]`

`[env]` 会生成 module tags：

```text
Container-Env-<KEY>=<VALUE>
```

示例：

```toml
[env]
RUNTIME_TYPE = "telegramcustomer"
```

生成：

```text
Container-Env-RUNTIME_TYPE=telegramcustomer
```

不要配置：

```toml
[env]
VMDOCKER_AGENT_PROFILE = "claude"
```

`VMDOCKER_AGENT_PROFILE` 由 `runtime_profile` 自动派生。重复配置会被拒绝。

## `[build_contexts]`

`[build_contexts]` 对应 Docker BuildKit 的 named build contexts。

```toml
[build_contexts]
extra_src = "${EXTRA_CONTEXT_PATH}"
```

生成 docker build 参数：

```text
--build-context extra_src=/expanded/path
```

规则：

- key 不能为空。
- value 会展开环境变量。
- 如果引用的环境变量不存在或为空，构建会失败。
- 展开后路径必须非空，并会转成绝对路径。

## `[module_tags]`

`[module_tags]` 是高级 metadata 入口。它适合放 image/runtime 描述信息，不适合放启动命令或容器 env。

可写示例：

```toml
[module_tags]
Openclaw-Version = "2026.3.1-beta.1"
Sandbox-Network = "default"
```

不要写：

```toml
[module_tags]
Start-Command = "..."
```

`Start-Command` 由 `start_command` 派生。默认 `Sandbox-Agent=shell` 也会自动生成，不需要显式写。

## `[assets]`

`[assets]` 当前只保留 build profile 需要显式声明的 bootstrap hook：

```toml
[assets]
bootstrap = ["claude.sh"]
```

`profiles`、`roles`、`skills` 不再写在 build profile 中，因为它们已经由 `runtime_profile` 指向的 harness profile 决定：

- profile 文件：来自顶层 `runtime_profile`
- role 文件：来自 harness profile 的 `role`
- skills：来自 harness profile 的 `[skills].include`

重要限制：当前代码不会根据 `[assets]` 自动复制文件。实际 bundle 内容仍由 Dockerfile 的 `COPY` 指令决定。`bootstrap` 只是声明这个 image 需要哪个 bootstrap hook，仍需要 Dockerfile 把它复制进 bundle。

例如 Telegram/TG Customer image 需要确保 Dockerfile 把对应文件复制进：

```text
/opt/vmdocker-agent-bundle/profiles/<profile>
/opt/vmdocker-agent-bundle/roles/<role>.md
/opt/vmdocker-agent-bundle/skills/
/opt/vmdocker-agent-bundle/bootstrap/<runtime>.sh
```

## 隐藏/派生 module tags

modulegen 会自动写入这些 tags：

| Tag | 来源 | 说明 |
| --- | --- | --- |
| `Start-Command` | `start_command` 或默认 wrapper | VMDocker 运行容器时执行的命令。 |
| `Sandbox-Agent` | 默认 `shell`，或 `[module_tags]` 覆盖 | Docker Sandbox agent 类型。当前 vmdocker_agent image 推荐使用 `shell`。 |
| `Image-Name` | `image_name` | 镜像名。 |
| `Image-ID` | 本地镜像 inspect 结果 | 镜像 digest/id。 |
| `Image-Source` | 固定 `module-data` | 表示 module payload 携带镜像归档。 |
| `Image-Archive-Format` | 固定 `docker-save+gzip` | 镜像归档格式。 |
| `Container-Env-*` | `[env]` | spawn 容器时注入的环境变量。 |
| `Container-Env-VMDOCKER_AGENT_PROFILE` | `runtime_profile` 父目录名 | runtime harness profile selector。 |

## 特殊构建环境

Telegram/Hermes image 构建可能需要访问私有 Go module。构建时可提供：

```bash
export GITHUB_TOKEN=...
# 或
export GH_TOKEN=...
```

如果两者都没有设置，构建命令会尝试读取 `gh auth token` 并临时作为 BuildKit secret 传入 Docker。Dockerfile 通过 BuildKit secret 使用 token，不会把 token bake 到最终 image。

## 环境变量职责边界

| 环境变量 | 推荐来源 | 说明 |
| --- | --- | --- |
| `VMDOCKER_AGENT_APP_ROOT` | Dockerfile `ENV` | 镜像内 agent server 路径，当前为 `/app`。脚本也有 `/app` 默认值，Dockerfile 保留它是为了直跑 image 时更清晰。 |
| `RUNTIME_TYPE` | build profile `[env]`，Dockerfile 可保留同值默认 | entrypoint 选择 bootstrap hook 的兼容 selector，例如 `claude` 或 `telegramcustomer`。module 运行时由 `Container-Env-RUNTIME_TYPE` 注入；Dockerfile 默认只服务于直接 `docker run`。 |
| `VMDOCKER_AGENT_PROFILE` | `runtime_profile` 自动派生 | harness profile selector，例如 `hermes`、`tg-customer`。不要写在 Dockerfile，也不要写在 `[env]`；`modulegen` 会生成 `Container-Env-VMDOCKER_AGENT_PROFILE`。 |

## 配置示例

### Claude

```toml
name = "claude"
runtime_profile = "harness/profiles/claude/profile.toml"
dockerfile = "Dockerfile.claude"
image_name = "chriswebber/docker-claude"

[assets]
bootstrap = ["claude.sh"]

[env]
RUNTIME_TYPE = "claude"
```

### Hermes

```toml
name = "hermes"
runtime_profile = "harness/profiles/hermes/profile.toml"
dockerfile = "Dockerfile.telegramcustomer"
image_name = "sandytest456/docker-telegramcustomer:latest"

[assets]
bootstrap = ["telegramcustomer.sh"]

[env]
RUNTIME_TYPE = "telegramcustomer"
```

### TG Customer

```toml
name = "tg-customer"
runtime_profile = "harness/profiles/tg-customer/profile.toml"
dockerfile = "Dockerfile.telegramcustomer"
image_name = "chriewebber/vmdocker-tg-customer:latest"

[assets]
bootstrap = ["telegramcustomer.sh"]

[env]
RUNTIME_TYPE = "telegramcustomer"
```

### 带额外 env

```toml
[env]
RUNTIME_TYPE = "claude"
ANTHROPIC_BASE_URL = "https://api.anthropic.com"
ANTHROPIC_MODEL = "claude-sonnet-4-5"
```

### 带额外 build context

```toml
[build_contexts]
extra_src = "${EXTRA_CONTEXT_PATH}"
```

运行前：

```bash
export EXTRA_CONTEXT_PATH=/path/to/extra/source
go run ./cmd/module -profile build/profiles/hermes.toml
```

### 带自定义 module tag

```toml
[module_tags]
Openclaw-Version = "2026.3.1-beta.1"
```

## 常见错误

- `runtime_profile = "claude"`：错误。应写 `harness/profiles/claude/profile.toml`。
- `[env] VMDOCKER_AGENT_PROFILE = "claude"`：错误。该值自动派生。
- `[module_tags] Start-Command = "..."`：错误。该 tag 自动派生。
- 忘记 Dockerfile bundle COPY：`[assets]` 不会自动复制文件，启动时会找不到 workspace entrypoint、profile、role、skills 或 bootstrap。
- 自定义 `start_command = "/usr/local/bin/start-vmdocker-agent.sh"`：错误。会绕过 workspace asset contract。
