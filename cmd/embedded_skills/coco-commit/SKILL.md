---
name: coco-commit
description: 当需要生成、修正或排查 commit message 时使用。适用于手动调用 gcmsg、调试 commit-msg hook、基于暂存区 diff 生成 message，以及查看 AI 失败后的本地兜底策略。
---

# coco-commit

用于管理 commit message 生成链路。默认情况下，短 message 会在 `commit-msg` hook 中自动优化；手动调用 skill 的场景通常是预览、补跑、排障或强制覆盖。

## 何时使用

- 用户要求生成或修正 commit message
- `commit-msg` hook 没按预期工作，需要手动补跑
- 需要基于暂存区 diff 直接生成 message
- 需要把生成结果写入 Git 提供的 commit message 文件

## 自动化约定

- `commit-msg` hook 会检测过短的 message，并自动执行：

```bash
coco-ext gcmsg --staged --commit-msg-file "$MSG_FILE"
```

- 优化失败时会保留原始 message，不阻塞 commit
- AI 失败时会退回本地兜底 message

## 常用命令

```bash
coco-ext gcmsg
coco-ext gcmsg --amend
coco-ext gcmsg --staged
coco-ext gcmsg --commit-msg-file .git/COMMIT_EDITMSG
```

## 选择策略

- 只想预览生成结果，使用 `coco-ext gcmsg`
- 需要覆盖最近一个 commit，使用 `coco-ext gcmsg --amend`
- 还没提交、仅基于暂存区生成，使用 `coco-ext gcmsg --staged`
- 在 hook 场景直接写文件，使用 `--commit-msg-file`

## 输出与日志

- `gcmsg` 会打印生成过程、最终 message 和耗时
- hook 场景的日志位于 `.livecoding/logs/gcmsg-*.log`
- 如果生成 changelog，产物位于 `.livecoding/changelog/`

## 排查顺序

1. 先看日志里是否卡在“正在生成 commit message...”
2. 如果超过约 30 秒仍无结果，按超时失败处理
3. 如果模型输出混入解释文字，当前实现会提取真正的 conventional commit message
4. 如果 AI 完全失败，会退回本地兜底 message，而不是阻塞提交

## 例外说明

- `Merge` 和 `Revert` message 默认跳过自动优化
- 短 message 的自动优化依赖暂存区 diff；如果没有 staged 变更，hook 不会触发生成
