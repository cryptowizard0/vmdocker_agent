# vmdocker_agent 模块化重构设计

日期：2026-05-12

## 背景

`vmdocker_agent` 当前同时承载 VMM HTTP API、runtime 选择、具体 Agent 执行、启动脚本、Docker 镜像构建和 module 打包逻辑。现有结构可以工作，但职责边界不够清晰：`runtime/` 既负责 hymx/vmdocker 协议适配，也承载 Agent 实现细节；Dockerfile 和 `docker_build_*.sh` 分散承载镜像约定；skill、role、context、home/workspace 等 harness 概念还没有统一模型。

本次重构目标是建立清晰的 `runtime / harness / build` 边界，并为后续方便添加新的 Agent 打基础。对外保持 `/vmm/*` API 和现有 `spawn/apply/checkpoint/restore` 语义兼容，允许增加 profile、manifest、env 和 module metadata 来支撑新架构。

## 目标

1. 明确划分模块职责：
   - `runtime`：负责与 hymx、vmdocker 对接，把 VMM action 转成内部 Agent 调用。
   - `harness`：负责 Agent 约束和运行环境，包括 workspace、home、skill 管理、role、context、memory 目录约定。
   - `build`：负责镜像构建、标准化 ENV/脚本、module 打包。
2. 支持方便添加新 Agent：采用 `profile + backend` 两层模型。
3. 支持预置 skill：skills 进入全局 skill 池，由 profile manifest 声明启用，不需要修改 runtime 代码。
4. 支持声明式构建：build profile manifest 成为镜像和 module 生成的事实来源。
5. 第一阶段采用 Strangler migration：迁移 `claude` 为参考实现，`openclaw`、`telegramcustomer` 先通过 legacy wrapper 兼容。

## 非目标

1. 第一阶段不重新定义 `/vmm/*` 外部协议。
2. 第一阶段不实现复杂记忆检索、向量库或远端 memory service。
3. 第一阶段不一次性迁移所有 runtime。
4. 第一阶段不删除现有 `docker_build_*.sh`，它们先作为兼容 wrapper 保留。

## 总体方案

采用 Strangler migration。新架构与旧 runtime factory 并存，先把 `claude` 迁移成新模型的参考路径。旧的 `openclaw` 和 `telegramcustomer` 通过 legacy adapter 接入统一 backend 接口，后续再逐步迁移。

核心分层如下：

```text
hymx/vmdocker
  -> /vmm/spawn, /vmm/apply, /vmm/checkpoint, /vmm/restore
  -> runtime API adapter
  -> profile resolver
  -> harness initializer
  -> backend
  -> VMM result envelope
```

`runtime` 知道 VMM 协议和 result envelope，不直接管理 skill 或 filesystem 细节。`harness` 知道 profile 资源、目录约定和初始化流程，不知道 `/vmm/*` 协议。`backend` 知道具体 Agent 如何执行，不知道 Docker/module 打包细节。

## 目标目录结构

```text
vmdocker_agent/
├── runtime/
│   ├── api/
│   │   └── VMM-facing orchestration
│   ├── backend/
│   │   ├── interface.go
│   │   ├── claude/
│   │   └── legacy/
│   └── profile/
│       └── resolver and compatibility mapping
├── harness/
│   ├── profiles/
│   │   └── claude/profile.toml
│   ├── skills/
│   │   └── <skill-name>/SKILL.md
│   ├── roles/
│   │   └── claude.md
│   └── scripts/
│       └── install/init helpers
├── build/
│   ├── profiles/
│   │   └── claude.toml
│   ├── docker/
│   └── scripts/
├── bootstrap/
│   └── <profile-or-backend>.sh
└── modulegen/
```

## Profile 与 Backend 模型

Agent 扩展拆成两层：

1. `backend`：具体执行适配，负责把内部 Agent request 转换为 Claude CLI、OpenClaw gateway 或其他 Agent 能力调用。
2. `profile`：运行配置组合，负责选择 backend，并绑定 role、skills、默认 env、workspace/home/context/memory 目录约定。

推荐的 profile manifest 形态：

```toml
name = "claude"
backend = "claude"
asset_root = "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent"
role = "roles/claude.md"

[paths]
workspace = "${VMDOCKER_AGENT_WORKSPACE}"
home = "${VMDOCKER_RUNTIME_HOME}"
context = "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/context"
memory = "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/memory"
skills = "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/skills"

[skills]
include = ["codex-basic", "hymx-runtime"]

[env]
RUNTIME_TYPE = "claude"
NODE_OPTIONS = "--use-env-proxy"
```

`VMDOCKER_AGENT_PROFILE` 是新的首选选择变量。为了兼容现有部署，如果未设置 `VMDOCKER_AGENT_PROFILE`，则从 `RUNTIME_TYPE` 映射默认 profile：

```text
claude -> claude
openclaw -> openclaw-legacy
telegramcustomer -> telegramcustomer-legacy
test -> test
```

此映射应集中在 `runtime/profile` 内，避免散落在 server、backend、bootstrap 或 build 脚本中。

## Harness 设计

第一阶段 harness 只负责目录和生命周期约定，不实现复杂 memory 检索系统。

`spawn` 阶段初始化 harness：

1. 解析 profile manifest。
2. 解析 workspace 和 home 路径，优先使用 vmdocker 提供的环境变量。
3. 创建 context 和 memory 目录。
4. 校验并安装 profile include 的 skills。
5. 暴露 role、skills、context、memory 相关路径给 backend。

Harness 路径必须服从 vmdocker 注入的 runtime workspace env。`vmdocker/vmdocker/runtimemanager/env.go` 已定义每个实例的 workspace 布局：

- `VMDOCKER_RUNTIME_WORKSPACE=<workspace>`
- `VMDOCKER_AGENT_WORKSPACE=<workspace>/workspace`
- `VMDOCKER_RUNTIME_HOME=<workspace>/.home`
- `HOME=<workspace>/.home`
- `TMPDIR=<workspace>/.tmp`
- `XDG_CONFIG_HOME=<workspace>/.xdg/config`
- `XDG_CACHE_HOME=<workspace>/.xdg/cache`
- `XDG_STATE_HOME=<workspace>/.xdg/state`

因此 harness 资源目录也统一落在 `VMDOCKER_RUNTIME_WORKSPACE` 下，而不是镜像内系统路径：

```text
${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/
  profiles/
  skills/
  roles/
  context/
  memory/
  bootstrap/
  bin/
```

推荐运行时环境变量：

```text
VMDOCKER_AGENT_PROFILE
VMDOCKER_AGENT_ASSET_ROOT=${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent
VMDOCKER_AGENT_PROFILE_DIR=${VMDOCKER_AGENT_ASSET_ROOT}/profiles
VMDOCKER_AGENT_SKILLS_DIR=${VMDOCKER_AGENT_ASSET_ROOT}/skills
VMDOCKER_AGENT_ROLE_PATH=${VMDOCKER_AGENT_ASSET_ROOT}/roles/<profile>.md
VMDOCKER_AGENT_CONTEXT_DIR=${VMDOCKER_AGENT_ASSET_ROOT}/context
VMDOCKER_AGENT_MEMORY_DIR=${VMDOCKER_AGENT_ASSET_ROOT}/memory
```

Skill 采用全局池：

```text
harness/skills/<skill-name>/SKILL.md
```

新增 skill 不需要改 runtime。启用到 profile 是 manifest-only change。build 阶段应优先校验 required skills 是否存在；运行时挂载 profile 的场景则在 spawn 阶段失败并返回清晰错误。

## Runtime 数据流

### Spawn

1. `/vmm/spawn` 仍接收现有 request。
2. runtime 从 `VMDOCKER_AGENT_PROFILE` 或 `RUNTIME_TYPE` 解析 profile。
3. 加载并校验 profile manifest。
4. 初始化 harness，生成 `HarnessContext`。
5. 根据 profile 的 `backend` 创建 backend。
6. 保存 runtime session 对象。

### Apply

1. `/vmm/apply` 仍接收现有 request。
2. runtime 把 VMM action、meta、params 转换为内部 Agent request。
3. backend 执行请求。
4. runtime 将 backend response 包装成现有 VMM result，包括 reply、tags、target、output 等字段。

### Checkpoint

新增统一 checkpoint envelope：

```json
{
  "format": "vmdocker_agent.runtime.v1",
  "profile": "claude",
  "backend": "claude",
  "harness": {
    "workspace": "...",
    "home": "...",
    "context": "...",
    "memory": "..."
  },
  "backendState": "{...}"
}
```

`backendState` 保存 backend 自己的 checkpoint，例如现有 Claude session ID 和配置。envelope 不序列化完整 manifest 文本，restore 时按当前镜像中的 profile manifest 重新加载。

### Restore

1. 如果 state 是 `vmdocker_agent.runtime.v1` envelope，先恢复 profile 和 harness，再把 `backendState` 交给 backend。
2. 如果 state 是旧 checkpoint 格式，交给对应 backend 或 legacy wrapper 兼容处理。
3. restore 后的 apply 行为与当前 API 保持一致。

## 错误处理

错误按生命周期分层：

1. profile 不存在：`spawn` 失败，错误信息列出可用 profile 或当前选择值。
2. manifest 无效：`spawn` 失败，指出文件路径和字段。
3. required skill 缺失：build 阶段优先失败；运行时 profile 挂载场景在 `spawn` 阶段失败。
4. harness 路径不可创建：`spawn` 失败。
5. backend 初始化失败：`spawn` 失败。
6. backend 执行失败：`apply` 失败，保持当前 apply error envelope 语义。
7. checkpoint/restore 格式不支持：返回明确 format 错误。

## Build 设计

`build/profiles/<name>.toml` 成为镜像构建和 module 打包的事实来源。以 Claude 为第一阶段参考：

```toml
name = "claude"
runtime_profile = "claude"
dockerfile = "build/docker/Dockerfile.claude"
image_name = "chriswebber/docker-claude"
start_command = "sh -lc 'exec \"$VMDOCKER_RUNTIME_WORKSPACE/.vmdocker-agent/bin/start-vmdocker-agent.sh\"'"

[assets]
profiles = ["claude"]
skills = ["codex-basic", "hymx-runtime"]
roles = ["claude"]
bootstrap = ["claude.sh"]

[env]
VMDOCKER_AGENT_PROFILE = "claude"
RUNTIME_TYPE = "claude"

[module_tags]
Sandbox-Agent = "shell"
Start-Command = "sh -lc 'exec \"$VMDOCKER_RUNTIME_WORKSPACE/.vmdocker-agent/bin/start-vmdocker-agent.sh\"'"
```

构建入口采用 `cmd/build` 作为事实入口，`build/scripts/build.sh` 和现有 `docker_build_*.sh` 只作为 thin wrapper。第一阶段要求：

1. 校验 build profile 和 runtime profile。
2. 校验 assets 引用存在。
3. 准备临时 build context。
4. 将 entrypoint、bootstrap、profiles、skills、roles 和需要暴露给 Agent 的辅助文件打包为 workspace materialization plan。
5. 构建 Docker image。
6. 复用或扩展 `modulegen` 输出 module artifact。

现有 `docker_build_claude.sh`、`docker_build_openclaw.sh`、`docker_build_telegramcustomer.sh` 第一阶段保留，逐步改为调用新 build 入口。

标准 runtime workspace 契约：

```text
${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/bin/start-vmdocker-agent.sh
${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/bootstrap/*.sh
${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/profiles/
${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/skills/
${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/roles/
${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/context/
${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/memory/
```

镜像可以有内部实现路径，但这些路径不是 Agent/harness 的外部契约。Agent 可见、可依赖的路径必须通过 vmdocker 注入的 env 解析到 runtime workspace 下。build 产物需要保证容器启动时在 `VMDOCKER_RUNTIME_WORKSPACE` 内完成 harness assets materialization，然后再启动 agent 服务。

## 第一阶段迁移范围

第一阶段只迁移 `claude`：

1. 新增 profile resolver。
2. 新增 harness manifest 解析和初始化。
3. 新增 backend 接口。
4. `runtime/backend/claude` 先包裹现有 `runtime/claudecode` 行为，避免第一阶段同时移动大量已验证代码；后续在测试稳定后再决定是否物理移动文件。
5. `openclaw` 和 `telegramcustomer` 暂时通过 legacy wrapper 接入。
6. 新增 Claude profile manifest 和 build manifest。
7. 保留旧 env 和脚本兼容。

## 测试策略

优先覆盖不依赖 Docker 的单元测试：

1. profile resolver：
   - `VMDOCKER_AGENT_PROFILE` 优先级高于 `RUNTIME_TYPE`。
   - 旧 `RUNTIME_TYPE` 能正确映射 legacy profile。
   - 不支持的 profile 返回清晰错误。
2. manifest parsing：
   - 必填字段校验。
   - backend/profile/skill 引用校验。
3. harness init：
   - workspace/home/context/memory 路径创建。
   - skill include 选择和安装计划正确。
   - role path 暴露正确。
4. Claude backend：
   - 保留现有 CLI 参数、model、base URL、timeout、session resume 行为。
   - checkpoint/restore 兼容现有 Claude state。
5. server API：
   - `/vmm/spawn` 兼容旧 request。
   - `/vmm/apply` result envelope 不回归。
   - `/vmm/checkpoint` 输出新 envelope。
   - `/vmm/restore` 接受新 envelope 和旧 Claude checkpoint。
6. build：
   - build manifest 校验。
   - asset copy plan 生成。
   - module tags 生成。

集成测试保持现有 smoke test 路径，并在 Claude 新 build 入口可用后补充镜像级验证。

## 成功标准

1. 旧的 Claude `/vmm/spawn`、`/vmm/apply`、`/vmm/checkpoint`、`/vmm/restore` 行为保持兼容。
2. `VMDOCKER_AGENT_PROFILE=claude` 可以完整启动 Claude profile。
3. 未设置 `VMDOCKER_AGENT_PROFILE` 时，`RUNTIME_TYPE=claude` 仍可工作。
4. Claude profile 能通过 manifest 选择 skills 和 role。
5. 新增 skill 不需要改 runtime 代码。
6. Claude build profile 能描述镜像 env、bootstrap、assets 和 module tags。
7. `openclaw`、`telegramcustomer` 旧路径不被破坏。

## 后续阶段

1. 将 `openclaw` 从 legacy wrapper 迁移成正式 backend/profile。
2. 将 `telegramcustomer` 从 legacy wrapper 迁移成正式 backend/profile。
3. 让现有 `docker_build_*.sh` 全部退化为新 build 入口 wrapper。
4. 根据实际 Agent 需求引入轻量文件 memory 或可插拔 memory service。
5. 扩展 module metadata，使 vmdocker 能更明确地识别 profile、backend 和 image contract。
