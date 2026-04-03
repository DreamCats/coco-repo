# coco-ext

面向仓库的 AI 开发工作流工具箱。除了生成和维护 `.livecoding/context/` 业务上下文，还提供 PRD refine/plan、代码 review、提交辅助、push 包装和本地 metrics 聚合能力。

## 为什么需要 coco-ext？

团队仓库业务知识复杂度高，AI agent 对代码理解强但缺乏业务上下文；同时，PRD 落地、review、commit、push 等研发动作也缺少统一的仓库内工作流。

coco-ext 解决这个问题：自动沉淀仓库上下文 → 基于 PRD 生成 task 产物 → 对代码变更执行 review → 辅助 commit / push → 产出本地可观测 metrics。

## 知识文件

```
.livecoding/context/
├── glossary.md          # 业务术语 <-> 代码标识符映射（核心）
├── architecture.md      # 仓库架构概览（目录结构、分层、技术栈）
├── patterns.md          # 代码模式（Handler/Service/Converter 骨架）
└── gotchas.md           # 踩坑记录、隐式约定、容易出错的点
```

## 安装

```bash
# 一键安装
GOPRIVATE=github.com/DreamCats/coco-acp-sdk go install github.com/DreamCats/coco-ext@latest

# 或从源码编译安装
git clone git@github.com:DreamCats/coco-ext.git
cd coco-ext
make install
```

## 使用

```bash
# 1. Context 知识库
cd /path/to/your/repo
coco-ext context init
coco-ext context update
coco-ext context update --range HEAD~3..HEAD   # 指定 commit 范围
coco-ext context query "讲解卡"
coco-ext context query --file glossary.md "PopCard"
coco-ext context status

# 2. PRD 工作流
cd /path/to/your/repo
coco-ext prd refine --prd "做一个支持飞书链接导入的 PRD 工作流"
coco-ext prd refine --prd https://bytedance.larkoffice.com/docx/xxx
coco-ext prd status
coco-ext prd plan

# 3. Code Review
coco-ext review              # 审查最后一个 commit
coco-ext review --full       # 审查分支整体 diff
coco-ext review --async      # 后台启动 review，立即返回日志和报告目录
coco-ext review --json       # 输出结构化 JSON，同时写入 report.md / *.json
coco-ext review --json-only  # 仅输出 JSON，不打印过程日志

# 4. Commit Message 生成
coco-ext gcmsg              # 生成 message
coco-ext gcmsg --amend      # 生成并覆盖上一个 commit
coco-ext gcmsg --staged     # 基于暂存区 diff 生成 message
coco-ext gcmsg --commit-msg-file .git/COMMIT_EDITMSG   # 写入 commit message 文件

# 5. Push 包装命令
cd /path/to/your/repo
coco-ext push              # 等价于 git push；成功后后台启动 review + lint
coco-ext push origin main  # 透传 git push 参数

# 6. Submit 工作流命令
cd /path/to/your/repo
coco-ext submit                  # AI 优先生成 message，失败时自动本地兜底
coco-ext submit "fix: 调整 hook"  # 若 message 足够规范，则直接使用

# 7. 本地指标聚合
cd /path/to/your/repo
coco-ext metrics
coco-ext metrics --json

# 8. 代码风格检查（golangci-lint）
coco-ext lint              # 前台运行 lint
coco-ext lint --async      # 后台异步运行
coco-ext lint --json       # JSON 输出

# 9. 安装钩子
cd /path/to/your/repo
coco-ext install            # 安装 commit-msg + pre-commit hook + 同步 skills + 生成 lint 配置
coco-ext uninstall          # 卸载 hooks + skills（仅删除从 coco-ext 安装的部分）

# 10. Daemon 管理
coco-ext daemon status      # 查看 daemon 状态
coco-ext daemon start       # 前台启动 daemon（阻塞）
coco-ext daemon start -d    # 后台启动 daemon
coco-ext daemon stop        # 停止 daemon
```

## PRD 工作流

- `coco-ext prd refine --prd <文本|本地文件|飞书链接>` 会为需求生成 task 目录，并落盘 `task.json`、`source.json`、`prd.source.md`、`prd-refined.md`
- `coco-ext prd status` 用于查看最近一个 task 的当前状态、缺失产物和下一步命令
- `coco-ext prd plan` 基于 refined PRD、context 和本地调研结果生成 `design.md` 与 `plan.md`
- `plan.md` 现在包含复杂度评估、拟改文件、任务列表、实施步骤、风险补充和验证建议，便于后续 codegen 或人工继续推进

## Review 产物

- `coco-ext review` 默认在 `.livecoding/review/<branch>-<commit>/` 下写入结果
- 除 `report.md` 外，还会输出 `facts.json`、`scope.json`、`release.json`、`impact.json`、`quality.json`、`summary.json`、`report.json`
- `coco-ext review --json` 会将完整结构化结果打印到标准输出，同时保留 markdown 与 JSON 产物
- `coco-ext review --json-only` 适合脚本消费；不会打印过程日志

## Lint 产物

- `coco-ext lint` 默认在 `.livecoding/lint/<branch>-<commit>/` 下写入结果
- 除 `lint.md` 外，还会输出 `lint.json`（含 issue 详情和 linter 统计）
- `coco-ext lint --json` 仅输出 golangci-lint 原始 JSON
- `coco-ext lint --async` 后台运行，日志写入 `.livecoding/logs/`
- lint 使用 `.livecoding/config/.golangci.yml` 作为配置（`coco-ext install` 时自动生成，已有则不覆盖）
- 未安装 golangci-lint 时，`lint` 命令会报错提示安装，`push` 中的异步 lint 静默跳过

## Git Hooks

### commit-msg hook
- 烂 commit message（< 10 字符）时，基于暂存区 diff 自动生成规范 message
- 直接写入 Git 传入的 `COMMIT_EDITMSG`，不再通过 `post-commit` 二次 `amend`
- AI 生成失败时自动回退到本地兜底 message，不阻塞 commit
- 输出优化耗时和日志路径，便于排查

### pre-commit hook
- 自动格式化已修改的 .go 文件（goimports）
- 保证 import 顺序一致

## Push 包装命令

- `coco-ext push` 先执行 `git push`
- 只有当 push 成功后，才会后台启动 `coco-ext review --async`
- 如果本机安装了 `golangci-lint`，push 后还会异步执行 lint 检查；未安装时静默跳过
- 这样可以避免 `pre-push` hook 与真实 push 过程互相干扰

## Submit 工作流命令

- `coco-ext submit` 只处理已 staged 的变更，不会默认执行 `git add .`
- commit message 生成策略：高质量用户 message 直接使用，否则优先 AI 生成，失败或超时则自动使用本地强兜底 message
- `submit` 成功后会继续执行 `coco-ext push`
- `submit` / `gcmsg` 会在 `.livecoding/metrics/events.jsonl` 中记录本地事件，供后续 metrics 聚合

## 本地 Metrics

- `coco-ext metrics` 会聚合 `.livecoding/review`、`.livecoding/tasks`、`.livecoding/metrics/events.jsonl`
- 当前指标包含三类：
  - review 运行次数、评级分布、P0/P1/P2 与 finding 总量
  - prd task 数量、状态分布、来源类型分布、plan complexity 分布
  - submit / gcmsg 成功率、message source 分布、最近事件时间
- `coco-ext metrics --json` 适合接脚本或后续 dashboard

## 内置 Skills

`coco-ext` 二进制内置了 skills 资源，执行 `coco-ext install` 时会直接同步到 `~/.trae/skills/`。当前内置 5 个 skill：

- `coco-repo-context`：初始化、更新、查询 `.livecoding/context/`
- `coco-repo-setup`：安装/卸载 hooks 与同步 repo 内置 skills
- `coco-review`：手动补跑 review、查看异步日志和报告
- `coco-commit`：生成或排查 commit message，理解 hook 与兜底策略
- `coco-submit`：基于 staged 变更自动生成 message、commit 并 push

这些 skill 的目标不是替代 CLI，而是把“何时调用、如何排查、产物在哪看”固化下来，方便 AI 在仓库内稳定复用。

## 生成与超时策略

- `gcmsg` 优先使用 AI 生成 commit message；当模型输出夹带说明文字时，会自动提取真正的 conventional commit message
- 如果 AI 生成失败，会根据变更文件生成本地兜底 message（如 `docs: 更新 AGENTS.md`）
- `gcmsg`、`context init/update` 使用 30 秒默认超时
- `review` 使用 3 分钟专用超时，避免大 diff 审查过早中断

## 前置依赖

- Go 1.24+
- [coco](https://github.com/anthropics/coco) CLI（`coco acp serve` 可用）
- [coco-acp-sdk](https://github.com/DreamCats/coco-acp-sdk)（自动通过 go module 引入）
- goimports（用于 pre-commit hook格式化）
- golangci-lint（可选，用于 `coco-ext lint` 和 push 后异步 lint）

> coco daemon 会在首次调用时自动拉起，无需手动启动。
> 更新 hook 行为后，请在目标仓库重新执行一次 `coco-ext install`。

## 开发

```bash
make build          # 编译
make test           # 运行测试
make build-all      # 交叉编译（darwin/linux, amd64/arm64）
make install        # 编译并安装到 ~/.local/bin/
```

## 项目结构

```
coco-ext/
├── main.go                  # 入口
├── cmd/                     # CLI 命令：context / prd / review / gcmsg / push / submit / lint / metrics / daemon / install
├── internal/
│   ├── config/              # 默认配置与超时
│   ├── generator/           # 调 coco daemon / AI 生成内容
│   ├── git/                 # git diff、branch、commit 等封装
│   ├── knowledge/           # .livecoding/context/ 文件读写
│   ├── lint/                # golangci-lint 执行与结果写入
│   ├── metrics/             # 本地事件采集
│   ├── prd/                 # PRD refine / status / plan 产物生成
│   ├── review/              # review facts/scope/release/impact/quality 管线
│   └── scanner/             # 仓库扫描（目录树、Go 类型、IDL 文件）
└── docs/                    # PRD、协议文档、技术设计
```

## License

Internal use only.
