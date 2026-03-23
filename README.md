# coco-repo

仓库级代码上下文知识库生成工具。通过 AI 自动扫描分析仓库，生成并维护 `.context/` 目录下的业务知识文件，让 AI 编码具备业务上下文。

## 为什么需要 coco-repo？

团队仓库业务知识复杂度高，AI agent 对代码理解强但缺乏业务上下文，导致生成的技术方案和代码不及预期。

coco-repo 解决这个问题：自动扫描仓库 → AI 生成业务知识文件 → 后续 AI 编码时自动获取上下文。

## 知识文件

```
.context/
├── glossary.md          # 业务术语 <-> 代码标识符映射（核心）
├── architecture.md      # 仓库架构概览（目录结构、分层、技术栈）
├── patterns.md          # 代码模式（Handler/Service/Converter 骨架）
└── gotchas.md           # 踩坑记录、隐式约定、容易出错的点
```

## 安装

```bash
# 从源码编译安装
make install

# 或手动编译
make build
```

## 使用

```bash
# 1. 初始化 — 首次为仓库生成知识库
cd /path/to/your/repo
coco-repo init

# 2. 增量更新 — 基于 git diff 更新知识文件
coco-repo update
coco-repo update --range HEAD~3..HEAD   # 指定 commit 范围

# 3. 查询 — 搜索知识库内容
coco-repo query "讲解卡"
coco-repo query --file glossary.md "PopCard"

# 4. 状态 — 查看知识库覆盖率
coco-repo status
```

## 前置依赖

- Go 1.24+
- [coco](https://github.com/anthropics/coco) CLI（`coco acp serve` 可用）
- [coco-acp-sdk](https://github.com/DreamCats/coco-acp-sdk)（自动通过 go module 引入）

> coco daemon 会在首次调用时自动拉起，无需手动启动。

## 开发

```bash
make build          # 编译
make test           # 运行测试
make build-all      # 交叉编译（darwin/linux, amd64/arm64）
make install        # 编译并安装到 ~/.local/bin/
```

## 项目结构

```
coco-repo/
├── main.go                  # 入口
├── cmd/                     # cobra 命令（init/update/query/status/version）
├── internal/
│   ├── config/              # 硬编码配置
│   ├── scanner/             # 仓库扫描（目录树、Go 类型、IDL 文件）
│   ├── generator/           # 调 coco-acp-sdk 生成知识文件
│   └── knowledge/           # .context/ 文件读写
└── docs/                    # PRD、协议文档、技术设计
```

## License

Internal use only.
