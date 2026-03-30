# coco-ext 技术设计文档

> 日期：2026-03-22
> 状态：Draft
> 作者：maifeng

---

## 1. 项目定位

coco-ext 是一个仓库级代码上下文知识库生成工具。CLI 命令名为 `coco-ext`。通过调用 coco ACP 的代码理解能力，自动扫描分析目标仓库，生成并维护 `.livecoding/context/` 目录下的业务知识文件，让后续 AI 编码具备业务上下文。

## 2. 核心命令

| 命令 | 用途 | 触发方式 |
|------|------|----------|
| `coco-ext init` | 首次为仓库生成知识库 | 人工执行 |
| `coco-ext update` | 基于 git diff 增量更新知识文件 | commit hook 或人工执行 |
| `coco-ext query` | 查询知识库内容 | MCP 工具调用 / 人工执行 |
| `coco-ext status` | 查看知识库状态和覆盖率 | 人工执行 |

暂不实现 `coco-ext config`，所有配置硬编码默认值。

## 3. 知识文件结构

```
.livecoding/context/
├── glossary.md          # 业务术语 <-> 代码标识符映射（核心）
├── architecture.md      # 仓库架构概览（目录结构、分层、技术栈）
├── patterns.md          # 代码模式（Handler/Service/Converter 骨架）
└── gotchas.md           # 踩坑记录、隐式约定、容易出错的点
```

## 4. 项目结构

```
coco-ext/
├── main.go                         # 入口，ldflags 注入版本信息
├── Makefile                        # build / test / install
├── go.mod                          # module github.com/DreamCats/coco-ext
├── cmd/                            # cobra 命令定义
│   ├── root.go                     # coco-ext 根命令
│   ├── init.go                     # coco-ext init
│   ├── update.go                   # coco-ext update
│   ├── query.go                    # coco-ext query
│   ├── status.go                   # coco-ext status
│   └── version.go                  # coco-ext version
├── internal/
│   ├── scanner/                    # 仓库扫描
│   │   └── scanner.go              # 分析代码结构、核心类型、RPC 接口
│   ├── generator/                  # 知识文件生成（核心）
│   │   ├── generator.go            # 调 coco-acp-sdk 让 AI 生成/更新内容
│   │   └── prompts.go              # 各知识文件的 prompt 模板
│   ├── knowledge/                  # .livecoding/context/ 文件读写
│   │   ├── reader.go               # 解析已有知识文件
│   │   └── writer.go               # 写入/更新知识文件
│   └── config/                     # 硬编码配置
│       └── defaults.go             # 默认 DialOption、模型、路径等
├── pkg/                            # 公开 API（供 MCP 工具或其他 agent import）
│   └── context/                    # 知识库查询接口
│       └── client.go
└── docs/                           # 文档
    ├── prd.md
    ├── coco-acp-protocol.md
    └── technical-design.md         # 本文档
```

参考项目：byte-auth（同作者，同风格 cobra CLI）。

## 5. 技术依赖

| 依赖 | 用途 | 来源 |
|------|------|------|
| `github.com/DreamCats/coco-acp-sdk` | 连接 coco ACP daemon | 同级项目 |
| `github.com/spf13/cobra` | CLI 框架 | 开源 |
| `github.com/fatih/color` | 终端彩色输出 | 开源 |

## 6. coco-acp-sdk 连接配置

当前硬编码默认值，后续可通过 `coco-ext config` 命令开放配置。

```go
// internal/config/defaults.go

const (
    DefaultConfigDir = "~/.config/coco-ext"
    DefaultModel     = "Doubao-Seed-2.0-Code"
    ContextDir       = ".context"
)

// DefaultDialOption 返回连接 coco daemon 的默认配置
func DefaultDialOption() *daemon.DialOption {
    return nil // 使用 coco-acp-sdk 默认值
}
```

连接方式：

```go
conn, err := daemon.Dial(repoPath, config.DefaultDialOption())
```

后续如需自定义：
- `ConfigDir` — 自定义 sock/pid 文件目录
- `DaemonCmd` — 自定义 daemon 二进制路径
- `DaemonArgs` — 自定义启动参数

## 7. 命令详细设计

### 7.1 coco-ext init

**流程：**

1. 检查当前目录是否为 git 仓库
2. 检查 `.livecoding/context/` 是否已存在（已存在则提示是否覆盖）
3. 调用 `scanner` 扫描仓库结构（目录树、Go 类型、RPC 接口、import 关系）
4. 将扫描结果作为上下文，调用 `generator` 通过 coco ACP 逐个生成知识文件
5. 写入 `.livecoding/context/` 目录
6. 输出生成结果摘要

**生成顺序：** glossary → architecture → patterns → gotchas（有依赖关系，术语表优先）

### 7.2 coco-ext update

**流程：**

1. 读取最近 commit 的 git diff（或指定 commit 范围）
2. 加载已有 `.livecoding/context/` 知识文件
3. 判断 diff 影响哪些知识文件（新类型 → glossary，新目录 → architecture，新模式 → patterns）
4. 仅对受影响的文件调用 `generator` 局部更新
5. 写回 `.livecoding/context/`

**触发方式：** 人工执行；如需自动化，可由外部脚本或 CI 调用。当前默认安装的 Git hooks 不会自动触发 `context update`。

### 7.3 coco-ext query

**流程：**

1. 接收查询关键词或术语
2. 在 `.livecoding/context/` 知识文件中搜索匹配内容
3. 返回结构化结果（术语定义、代码位置、相关模式等）

**设计为 MCP 工具：** coco 编码时可通过 MCP 协议调用 `coco-ext query` 获取业务上下文，无需人工喂信息。

### 7.4 coco-ext status

**输出示例：**

```
.livecoding/context/ 知识库状态
──────────────────────────
  glossary.md       32 条目  (28 已确认 / 4 待确认)
  architecture.md   已生成   最后更新: 2026-03-22
  patterns.md       5 个模式 最后更新: 2026-03-20
  gotchas.md        8 条记录 最后更新: 2026-03-21
──────────────────────────
  覆盖率: 87%  上次更新: 2 天前
```

## 8. 约定

- 用户文案、错误信息、注释全部使用中文
- 文件权限：配置文件 `0600`，目录 `0700`
- `.livecoding/context/` 应纳入 git 版本管理，团队共享
- commit message 风格与 byte-auth 保持一致

## 9. Git Hook 设计

当前默认安装 3 个 hook：

### 9.1 commit-msg

- 触发时机：`git commit` 写入 `COMMIT_EDITMSG` 后、真正创建 commit 前
- 目标：对过短的 commit message 做自动优化
- 行为：
  1. 读取 `COMMIT_EDITMSG` 第一行
  2. 若长度小于 10，则调用 `coco-ext gcmsg --staged --commit-msg-file`
  3. `gcmsg` 基于暂存区 diff 生成 commit message，并直接写回 `COMMIT_EDITMSG`
  4. AI 失败时，退回本地兜底 message；若仍失败，则保留原始 message，不阻塞 commit

### 9.2 pre-commit

- 触发时机：`git commit` 创建前
- 目标：格式化暂存区中的 `.go` 文件
- 行为：执行 `goimports -w` 并重新 `git add`

### 9.3 push 包装命令

- 触发时机：用户显式执行 `coco-ext push`
- 目标：在 `git push` 成功后后台触发本地 Code Review
- 行为：
  1. 执行 `git push [args...]`
  2. 若 push 失败，直接返回失败，不触发 review
  3. 若 push 成功，后台执行 `coco-ext review --async`
  4. 输出 review 日志路径和报告目录，便于排查

## 10. gcmsg 设计补充

`gcmsg` 当前支持两类入口：

- **已提交 commit 模式**：默认读取 `HEAD` 的 diff，用于人工执行 `coco-ext gcmsg` 或 `--amend`
- **暂存区模式**：通过 `--staged` 读取 `git diff --cached`，用于 `commit-msg` hook

输出处理策略：

1. 优先使用 AI 生成 commit message
2. 若模型输出包含解释性前言或 Markdown 代码块，提取真正的 conventional commit message
3. 若 AI 失败，则根据变更文件类型生成本地兜底 message，如：
   - `docs: 更新 AGENTS.md`
   - `build: 更新 2 个构建配置文件`

## 11. 超时与会话策略

- 每次 `generator.New()` 都会新建一个 coco ACP session，不复用历史 session
- `Prompt`、`Generate`、`Update` 默认使用 30 秒超时
- review 生成报告使用 3 分钟专用超时
- 超时后主动关闭当前 daemon 连接，避免 `gcmsg`、`review` 或知识库生成长时间卡住
