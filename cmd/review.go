package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/config"
	"github.com/DreamCats/coco-ext/internal/generator"
	"github.com/DreamCats/coco-ext/internal/gitlab"
	"github.com/DreamCats/coco-ext/internal/review"
)

var reviewMRID int
var reviewAsync bool
var reviewOutputDir string

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "AI Code Review",
	Long:  "对 GitLab MR 进行 AI Code Review，支持手动触发和 Webhook 异步模式。",
	RunE:  runReview,
}

func init() {
	rootCmd.AddCommand(reviewCmd)
	reviewCmd.Flags().IntVarP(&reviewMRID, "mr", "", 0, "GitLab MR ID")
	reviewCmd.Flags().BoolVarP(&reviewAsync, "async", "", false, "异步模式，不等待结果立即返回")
	reviewCmd.Flags().StringVarP(&reviewOutputDir, "output", "", "", "自定义输出目录")
}

func runReview(cmd *cobra.Command, args []string) error {
	if reviewMRID == 0 {
		return fmt.Errorf("必须指定 MR ID: --mr <id>")
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	// 获取 MR 信息和 diff
	color.Cyan("正在获取 MR #%d 的信息...", reviewMRID)

	mrInfo, diff, err := gitlab.GetMRDiff(repoRoot, reviewMRID)
	if err != nil {
		return fmt.Errorf("获取 MR 信息失败: %w", err)
	}

	color.Cyan("MR #%d: %s", reviewMRID, mrInfo.Title)
	color.Cyan("作者: %s | 分支: %s -> %s", mrInfo.Author, mrInfo.SourceBranch, mrInfo.TargetBranch)
	color.Cyan("文件变更: +%d -%d", mrInfo.Additions, mrInfo.Deletions)

	// 确定输出目录
	outputDir := reviewOutputDir
	if outputDir == "" {
		outputDir = filepath.Join(repoRoot, config.ReviewOutputDir, fmt.Sprintf("mr-%d", reviewMRID))
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 保存 diff 到临时文件
	diffPath := filepath.Join(outputDir, "diff.patch")
	if err := os.WriteFile(diffPath, []byte(diff), 0600); err != nil {
		return fmt.Errorf("保存 diff 失败: %w", err)
	}

	// 写入 MR 元信息
	metaPath := filepath.Join(outputDir, "meta.json")
	if err := mrInfo.WriteMeta(metaPath); err != nil {
		return fmt.Errorf("写入元信息失败: %w", err)
	}

	// 连接 coco daemon 执行 review
	color.Cyan("正在连接 coco daemon 进行 Code Review...")

	gen, err := generator.New(repoRoot)
	if err != nil {
		return fmt.Errorf("连接 coco daemon 失败: %w", err)
	}
	defer gen.Close()

	// 生成 review 报告
	reportPath := filepath.Join(outputDir, "report.md")
	reportContent, err := review.GenerateReport(gen, mrInfo, diff, func(chunk string) {
		fmt.Print(chunk)
	})
	if err != nil {
		color.Red("Review 生成失败: %v", err)
		return err
	}

	// 保存报告
	if err := os.WriteFile(reportPath, []byte(reportContent), 0600); err != nil {
		return fmt.Errorf("保存报告失败: %w", err)
	}

	color.Green("\n✓ Review 完成!")
	color.Green("报告已生成: %s", reportPath)

	return nil
}
