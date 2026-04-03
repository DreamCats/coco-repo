# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目简介

coco-ext 是一个仓库级 AI 开发工作流工具箱，通过扫描代码仓库并借助 LLM（coco-acp-sdk daemon）自动生成知识文件，同时提供 PRD refine/plan、代码 review、commit/push 辅助、本地 metrics 聚合和 AGENTS.md 生成能力。

## 常用命令

```bash
# 构建（注入版本、commit、日期）
make build

# 交叉编译（darwin/linux × amd64/arm64）
make build-all

# 运行全部测试
make test

# 运行单个包的测试
go test ./internal/scanner/ -v

# 安装到 ~/.local/bin/
make install

# 依赖管理
go mod tidy
```

## 架构概览

三层结构：CLI（Cobra 命令）→ 业务逻辑（internal/）→ 外部依赖（coco-acp-sdk daemon + git）

```
main.go                         # 入口，ldflags 注入 version/gitCommit/buildDate
cmd/                            # Cobra CLI 命令层
  root.go                       #   根命令，绑定所有子命令
  context.go / init / update / query / status
  review.go                     #   AI Code Review（结构化管线）
  gcmsg.go                      #   Commit message 生成（--amend / --commit-msg-file）
  submit.go                     #   staged 变更 → commit → push 一键流程
  push.go                       #   git push 包装，成功后后台 review
  prd.go / prd_refine / prd_status / prd_plan
  agents.go                     #   生成 AGENTS.md 行为约束文件
  doctor.go                     #   环境诊断（--fix 自动修复）
  metrics.go                    #   本地指标聚合（review/prd/events）
  install.go                    #   安装/卸载 git hooks + skills
  daemon.go                     #   daemon 管理（start/status/stop）
  version.go                    #   版本信息
internal/
  config/defaults.go            # 超时、目录路径、知识文件定义
  scanner/scanner.go            # 仓库扫描（目录树 + Go AST + IDL）
  generator/
    generator.go                # coco-acp-sdk daemon 连接，流式生成
    prompts.go                  # 中文 prompt 模板
  knowledge/
    reader.go                   # 知识文件读取与状态查询
    writer.go                   # 知识文件写入（0600/0700 权限）
  git/git.go                    # git diff/branch/commit 封装
  gcmsg/gcmsg.go                # AI + 本地兜底 commit message 生成
  review/
    review.go                   # review 主流程
    facts.go / scope.go / release.go / impact.go / quality.go / summary.go
    types.go                    # Finding/FileChange/PipelineResult 等类型
  prd/
    refine.go                   # PRD refine（文本/文件/飞书链接 → task 目录）
    status.go                   # task 状态查询
    plan.go                     # design.md + plan.md 生成
    design_template.go          # design 模板
  metrics/events.go             # 本地事件采集（JSONL 追加写入）
  changelog/changelog.go        # commit 优化历史
  daemonutil/daemonutil.go      # daemon 状态修复辅助
```

## 核心流程

**context init/update**：检查 git 仓库 → `scanner.Scan()` → 连接 daemon → 生成 4 个知识文件（glossary → architecture → patterns → gotchas）→ 写入 `.livecoding/context/`。update 仅更新 git diff 影响部分，返回 "NO_UPDATE" 表示无需更新。

**review 管线**：git diff → 5 阶段结构化分析（facts → scope → release → impact → quality）→ summary 汇总 → 输出 report.md + 7 个 JSON 文件到 `.livecoding/review/<branch>-<commit>/`。支持 `--async` 后台运行、`--json` / `--json-only` 结构化输出。

**gcmsg**：基于 diff 生成 conventional commit message。AI 失败时自动降级到本地兜底（按变更文件名生成）。支持 `--amend`、`--staged`、`--commit-msg-file`。

**submit**：仅处理 staged 变更。message 策略：用户高质量 message → AI 生成 → 本地兜底。成功后自动执行 `coco-ext push`。全程记录 metrics 事件。

**push**：先 `git push`，成功后后台 `coco-ext review --async`。替代了不稳定的 pre-push hook 方案。

**PRD 工作流**：`prd refine` 接受文本/文件/飞书链接，生成 task 目录（task.json + source.json + prd.source.md + prd-refined.md）。`prd plan` 基于 refined PRD + context 生成 design.md 和 plan.md（含复杂度评估、拟改文件、任务列表）。

**agents**：在仓库根目录生成/更新 AGENTS.md，通过 `<!-- coco-ext-agents:start/end -->` 标记管理 section，支持 `--force` 强制替换。

**doctor**：7 项检查（repository / workspace / hooks / tooling / skills / daemon / logs），`--fix` 可自动修复 hooks、skills、workspace 目录和 daemon。`--json` / `--verbose` 支持详细输出。

**metrics**：聚合 `.livecoding/review/`、`.livecoding/tasks/`、`.livecoding/metrics/events.jsonl` 三类数据，输出 review 运行/评级分布、PRD task 状态/复杂度分布、submit/gcmsg 成功率。

## 关键约定

- Go 模块：`github.com/DreamCats/coco-ext`，Go 1.24.11
- CLI 框架：Cobra（`spf13/cobra`）
- 默认模型：`Doubao-Seed-2.0-Code`（字节跳动模型）
- 超时配置（`internal/config/defaults.go`）：
  - gcmsg：30s（`DefaultPromptTimeout`）
  - context init/update：5min（`ContextPromptTimeout`）
  - review：3min（`ReviewPromptTimeout`）
  - daemon 空闲退出：60min（可通过 `COCO_EXT_DAEMON_IDLE_TIMEOUT` 环境变量覆盖）
- 目录约定：`.livecoding/context/`（知识库）、`.livecoding/review/`（review 产物）、`.livecoding/tasks/`（PRD task）、`.livecoding/metrics/`（事件日志）、`.livecoding/changelog/`（commit 优化历史）
- scanner 跳过的目录：.git, .livecoding, vendor, node_modules, kitex_gen, dist, .idea, .vscode
- prompt 和用户界面均为中文
- 版本信息通过 Makefile ldflags 注入到 main 包变量
- review `--async` 拉起后台子进程，主进程立即返回日志路径和报告目录
- daemon 连接通过 `coco-acp-sdk` 的 `daemon.Dial()`，配置目录 `~/.config/coco-ext/`，支持自动启动
- `Prompt` / `Generate` / `Update` 统一超时保护，超时后主动关闭连接
- skills 安装到 `~/.trae/skills/`，由 `cmd/embedded_skills.go` 从二进制内置资源同步
- commit-msg hook：`COMMIT_EDITMSG` 第一行 < 10 字符时触发 AI 优化
- pre-commit hook：暂存区 .go 文件自动 goimports 格式化
