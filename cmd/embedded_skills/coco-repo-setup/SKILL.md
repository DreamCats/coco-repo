---
name: coco-repo-setup
description: 当需要为仓库安装、更新或卸载 coco-ext 协作环境时使用。适用于安装 git hooks、同步 repo 内置 skills、排查本地协作链路是否生效。
---

# coco-repo-setup

用于把 `coco-ext` 接入目标仓库。重点是安装 hooks、同步 repo 内置 skills，以及确认本地协作链路是否按预期工作。

## 何时使用

- 用户要求在某个仓库启用 `coco-ext`
- 目标仓库更新了 hook 或 skills，需要重新安装
- 用户反馈 `gcmsg`、`review`、`goimports` 等自动化没有生效
- 用户要求卸载 `coco-ext` 在仓库里安装的内容

## 默认做法

1. 在目标仓库根目录执行 `coco-ext install`。
2. 安装后确认以下三类行为是否存在：
   - `commit-msg`：短 message 自动优化
   - `pre-commit`：对暂存的 Go 文件执行 `goimports`
   - `coco-ext push`：push 成功后后台触发 review
3. 安装时会将二进制内置的 skills 同步到 `~/.trae/skills/`。
4. 需要移除时执行 `coco-ext uninstall`。

## 常用命令

```bash
coco-ext install
coco-ext uninstall
coco-ext daemon status
```

## 安装后应具备的行为

- `.git/hooks/commit-msg`
- `.git/hooks/pre-commit`
- `~/.trae/skills/` 下同步的 repo 内置 skills

## 排查顺序

1. 先确认当前目录是否为目标仓库根目录。
2. 再确认 `.git/hooks/` 中的脚本是否已刷新为最新版本。
3. 如果是 review 或 gcmsg 行为异常，再查看 `.livecoding/logs/`。
4. 如果涉及模型连接，再检查 `coco-ext daemon status`。

## 例外说明

- `commit-msg` 优化失败时会保留原始 message，不阻塞 commit
- `pre-commit` 依赖 `goimports`，未安装时只告警，不中断提交
