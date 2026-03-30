---
name: coco-submit
description: 当需要基于 staged 变更自动生成 commit message、执行 git commit 并 push 时使用。适用于交互式提交流程、AI 失败自动兜底、以及统一人和 AI 的提交流程。
---

# coco-submit

用于执行一条受控的提交工作流：只处理已 staged 的变更，自动生成 commit message，提交成功后再执行 `coco-ext push`。

## 何时使用

- 用户要求“提交并推送当前已暂存的改动”
- 需要统一 AI 与终端用户的提交流程
- 希望在 AI 生成失败时自动走本地兜底，而不是回退到手输 message

## 默认做法

1. 先检查当前目录是否为 git 仓库。
2. 再检查是否有 staged 变更；默认不执行 `git add .`。
3. message 生成策略如下：
   - 高质量用户 message：直接使用
   - 否则优先 AI 生成
   - AI 失败或超时：自动使用本地强兜底 message
4. 执行 `git commit`
5. 执行 `coco-ext push`

## 常用命令

```bash
coco-ext submit
coco-ext submit "fix: 调整 review 触发方式"
```

## 例外说明

- 如果没有 staged 变更，直接失败并提示用户先执行 `git add`
- 如果用户提供的是明显过短或不规范的 message，仍会改走 AI / 本地兜底生成
- `submit` 不负责决定哪些文件应该被 add，这个控制权保留给用户
