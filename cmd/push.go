package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	internallint "github.com/DreamCats/coco-ext/internal/lint"
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

	return triggerPushFlow(repoRoot, args)
}

func triggerPushFlow(repoRoot string, args []string) error {
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

	// golangci-lint 可用时，异步触发 lint
	if internallint.IsGolangciLintAvailable() {
		if err := startLintAsync(repoRoot); err != nil {
			color.Yellow("⚠ 后台 lint 启动失败: %v", err)
		}
	}

	return nil
}
