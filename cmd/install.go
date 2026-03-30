package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var installHooks bool
var installSkills bool

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "安装 git hooks 和同步 skills",
	Long:  "安装 git pre-push/pre-commit/post-commit hook 和同步 skills 到用户目录",
	RunE:  runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().BoolVarP(&installHooks, "hooks", "", true, "安装 git pre-push hook")
	installCmd.Flags().BoolVarP(&installSkills, "skills", "", true, "同步 skills 到 ~/.trae/skills/")
}

func runInstall(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	if installHooks {
		if err := installGitHook(repoRoot); err != nil {
			return err
		}
		if err := installPreCommitHook(repoRoot); err != nil {
			return err
		}
		if err := installPostCommitHook(repoRoot); err != nil {
			return err
		}
	}

	if installSkills {
		if err := syncSkills(); err != nil {
			return err
		}
	}

	color.Green("✓ 安装完成")
	return nil
}

// installGitHook 安装 pre-push hook
func installGitHook(repoRoot string) error {
	color.Cyan("正在安装 git pre-push hook...")

	hooksDir := filepath.Join(repoRoot, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("创建 hooks 目录失败: %w", err)
	}

	hookPath := filepath.Join(hooksDir, "pre-push")
	hookContent := `#!/bin/bash
# coco-ext pre-push hook
# 1. 仅修改 go.mod/go.sum 时跳过所有检查
# 2. 其他情况异步触发 review

BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$BRANCH" = "HEAD" ]; then
    # detached HEAD 状态，跳过
    exit 0
fi

# 检查是否仅修改了 go.mod/go.sum（这些不需要检查 message 和 review）
CHANGES=$(git diff --name-only HEAD~1 HEAD 2>/dev/null | grep -v '^$' | grep -v -E '^(go\.mod|go\.sum|go\.mod\.lock)$' | wc -l)
if [ "$CHANGES" = "0" ]; then
    echo "仅修改 go.mod/go.sum，跳过检查"
    exit 0
fi

# 执行 review（异步模式，不阻塞 push）
COMMIT_ID=$(git rev-parse --short HEAD 2>/dev/null)
LOG_FILE=".livecoding/logs/review-${COMMIT_ID}-${BRANCH}-$(date +%Y%m%d%H%M%S).log"
mkdir -p .livecoding/logs
nohup coco-ext review --async > "$LOG_FILE" 2>&1 < /dev/null &
echo "Review 已触发，请在 .livecoding/review/ 目录查看报告"

exit 0
`

	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		return fmt.Errorf("写入 hook 失败: %w", err)
	}

	color.Green("✓ pre-push hook 已安装")
	return nil
}

// installPostCommitHook 安装 post-commit hook
func installPostCommitHook(repoRoot string) error {
	color.Cyan("正在安装 post-commit hook...")

	hooksDir := filepath.Join(repoRoot, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("创建 hooks 目录失败: %w", err)
	}

	hookPath := filepath.Join(hooksDir, "post-commit")
	hookContent := `#!/bin/bash
# coco-ext post-commit hook
# 1. 烂 commit message 时同步优化 message
# 2. 使用环境变量防止 amend 触发递归
# 3. 输出本次优化耗时

if [ "${COCO_EXT_SKIP_POST_COMMIT:-0}" = "1" ]; then
    exit 0
fi

BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
if [ "$BRANCH" = "HEAD" ]; then
    exit 0
fi

COMMIT_MSG=$(git log -1 --pretty=%B 2>/dev/null | head -n 1 | tr -d '[:space:]')
if [ -n "$COMMIT_MSG" ] && [ ${#COMMIT_MSG} -ge 10 ]; then
    exit 0
fi

ORIGINAL_COMMIT_ID=$(git rev-parse --short HEAD 2>/dev/null)
START_TIME=$(date +%s)

echo "⚠ commit message 太简短，正在优化..."
echo ""
echo "📝 优化进度:"
echo "   [1/3] 生成规范 commit message..."
echo "   [2/3] 调用 AI 生成 message..."

mkdir -p .livecoding/logs
LOG_FILE=".livecoding/logs/gcmsg-${ORIGINAL_COMMIT_ID}-$(date +%Y%m%d%H%M%S).log"

COCO_EXT_SKIP_POST_COMMIT=1 coco-ext gcmsg --amend --changelog --commit-id="$ORIGINAL_COMMIT_ID" 2>&1 | tee "$LOG_FILE"
EXIT_CODE=${PIPESTATUS[0]}
END_TIME=$(date +%s)
ELAPSED_SECONDS=$((END_TIME - START_TIME))

if [ $EXIT_CODE -eq 0 ]; then
    echo "   [3/3] commit message 已更新 ✓"
    echo ""
    echo "⏱ 本次优化耗时: ${ELAPSED_SECONDS}s"
else
    echo ""
    echo "✗ 优化失败，请检查日志: $LOG_FILE"
    echo "⏱ 本次优化耗时: ${ELAPSED_SECONDS}s"
fi

exit 0
`

	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		return fmt.Errorf("写入 hook 失败: %w", err)
	}

	color.Green("✓ post-commit hook 已安装")
	return nil
}

// installPreCommitHook 安装 pre-commit hook
func installPreCommitHook(repoRoot string) error {
	color.Cyan("正在安装 pre-commit hook...")

	// 检测 goimports 是否安装
	if !isCommandAvailable("goimports") {
		color.Yellow("⚠ goimports 未安装，pre-commit hook 将无法工作")
		color.Yellow("  请运行: go install golang.org/x/tools/cmd/goimports@latest")
	} else {
		color.Green("✓ goimports 已安装")
	}

	hooksDir := filepath.Join(repoRoot, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("创建 hooks 目录失败: %w", err)
	}

	hookPath := filepath.Join(hooksDir, "pre-commit")
	hookContent := `#!/bin/bash
# coco-ext pre-commit hook
# 自动格式化已修改的 .go 文件（goimports），保证 import 顺序一致

# 获取暂存区中已修改的 .go 文件
GO_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$' | grep -v '_test\.go$')
if [ -z "$GO_FILES" ]; then
    # 没有暂存的 .go 文件，跳过
    exit 0
fi

# 检查 goimports 是否安装
if ! command -v goimports &> /dev/null; then
    echo "⚠ goimports 未安装，跳过格式化"
    echo "  安装: go install golang.org/x/tools/cmd/goimports@latest"
    exit 0
fi

# 格式化每个文件
echo "$GO_FILES" | while read -r file; do
    if [ -f "$file" ]; then
        goimports -w "$file"
        git add "$file"
        echo "✓ 格式化: $file"
    fi
done

exit 0
`

	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		return fmt.Errorf("写入 hook 失败: %w", err)
	}

	color.Green("✓ pre-commit hook 已安装")
	return nil
}

// isCommandAvailable 检测命令是否可用
func isCommandAvailable(name string) bool {
	cmd := exec.Command("command", "-v", name)
	return cmd.Run() == nil
}

// syncSkills 同步 skills 到 ~/.trae/skills/
func syncSkills() error {
	color.Cyan("正在同步 skills...")

	// 检查仓库是否有 .trae/skills 目录
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	repoSkillsDir := filepath.Join(repoRoot, "skills")
	if _, err := os.Stat(repoSkillsDir); os.IsNotExist(err) {
		color.Yellow("⚠ 仓库中没有 skills/ 目录，跳过 skills 同步")
		return nil
	}

	// 获取用户目录
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %w", err)
	}

	userSkillsDir := filepath.Join(home, ".trae", "skills")
	if err := os.MkdirAll(userSkillsDir, 0755); err != nil {
		return fmt.Errorf("创建用户 skills 目录失败: %w", err)
	}

	// 读取仓库 skills 目录内容
	entries, err := os.ReadDir(repoSkillsDir)
	if err != nil {
		return fmt.Errorf("读取仓库 skills 目录失败: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// 递归复制目录
			src := filepath.Join(repoSkillsDir, entry.Name())
			dst := filepath.Join(userSkillsDir, entry.Name())
			if err := copyDir(src, dst); err != nil {
				color.Yellow("⚠ 复制 %s 失败: %v", entry.Name(), err)
			} else {
				color.Green("✓ 同步 skill: %s", entry.Name())
			}
		} else {
			// 复制文件
			src := filepath.Join(repoSkillsDir, entry.Name())
			dst := filepath.Join(userSkillsDir, entry.Name())
			if err := copyFile(src, dst); err != nil {
				color.Yellow("⚠ 复制 %s 失败: %v", entry.Name(), err)
			} else {
				color.Green("✓ 同步 skill: %s", entry.Name())
			}
		}
	}

	color.Green("✓ skills 同步完成")
	return nil
}

// copyFile 复制单个文件
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// copyDir 递归复制目录
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// getChangedFiles 获取两个 commit 之间变更的文件列表
func getChangedFiles(repoRoot, from, to string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", from, to)
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	result := make([]string, 0, len(files))
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f != "" {
			result = append(result, f)
		}
	}
	return result, nil
}

// uninstall 相关

var uninstallHooks bool
var uninstallSkills bool

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "卸载 git hooks 和 skills",
	Long:  "卸载 git pre-push/pre-commit/post-commit hook 和 skills（仅删除从 coco-ext 安装的部分）",
	RunE:  runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
	uninstallCmd.Flags().BoolVarP(&uninstallHooks, "hooks", "", true, "卸载 git hooks")
	uninstallCmd.Flags().BoolVarP(&uninstallSkills, "skills", "", true, "删除 skills")
}

func runUninstall(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	if uninstallHooks {
		if err := removeGitHook(repoRoot); err != nil {
			return err
		}
		if err := removePreCommitHook(repoRoot); err != nil {
			return err
		}
		if err := removePostCommitHook(repoRoot); err != nil {
			return err
		}
	}

	if uninstallSkills {
		if err := removeSkills(); err != nil {
			return err
		}
	}

	color.Green("✓ 卸载完成")
	return nil
}

func removeGitHook(repoRoot string) error {
	color.Cyan("正在卸载 git pre-push hook...")
	hooksDir := filepath.Join(repoRoot, ".git", "hooks")
	hookPath := filepath.Join(hooksDir, "pre-push")
	if _, err := os.Stat(hookPath); err == nil {
		if err := os.Remove(hookPath); err != nil {
			return fmt.Errorf("删除 pre-push hook 失败: %w", err)
		}
		color.Green("✓ pre-push hook 已卸载")
	} else {
		color.Yellow("⚠ pre-push hook 不存在，跳过")
	}
	return nil
}

func removePreCommitHook(repoRoot string) error {
	color.Cyan("正在卸载 pre-commit hook...")
	hooksDir := filepath.Join(repoRoot, ".git", "hooks")
	hookPath := filepath.Join(hooksDir, "pre-commit")
	if _, err := os.Stat(hookPath); err == nil {
		if err := os.Remove(hookPath); err != nil {
			return fmt.Errorf("删除 pre-commit hook 失败: %w", err)
		}
		color.Green("✓ pre-commit hook 已卸载")
	} else {
		color.Yellow("⚠ pre-commit hook 不存在，跳过")
	}
	return nil
}

func removePostCommitHook(repoRoot string) error {
	color.Cyan("正在卸载 post-commit hook...")
	hooksDir := filepath.Join(repoRoot, ".git", "hooks")
	hookPath := filepath.Join(hooksDir, "post-commit")
	if _, err := os.Stat(hookPath); err == nil {
		if err := os.Remove(hookPath); err != nil {
			return fmt.Errorf("删除 post-commit hook 失败: %w", err)
		}
		color.Green("✓ post-commit hook 已卸载")
	} else {
		color.Yellow("⚠ post-commit hook 不存在，跳过")
	}
	return nil
}

func removeSkills() error {
	color.Cyan("正在卸载 skills...")

	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	repoSkillsDir := filepath.Join(repoRoot, "skills")
	_, err = os.Stat(repoSkillsDir)
	if os.IsNotExist(err) {
		color.Yellow("⚠ 仓库中没有 skills/ 目录，无需卸载")
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %w", err)
	}

	userSkillsDir := filepath.Join(home, ".trae", "skills")
	if _, err := os.Stat(userSkillsDir); os.IsNotExist(err) {
		color.Yellow("⚠ 用户 skills 目录不存在，跳过")
		return nil
	}

	entries, err := os.ReadDir(repoSkillsDir)
	if err != nil {
		return fmt.Errorf("读取仓库 skills 目录失败: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			targetPath := filepath.Join(userSkillsDir, entry.Name())
			if _, err := os.Stat(targetPath); err == nil {
				if err := os.RemoveAll(targetPath); err != nil {
					color.Yellow("⚠ 删除 %s 失败: %v", entry.Name(), err)
				} else {
					color.Green("✓ 删除 skill: %s", entry.Name())
				}
			} else {
				color.Yellow("⚠ %s 不存在，跳过", entry.Name())
			}
		}
	}

	color.Green("✓ skills 卸载完成")
	return nil
}
