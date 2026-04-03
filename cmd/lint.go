package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/config"
	internallint "github.com/DreamCats/coco-ext/internal/lint"
)

var lintAsync bool
var lintJSON bool

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "运行 golangci-lint 代码风格检查",
	Long:  "基于 golangci-lint 对代码变更进行风格检查，结果输出到 .livecoding/lint/ 目录。",
	RunE:  runLint,
}

func init() {
	rootCmd.AddCommand(lintCmd)
	lintCmd.Flags().BoolVarP(&lintAsync, "async", "", false, "异步模式，不等待结果立即返回")
	lintCmd.Flags().BoolVarP(&lintJSON, "json", "", false, "输出结构化 JSON 结果")
}

func runLint(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	if !internallint.IsGolangciLintAvailable() {
		return fmt.Errorf("golangci-lint 未安装，请运行: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest")
	}

	if lintAsync {
		return startLintAsync(repoRoot)
	}

	return runLintSync(repoRoot)
}

func runLintSync(repoRoot string) error {
	startedAt := time.Now()
	color.Cyan("正在运行 golangci-lint...")

	result, err := internallint.RunLint(repoRoot)
	if err != nil {
		return fmt.Errorf("lint 执行失败: %w", err)
	}

	// 确定输出目录
	dirName := buildLintDirName(repoRoot)
	outputDir := filepath.Join(repoRoot, config.LintOutputDir, dirName)

	if err := internallint.WriteResult(outputDir, result); err != nil {
		return fmt.Errorf("写入 lint 结果失败: %w", err)
	}

	elapsed := time.Since(startedAt)

	if lintJSON {
		fmt.Println(result.RawJSON)
		return nil
	}

	color.Green("\n✓ Lint 完成! (%s)", elapsed.Round(time.Second))
	color.Green("结果目录: %s", outputDir)
	if result.Total > 0 {
		color.Yellow("发现 %d 个问题:", result.Total)
		for linter, count := range result.LinterMap {
			color.Yellow("  - %s: %d", linter, count)
		}
	} else {
		color.Green("未发现代码风格问题")
	}

	return nil
}

func startLintAsync(repoRoot string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	logDir := filepath.Join(repoRoot, ".livecoding", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	logFileName := fmt.Sprintf("lint-%s.log", time.Now().Format("20060102150405"))
	logPath := filepath.Join(logDir, logFileName)

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %w", err)
	}
	defer logFile.Close()

	lintArgs := []string{"lint"}
	if lintJSON {
		lintArgs = append(lintArgs, "--json")
	}

	cmd := exec.Command(exe, lintArgs...)
	cmd.Dir = repoRoot
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动异步 lint 失败: %w", err)
	}

	color.Green("Lint 已在后台启动")
	color.Green("日志: %s", logPath)
	color.Green("结果目录: %s", filepath.Join(repoRoot, config.LintOutputDir))

	return nil
}

func buildLintDirName(repoRoot string) string {
	branchName := getGitBranchName(repoRoot)
	commitID := getGitShortCommitID(repoRoot)
	return fmt.Sprintf("%s-%s", branchName, commitID)
}

func getGitBranchName(repoRoot string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "current"
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "current"
	}
	return filepath.Base(branch)
}

func getGitShortCommitID(repoRoot string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}
