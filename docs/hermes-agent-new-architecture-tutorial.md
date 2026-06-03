# Hermes Agent 新流程教程

日期：2026-06-03

Hermes Agent 使用新架构的 profile-driven module 流程。对外 profile 名称是 `hermes`，内部 runtime 仍复用当前 `telegramcustomer` backend。

## 1. 准备配置

在仓库根目录执行：

```bash
cd /Users/webbergao/work/src/HymxWorkspace/vmdocker_agent
```

`.env` 只需要 SDK/签名配置：

```dotenv
VMDOCKER_URL=http://127.0.0.1:8080
VMDOCKER_PRIVATE_KEY=<your-private-key>
```

Hermes Dockerfile 会直接通过 Go module 下载 `github.com/xingj404-lab/agent-hub`。运行构建前需要确保当前环境有该仓库的访问权限。

如果 `agent-hub` 是私有仓库，构建镜像前设置 GitHub token。该 token 会通过 BuildKit secret 传入 Docker build，不会写入最终镜像：

```bash
export GITHUB_TOKEN=<token-with-agent-hub-access>
```

如果你使用 GitHub CLI，也可以临时传入：

```bash
GITHUB_TOKEN="$(gh auth token)" go run ./cmd/module -profile hermes
```

## 2. 校验 Build Profile

```bash
go run ./cmd/build -profile build/profiles/hermes.toml
```

Hermes build profile 位于 `build/profiles/hermes.toml`，核心字段：

```toml
name = "hermes"
runtime_profile = "hermes"
dockerfile = "Dockerfile.telegramcustomer"
context = "."
image_name = "sandytest456/docker-telegramcustomer:latest"
```

## 3. 生成 Module

```bash
go run ./cmd/module -profile hermes
```

命令会自动：

1. 读取 `build/profiles/hermes.toml`
2. 检查本地是否已有 `sandytest456/docker-telegramcustomer:latest`
3. 如果镜像缺失，使用 `Dockerfile.telegramcustomer` 构建
4. `docker save` 导出镜像
5. gzip 压缩
6. 保存 `mod/mod-<module-id>.json`

## 4. Runtime Profile

Hermes runtime profile 位于：

```text
harness/profiles/hermes/profile.toml
```

它使用：

```toml
name = "hermes"
backend = "telegramcustomer-legacy"

[env]
RUNTIME_TYPE = "telegramcustomer"
```

旧路径 `RUNTIME_TYPE=telegramcustomer` 仍映射到 `telegramcustomer-legacy` profile；新流程推荐使用 `VMDOCKER_AGENT_PROFILE=hermes`。

## 5. Smoke Test

构建镜像后可运行：

```bash
BOT_TOKEN=<telegram-bot-token> \
IMAGE_NAME=sandytest456/docker-telegramcustomer:latest \
./scripts/docker_test_telegramcustomer.sh
```

基础回归：

```bash
go test ./runtime/profile ./harness ./buildmanifest ./modulegen ./cmd/module
./scripts/test_start_vmdocker_agent.sh
```
