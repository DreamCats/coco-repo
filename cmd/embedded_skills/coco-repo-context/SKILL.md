---
name: coco-repo-context
description: 当需要为代码仓库初始化、增量更新或查询 .livecoding/context/ 业务知识库时使用。适用于编码前补充业务上下文、刷新知识文件、查询术语或架构信息。
---

# coco-repo-context

用于管理仓库级上下文知识库。目标不是解释代码语法，而是让 AI 先获取业务术语、架构分层、代码模式和易错点。

## 何时使用

- 用户要求初始化或更新 `.livecoding/context/`
- 编码前需要先补齐业务上下文
- 用户要求解释仓库术语、模块职责或历史约定
- 发现已有上下文可能过期，需要按 diff 增量刷新

## 默认做法

1. 先检查 `.livecoding/context/` 是否已存在。
2. 如果只是查询术语、模块或约定，优先使用 `coco-ext context query`，不要先做全量初始化。
3. 首次接入仓库时使用 `coco-ext context init`。
4. 已有知识库时优先使用 `coco-ext context update`，必要时加 `--range` 限定变更范围。

## 常用命令

```bash
coco-ext context init
coco-ext context update
coco-ext context update --range HEAD~3..HEAD
coco-ext context query "术语或模块名"
coco-ext context query --file glossary.md "PopCard"
```

## 关键产物

- `.livecoding/context/glossary.md`
- `.livecoding/context/architecture.md`
- `.livecoding/context/patterns.md`
- `.livecoding/context/gotchas.md`

## 选择策略

- 需要了解术语映射，先看 `glossary.md`
- 需要了解目录分层、职责边界，先看 `architecture.md`
- 需要复用实现骨架，先看 `patterns.md`
- 需要避坑和隐式约定，先看 `gotchas.md`

## 异常与降级

- 如果 AI 生成超时，命令会在约 30 秒后失败，不会无限卡住
- 如果只是临时查询失败，优先直接读取已有 `.livecoding/context/` 文件，而不是重复执行全量生成
- 如果知识库明显过期，再执行 `context update`，不要默认回退到 `context init`
