# vmdocker Agent 能力 Export / Import 设计

- 日期：2026-06-16
- 状态：已评审，待实现
- 涉及仓库：`vmdocker`（host 编排，目录布局）、`vmdocker_agent`（容器内服务，打包/解包 + HTTP 接口）

## 1. 背景与目标

让用户把一个 agent 的**能力**（`skills/`、`SOUL.md` 等）导出成一个可移植的 **能力 Module 文件**，
并在另一个 agent 上导入，从而**复刻**这个 agent。导出**不包含**任何私有配置/用户数据。

非目标（YAGNI）：
- 不做 Arweave 实际上链（只产出本地 module 文件；上链走现有/未来流程）。
- 不做 public/private 之外的细粒度权限或内容加密。
- 不做 CLI（本期只暴露 HTTP 接口）。

## 2. 确定的设计决策

| 决策点 | 选择 |
|---|---|
| 原文件位置 | 在 agent workspace 内（`sandbox_workspace/<pid>/workspace/`） |
| public/private 视图 | 只放**相对软链接**，指向 `workspace/` 内真实文件；原文件位置/结构不动 |
| 哪些进 public | **约定路径自动软链接**（`SOUL.md`、`skills/`） |
| 触发方式 | vmdocker_agent server 的 HTTP 端点 `/vmm/export`、`/vmm/import` |
| module 格式 | 复用 modulegen 的 `ModuleArtifact` + hymx `SaveModule`（签名 ANS-104 BundleItem） |
| 导出签名 | 临时密钥自签（沙箱无需预置密钥）；导入端不强校验签名 |

## 3. 目录布局（vmdocker 侧）

在 `sandbox_workspace/<pid>/` 下新增 `public/`、`private/` 两个**视图目录**，与现有目录平级：

```
sandbox_workspace/<pid>/
├── workspace/                ← 原文件真实存放处（VMDOCKER_AGENT_WORKSPACE）
│   ├── SOUL.md               ← 真实文件
│   ├── skills/<name>/...      ← 真实目录
│   └── data/                  ← 用户数据（示例）
├── public/                   ← 新增：只放相对软链接（可导出）
│   ├── SOUL.md  → ../workspace/SOUL.md
│   └── skills   → ../workspace/skills
├── private/                  ← 新增：只放相对软链接（不可导出，仅供检视）
│   ├── openclaw → ../.openclaw
│   └── data     → ../workspace/data
├── .openclaw/  .home/  .tmp/  .xdg/   ← 不变
```

约定路径表（常量，可扩展）：
- public：`SOUL.md`（文件）、`skills`（目录）
- private：`.openclaw`（state/凭据/session）、`workspace/data`（用户数据约定目录）

软链接一律用**相对路径**，保证现有 checkpoint/restore（`utils.CompressDirectory` 整体搬动 workspace）后链接不断。

### 3.1 vmdocker 改动点

文件：`vmdocker/vmdocker/runtimemanager/env.go`

- 新增常量：`publicViewDirName = "public"`、`privateViewDirName = "private"`。
- 新增约定链接表：
  ```go
  type viewLink struct{ Link, Target string } // 均相对 workspace 根
  var publicConventionLinks  = []viewLink{{"public/SOUL.md","workspace/SOUL.md"},{"public/skills","workspace/skills"}}
  var privateConventionLinks = []viewLink{{"private/openclaw",".openclaw"},{"private/data","workspace/data"}}
  ```
- `runtimeWorkspaceLayoutDirs(workspace)` 追加 `public/`、`private/`。
- 新增 `ensureWorkspaceViews(workspace string) error`（幂等）：对每条约定链接：
  - 目标不存在 → 跳过（不创建悬空链接）。
  - 链接已存在且指向正确 → 跳过。
  - 链接存在但指向错误 → 修正（删除重建）。
  - 同名位置是**真实文件/目录**（非软链接）→ `log.Warn` 跳过，**绝不覆盖**。
- 新增 `resolveWithinWorkspace(workspace, path string) (string, error)`：解析绝对路径并确认在 `workspace` 根内，越界返回 error。建链与导出都复用。
- 在 `ensureRuntimeWorkspaceLayout` 末尾调用 `ensureWorkspaceViews`。

## 4. 能力 Module 文件格式（精确）

复用现有 module 链路：`ModuleArtifact{ModuleBytes, Tags}` → `hymxSchema.Module` → `SaveModule` → 签名 `goarSchema.BundleItem` → JSON 落盘。
能力 module 落盘文件名为 `cap-<itemId>.json`（与镜像 module 的 `mod-<id>.json` 区分）。

### 4.1 三层结构

```
第3层  cap-<itemId>.json            = 签名 BundleItem（JSON）
        ├─ tags[]                    = Module-Format / Capability-* 元数据（goar base64 编码）
        ├─ data                      = base64url( 第2层 capability.tar.gz )
        └─ signature/owner/id/...    = goar 签名字段（临时密钥）
第2层  capability.tar.gz            = ModuleArtifact.ModuleBytes
第1层  tar 内容
        ├─ capability.manifest.json  ← 第一个条目，便于流式校验
        ├─ SOUL.md                   ← 解引用后的真实内容
        └─ skills/<name>/...         ← 解引用后的真实内容
```

### 4.2 BundleItem tags（第3层）

经 `hymxSchema.Module` + `utils.ModuleToTags` 生成：

| Tag Name | 值 |
|---|---|
| `Data-Protocol` | `hymx` |
| `Variant` | `v0.1.0` |
| `Type` | `Module` |
| `Module-Format` | `hymx.vmdocker.capability.v0.0.1` |
| `Capability-Source` | `public-workspace` |
| `Capability-Manifest-SHA256` | manifest 的 sha256（hex） |
| `Capability-Skills` | 逗号分隔 skill 名，便于不解包预览 |
| `Capability-Created-At` | RFC3339 |

### 4.3 `capability.manifest.json`（第1层，tar 内）

```json
{
  "format": "hymx.vmdocker.capability.v0.0.1",
  "created_at": "2026-06-16T12:00:00Z",
  "soul":   { "path": "SOUL.md", "sha256": "…", "size": 1234 },
  "skills": [
    { "name": "code-review",
      "files": [ { "path": "skills/code-review/SKILL.md", "sha256": "…", "size": 1234 } ] }
  ],
  "entries": [ { "path": "SOUL.md", "type": "file", "sha256": "…", "size": 1234 } ]
}
```

- `entries[]`：**全量**文件清单（含每个 skill 内全部文件），导入端逐条 sha256 比对。
- `soul` / `skills`：结构化展示视图。

## 5. vmdocker_agent 侧：打包 / 解包

新增文件 `vmdocker_agent/modulegen/capability.go`，与现有 `GenerateModuleArtifact()` 平行。

### 5.1 常量

```go
const (
    CapabilityModuleFormat   = "hymx.vmdocker.capability.v0.0.1"
    CapabilitySourceTag      = "Capability-Source"
    CapabilitySourceValue    = "public-workspace"
    CapabilityManifestTag    = "Capability-Manifest-SHA256"
    CapabilitySkillsTag      = "Capability-Skills"
    CapabilityCreatedAtTag   = "Capability-Created-At"
    DefaultCapabilityMaxBytes = 64 << 20 // 64 MiB
)
```

### 5.2 导出

```go
func GenerateCapabilityModuleArtifact(workspaceRoot string) (ModuleArtifact, error)
```

**三重过滤**（针对软链接视图的安全收口）：
1. **约定清单**：只遍历约定的 public 入口（`SOUL.md`、`skills`），**不盲目 tar 整个 `public/`**——即使有人往 public 里塞了额外软链接也不进包。
2. **解引用**：跟随软链接读真实内容（`os.Readlink` + `resolveWithinWorkspace`）。
3. **越界校验**：每个解引用目标必须落在 `workspaceRoot` 内，否则整体失败（`PATH_ESCAPE`）。

步骤：
1. 解析 `public/SOUL.md`、`public/skills` 的真实目标，校验在 workspace 内。
2. 流式遍历真实内容，计算每个文件 sha256，构建 `manifest`。
3. 写 tar：先写 `capability.manifest.json`，再写各文件（路径用 tar 内相对路径 `SOUL.md`、`skills/...`）；gzip 压缩 → `ModuleBytes`。
4. 计算 manifest sha256，组装 tags（4.2 表）→ 返回 `ModuleArtifact`。

### 5.3 导入

```go
type ImportOptions struct {
    OnConflict string // "skip"(默认) | "overwrite" | "fail"
    MaxBytes   int64  // 0 → DefaultCapabilityMaxBytes
}
type ImportResult struct {
    Imported []string `json:"imported"`
    Skipped  []string `json:"skipped"`
    Skills   []string `json:"skills"`
    Soul     bool     `json:"soul"`
}

func ImportCapabilityModule(moduleFileBytes []byte, workspaceRoot string, opts ImportOptions) (ImportResult, error)
```

步骤：
1. `json.Unmarshal(moduleFileBytes, &goarSchema.BundleItem{})`。
2. `tags = arutils.TagsDecode(item.Tags)` → `hymxUtils.TagsToModule(tags)` → 断言 `Module-Format == CapabilityModuleFormat`，否则 `FORMAT_MISMATCH`。
3. `tarGz = arutils.Base64Decode(item.Data)`；`len(tarGz) ≤ MaxBytes`，否则 `TOO_LARGE`。
4. gunzip，读第一个 tar 条目 `capability.manifest.json`，校验其 sha256 == tag `Capability-Manifest-SHA256`，否则 `MANIFEST_MISMATCH`。
5. 解包到临时目录 `workspace/.import-<ts>/`：
   - 路径净化：拒绝绝对路径、`..`、tar 内 symlink 条目；目标 = `Join(workspaceRoot,"workspace",entry.path)` 并再次确认在 `workspace/` 内（否则 `PATH_ESCAPE`）。
   - 写**真实文件**，边写边算 sha256，与 manifest `entries[]` 比对，不符 → 整体失败回滚。
6. 冲突策略对 `workspace/` 既有路径：
   - `skip`（默认）：已存在则跳过，记入 `Skipped`。
   - `overwrite`：覆盖。
   - `fail`：遇冲突整体失败。
7. 校验通过后，将临时目录内文件按策略原子 `rename` 就位到 `workspace/`。
8. **不建软链接**：`public/` 软链接由 vmdocker 下次 spawn 时 `ensureWorkspaceViews` 自动重建。
9. 返回 `ImportResult`。

## 6. HTTP 接口（vmdocker_agent server）

`server/api.go` 的 `/vmm` 组新增两路由；请求/响应类型放入 `server/runtime_api_types.go`。
workspace 根解析顺序（对齐 `runtime/claudecode`）：`VMDOCKER_RUNTIME_WORKSPACE` → `VMDOCKER_AGENT_WORKSPACE` 的父目录 → `os.Getwd`。

### 6.1 `POST /vmm/export`

- 请求体（可选）：`{ "sign": true }`（默认 true）。
- 处理：`GenerateCapabilityModuleArtifact(root)` → 用 `hymxSchema.Module{Base:DefaultBaseModule, ModuleFormat:CapabilityModuleFormat, Tags:artifact.Tags}` 经 `SaveModule`-等价逻辑生成 BundleItem。
  - 签名密钥来自 `VMDOCKER_MODULE_SIGNER_KEY`；**缺省时生成一次性临时密钥**自签（决策 D-1）。秘密永不进沙箱。
- 响应：`200 application/json`，体即 `cap-<id>.json` 的 BundleItem JSON（可直接存盘）。
- 响应头：`X-Capability-Item-Id`、`X-Capability-Skills`。

### 6.2 `POST /vmm/import`

- 请求：`Content-Type: application/json`，体 = BundleItem JSON；`on_conflict=skip|overwrite|fail`（query 或 body）。
- 响应：`200`
  ```json
  { "imported": ["SOUL.md","skills/code-review/SKILL.md"], "skipped": [], "skills": ["code-review"], "soul": true }
  ```
- 失败：`400 FORMAT_MISMATCH` / `413 TOO_LARGE` / `422 MANIFEST_MISMATCH|PATH_ESCAPE`，体 `{ "error":"…", "code":"…" }`。

### 6.3 复刻流程

A 容器 `POST /vmm/export` → 得 `cap-X.json` → 发给 B 容器 `POST /vmm/import` →
B 的 `workspace/` 出现同样的 `SOUL.md` + `skills/` → B 下次 spawn 自动建 `public/` 软链接 → B 复刻 A 的能力。

## 7. 安全与边界

- **路径穿越**：所有软链接解引用与 tar 解包目标都过 `resolveWithinWorkspace` 白名单；拒绝绝对路径软链接、`../` 越界、tar 内 symlink 条目。
- **private 永不进包**：导出器只认 public 约定清单，从不读 `private/` 或 `workspace/` 内非约定路径。
- **不覆盖真实文件**：建视图遇同名真实文件 → warn 跳过。
- **大小限制**：导入 `MaxBytes`（默认 64 MiB，可由 `VMDOCKER_CAPABILITY_MAX_BYTES` 覆盖），防 zip-bomb。
- **原子性**：导入先写临时目录、全量 sha256 校验通过后再 rename 就位；任一步失败整体回滚。

## 8. 测试

- `vmdocker/vmdocker/runtimemanager/env_test.go`
  - 视图目录创建；软链接为相对路径且指向正确；幂等（重复调用不报错、不重建）。
  - 同名位置是真实文件时不覆盖、仅 warn。
  - 目标不存在时跳过、不建悬空链接。
  - `resolveWithinWorkspace` 越界目标被拒。
- `vmdocker_agent/modulegen/capability_test.go`
  - 导出只含约定内容；软链接解引用正确；恶意越界软链接被拒；`private/` 内容不进包。
  - manifest sha256 与文件内容一致；tags 正确。
  - 导入：还原真实文件；`skip`/`overwrite`/`fail` 三策略；超限 `TOO_LARGE`；format 不符 `FORMAT_MISMATCH`；manifest 篡改 `MANIFEST_MISMATCH`；tar 路径穿越 `PATH_ESCAPE`。
  - **round-trip**：export → import 后两侧 `workspace/` 内容字节一致。
- `vmdocker_agent/server/api_test.go`
  - `/vmm/export` 成功返回合法 BundleItem JSON（httptest）。
  - `/vmm/import` 成功路径 + 各错误码路径。
  - export→import 端到端复刻。

## 9. 实施顺序建议

1. vmdocker：目录布局 + 视图软链接（§3.1）+ 测试。
2. vmdocker_agent：`modulegen/capability.go` 导出 + 导入（§5）+ 单测。
3. vmdocker_agent：HTTP 接口（§6）+ api 测试。
4. 端到端 round-trip 验证。
