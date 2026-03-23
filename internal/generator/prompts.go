package generator

import "fmt"

// 各知识文件的 prompt 模板

const glossaryPrompt = `你是一个代码仓库分析专家。请根据以下仓库扫描结果，生成业务术语表（glossary）。

要求：
1. 从导出的 struct/interface 名称推测业务术语
2. 按模块分组
3. 使用 Markdown 表格格式，包含列：业务术语、代码标识符、别名、所在模块、状态
4. AI 推测的条目标记为 ❓，待人工确认
5. 输出纯 Markdown，不要包含代码块标记

示例格式：
# 业务术语表

> 由 coco-repo init 自动生成
> 标记说明：✅ = 已确认 | ❓ = AI 推测，需人工确认

## 模块名

| 业务术语 | 代码标识符 | 别名 | 所在模块 | 状态 |
|----------|-----------|------|---------|------|
| xxx | XxxStruct | xxx_struct | path/to/module | ❓ |

仓库扫描结果：
%s`

const architecturePrompt = `你是一个代码仓库分析专家。请根据以下仓库扫描结果，生成仓库架构文档。

要求：
1. 描述整体目录结构和分层设计
2. 识别技术栈（框架、中间件）
3. 描述核心模块之间的依赖关系
4. 输出纯 Markdown

仓库扫描结果：
%s`

const patternsPrompt = `你是一个代码仓库分析专家。请根据以下仓库扫描结果，提取代码模式文档。

要求：
1. 识别重复出现的代码模式（如 Handler → Service → Converter）
2. 每个模式包含：适用场景、目录结构、典型代码骨架
3. 如果能识别框架（Hertz/Kitex/Gin 等），给出框架特定的模式
4. 输出纯 Markdown

仓库扫描结果：
%s`

const gotchasPrompt = `你是一个代码仓库分析专家。请根据以下仓库扫描结果，生成踩坑记录和注意事项文档。

要求：
1. 从代码结构推测可能的隐式约定和陷阱
2. 包括：自动生成代码目录（不可手改）、命名约定、框架限制等
3. 标记 AI 推测的条目为 ❓
4. 输出纯 Markdown

仓库扫描结果：
%s`

// GetPrompt 根据知识文件名返回对应的 prompt
func GetPrompt(name, scanSummary string) string {
	switch name {
	case "glossary.md":
		return fmt.Sprintf(glossaryPrompt, scanSummary)
	case "architecture.md":
		return fmt.Sprintf(architecturePrompt, scanSummary)
	case "patterns.md":
		return fmt.Sprintf(patternsPrompt, scanSummary)
	case "gotchas.md":
		return fmt.Sprintf(gotchasPrompt, scanSummary)
	default:
		return ""
	}
}

// GetUpdatePrompt 生成增量更新的 prompt
func GetUpdatePrompt(name, existingContent, diffContent string) string {
	return fmt.Sprintf(`你是一个代码仓库分析专家。以下是知识文件 %s 的当前内容和最近的代码变更 diff。

请根据 diff 判断是否需要更新该知识文件。如果需要，输出更新后的完整文件内容。如果不需要更新，输出 "NO_UPDATE"。

当前内容：
%s

代码变更 diff：
%s`, name, existingContent, diffContent)
}
