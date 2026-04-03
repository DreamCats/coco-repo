package lint

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/DreamCats/coco-ext/internal/config"
)

// IsGolangciLintAvailable 检测 golangci-lint 是否已安装
func IsGolangciLintAvailable() bool {
	_, err := exec.LookPath("golangci-lint")
	return err == nil
}

// LintIssue 表示 golangci-lint 输出的一条 issue
type LintIssue struct {
	FromLinter string `json:"FromLinter"`
	Text       string `json:"Text"`
	Pos        struct {
		Filename string `json:"Filename"`
		Line     int    `json:"Line"`
		Column   int    `json:"Column"`
	} `json:"Pos"`
	Severity string `json:"Severity"`
}

// LintResult lint 运行结果
type LintResult struct {
	Issues    []LintIssue `json:"issues"`
	RawJSON   string      `json:"-"`
	Error     string      `json:"error,omitempty"`
	Total     int         `json:"total"`
	LinterMap map[string]int `json:"linter_counts"`
}

// RunLint 在仓库根目录执行 golangci-lint
func RunLint(repoRoot string) (*LintResult, error) {
	args := []string{"run", "--new-from-rev=HEAD~1", "--out-format=json"}

	// 如果存在自定义配置文件，通过 --config 指定
	configPath := filepath.Join(repoRoot, config.LintConfigDir, config.LintConfigFile)
	if _, err := os.Stat(configPath); err == nil {
		args = append(args, "--config", configPath)
	}

	cmd := exec.Command("golangci-lint", args...)
	cmd.Dir = repoRoot

	output, err := cmd.Output()
	rawJSON := string(output)

	if err != nil {
		// golangci-lint 发现问题时退出码非 0，但 stdout 仍有 JSON 结果
		if exitErr, ok := err.(*exec.ExitError); ok {
			// 如果有 JSON 输出，说明是正常的 lint 发现问题
			if len(output) > 0 {
				return parseLintResult(rawJSON)
			}
			return nil, fmt.Errorf("golangci-lint 执行失败: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("golangci-lint 执行失败: %w", err)
	}

	return parseLintResult(rawJSON)
}

// parseLintResult 解析 golangci-lint JSON 输出
func parseLintResult(rawJSON string) (*LintResult, error) {
	result := &LintResult{
		RawJSON:   rawJSON,
		LinterMap: make(map[string]int),
	}

	var issues []LintIssue
	if err := json.Unmarshal([]byte(rawJSON), &issues); err != nil {
		return nil, fmt.Errorf("解析 golangci-lint JSON 输出失败: %w", err)
	}

	result.Issues = issues
	result.Total = len(issues)

	for _, issue := range issues {
		result.LinterMap[issue.FromLinter]++
	}

	return result, nil
}

// LoadLatestResult 从 .livecoding/lint/ 读取最近一次 lint 结果。
// 如果没有找到有效结果，返回 nil。
func LoadLatestResult(repoRoot string) *LintResult {
	lintRoot := filepath.Join(repoRoot, config.LintOutputDir)
	entries, err := os.ReadDir(lintRoot)
	if err != nil {
		return nil
	}

	type dirInfo struct {
		name    string
		modTime time.Time
	}
	dirs := make([]dirInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		dirs = append(dirs, dirInfo{name: entry.Name(), modTime: info.ModTime()})
	}
	if len(dirs) == 0 {
		return nil
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].modTime.After(dirs[j].modTime)
	})

	latestPath := filepath.Join(lintRoot, dirs[0].name, "lint.json")
	data, err := os.ReadFile(latestPath)
	if err != nil {
		return nil
	}

	var result LintResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return &result
}

// WriteResult 将 lint 结果写入指定输出目录
func WriteResult(outputDir string, result *LintResult) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建 lint 输出目录失败: %w", err)
	}

	// 写入 JSON
	jsonPath := filepath.Join(outputDir, "lint.json")
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 lint 结果失败: %w", err)
	}
	if err := os.WriteFile(jsonPath, data, 0600); err != nil {
		return fmt.Errorf("写入 lint.json 失败: %w", err)
	}

	// 写入 Markdown 摘要
	mdPath := filepath.Join(outputDir, "lint.md")
	var sb strings.Builder
	sb.WriteString("# golangci-lint 检查结果\n\n")
	sb.WriteString(fmt.Sprintf("共发现 **%d** 个问题\n\n", result.Total))

	if result.Total > 0 {
		sb.WriteString("## 按 Linter 统计\n\n")
		for linter, count := range result.LinterMap {
			sb.WriteString(fmt.Sprintf("- **%s**: %d 个\n", linter, count))
		}
		sb.WriteString("\n## 问题列表\n\n")
		for i, issue := range result.Issues {
			sb.WriteString(fmt.Sprintf("%d. **%s** (%s)\n   `%s:%d`\n   %s\n\n",
				i+1, issue.FromLinter, issue.Severity,
				issue.Pos.Filename, issue.Pos.Line,
				issue.Text,
			))
		}
	} else {
		sb.WriteString("未发现代码风格问题。\n")
	}

	if err := os.WriteFile(mdPath, []byte(sb.String()), 0600); err != nil {
		return fmt.Errorf("写入 lint.md 失败: %w", err)
	}

	return nil
}
