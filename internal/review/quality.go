package review

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/DreamCats/coco-ext/internal/config"
	"github.com/DreamCats/coco-ext/internal/generator"
	"github.com/DreamCats/coco-ext/internal/git"
)

//go:embed prompts/review.md
var qualityPromptTemplate string

func AnalyzeQuality(gen *generator.Generator, facts Facts, onChunk func(string)) (QualityResult, error) {
	prompt := fmt.Sprintf(
		qualityPromptTemplate,
		facts.DiffInfo.SourceBranch,
		facts.DiffInfo.TargetBranch,
		facts.DiffInfo.CommitMessage,
		facts.DiffInfo.Author,
		facts.DiffInfo.Additions,
		facts.DiffInfo.Deletions,
		facts.DiffInfo.FileCount,
		renderFactsSummary(facts),
		truncateDiff(facts.DiffInfo.Diff, 40000),
	)

	result, err := gen.PromptWithTimeout(prompt, config.ReviewPromptTimeout, onChunk)
	if err != nil {
		return QualityResult{}, fmt.Errorf("生成代码质量审查失败: %w", err)
	}

	quality := parseQualityOutput(strings.TrimSpace(result))
	quality.Raw = strings.TrimSpace(result)
	return quality, nil
}

func renderFactsSummary(facts Facts) string {
	lines := make([]string, 0, 8)
	if len(facts.ConfigFiles) > 0 {
		lines = append(lines, "- 配置文件变更: "+strings.Join(facts.ConfigFiles, ", "))
	}
	if len(facts.IDLFiles) > 0 {
		lines = append(lines, "- IDL 文件变更: "+strings.Join(facts.IDLFiles, ", "))
	}
	if len(facts.PublicSignatureChange) > 0 {
		lines = append(lines, "- 公开签名变更: "+strings.Join(facts.PublicSignatureChange, "; "))
	}
	if len(facts.MissingTests) > 0 {
		lines = append(lines, "- 缺少测试文件: "+strings.Join(facts.MissingTests, ", "))
	}
	if len(facts.SensitiveFindings) > 0 {
		lines = append(lines, "- 敏感信息风险: "+strings.Join(facts.SensitiveFindings, "; "))
	}
	if len(facts.LintIssues) > 0 {
		linterCounts := make(map[string]int)
		for _, issue := range facts.LintIssues {
			linterCounts[issue.FromLinter]++
		}
		var parts []string
		for linter, count := range linterCounts {
			parts = append(parts, fmt.Sprintf("%s %d 个", linter, count))
		}
		summary := "- golangci-lint 已发现问题: " + strings.Join(parts, ", ")
		if facts.LintOutputDir != "" {
			summary += " (详情: " + facts.LintOutputDir + ")"
		}
		lines = append(lines, summary)
	}
	if len(lines) == 0 {
		return "- 未检测到额外的程序化风险信号"
	}
	return strings.Join(lines, "\n")
}

func parseQualityOutput(raw string) QualityResult {
	result := QualityResult{
		Findings: make([]Finding, 0),
	}
	if raw == "" {
		return result
	}

	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "SUMMARY:"):
			result.Summary = strings.TrimSpace(strings.TrimPrefix(line, "SUMMARY:"))
		case strings.HasPrefix(line, "P0:"):
			if finding, ok := parseQualityFinding(SeverityP0, line); ok {
				result.Findings = append(result.Findings, finding)
			}
		case strings.HasPrefix(line, "P1:"):
			if finding, ok := parseQualityFinding(SeverityP1, line); ok {
				result.Findings = append(result.Findings, finding)
			}
		case strings.HasPrefix(line, "P2:"):
			if finding, ok := parseQualityFinding(SeverityP2, line); ok {
				result.Findings = append(result.Findings, finding)
			}
		}
	}

	return result
}

func parseQualityFinding(severity Severity, line string) (Finding, bool) {
	content := strings.TrimSpace(line[3:])
	parts := strings.SplitN(content, "|", 4)
	if len(parts) != 4 {
		return Finding{}, false
	}

	return Finding{
		Severity:   severity,
		Source:     "code-reviewer",
		File:       strings.TrimSpace(parts[0]),
		Title:      strings.TrimSpace(parts[1]),
		Detail:     strings.TrimSpace(parts[2]),
		Suggestion: strings.TrimSpace(parts[3]),
	}, true
}

func GenerateReport(gen *generator.Generator, repoRoot string, diffInfo *git.DiffInfo, onChunk func(string)) (string, error) {
	result, err := GeneratePipeline(gen, repoRoot, diffInfo, onChunk)
	if err != nil {
		return "", err
	}
	return result.ReportMD, nil
}

func GeneratePipeline(gen *generator.Generator, repoRoot string, diffInfo *git.DiffInfo, onChunk func(string)) (PipelineResult, error) {
	facts := CollectFacts(repoRoot, diffInfo)
	scope := AnalyzeScope(facts)
	release := AnalyzeRelease(facts)
	impact := AnalyzeImpact(facts)
	quality, err := AnalyzeQuality(gen, facts, onChunk)
	if err != nil {
		return PipelineResult{}, err
	}
	inputs := ReportInputs{
		Facts:   facts,
		Scope:   scope,
		Release: release,
		Impact:  impact,
		Quality: quality,
	}
	return PipelineResult{
		Facts:    facts,
		Scope:    scope,
		Release:  release,
		Impact:   impact,
		Quality:  quality,
		Summary:  BuildSummary(inputs),
		ReportMD: BuildReport(inputs),
	}, nil
}
