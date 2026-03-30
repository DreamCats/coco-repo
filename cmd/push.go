package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:                "push [git-push-args...]",
	Short:              "包装 git push，成功后后台触发 review",
	Long:               "执行 git push；当 push 成功后，后台触发 coco-ext review --async。",
	DisableFlagParsing: true,
	RunE:               runPush,
}

func init() {
	rootCmd.AddCommand(pushCmd)
}

func runPush(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	gitArgs := append([]string{"push"}, args...)
	gitPushCmd := exec.Command("git", gitArgs...)
	gitPushCmd.Dir = repoRoot
	gitPushCmd.Stdin = os.Stdin
	gitPushCmd.Stdout = os.Stdout
	gitPushCmd.Stderr = os.Stderr

	if err := gitPushCmd.Run(); err != nil {
		return err
	}

	color.Green("Push 成功，正在后台启动 review...")

	prevLowPriority := reviewLowPriority
	reviewLowPriority = true
	defer func() {
		reviewLowPriority = prevLowPriority
	}()

	if err := startReviewAsync(repoRoot, ""); err != nil {
		return fmt.Errorf("push 成功，但启动后台 review 失败: %w", err)
	}

	return nil
}
