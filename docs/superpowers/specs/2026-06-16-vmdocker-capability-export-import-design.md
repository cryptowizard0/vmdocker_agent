# vmdocker Agent Profile / Module 构建与 Export / Import 架构设计

- 日期：2026-06-16
- 状态：已评审（v3，profile 驱动），待实现
- 涉及仓库：
  - `vmdocker`：host 侧编排、docker 生命周期、**profile→Dockerfile→build→module 全套构建逻辑（收拢于此）**、离线构建 CLI、运行时 Export/Import、目录视图
  - `vmdocker_agent`：容器内运行时服务，**对本功能无感知**（仅作为被构建进镜像的程序）

## 1. 一句话说明

围绕一份**声明式 Profile** 标准化地生成 Dockerfile、构建镜像，并打包成一个**统一 Module**（内含镜像 + profile，Export 时再加 public.zip）。

- **构建 module**（离线工具）：profile → 标准化 Dockerfile → 构建镜像 → 打包 `image + profile`。
- **Export**（运行时）：对一个运行中的 agent，导出 public→zip + profile → 重新生成 Dockerfile → 构建镜像 → 打包 `image + profile + public.zip`。
- **Import**（运行时）：取 module 内的 `public.zip + profile`，**解包覆盖**到另一个已运行 agent 的 workspace，实现能力复刻。

兼容多种 agent runtime（openclaw、hermes、claude code 等），通过 profile 的 `base` 字段选择。

## 2. 设计目标

### 2.1 要解决的问题

- 让 agent 镜像的构建**标准化、可声明**：用户只描述意图（基础镜像、bin、工具、public、startup、自定义 RUN），系统生成统一、加固的 Dockerfile。
- 让一个 agent 的**能力可被复刻**：把 public 目录（如 `skills/`、`persona/`）+ 构建配方打包，导入到另一个 agent。
- 让构建产物成为**自包含、可移植的 module**：携带镜像（可直接 run 出全新 agent）+ profile（可重建）+ public.zip（可覆盖导入）。

### 2.2 成功标准

1. 给定一份 profile，离线工具生成标准化 Dockerfile、构建镜像、产出含 `image + profile` 的 module。
2. 标准化 Dockerfile 始终注入约定加固：固定用户 `hymx`、仅 `HOME=/home/hymx` 可访问、profile 被 copy 进镜像。
3. 向运行中 agent 发 `Action=Export`，得到含 `image + profile + public.zip` 的 module。
4. 向另一个运行中 agent 发 `Action=Import`（module 经 `meta.Data` 传入），其 workspace 出现与来源相同的 public 内容。
5. Export 只导出 profile.public 声明的目录；runtime 状态、凭据、HOME、用户数据一律不导出。
6. `vmdocker_agent` 无任何改动。

### 2.3 非目标

- 不做 Arweave 实际上链；本期只生成本地 module 文件。
- 不做 public 之外的细粒度权限或内容加密。
- profile 的**编辑/上传界面**（前端/控制台）不在本 spec；本 spec 只定义 profile schema 与消费它的构建/导出/导入流程。

## 3. 核心概念

| 概念 | 定义 |
|---|---|
| **Profile** | 一份声明式 JSON，描述如何标准化构建一个 agent 镜像（§5）。是 module 的一等成员。 |
| **标准化 Dockerfile** | 由 profile 确定性生成的 Dockerfile，叠加用户不可见的约定加固（§6）。 |
| **Module** | 统一载体：一个签名 BundleItem，其 `data` 是一个容器 tar，成员为 `image.tar.gz` + `profile.toml`（+ Export 时 `public.zip`）（§7）。 |
| **public** | 由 `[vmdocker].public` **目录**清单定义的可导出内容；运行时另以软链接视图呈现（§11）。 |

## 4. 关键事实与依据

| 事实 | 代码位置 | 含义 |
|---|---|---|
| host 已持有 docker 构建/保存能力 | `vmdocker_agent/modulegen/modulegen.go`（`dockerBuild`/`exportImageArchive`） | profile→build→save 逻辑可直接迁入 host vmdocker |
| host 已管理 docker 生命周期 | `vmdocker/vmdocker/runtimemanager/docker.go`（`DockerManager`） | 运行时 Export 在 host 侧 build 镜像有原生支撑 |
| sandbox 工作区 bind-mount 进容器，host 可直读写 | `runtimemanager/docker.go:303-304` | Export/Import 可在 host 侧直接读写 public/profile，agent 无感知 |
| 两仓互不 import | `vmdocker/go.mod`、`vmdocker_agent/go.mod` | 共享构建逻辑需明确归属——**决策：收拢于 host vmdocker**（§11） |
| 现有 Dockerfile 模板与加固 | `Dockerfile.openclaw` / `Dockerfile.claude` | 标准化 Dockerfile 以其为蓝本参数化（§6） |
| `Vm` 接口固定为 Apply/Checkpoint/Restore/Close | `hymx/vmm/schema/schema.go:33` | 运行时 Export/Import 只能挂在 `Apply` + `Action`（§10） |

## 5. Profile 规范

profile 为 **TOML** 文件（`profile.toml`），用配置段区分「Dockerfile 构建配置」与「vmdocker 配置」。

### 5.1 用户可见配置

`[dockerfile]` 段的 key **直接采用 Dockerfile 指令名**（即所写即所生成），便于理解：

```toml
[dockerfile]
FROM       = "openclaw"                     # 基础镜像别名：openclaw | hermes | claude（解析见 §5.3）
bin        = "bin"                          # 便捷键：可执行程序目录（整目录 COPY + chmod +x，标准化到 /usr/local/bin）
tools      = ["curl", "ripgrep", "jq"]      # 便捷键：要安装的工具（展开为跨发行版 RUN 安装）
RUN        = ["pip install --no-cache-dir foo"]  # 自定义 RUN（值不含 RUN 前缀，生成器逐条加 RUN）
ENTRYPOINT = "startup.sh"                   # 上传的 ENTRYPOINT 脚本（包内相对路径）

[vmdocker]
public = ["skills", "persona"]              # 可导出目录清单（导出白名单）。默认只支持目录
```

- **两类 key**：
  - **指令键（大写，= 字面 Dockerfile 指令）**：`FROM`、`RUN`、`ENTRYPOINT`，值即该指令的参数（不含指令前缀）。
  - **便捷键（小写，vmdocker 展开为指令）**：`bin`（→ `COPY` + `chmod +x`）、`tools`（→ 跨发行版安装 `RUN`），因含标准化/加固处理而单列。
- **`bin` 是目录**：放可执行程序的标准化目录，整目录 `COPY` 进镜像、`chmod +x`、标准化到 `/usr/local/bin`；`ENTRYPOINT` 脚本启动其中的程序。
- **`public` 只放目录**：每项是**相对 HOME 的目录**路径；导出时按目录结构压成 `public.zip`，导入时**直接解压到 `/home/hymx`**，还原同样的目录结构。默认只支持目录——若要携带单文件，置于某个 public 目录内。
- **分段语义**：`[dockerfile]` 段只喂给 Dockerfile 生成器（§6）；`[vmdocker]` 段只喂给运行时 Export/Import 与目录视图（§9/§10/§11）。两段互不串用。

### 5.2 约定注入（用户不可见，构建器强制写入/校验）

```toml
[convention]            # 不在用户编写的文件里，由构建器强制注入并校验
user   = "hymx"         # 固定运行用户
home   = "/home/hymx"   # 唯一可访问目录
harden = ["去除 sudo/docker 组", "rm /etc/sudoers.d/*", "禁止 passwordless sudo"]
```

- **固定用户 `hymx`**：标准化 Dockerfile 以 `USER hymx`、`WORKDIR /home/hymx` 结束（替换现有模板的 `agent`/`/workspace`）。
- **仅 HOME 可访问**：工作区、state、tmp、xdg 全部归置到 `/home/hymx` 下；配合 host 侧 `ReadonlyRootfs` + bind-mount 单目录。
- **profile copy 进镜像**：`COPY profile.toml /home/hymx/profile.toml`（HOME 根目录，不嵌套）。host 侧 spawn 时把它种入工作区（bind-mount 会遮蔽镜像内 HOME），使运行时与 Export 都能读到构建配方。

### 5.3 FROM 解析

`FROM` 取**别名**（非任意镜像），映射到一组基础镜像 + runtime 装配（以现有 Dockerfile 为蓝本）：

| FROM | 基础镜像 / 装配 | RUNTIME_TYPE |
|---|---|---|
| `openclaw` | `ghcr.io/openclaw/openclaw` + `docker/sandbox-templates:shell` | `openclaw` |
| `hermes` | hermes 基础镜像（待补） | `hermes` |
| `claude` | `docker/sandbox-templates:claude-code` | `claude` |

## 6. 标准化 Dockerfile 生成

仅消费 `[dockerfile]` 段，确定性渲染成多阶段 Dockerfile（以 `Dockerfile.openclaw` 为参数化蓝本）。指令键原样映射到 Dockerfile 指令；便捷键 `bin`/`tools` 由生成器展开：

```dockerfile
# 1) base 阶段：[dockerfile].FROM 别名 → 实际基础镜像（§5.3）
FROM {{.ResolvedFROM}}
USER root
WORKDIR /app

# 2) bin 目录整目录 COPY + chmod；启动脚本；profile
COPY {{.Bin}}/ /usr/local/bin/
RUN chmod +x /usr/local/bin/*
COPY {{.ENTRYPOINT}} /usr/local/bin/start-vmdocker-agent.sh
COPY profile.toml /home/hymx/profile.toml

# 3) tools 便捷键 → 跨发行版安装
RUN install {{.Tools}}

# 4) 约定加固（不可见）：建 hymx 用户、去 sudo/docker 组、清 sudoers
RUN useradd hymx ...; gpasswd -d hymx sudo || true; rm -f /etc/sudoers.d/*

# 5) [dockerfile].RUN 逐条加 RUN 前缀（值本身不含前缀）
{{range .RUN}}RUN {{.}}
{{end}}

# 6) 收尾：权限、属主、约定环境
RUN chmod +x /usr/local/bin/start-vmdocker-agent.sh; chown -R hymx:hymx /home/hymx /app
ENV HOME=/home/hymx
ENV RUNTIME_TYPE={{.RuntimeType}}
USER hymx
WORKDIR /home/hymx
ENTRYPOINT ["/usr/local/bin/start-vmdocker-agent.sh"]
```

要点：
- **指令键即所写即所生成**：`RUN`/`ENTRYPOINT`/`FROM` 与 Dockerfile 同名；`RUN` 值不含 `RUN ` 前缀，生成器逐条补。
- **`bin` 为目录**：整目录 COPY 到 `/usr/local/bin/` 并 `chmod +x`；`ENTRYPOINT` 脚本启动 bin 目录内程序。
- 加固段（4、6）由构建器无条件注入，profile 不能关闭。
- 用户 `RUN`（段 5）在加固之后、收尾之前插入，避免重新打开 sudo 等。
- 构建器校验 `ENTRYPOINT` 脚本可执行与基本安全（不强行覆盖现有 entrypoint 契约）。

## 7. Module 文件格式

一个签名 `goarSchema.BundleItem`，落盘 `mod-<itemId>.json`（构建）/ 经 `Result.Data` 回传（Export）。

### 7.1 多负载：data = 容器 tar

BundleItem 只有一个 `data` 字段，因此 `data = gzip(tar)`，成员名固定：

```mermaid
flowchart TB
    BI["BundleItem (签名)"]
    Tags["tags[]<br/>Module-Format / Module-Members / 各成员 sha256"]
    Data["data = gzip(container tar)"]
    M1["image.tar.gz<br/>docker save 镜像"]
    M2["profile.toml<br/>构建配方"]
    M3["public.zip<br/>仅 Export 流程"]
    Mani["module.manifest.json<br/>成员清单 + sha256"]

    BI --> Tags
    BI --> Data
    Data --> Mani
    Data --> M1
    Data --> M2
    Data --> M3
```

### 7.2 tags

经 `hymxSchema.Module` + `hymxUtils.ModuleToTags`：

| Tag | 值 |
|---|---|
| `Data-Protocol` | `hymx` |
| `Variant` | `v0.1.0` |
| `Type` | `Module` |
| `Module-Format` | `hymx.vmdocker.module.v0.0.1` |
| `Module-Members` | `image,profile`（构建）/ `image,profile,public`（Export） |
| `Image-Name` / `Image-ID` | docker 镜像名与 ID |
| `Module-Manifest-SHA256` | `module.manifest.json` 的 sha256 |
| `Capability-Public` | profile.public 路径，逗号分隔，便于预览 |
| `Created-At` | RFC3339 |

### 7.3 module.manifest.json（容器 tar 内第一个条目）！！删了这个 json

```json
{
  "format": "hymx.vmdocker.module.v0.0.1",
  "created_at": "2026-06-16T12:00:00Z",
  "members": [
    { "name": "image",   "path": "image.tar.gz",  "sha256": "...", "size": 0 },
    { "name": "profile", "path": "profile.toml",   "sha256": "...", "size": 0 },
    { "name": "public",  "path": "public.zip",     "sha256": "...", "size": 0 }
  ],
  "public": ["skills", "persona"]
}
```

## 8. 构建 module 流程（离线工具）

入口：host vmdocker 的离线 CLI（`vmdocker/cmd/module`，由现 `vmdocker_agent/cmd/module` 迁移而来）。

```mermaid
sequenceDiagram
    participant U as Builder
    participant CLI as vmdocker cmd/module
    participant MB as modulebuild 包
    participant D as docker

    U->>CLI: 提供 profile.toml
    CLI->>MB: GenerateDockerfile(profile)
    MB-->>CLI: 标准化 Dockerfile
    CLI->>D: docker build
    CLI->>D: docker save | gzip → image.tar.gz
    CLI->>MB: PackModule(image.tar.gz, profile)
    MB-->>CLI: BundleItem(data=tar{image,profile})
    CLI->>CLI: SaveModule → mod-<id>.json
```

1. 读 profile → `GenerateDockerfile`。
2. `docker build`（复用现有 `dockerBuild`）。
3. `docker save | gzip` → `image.tar.gz`（复用现有 `exportImageArchive`）。
4. 组装容器 tar：`module.manifest.json` + `image.tar.gz` + `profile.toml`（无 public）。
5. `SaveModule` 签名 → `mod-<id>.json`。

## 9. Export 流程（运行时，host 侧）

触发：对运行中 agent 发 `Apply(Action=Export)`，由 `vmdocker.apply()` 短路、host 侧处理（§10）。

```mermaid
sequenceDiagram
    participant H as hymx
    participant V as vmdocker.apply()
    participant FS as bind-mount 工作区
    participant MB as modulebuild
    participant D as docker

    H->>V: Apply(Action=Export)
    V->>FS: 读 profile.toml（HOME 根 = bind-mount 工作区根）
    V->>FS: 按 [vmdocker].public 目录解引用 + 越界校验 → public.zip
    V->>MB: GenerateDockerfile(profile)
    V->>D: docker build + docker save → image.tar.gz
    V->>MB: PackModule(image, profile, public.zip)
    MB-->>V: BundleItem
    V-->>H: Result.Data = base64(mod bytes); Result.Output = 摘要
```

1. 读取该实例 HOME 根的 `profile.toml`（构建时 copy 进镜像、spawn 时种入工作区）。
2. 按 `[vmdocker].public` 收集**目录**（默认只支持目录），**解引用软链接 + 越界校验**（§12），按目录结构打成 `public.zip`（zip 内路径相对 HOME）。
3. `GenerateDockerfile(profile)` → `docker build` → `docker save` → `image.tar.gz`（复用 §8 同一套 `modulebuild`）。
4. 组装容器 tar：`manifest` + `image.tar.gz` + `profile.toml` + `public.zip`，`Module-Members = image,profile,public`。
5. 签名 → 经 `Result.Data` 回传。

> Export 重建镜像，确保导出的镜像嵌入最新 public 内容；语义为“调用时刻快照”，与现有 `Checkpoint()` host 侧归档同源。

## 10. Import 流程（运行时，host 侧）

触发：对目标 agent 发 `Apply(Action=Import)`，module 字节经 `meta.Data`（base64）传入。**只用 module 里的 `public.zip + profile`，不使用其中镜像**（复刻 = 覆盖到已有 agent）。

```mermaid
sequenceDiagram
    participant H as hymx
    participant V as vmdocker.apply()
    participant MB as modulebuild
    participant FS as 目标 bind-mount 工作区

    H->>V: Apply(Action=Import, meta.Data=base64(module), Params[On-Conflict])
    V->>MB: 解析 BundleItem → 校验 Module-Format/manifest
    MB-->>V: profile.toml + public.zip
    V->>FS: 解 public.zip 到 /home/hymx/.import-<ts>/，逐文件 sha256 校验
    V->>FS: 按 On-Conflict 原子落位到 /home/hymx
    V-->>H: Result.Output = ImportResult
```

1. `BundleItem` 反序列化 → `TagsToModule` 断言 `Module-Format == hymx.vmdocker.module.v0.0.1`（否则 `FORMAT_MISMATCH`）。
2. 解容器 tar，校验 `module.manifest.json` sha256（`MANIFEST_MISMATCH`）；取 `profile.toml` 与 `public.zip`（缺 public → `NO_PUBLIC`）。
3. `len(public.zip) ≤ MaxBytes`（默认 64 MiB，`VMDOCKER_CAPABILITY_MAX_BYTES` 可覆盖，否则 `TOO_LARGE`）。
4. 解到临时目录 `/home/hymx/.import-<ts>/`：路径净化（拒绝绝对路径、`..`、zip 内 symlink），目标确认在 HOME（`/home/hymx`）内（`PATH_ESCAPE`）；逐文件 sha256 比对 manifest。
5. 冲突策略 `meta.Params["On-Conflict"]`：`skip`（默认）/ `overwrite` / `fail`。
6. 全量校验通过 → 原子 `rename` 落位到 `/home/hymx`（按 public 目录结构还原）。可选：用导入的 `profile.toml` 更新目标 HOME 根的 `profile.toml`（便于目标下次 Export 携带新 public 定义）。
7. **不建软链接**；`public/` 视图由 vmdocker 下次 spawn 依 profile 重建（§11 视图）。返回 `ImportResult{imported, skipped, public, profileUpdated}`。

## 11. 目录视图（运行时，profile 驱动）

`vmdocker` 在 HOME 根（`/home/hymx`，即 bind-mount 工作区根）下维护 `public/`、`private/` 两个**软链接视图**，仅供人/agent 检视：

```text
/home/hymx/                    HOME 根 = bind-mount 工作区根
├── profile.toml               构建配方（HOME 根）
├── skills/                    真实目录（profile.public 项）
├── persona/                   真实目录（profile.public 项）
├── public/                    [vmdocker].public 驱动的相对软链接（导出白名单可视化，只链目录）
│   ├── skills  -> ../skills
│   └── persona -> ../persona
├── private/                   best-effort 检视视图，按 runtime 实际目录建链
│   ├── runtime-state -> ../.openclaw   # openclaw 示例；claude→.claude、hermes 另有
│   ├── home -> ../.home
│   └── xdg  -> ../.xdg
└── .openclaw/|.claude/... .home .tmp .xdg
```

- **public 视图由 `[vmdocker].public` 生成**，**只链目录**（与 §5.1「public 只放目录」一致）。
- **白名单 ≠ 黑名单**：导出边界是 `[vmdocker].public`；其余一切默认私有、永不导出——与是哪种 runtime、是否出现在 `private/` 无关。新增 runtime 的状态目录即便未登记进 private 视图也不会泄露。
- `private/` 仅 best-effort 可视化，按 `RUNTIME_TYPE` 解析 runtime 状态目录，仅对存在目标建链。
- 文件：`vmdocker/vmdocker/runtimemanager/env.go` 新增 `ensureWorkspaceViews(home, profile, runtimeType)`（幂等；遇同名真实文件只 warn 不覆盖；目标不存在则跳过）。

## 12. 触发与分发

`vmdocker.apply()` 现无条件透传 `/vmm/apply`（`vmdocker.go:641`）；改为先按 `meta.Action` 短路：

```mermaid
flowchart TB
    A["vmdocker.apply(meta)"]
    S{"meta.Action"}
    E["host 侧 Export（§9）"]
    I["host 侧 Import（§10）"]
    P["透传 /vmm/apply（现状）"]
    A --> S
    S -->|Export| E
    S -->|Import| I
    S -->|其它| P
```

- `ActionExport="Export"`、`ActionImport="Import"`（大小写不敏感）。命中即 host 侧处理，**永不发往 agent**。
- 输入/输出：Export 无输入、`Result.Data` 出；Import `meta.Data` 入、`Result.Output` 出；错误经 `Result.Error`（错误码见 §10/§13）。

## 13. 代码归属与重构（收拢到 host vmdocker）

| 现状（vmdocker_agent） | 目标（vmdocker） |
|---|---|
| `modulegen/`（docker build/save、pull、tags） | 迁入 `vmdocker/vmdocker/modulebuild/`，扩展 profile→Dockerfile 生成 + 多负载打包 |
| `cmd/module`（离线 CLI） | 迁入 `vmdocker/cmd/module`，消费 `modulebuild` |
| —（无） | 新增 `vmdocker/vmdocker/capability/`：public.zip 打包/解包、Import 落位 |
| —（无） | `vmdocker.apply()` 加 Export/Import Action 分发 |
| —（无） | `runtimemanager/env.go` 加 profile 驱动的目录视图 |

`vmdocker_agent` 侧：**无改动**；它仍只是被构建进镜像、提供 `/vmm/*` 运行时服务的程序。`vmdocker_agent/modulegen` 与 `cmd/module` 在迁移完成后废弃。

> 错误码：`FORMAT_MISMATCH` / `TOO_LARGE` / `MANIFEST_MISMATCH` / `PATH_ESCAPE` / `NO_PUBLIC` / `CONFLICT`，统一经 `Result.Error` 回传。

## 14. 安全与边界

- **构建加固不可关闭**：固定 `hymx` 用户、去 sudo/docker 组、清 sudoers、仅 HOME 可访问；自定义 RUN 在加固之后插入。
- **白名单边界**：导出只读 `profile.public`，其余默认私有、永不导出（§11）。
- **路径穿越**：public 收集与 zip 解包都过 `resolveWithinWorkspace`；拒绝绝对路径软链接、`../` 越界、归档内 symlink 条目。
- **大小限制**：Import `MaxBytes`（默认 64 MiB，可覆盖），防 zip-bomb。
- **原子性**：Import 先写临时目录、全量 sha256 校验通过后再 rename；任一步失败整体回滚。
- **导出签名**：临时密钥自签（沙箱/host 无需预置密钥；配置 `VMDOCKER_MODULE_SIGNER_KEY` 则用之）；导入端不强校验签名，只校验 `Module-Format` + manifest sha256。
- **镜像可信**：Import 不执行 module 内镜像（只取 public/profile），规避导入未知镜像的执行风险；用 module 镜像直接 run 出新 agent 走现有 module-spawn 信任链。

## 15. 测试

- `vmdocker/vmdocker/modulebuild/dockerfile_test.go`：profile→Dockerfile 渲染（各 `FROM` 别名、`bin` 目录 COPY+chmod、`tools` 展开、`RUN` 逐条补前缀、`ENTRYPOINT`、加固段强制注入、profile copy）。
- `vmdocker/vmdocker/modulebuild/module_test.go`：容器 tar 成员与 manifest sha256；构建态 `image,profile`、导出态 `image,profile,public`；tags 正确。
- `vmdocker/vmdocker/capability/capability_test.go`：public.zip 收集只含 `profile.public`；越界软链接被拒；private 不进包；Import skip/overwrite/fail；`TOO_LARGE`/`FORMAT_MISMATCH`/`MANIFEST_MISMATCH`/`PATH_ESCAPE`/`NO_PUBLIC`；round-trip 字节一致。
- `vmdocker/vmdocker/runtimemanager/env_test.go`：profile 驱动 public 视图；private best-effort；幂等；不覆盖真实文件；越界目标被拒。
- `vmdocker/vmdocker/vmdocker_test.go`：`Apply(Action=Export)` 产合法 module 于 `Result.Data` 且不触达 `/vmm/apply`；`Apply(Action=Import)` 正确覆盖；其它 Action 仍透传（回归）。
- `vmdocker/cmd/module`：端到端离线构建产物可被 spawn。

## 16. 实施顺序

1. host：`modulebuild` 包——迁移现有 `modulegen` + 新增 `GenerateDockerfile(profile)` + 多负载 `PackModule` + 单测。
2. host：`cmd/module` 迁移为消费 `modulebuild` 的离线 CLI（构建 module 流程）+ 端到端。
3. host：`capability` 包——public.zip 打包/解包/Import 落位 + 单测（含 round-trip）。
4. host：`runtimemanager/env.go` profile 驱动目录视图 + 单测。
5. host：`vmdocker.apply()` Export/Import Action 分发（§12）+ 测试。
6. 端到端：构建 module → spawn 出 agent A → Export → 对 agent B Import → 复刻验证。

## 17. 版本变更摘要

| 项 | v2 | **v3（本版）** |
|---|---|---|
| 核心抽象 | 约定路径 + capability.tar.gz | **Profile 驱动**的标准化 Dockerfile + 统一 Module |
| Module 内容 | public.tar.gz + manifest | **image + profile（+ Export 时 public.zip）** 容器 tar |
| 构建 | 无 | profile→Dockerfile→build→pack（离线 CLI） |
| Export | 纯 FS 打包 public | zip public + 重建镜像 → 多负载 module |
| Import | 解包 public 覆盖 workspace | 取 module 内 public.zip+profile 覆盖 workspace（不用其镜像） |
| profile 格式 | （v2 无 profile） | **TOML，`[dockerfile]` / `[vmdocker]` 分段** |
| bin | 单文件路径 | **目录**（整目录 COPY + chmod +x） |
| public 定义 | 硬编码 `SOUL.md`/`skills` | **`[vmdocker].public` 目录清单**；只支持目录，解压到 `/home/hymx` |
| 代码归属 | vmdocker（capability 包） | **host vmdocker 收拢**：modulebuild + cmd/module + capability；agent 无感知 |
| 构建加固 | 无 | 固定 `hymx` 用户、仅 HOME、profile 入镜像、不可关闭 |
