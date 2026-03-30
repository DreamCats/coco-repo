package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/config"
	"github.com/DreamCats/coco-ext/internal/generator"
	"github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/review"
)

var reviewBase string
var reviewAsync bool
var reviewOutputDir string
var reviewFull bool

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "AI Code Review",
	Long:  "对本地代码变更进行 AI Code Review，基于 git diff。",
	RunE:  runReview,
}

func init() {
	rootCmd.AddCommand(reviewCmd)
	reviewCmd.Flags().StringVarP(&reviewBase, "base", "", "", "对比的基准分支（如 main），默认自动检测")
	reviewCmd.Flags().BoolVarP(&reviewAsync, "async", "", false, "异步模式，不等待结果立即返回")
	reviewCmd.Flags().StringVarP(&reviewOutputDir, "output", "", "", "自定义输出目录")
	reviewCmd.Flags().BoolVarP(&reviewFull, "full", "", false, "分析分支整体 diff（默认只分析最后一个 commit）")
}

func runReview(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	// 获取 git diff 信息
	color.Cyan("正在获取代码变更信息...")

	diffInfo, err := git.GetDiffInfo(repoRoot, reviewBase, reviewFull)
	if err != nil {
		return fmt.Errorf("获取代码变更失败: %w", err)
	}

	if diffInfo.IsEmpty() {
		color.Yellow("没有检测到代码变更，无需 review")
		return nil
	}

	color.Cyan("分支: %s → %s", diffInfo.SourceBranch, diffInfo.TargetBranch)
	color.Cyan("提交: %s", diffInfo.CommitMessage)
	color.Cyan("作者: %s", diffInfo.Author)
	color.Cyan("文件变更: +%d -%d，%d 个文件", diffInfo.Additions, diffInfo.Deletions, diffInfo.FileCount)

	// 确定输出目录
	branchSlug := filepath.Base(diffInfo.SourceBranch)
	if branchSlug == "" {
		branchSlug = "current"
	}
	// 格式：分支名-短commitid
	dirName := fmt.Sprintf("%s-%s", branchSlug, diffInfo.CommitID)
	outputDir := reviewOutputDir
	if outputDir == "" {
		outputDir = filepath.Join(repoRoot, config.ReviewOutputDir, dirName)
	}

	if reviewAsync {
		return startReviewAsync(repoRoot, outputDir)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 保存 diff
	diffPath := filepath.Join(outputDir, "diff.patch")
	if err := os.WriteFile(diffPath, []byte(diffInfo.Diff), 0600); err != nil {
		return fmt.Errorf("保存 diff 失败: %w", err)
	}

	// 保存元信息
	metaPath := filepath.Join(outputDir, "meta.json")
	if err := diffInfo.WriteMeta(metaPath); err != nil {
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
	reportContent, err := review.GenerateReport(gen, diffInfo, func(chunk string) {
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

func startReviewAsync(repoRoot, outputDir string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	logDir := filepath.Join(repoRoot, ".livecoding", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	logFileName := fmt.Sprintf("review-%s.log", time.Now().Format("20060102150405"))
	logPath := filepath.Join(logDir, logFileName)

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %w", err)
	}
	defer logFile.Close()

	reviewArgs := []string{"review"}
	if reviewBase != "" {
		reviewArgs = append(reviewArgs, "--base", reviewBase)
	}
	if reviewFull {
		reviewArgs = append(reviewArgs, "--full")
	}
	reviewArgs = append(reviewArgs, "--output", outputDir)

	reviewCmd := exec.Command(exe, reviewArgs...)
	reviewCmd.Dir = repoRoot
	reviewCmd.Stdin = nil
	reviewCmd.Stdout = logFile
	reviewCmd.Stderr = logFile
	reviewCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := reviewCmd.Start(); err != nil {
		return fmt.Errorf("启动异步 review 失败: %w", err)
	}

	color.Green("Review 已在后台启动")
	color.Green("日志: %s", logPath)
	color.Green("报告目录: %s", outputDir)

	return nil
}
