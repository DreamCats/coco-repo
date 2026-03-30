---
name: coco-review
description: 当需要审查当前 commit、分支整体变更，或排查 .livecoding/review 与 review 日志时使用。适用于代码评审、风险检查、手动补跑 review、查看异步报告。
---

# coco-review

用于生成和查看代码审查报告。默认建议通过 `coco-ext push` 在 push 成功后后台触发 review；skill 负责说明何时手动补跑、如何查看产物、以及异常时如何定位。

## 何时使用

- 用户要求做代码 review、风险排查或变更审查
- 需要手动补跑 review，而不是等 `pre-push`
- 需要查看最近一次异步 review 的日志或报告
- 需要比较“最后一个 commit”与“整个分支 diff”的审查范围

## 自动化约定

- 执行 `coco-ext push` 时，push 成功后会后台触发 `coco-ext review --async`
- 自动触发只负责后台启动，不保证报告在命令返回前生成完成
- 自动触发后，先看 `.livecoding/logs/` 和 `.livecoding/review/`

## 常用命令

```bash
coco-ext review
coco-ext review --full
coco-ext review --async
```

## 选择策略

- 只看最近一次提交，使用 `coco-ext review`
- 需要看当前分支整体改动，使用 `coco-ext review --full`
- 不希望阻塞当前终端，使用 `coco-ext review --async`

## 关键产物

- 报告目录：`.livecoding/review/<branch>-<commit>/`
- 审查报告：`.livecoding/review/<branch>-<commit>/report.md`
- 日志目录：`.livecoding/logs/`

## 排查顺序

1. 先确认报告目录是否已经生成。
2. 如果没有报告，再检查最新的 `review-*.log`。
3. 如果是 `--async`，先看日志中的 `Background spawned at`、`Review started at`、`Review finished at`。
4. 如果模型长时间无响应，命令会在约 30 秒后超时返回。

## 例外说明

- `review --async` 的成功返回只表示后台任务已启动，不表示报告已经生成完成
- 如果 `coco-ext push` 已经自动触发 review，除非用户明确要求，否则不要重复再跑一次同步 review
