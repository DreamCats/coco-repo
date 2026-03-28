package review

import (
	"fmt"
	"strings"

	"github.com/DreamCats/coco-ext/internal/generator"
	"github.com/DreamCats/coco-ext/internal/gitlab"
)

const reviewPromptTemplate = `## Code Review 任务

请对以下 GitLab Merge Request 进行 Code Review。

### MR 信息
- **标题**: %s
- **作者**: %s
- **源分支**: %s → **目标分支**: %s
- **链接**: %s

### 代码变更 (diff)
%s

---

## Review 要求

请从以下维度进行 review：

### 1. 正确性 (Correctness)
- 代码逻辑是否正确
- 边界条件是否处理
- 错误处理是否完善

### 2. 安全性 (Security)
- 是否有安全漏洞（SQL注入、XSS、敏感信息泄露等）
- 权限检查是否到位

### 3. 性能 (Performance)
- 是否有性能问题
- 数据库查询是否需要优化
- 是否有不必要的重复计算

### 4. 可维护性 (Maintainability)
- 代码是否清晰易读
- 命名是否规范
- 是否有重复代码可以提取

### 5. 测试 (Testing)
- 是否有必要的单元测试
- 测试覆盖是否充分
- 边界条件是否有测试

### 6. 影响面评估 (Impact)
- 这个 MR 会影响哪些模块
- 是否有破坏性变更
- 是否需要同步修改相关文档

### 7. 风险点 (Risks)
- 高风险项（如果有）
- 需要特别注意的地方

---

## 输出格式

请输出 Markdown 格式的 Review 报告：

### 概要
- 整体评价
- 建议的 merge 策略（可以直接合并 / 需要修改后合并 / 建议拒绝）

### 详细 Review
#### 正确性
[详细说明]

#### 安全性
[详细说明]

...

### 影响面评估
[详细说明]

### 风险点
[详细说明]

### 建议
[如果需要修改的话]

---

请开始 Review。
`

// GenerateReport 生成 review 报告
func GenerateReport(gen *generator.Generator, mrInfo *gitlab.MRInfo, diff string, onChunk func(string)) (string, error) {
	prompt := fmt.Sprintf(reviewPromptTemplate,
		mrInfo.Title,
		mrInfo.Author,
		mrInfo.SourceBranch,
		mrInfo.TargetBranch,
		mrInfo.WebURL,
		truncateDiff(diff, 50000), // 限制 diff 长度
	)

	result, err := gen.Prompt(prompt, onChunk)
	if err != nil {
		return "", fmt.Errorf("生成 review 报告失败: %w", err)
	}

	return strings.TrimSpace(result), nil
}

// truncateDiff 截断过长的 diff
func truncateDiff(diff string, maxLen int) string {
	if len(diff) <= maxLen {
		return diff
	}
	return diff[:maxLen] + fmt.Sprintf("\n\n... (diff 已截断，原始长度 %d 字符)", len(diff))
}
