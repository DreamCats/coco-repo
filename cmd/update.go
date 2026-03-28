package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/config"
	"github.com/DreamCats/coco-ext/internal/generator"
	"github.com/DreamCats/coco-ext/internal/knowledge"
)

var commitRange string

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "增量更新知识库",
	Long:  "读取最近 commit 的 diff，判断哪些知识文件受影响，局部更新受影响的段落。",
	RunE:  runUpdate,
}

func init() {
	contextCmd.AddCommand(updateCmd)
	updateCmd.Flags().StringVarP(&commitRange, "range", "r", "HEAD~1..HEAD", "commit 范围（默认最近一次提交）")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	// 检查 .livecoding/context/ 是否存在
	contextDir := filepath.Join(repoRoot, config.ContextDir)
	if _, err := os.Stat(contextDir); err != nil {
		return fmt.Errorf(".livecoding/context/ 不存在，请先运行 coco-ext init")
	}

	// 1. 获取 git diff
	color.Cyan("正在获取 git diff (%s)...", commitRange)
	diffOutput, err := exec.Command("git", "diff", commitRange).Output()
	if err != nil {
		return fmt.Errorf("获取 git diff 失败: %w", err)
	}
	diffContent := string(diffOutput)
	if strings.TrimSpace(diffContent) == "" {
		color.Green("没有代码变更，无需更新")
		return nil
	}

	// 2. 读取已有知识文件
	existingFiles, err := knowledge.ReadAll(repoRoot)
	if err != nil {
		return err
	}

	// 3. 连接 coco daemon
	color.Cyan("正在连接 coco daemon...")
	gen, err := generator.New(repoRoot)
	if err != nil {
		return err
	}
	defer gen.Close()

	// 4. 逐个检查并更新
	updated := 0
	for name := range config.KnowledgeFiles {
		existing, ok := existingFiles[name]
		if !ok {
			color.Yellow("跳过 %s（文件不存在）", name)
			continue
		}

		color.Cyan("正在检查 %s...", name)
		newContent, err := gen.Update(name, existing, diffContent, nil)
		if err != nil {
			color.Red("更新 %s 失败: %v", name, err)
			continue
		}
		if newContent == "" {
			fmt.Printf("  %s 无需更新\n", name)
			continue
		}

		if err := knowledge.WriteFile(repoRoot, name, newContent); err != nil {
			color.Red("写入 %s 失败: %v", name, err)
			continue
		}
		color.Green("✓ %s 已更新", name)
		updated++
	}

	fmt.Println()
	if updated > 0 {
		color.Green("增量更新完成，更新了 %d 个文件", updated)
	} else {
		color.Green("所有知识文件均为最新")
	}

	return nil
}
