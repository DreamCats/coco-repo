package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var agentsForce bool

const cocoExtSectionStartMarker = "<!-- coco-ext-agents:start -->"
const cocoExtSectionEndMarker = "<!-- coco-ext-agents:end -->"
const cocoExtSectionMarker = "## AI 编码协作准则"

//go:embed templates/agents-section.md
var agentsSectionTemplate string

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "生成 AGENTS.md 行为约束文件",
	Long:  "在仓库根目录生成或更新 AGENTS.md，定义 AI 编码时的行为约束准则。",
	RunE:  runAgents,
}

func init() {
	rootCmd.AddCommand(agentsCmd)
	agentsCmd.Flags().BoolVarP(&agentsForce, "force", "f", false, "强制替换已有的 Coco-Ext 约束 section")
}

func runAgents(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err != nil {
		return fmt.Errorf("当前目录不是 git 仓库: %s", repoRoot)
	}

	agentsPath := filepath.Join(repoRoot, "AGENTS.md")

	existingContent, err := os.ReadFile(agentsPath)
	hasExisting := false
	if err == nil {
		hasExisting = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("读取 AGENTS.md 失败: %w", err)
	}

	newSection := renderAgentsSection()

	finalContent, action, changed := mergeAgentsContent(string(existingContent), hasExisting, newSection, agentsForce)
	if !changed {
		color.Green("✓ AGENTS.md 已是最新，无需更新")
		color.Yellow("提示：请 review 文件内容，按需调整约束条款")
		return nil
	}

	if err := os.WriteFile(agentsPath, []byte(finalContent), 0600); err != nil {
		return fmt.Errorf("写入 AGENTS.md 失败: %w", err)
	}

	color.Green("✓ AGENTS.md %s: %s", action, agentsPath)
	color.Yellow("提示：请 review 文件内容，按需调整约束条款")

	return nil
}

func renderAgentsSection() string {
	body := strings.TrimSpace(agentsSectionTemplate)
	return strings.Join([]string{
		cocoExtSectionStartMarker,
		body,
		cocoExtSectionEndMarker,
	}, "\n")
}

func mergeAgentsContent(existing string, hasExisting bool, newSection string, force bool) (string, string, bool) {
	if !hasExisting {
		header := `# AGENTS.md

本文件定义了 AI 编码时的行为约束准则，所有 AI 协作必须遵守。`
		finalContent := strings.TrimSpace(header) + "\n\n" + newSection + "\n"
		return finalContent, "已生成", true
	}

	if start, end, ok := findManagedSection(existing); ok {
		finalContent := replaceSection(existing, start, end, newSection)
		if !force && normalizeAgentsContent(finalContent) == normalizeAgentsContent(existing) {
			return existing, "已存在", false
		}
		return finalContent, "已更新，替换了 Coco-Ext 行为约束", true
	}

	if start, end, ok := findLegacySection(existing); ok {
		finalContent := replaceSection(existing, start, end, newSection)
		if !force && normalizeAgentsContent(finalContent) == normalizeAgentsContent(existing) {
			return existing, "已存在", false
		}
		return finalContent, "已更新，替换了 Coco-Ext 行为约束", true
	}

	finalContent := appendSection(existing, newSection)
	if !force && normalizeAgentsContent(finalContent) == normalizeAgentsContent(existing) {
		return existing, "已存在", false
	}
	return finalContent, "已更新，追加了 Coco-Ext 行为约束", true
}

func findManagedSection(content string) (int, int, bool) {
	start := strings.Index(content, cocoExtSectionStartMarker)
	if start == -1 {
		return 0, 0, false
	}

	endMarkerIndex := strings.Index(content[start:], cocoExtSectionEndMarker)
	if endMarkerIndex == -1 {
		return 0, 0, false
	}

	end := start + endMarkerIndex + len(cocoExtSectionEndMarker)
	for end < len(content) && (content[end] == '\n' || content[end] == '\r') {
		end++
	}
	return start, end, true
}

func findLegacySection(content string) (int, int, bool) {
	start := strings.Index(content, cocoExtSectionMarker)
	if start == -1 {
		return 0, 0, false
	}

	searchFrom := start + len(cocoExtSectionMarker)
	nextSectionOffset := strings.Index(content[searchFrom:], "\n## ")
	nextHeaderOffset := strings.Index(content[searchFrom:], "\n# ")

	end := len(content)
	switch {
	case nextSectionOffset != -1 && nextHeaderOffset != -1:
		if nextSectionOffset < nextHeaderOffset {
			end = searchFrom + nextSectionOffset + 1
		} else {
			end = searchFrom + nextHeaderOffset + 1
		}
	case nextSectionOffset != -1:
		end = searchFrom + nextSectionOffset + 1
	case nextHeaderOffset != -1:
		end = searchFrom + nextHeaderOffset + 1
	}

	for end < len(content) && (content[end] == '\n' || content[end] == '\r') {
		end++
	}
	return start, end, true
}

func replaceSection(content string, start, end int, newSection string) string {
	prefix := strings.TrimRight(content[:start], "\n")
	suffix := strings.TrimLeft(content[end:], "\n")

	switch {
	case prefix == "" && suffix == "":
		return newSection + "\n"
	case prefix == "":
		return newSection + "\n\n" + suffix + "\n"
	case suffix == "":
		return prefix + "\n\n" + newSection + "\n"
	default:
		return prefix + "\n\n" + newSection + "\n\n" + suffix + "\n"
	}
}

func appendSection(content, newSection string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return newSection + "\n"
	}
	return trimmed + "\n\n" + newSection + "\n"
}

func normalizeAgentsContent(content string) string {
	return strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
}
