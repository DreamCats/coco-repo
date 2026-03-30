# coco-ext

仓库级代码上下文知识库生成工具。通过 AI 自动扫描分析仓库，生成并维护 `.livecoding/context/` 目录下的业务知识文件，让 AI 编码具备业务上下文。

## 为什么需要 coco-ext？

团队仓库业务知识复杂度高，AI agent 对代码理解强但缺乏业务上下文，导致生成的技术方案和代码不及预期。

coco-ext 解决这个问题：自动扫描仓库 → AI 生成业务知识文件 → 后续 AI 编码时自动获取上下文。

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
# 1. 初始化 — 首次为仓库生成知识库
cd /path/to/your/repo
coco-ext context init

# 2. 增量更新 — 基于 git diff 更新知识文件
coco-ext context update
coco-ext context update --range HEAD~3..HEAD   # 指定 commit 范围

# 3. 查询 — 搜索知识库内容
coco-ext context query "讲解卡"
coco-ext context query --file glossary.md "PopCard"

# 4. 状态 — 查看知识库覆盖率
coco-ext context status

# 5. Code Review — AI 审查代码变更
coco-ext review              # 审查最后一个 commit
coco-ext review --full       # 审查分支整体 diff
coco-ext review --async      # 后台启动 review，立即返回日志和报告目录

# 6. Commit Message 生成 — 自动生成规范 commit message
coco-ext gcmsg              # 生成 message
coco-ext gcmsg --amend      # 生成并覆盖上一个 commit
coco-ext gcmsg --staged     # 基于暂存区 diff 生成 message
coco-ext gcmsg --commit-msg-file .git/COMMIT_EDITMSG   # 写入 commit message 文件

# 7. Push 包装命令 — push 成功后后台触发 review
cd /path/to/your/repo
coco-ext push              # 等价于 git push；成功后后台启动 review
coco-ext push origin main  # 透传 git push 参数

# 8. Submit 工作流命令 — 提交 staged 变更并 push
cd /path/to/your/repo
coco-ext submit                  # AI 优先生成 message，失败时自动本地兜底
coco-ext submit "fix: 调整 hook"  # 若 message 足够规范，则直接使用

# 9. 安装钩子 — 一键安装 git hooks
cd /path/to/your/repo
coco-ext install            # 安装 commit-msg + pre-commit hook + 同步 skills
coco-ext uninstall          # 卸载 hooks + skills（仅删除从 coco-ext 安装的部分）

# 10. Daemon 管理 — 手动管理 coco daemon
coco-ext daemon status      # 查看 daemon 状态
coco-ext daemon start       # 前台启动 daemon（阻塞）
coco-ext daemon start -d    # 后台启动 daemon
coco-ext daemon stop        # 停止 daemon

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
- 这样可以避免 `pre-push` hook 与真实 push 过程互相干扰

## Submit 工作流命令

- `coco-ext submit` 只处理已 staged 的变更，不会默认执行 `git add .`
- commit message 生成策略：
  - 高质量用户 message：直接使用
  - 否则优先 AI 生成
  - AI 失败或超时：自动使用本地强兜底 message
- `submit` 成功后会继续执行 `coco-ext push`

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
├── cmd/                     # cobra 命令（init/update/query/status/version）
├── internal/
│   ├── config/              # 硬编码配置
│   ├── scanner/             # 仓库扫描（目录树、Go 类型、IDL 文件）
│   ├── generator/           # 调 coco-acp-sdk 生成知识文件
│   └── knowledge/           # .livecoding/context/ 文件读写
└── docs/                    # PRD、协议文档、技术设计
```

## License

Internal use only.
