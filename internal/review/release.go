package review

import (
	"fmt"
	"strings"
)

func AnalyzeRelease(facts Facts) ReleaseResult {
	result := ReleaseResult{Findings: make([]Finding, 0)}

	for _, item := range facts.SensitiveFindings {
		result.Findings = append(result.Findings, Finding{
			Severity:   SeverityP0,
			Source:     "risk-check",
			Title:      "检测到敏感信息或敏感文件",
			Detail:     item,
			Suggestion: "确认是否误提交密钥、密码、Token 等敏感内容，必要时立即回退并重新生成凭据。",
		})
	}

	for _, item := range facts.DDLFindings {
		severity := SeverityP1
		suggestion := "确认这类数据库变更是否已评估上线步骤和回滚方案。"
		upper := strings.ToUpper(item)
		if strings.Contains(upper, "DROP TABLE") || strings.Contains(upper, "DROP COLUMN") || strings.Contains(upper, "REMOVED REQUIRED FIELD") {
			severity = SeverityP0
			suggestion = "这是高风险破坏性变更，请确认兼容性、灰度策略和回滚方案。"
		}
		result.Findings = append(result.Findings, Finding{
			Severity:   severity,
			Source:     "risk-check",
			Title:      "检测到数据库或 IDL 高风险变更",
			Detail:     item,
			Suggestion: suggestion,
		})
	}

	for _, path := range facts.ConfigFiles {
		result.Findings = append(result.Findings, Finding{
			Severity:   SeverityP1,
			Source:     "risk-check",
			File:       path,
			Title:      "检测到配置变更",
			Detail:     "配置文件发生修改，可能需要同步线上环境或发布参数。",
			Suggestion: "确认相关环境配置、开关值和默认值是否已同步。",
		})
	}

	if facts.FIXMECount > 0 {
		result.Findings = append(result.Findings, Finding{
			Severity:   SeverityP1,
			Source:     "risk-check",
			Title:      "新增 FIXME 标记",
			Detail:     fmt.Sprintf("本次改动新增了 %d 处 FIXME。", facts.FIXMECount),
			Suggestion: "确认这些 FIXME 是否能在本次提交前处理，或至少补充明确注释和后续计划。",
		})
	}
	if facts.HACKCount > 0 {
		result.Findings = append(result.Findings, Finding{
			Severity:   SeverityP1,
			Source:     "risk-check",
			Title:      "新增 HACK 标记",
			Detail:     fmt.Sprintf("本次改动新增了 %d 处 HACK。", facts.HACKCount),
			Suggestion: "确认临时方案是否可接受，必要时补充风险说明。",
		})
	}
	if facts.TODOCount > 0 {
		result.Findings = append(result.Findings, Finding{
			Severity:   SeverityP2,
			Source:     "risk-check",
			Title:      "新增 TODO 标记",
			Detail:     fmt.Sprintf("本次改动新增了 %d 处 TODO。", facts.TODOCount),
			Suggestion: "确认 TODO 不影响当前提交目标，并记录后续处理计划。",
		})
	}

	for _, path := range facts.MissingTests {
		result.Findings = append(result.Findings, Finding{
			Severity:   SeverityP1,
			Source:     "risk-check",
			File:       path,
			Title:      "缺少对应测试文件",
			Detail:     "检测到 Go 源文件改动，但仓库中不存在同名 _test.go 文件。",
			Suggestion: "确认是否需要补充测试，至少为关键逻辑增加覆盖。",
		})
	}

	for _, path := range facts.LargeFiles {
		if isDocOnlyPath(facts, path) {
			continue
		}
		result.Findings = append(result.Findings, Finding{
			Severity:   SeverityP2,
			Source:     "risk-check",
			File:       path,
			Title:      "单文件改动较大",
			Detail:     "单文件改动超过 200 行，review 和回滚成本较高。",
			Suggestion: "确认是否需要拆分提交或补充更详细的说明。",
		})
	}

	if facts.TotalChangedLines > 500 && hasNonDocRiskFiles(facts) {
		result.Findings = append(result.Findings, Finding{
			Severity:   SeverityP1,
			Source:     "risk-check",
			Title:      "整体变更规模较大",
			Detail:     fmt.Sprintf("本次总改动达到 %d 行。", facts.TotalChangedLines),
			Suggestion: "确认是否适合拆分为更小的提交或分阶段合入。",
		})
	}

	if len(facts.LintIssues) > 0 {
		linterCounts := make(map[string]int)
		for _, issue := range facts.LintIssues {
			linterCounts[issue.FromLinter]++
		}
		var parts []string
		for linter, count := range linterCounts {
			parts = append(parts, fmt.Sprintf("%s: %d", linter, count))
		}
		severity := SeverityP2
		if facts.LintIssues != nil && len(facts.LintIssues) > 10 {
			severity = SeverityP0
		} else if facts.LintIssues != nil && len(facts.LintIssues) > 5 {
			severity = SeverityP1
		}
		result.Findings = append(result.Findings, Finding{
			Severity:   severity,
			Source:     "lint",
			Title:      "golangci-lint 发现代码风格问题",
			Detail:     fmt.Sprintf("共 %d 个 lint issue（按 linter：%s）", len(facts.LintIssues), strings.Join(parts, ", ")),
			Suggestion: "建议在提交前处理高优先级 lint issue。",
		})
	}

	return result
}

func hasNonDocRiskFiles(facts Facts) bool {
	for _, file := range facts.Files {
		if !file.IsDocLike {
			return true
		}
	}
	return false
}

func isDocOnlyPath(facts Facts, path string) bool {
	for _, file := range facts.Files {
		if file.Path == path {
			return file.IsDocLike
		}
	}
	return false
}

func AnalyzeImpact(facts Facts) ImpactResult {
	if facts.GoFileCount == 0 {
		return ImpactResult{
			Skipped: true,
			Reason:  "未检测到 Go 代码变更，跳过影响面分析。",
		}
	}
	if len(facts.PublicSignatureChange) == 0 {
		return ImpactResult{
			Skipped: true,
			Reason:  "未检测到公开函数签名变更，跳过影响面分析。",
		}
	}

	return ImpactResult{
		Findings: []Finding{
			{
				Severity:   SeverityP1,
				Source:     "impact",
				Title:      "检测到需要人工确认的影响面",
				Detail:     strings.Join(facts.PublicSignatureChange, "; "),
				Suggestion: "重点检查 handler、service、脚本入口及外部调用方是否需要同步适配。",
			},
		},
	}
}
