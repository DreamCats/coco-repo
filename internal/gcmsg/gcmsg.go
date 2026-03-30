package gcmsg

import (
	"fmt"
	"strings"

	"github.com/DreamCats/coco-ext/internal/generator"
)

const gcmsgPrompt = `你是一个 commit message 生成专家。根据以下代码 diff，生成一个符合规范的 commit message。

要求：
1. 第一行是标题，不超过 72 字符，简明扼要
2. 第二行空
3. 后续是详细说明（可选）
4. 使用中文
5. 遵循 conventional commits 规范（feat/fix/docs/refactor/test/chore 等前缀）

代码 diff：
%s

生成的 commit message：`

// GenerateCommitMsg 根据 diff 生成 commit message
func GenerateCommitMsg(repoRoot, diff string) (string, error) {
	gen, err := generator.New(repoRoot)
	if err != nil {
		return "", fmt.Errorf("连接 coco daemon 失败: %w", err)
	}
	defer gen.Close()

	prompt := fmt.Sprintf(gcmsgPrompt, diff)

	var result strings.Builder
	_, err = gen.Prompt(
		prompt,
		func(text string) {
			result.WriteString(text)
		},
	)
	if err != nil {
		return "", fmt.Errorf("生成失败: %w", err)
	}

	msg, err := extractCommitMessage(result.String())
	if err != nil {
		return "", err
	}

	return msg, nil
}

func extractCommitMessage(raw string) (string, error) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return "", fmt.Errorf("生成失败: 模型未返回 commit message")
	}

	candidates := extractFencedBlocks(normalized)
	candidates = append(candidates, normalized)

	for _, candidate := range candidates {
		msg := extractFromCandidate(candidate)
		if msg != "" {
			return msg, nil
		}
	}

	return "", fmt.Errorf("生成失败: 未提取到合法的 conventional commit message")
}

func extractFencedBlocks(text string) []string {
	parts := strings.Split(text, "```")
	if len(parts) < 2 {
		return nil
	}

	blocks := make([]string, 0, len(parts)/2)
	for i := 1; i < len(parts); i += 2 {
		block := strings.TrimSpace(parts[i])
		if block == "" {
			continue
		}
		blocks = append(blocks, stripFenceLanguage(block))
	}
	return blocks
}

func stripFenceLanguage(block string) string {
	lines := strings.Split(block, "\n")
	if len(lines) == 0 {
		return ""
	}

	firstLine := strings.TrimSpace(lines[0])
	switch firstLine {
	case "git", "markdown", "md", "text":
		return strings.TrimSpace(strings.Join(lines[1:], "\n"))
	default:
		return strings.TrimSpace(block)
	}
}

func extractFromCandidate(candidate string) string {
	lines := strings.Split(stripFenceLanguage(strings.TrimSpace(candidate)), "\n")
	titleIndex := -1
	for i, line := range lines {
		if isConventionalCommitTitle(strings.TrimSpace(line)) {
			titleIndex = i
			break
		}
	}
	if titleIndex == -1 {
		return ""
	}

	selected := make([]string, 0, len(lines)-titleIndex)
	for _, line := range lines[titleIndex:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "```" {
			break
		}
		selected = append(selected, strings.TrimRight(line, " \t"))
	}

	return strings.TrimSpace(strings.Join(selected, "\n"))
}

func isConventionalCommitTitle(line string) bool {
	if line == "" {
		return false
	}

	separatorIndex := strings.Index(line, ": ")
	if separatorIndex == -1 {
		return false
	}

	prefix := line[:separatorIndex]
	if strings.HasSuffix(prefix, "!") {
		prefix = strings.TrimSuffix(prefix, "!")
	}

	scopeStart := strings.Index(prefix, "(")
	scopeEnd := strings.LastIndex(prefix, ")")
	if scopeStart != -1 || scopeEnd != -1 {
		if scopeStart == -1 || scopeEnd != len(prefix)-1 || scopeStart >= scopeEnd {
			return false
		}
		prefix = prefix[:scopeStart]
	}

	switch prefix {
	case "feat", "fix", "docs", "refactor", "test", "chore", "style", "perf", "build", "ci", "revert":
		return true
	default:
		return false
	}
}
