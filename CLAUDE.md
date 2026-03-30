# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目简介

coco-ext 是一个仓库级知识库生成工具，通过扫描代码仓库结构并借助 LLM（通过 coco-acp-sdk daemon）自动生成 4 个知识文件（glossary.md、architecture.md、patterns.md、gotchas.md），存放在 `.livecoding/context/` 目录下。

## 常用命令

```bash
# 构建（注入版本、commit、日期）
make build

# 交叉编译（darwin/linux × amd64/arm64）
make build-all

# 运行测试
make test            # 等价于 go test ./... -v

# 安装到 ~/.local/bin/
make install

# 依赖管理
go mod tidy
```

## 架构概览

三层结构：CLI → 业务逻辑 → 外部依赖

```
main.go                     # 入口，通过 ldflags 注入版本信息
cmd/                        # Cobra CLI 命令层
  root.go                   #   根命令
  context.go                #   context 子命令（知识库管理）
  init.go                   #   coco-ext context init
  update.go                 #   coco-ext context update
  query.go                  #   coco-ext context query
  status.go                 #   coco-ext context status
  review.go                 #   AI Code Review（基于 git diff）
  gcmsg.go                  #   Commit message 生成（支持 --amend）
  install.go                #   安装/卸载 git hooks + skills
  daemon.go                 #   daemon 管理（start/status/stop）
  version.go                #   版本信息
internal/
  config/defaults.go        # 硬编码配置（模型名、目录路径、知识文件定义）
  scanner/scanner.go        # 仓库扫描引擎（目录树 + Go AST 解析 + IDL 文件）
  generator/
    generator.go            # 封装 coco-acp-sdk daemon 连接，流式生成
    prompts.go              # 中文 prompt 模板（每个知识文件一个）
  knowledge/
    reader.go               # 知识文件读取与状态查询
    writer.go               # 知识文件写入（0600/0700 权限）
```

## 核心流程

**context init**：检查 git 仓库 → `scanner.Scan()` 扫描代码结构 → `generator.New()` 连接 daemon（自动启动）→ 按序生成 4 个知识文件（glossary → architecture → patterns → gotchas）→ 写入 `.livecoding/context/`

**context update**：获取 git diff → 加载已有知识文件 → 判断哪些文件受影响 → 仅更新变更部分，返回 "NO_UPDATE" 表示无需更新

**review 命令**：获取 git diff（最后一个 commit 或分支整体）→ 连接 coco daemon → 生成 review 报告 → 保存到 `.livecoding/review/`

**gcmsg 命令**：基于当前 commit diff 或暂存区 diff 生成规范 commit message；支持 `--amend` 覆盖上一个 commit，也支持 `--commit-msg-file` 直接写入 Git 的 commit message 文件。AI 失败时会退回本地兜底 message。

**submit 命令**：仅处理已 staged 的变更。优先使用用户提供的高质量 message，否则调用 AI 生成；若 AI 失败，则自动使用本地强兜底 message。随后执行 `git commit` 和 `coco-ext push`。

**install 命令**：安装 commit-msg hook（短 message 自动优化）和 pre-commit hook（goimports 格式化），并从二进制内置资源同步 skills。install 时检测 goimports 是否安装，未安装给出警告，并清理旧的 legacy post-commit / pre-push hook。

**uninstall 命令**：卸载 git hooks 和 skills（仅删除从 coco-ext 安装的部分，不影响其他来源的 skills）。

**commit-msg hook**：检测 `COMMIT_EDITMSG` 第一行是否过短；若过短，则调用 `coco-ext gcmsg --staged --commit-msg-file` 基于暂存区 diff 生成 message 并直接写回文件。优化失败时保留原始 message，不阻塞 commit。

**pre-commit hook**：检测暂存区中已修改的 .go 文件，运行 `goimports -w` 格式化后重新 add。

**push 命令**：`coco-ext push` 先执行 `git push`；当 push 成功后，再后台触发 `coco-ext review --async`。这个命令替代了此前不稳定的 `pre-push review` 方案。

**daemon 连接**：通过 `coco-acp-sdk` 的 `daemon.Dial()` 连接，配置目录 `~/.config/coco-ext/`，支持自动启动、流式 prompt、状态查询、关闭。`Prompt` / `Generate` / `Update` 统一带 30 秒超时保护，超时后主动关闭当前连接。

## 关键约定

- Go 模块：`github.com/DreamCats/coco-ext`，Go 1.24.11
- CLI 框架：Cobra（`spf13/cobra`）
- 默认模型：`Doubao-Seed-2.0-Code`（字节跳动模型）
- 知识文件目录：`.livecoding/context/`（已 gitignore）
- Changelog 目录：`.livecoding/changelog/`（按分支名组织，记录 commit 优化历史）
- scanner 跳过的目录：.git, .livecoding, vendor, node_modules, kitex_gen, dist, .idea, .vscode
- prompt 和用户界面均为中文
- 版本信息通过 Makefile ldflags 注入到 main 包变量
- `review --async` 会真正拉起后台子进程，主进程立即返回日志路径和报告目录
- `review` 使用 3 分钟专用超时；`gcmsg` / `context` 仍使用 30 秒默认超时
