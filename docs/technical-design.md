# coco-repo 技术设计文档

> 日期：2026-03-22
> 状态：Draft
> 作者：maifeng

---

## 1. 项目定位

coco-repo 是一个仓库级代码上下文知识库生成工具。CLI 命令名为 `coco-repo`。通过调用 coco ACP 的代码理解能力，自动扫描分析目标仓库，生成并维护 `.context/` 目录下的业务知识文件，让后续 AI 编码具备业务上下文。

## 2. 核心命令

| 命令 | 用途 | 触发方式 |
|------|------|----------|
| `coco-repo init` | 首次为仓库生成知识库 | 人工执行 |
| `coco-repo update` | 基于 git diff 增量更新知识文件 | commit hook 或人工执行 |
| `coco-repo query` | 查询知识库内容 | MCP 工具调用 / 人工执行 |
| `coco-repo status` | 查看知识库状态和覆盖率 | 人工执行 |

暂不实现 `coco-repo config`，所有配置硬编码默认值。

## 3. 知识文件结构

```
.context/
├── glossary.md          # 业务术语 <-> 代码标识符映射（核心）
├── architecture.md      # 仓库架构概览（目录结构、分层、技术栈）
├── patterns.md          # 代码模式（Handler/Service/Converter 骨架）
└── gotchas.md           # 踩坑记录、隐式约定、容易出错的点
```

## 4. 项目结构

```
coco-repo/
├── main.go                         # 入口，ldflags 注入版本信息
├── Makefile                        # build / test / install
├── go.mod                          # module github.com/DreamCats/coco-repo
├── cmd/                            # cobra 命令定义
│   ├── root.go                     # coco-repo 根命令
│   ├── init.go                     # coco-repo init
│   ├── update.go                   # coco-repo update
│   ├── query.go                    # coco-repo query
│   ├── status.go                   # coco-repo status
│   └── version.go                  # coco-repo version
├── internal/
│   ├── scanner/                    # 仓库扫描
│   │   └── scanner.go              # 分析代码结构、核心类型、RPC 接口
│   ├── generator/                  # 知识文件生成（核心）
│   │   ├── generator.go            # 调 coco-acp-sdk 让 AI 生成/更新内容
│   │   └── prompts.go              # 各知识文件的 prompt 模板
│   ├── knowledge/                  # .context/ 文件读写
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

当前硬编码默认值，后续可通过 `coco-repo config` 命令开放配置。

```go
// internal/config/defaults.go

const (
    DefaultConfigDir = "~/.config/coco-repo"
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

### 7.1 coco-repo init

**流程：**

1. 检查当前目录是否为 git 仓库
2. 检查 `.context/` 是否已存在（已存在则提示是否覆盖）
3. 调用 `scanner` 扫描仓库结构（目录树、Go 类型、RPC 接口、import 关系）
4. 将扫描结果作为上下文，调用 `generator` 通过 coco ACP 逐个生成知识文件
5. 写入 `.context/` 目录
6. 输出生成结果摘要

**生成顺序：** glossary → architecture → patterns → gotchas（有依赖关系，术语表优先）

### 7.2 coco-repo update

**流程：**

1. 读取最近 commit 的 git diff（或指定 commit 范围）
2. 加载已有 `.context/` 知识文件
3. 判断 diff 影响哪些知识文件（新类型 → glossary，新目录 → architecture，新模式 → patterns）
4. 仅对受影响的文件调用 `generator` 局部更新
5. 写回 `.context/`

**触发方式：** 可配置为 git commit hook（`post-commit`）或人工执行。

### 7.3 coco-repo query

**流程：**

1. 接收查询关键词或术语
2. 在 `.context/` 知识文件中搜索匹配内容
3. 返回结构化结果（术语定义、代码位置、相关模式等）

**设计为 MCP 工具：** coco 编码时可通过 MCP 协议调用 `coco-repo query` 获取业务上下文，无需人工喂信息。

### 7.4 coco-repo status

**输出示例：**

```
.context/ 知识库状态
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
- `.context/` 应纳入 git 版本管理，团队共享
- commit message 风格与 byte-auth 保持一致
