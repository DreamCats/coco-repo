package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/knowledge"
)

var queryFile string

var queryCmd = &cobra.Command{
	Use:   "query [关键词]",
	Short: "查询知识库",
	Long:  "在 .livecoding/context/ 知识文件中搜索匹配内容，返回术语定义、代码位置、相关模式等。可作为 MCP 工具供 coco 调用。",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runQuery,
}

func init() {
	contextCmd.AddCommand(queryCmd)
	queryCmd.Flags().StringVar(&queryFile, "file", "", "指定查询的知识文件（如 glossary.md）")
}

func runQuery(cmd *cobra.Command, args []string) error {
	keyword := strings.Join(args, " ")

	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	files, err := knowledge.ReadAll(repoRoot)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf(".livecoding/context/ 为空，请先运行 coco-ext init")
	}

	found := false
	for name, content := range files {
		if queryFile != "" && name != queryFile {
			continue
		}

		matches := searchInContent(content, keyword)
		if len(matches) == 0 {
			continue
		}

		found = true
		color.Cyan("── %s ──", name)
		for _, line := range matches {
			// 高亮关键词
			highlighted := strings.ReplaceAll(line, keyword, color.YellowString(keyword))
			fmt.Println(highlighted)
		}
		fmt.Println()
	}

	if !found {
		fmt.Printf("未找到与 \"%s\" 相关的内容\n", keyword)
	}

	return nil
}

// searchInContent 在内容中搜索包含关键词的行（及上下文）
func searchInContent(content, keyword string) []string {
	lines := strings.Split(content, "\n")
	var matches []string
	lowerKeyword := strings.ToLower(keyword)

	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), lowerKeyword) {
			// 包含前一行作为上下文（通常是表头或标题）
			if i > 0 && len(matches) == 0 {
				matches = append(matches, lines[i-1])
			}
			matches = append(matches, line)
		}
	}

	return matches
}
