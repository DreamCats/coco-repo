package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/gcmsg"
)

var gcmsgAmend bool

var gcmsgCmd = &cobra.Command{
	Use:   "gcmsg",
	Short: "生成 commit message",
	Long:  "根据代码变更生成符合规范的 commit message，支持 --amend 覆盖上一个 commit",
	RunE:  runGcmsg,
}

func init() {
	rootCmd.AddCommand(gcmsgCmd)
	gcmsgCmd.Flags().BoolVarP(&gcmsgAmend, "amend", "", false, "自动 amend 到上一个 commit")
}

func runGcmsg(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	// 获取当前 commit 的 diff
	color.Cyan("正在获取代码变更...")
	diff, err := getCurrentCommitDiff(repoRoot)
	if err != nil {
		return fmt.Errorf("获取 diff 失败: %w", err)
	}

	// 生成 commit message
	color.Cyan("正在生成 commit message...")
	newMsg, err := gcmsg.GenerateCommitMsg(repoRoot, diff)
	if err != nil {
		color.Red("生成失败: %v", err)
		return err
	}

	// 打印生成的消息
	fmt.Println("\n生成的 commit message:")
	color.Green("---")
	fmt.Println(newMsg)
	color.Green("---\n")

	if gcmsgAmend {
		color.Cyan("正在执行 amend...")
		if err := amendCommit(repoRoot, newMsg); err != nil {
			color.Red("Amend 失败: %v", err)
			return err
		}
		color.Green("✓ commit message 已更新")
	}

	return nil
}

// getCurrentCommitDiff 获取当前分支最新 commit 的 diff
func getCurrentCommitDiff(repoRoot string) (string, error) {
	cmd := exec.Command("git", "diff", "HEAD~1", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// amendCommit 用新 message 覆盖上一个 commit
func amendCommit(repoRoot, newMsg string) error {
	// 使用 git commit --amend
	cmd := exec.Command("git", "commit", "--amend", "-m", newMsg)
	cmd.Dir = repoRoot
	_, err := cmd.Output()
	if err != nil {
		return err
	}
	return nil
}

// cleanCommitMsg 清理 commit message（去掉多余空白）
func cleanCommitMsg(msg string) string {
	lines := strings.Split(msg, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}
	return strings.Join(cleanLines, "\n")
}