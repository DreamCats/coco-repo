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

	msg := strings.TrimSpace(result.String())

	// 清理可能的 markdown 代码块标记
	msg = strings.TrimPrefix(msg, "```")
	msg = strings.TrimPrefix(msg, "git\n")
	msg = strings.TrimPrefix(msg, "markdown\n")
	msg = strings.TrimSuffix(msg, "```")
	msg = strings.TrimSpace(msg)

	return msg, nil
}