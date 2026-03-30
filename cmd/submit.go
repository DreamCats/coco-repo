package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	internalgcmsg "github.com/DreamCats/coco-ext/internal/gcmsg"
	"github.com/DreamCats/coco-ext/internal/git"
)

var submitCmd = &cobra.Command{
	Use:   "submit [message]",
	Short: "基于 staged 变更自动生成 message、commit 并 push",
	Long:  "仅处理已 staged 的变更。优先使用用户提供的高质量 message，其次使用 AI 生成，失败时自动使用本地兜底 message，然后执行 git commit 和 coco-ext push。",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSubmit,
}

func init() {
	rootCmd.AddCommand(submitCmd)
}

func runSubmit(cmd *cobra.Command, args []string) error {
	startedAt := time.Now()
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}
	if !git.IsGitRepo(repoRoot) {
		return fmt.Errorf("当前目录不是 git 仓库")
	}

	hasStaged, err := hasStagedChanges(repoRoot)
	if err != nil {
		return fmt.Errorf("检查暂存区失败: %w", err)
	}
	if !hasStaged {
		return fmt.Errorf("未检测到 staged 变更，请先执行 git add")
	}

	color.Cyan("📝 提交流程:")
	color.Cyan("   [1/3] 生成 commit message...")

	message, source, messageElapsed, err := resolveSubmitMessage(repoRoot, args)
	if err != nil {
		return err
	}
	color.Green("   [1/3] commit message 已生成 ✓")
	color.Cyan("⏱ commit message 生成耗时: %s", formatDurationSeconds(messageElapsed))

	color.Cyan("   [2/3] 执行 commit...")
	color.Cyan("正在执行 commit...")
	if err := commitWithMessage(repoRoot, message); err != nil {
		return fmt.Errorf("git commit 失败: %w", err)
	}

	commitID, err := getShortCommitID(repoRoot)
	if err != nil {
		commitID = "unknown"
	}

	color.Green("✓ commit 成功: %s", commitID)
	color.Green("✓ commit message 来源: %s", source)

	color.Cyan("   [3/3] 执行 push 并在成功后后台触发 review...")
	if err := triggerPushFlow(repoRoot, nil); err != nil {
		return err
	}

	color.Green("⏱ 本次 submit 总耗时: %s", formatDurationSeconds(time.Since(startedAt)))
	return nil
}

func hasStagedChanges(repoRoot string) (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet", "--exit-code")
	cmd.Dir = repoRoot
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	var exitErr *exec.ExitError
	if ok := asExitError(err, &exitErr); ok && exitErr.ExitCode() == 1 {
		return true, nil
	}
	return false, err
}

func asExitError(err error, target **exec.ExitError) bool {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return false
	}
	*target = exitErr
	return true
}

func resolveSubmitMessage(repoRoot string, args []string) (string, string, time.Duration, error) {
	startedAt := time.Now()

	if len(args) > 0 {
		userMsg := strings.TrimSpace(args[0])
		if isHighQualityCommitMessage(userMsg) {
			color.Cyan("检测到用户提供了可直接使用的 commit message")
			return userMsg, "user", time.Since(startedAt), nil
		}
		color.Yellow("⚠ 用户提供的 commit message 过短或不规范，自动改用 AI / 本地兜底生成")
	}

	color.Cyan("      - 正在获取 staged 代码变更...")
	diff, err := getStagedDiff(repoRoot)
	if err != nil {
		return "", "", 0, fmt.Errorf("获取 staged diff 失败: %w", err)
	}

	color.Cyan("      - 正在调用 AI 生成 commit message...")
	message, err := internalgcmsg.GenerateCommitMsg(repoRoot, diff)
	if err == nil {
		return message, "ai", time.Since(startedAt), nil
	}

	color.Yellow("⚠ AI 生成失败，使用本地兜底 message: %v", err)
	color.Cyan("      - 正在生成本地兜底 commit message...")
	message, fallbackErr := buildFallbackCommitMsgWithMode(repoRoot, true)
	if fallbackErr != nil {
		return "", "", 0, fmt.Errorf("生成 commit message 失败: %w", fallbackErr)
	}
	return message, "fallback", time.Since(startedAt), nil
}

func isHighQualityCommitMessage(msg string) bool {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return false
	}
	firstLine := msg
	if newline := strings.Index(msg, "\n"); newline >= 0 {
		firstLine = msg[:newline]
	}
	firstLine = strings.TrimSpace(firstLine)
	if len(firstLine) < 10 {
		return false
	}
	if strings.EqualFold(firstLine, ".") || strings.EqualFold(firstLine, "tt") {
		return false
	}
	switch firstLine {
	case "wip", "fix", "test", "tmp", "temp":
		return false
	default:
		return true
	}
}

func commitWithMessage(repoRoot, message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func formatDurationSeconds(d time.Duration) string {
	if d < time.Second {
		return d.Round(100 * time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}
