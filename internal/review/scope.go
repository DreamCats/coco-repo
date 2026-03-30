package review

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
)

func AnalyzeScope(facts Facts) ScopeResult {
	result := ScopeResult{
		Findings: make([]Finding, 0),
		Core:     make([]string, 0),
		Edge:     make([]string, 0),
		Outliers: make([]string, 0),
	}
	if len(facts.Files) <= 1 {
		for _, file := range facts.Files {
			result.Core = append(result.Core, file.Path)
		}
		return result
	}

	for _, file := range facts.Files {
		score := averageRelation(file, facts.Files)
		switch {
		case score < 0.18:
			result.Outliers = append(result.Outliers, file.Path)
			result.Findings = append(result.Findings, Finding{
				Severity:   SeverityP0,
				Source:     "diff-guard",
				File:       file.Path,
				Title:      "检测到离群改动",
				Detail:     fmt.Sprintf("与其余改动的平均关联分数仅为 %.2f，可能是与主需求无关的顺手改。", score),
				Suggestion: "确认这处改动是否属于本次需求；如果无关，建议拆分或回退。",
			})
		case score < 0.35:
			result.Edge = append(result.Edge, file.Path)
			result.Findings = append(result.Findings, Finding{
				Severity:   SeverityP1,
				Source:     "diff-guard",
				File:       file.Path,
				Title:      "检测到边缘改动",
				Detail:     fmt.Sprintf("与主改动的平均关联分数为 %.2f，建议人工确认是否确实需要一并提交。", score),
				Suggestion: "确认这处改动的必要性；如属于独立需求，建议拆分提交。",
			})
		default:
			result.Core = append(result.Core, file.Path)
		}
	}

	return result
}

func averageRelation(target FileChange, files []FileChange) float64 {
	total := 0.0
	count := 0
	for _, other := range files {
		if other.Path == target.Path {
			continue
		}
		total += relationScore(target, other)
		count++
	}
	if count == 0 {
		return 1
	}
	return total / float64(count)
}

func relationScore(left, right FileChange) float64 {
	score := 0.0
	if left.TopDir != "" && left.TopDir == right.TopDir {
		score += 0.25
	}
	if filepath.Dir(left.Path) == filepath.Dir(right.Path) {
		score += 0.30
	}
	if left.Category == right.Category {
		score += 0.15
	}
	if filepath.Ext(left.Path) == filepath.Ext(right.Path) {
		score += 0.10
	}
	score += sharedTokenScore(left.Path, right.Path) * 0.20

	sizeLeft := left.Additions + left.Deletions
	sizeRight := right.Additions + right.Deletions
	score += sizeSimilarity(sizeLeft, sizeRight) * 0.10

	return math.Min(score, 1.0)
}

func sharedTokenScore(left, right string) float64 {
	leftTokens := tokenizePath(left)
	rightTokens := tokenizePath(right)
	if len(leftTokens) == 0 || len(rightTokens) == 0 {
		return 0
	}

	shared := 0
	for token := range leftTokens {
		if rightTokens[token] {
			shared++
		}
	}
	union := len(leftTokens) + len(rightTokens) - shared
	if union == 0 {
		return 0
	}
	return float64(shared) / float64(union)
}

func tokenizePath(path string) map[string]bool {
	path = strings.ToLower(strings.TrimSuffix(path, filepath.Ext(path)))
	replacer := strings.NewReplacer("/", " ", "_", " ", "-", " ", ".", " ")
	parts := strings.Fields(replacer.Replace(path))
	result := make(map[string]bool, len(parts))
	for _, part := range parts {
		if len(part) >= 2 {
			result[part] = true
		}
	}
	return result
}

func sizeSimilarity(left, right int) float64 {
	if left == 0 || right == 0 {
		return 0
	}
	smaller := min(left, right)
	larger := max(left, right)
	return float64(smaller) / float64(larger)
}
