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
	Long:  "安装 git pre-push hook 和同步 skills 到用户目录",
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
# 2. 烂 commit message 时阻塞 push 并自动优化
# 3. 其他情况异步触发 review

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

# 获取最新 commit 的 message，烂 message 时阻塞 push
COMMIT_MSG=$(git log -1 --pretty=%B 2>/dev/null | head -n 1 | tr -d '[:space:]')
if [ -z "$COMMIT_MSG" ] || [ ${#COMMIT_MSG} -lt 10 ]; then
    echo "⚠ commit message 太简短，正在生成更好的 message..."
    if coco-ext gcmsg --amend; then
        echo "✓ commit message 已优化"
    else
        echo "✗ commit message 优化失败，将使用原 message 推送"
    fi
fi

# 执行 review（异步模式，不阻塞 push）
coco-ext review --async 2>/dev/null &
echo "Review 已触发，请在 .livecoding/review/ 目录查看报告"

exit 0
`

	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		return fmt.Errorf("写入 hook 失败: %w", err)
	}

	color.Green("✓ pre-push hook 已安装")
	return nil
}

// installPreCommitHook 安装 pre-commit hook
func installPreCommitHook(repoRoot string) error {
	color.Cyan("正在安装 pre-commit hook...")

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
