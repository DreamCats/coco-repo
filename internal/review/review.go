package review

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/DreamCats/coco-ext/internal/generator"
	"github.com/DreamCats/coco-ext/internal/git"
)

//go:embed prompts/review.md
var reviewPromptTemplate string

// GenerateReport 生成 review 报告
func GenerateReport(gen *generator.Generator, diffInfo *git.DiffInfo, onChunk func(string)) (string, error) {
	prompt := fmt.Sprintf(reviewPromptTemplate,
		diffInfo.SourceBranch,
		diffInfo.TargetBranch,
		diffInfo.CommitMessage,
		diffInfo.Author,
		diffInfo.Additions,
		diffInfo.Deletions,
		diffInfo.FileCount,
		truncateDiff(diffInfo.Diff, 50000), // 限制 diff 长度
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
