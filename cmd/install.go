package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var installHooks bool
var installSkills bool

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "安装 git hooks 和同步 skills",
	Long:  "安装 git pre-commit/commit-msg hook 和同步 skills 到用户目录",
	RunE:  runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().BoolVarP(&installHooks, "hooks", "", true, "安装 git hooks")
	installCmd.Flags().BoolVarP(&installSkills, "skills", "", true, "同步 skills 到 ~/.trae/skills/")
}

func runInstall(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	if installHooks {
		if err := removeLegacyPrePushHook(repoRoot); err != nil {
			return err
		}
		if err := installPreCommitHook(repoRoot); err != nil {
			return err
		}
		if err := installCommitMsgHook(repoRoot); err != nil {
			return err
		}
		if err := removeLegacyPostCommitHook(repoRoot); err != nil {
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

// installCommitMsgHook 安装 commit-msg hook
func installCommitMsgHook(repoRoot string) error {
	color.Cyan("正在安装 commit-msg hook...")

	hooksDir := filepath.Join(repoRoot, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("创建 hooks 目录失败: %w", err)
	}

	hookPath := filepath.Join(hooksDir, "commit-msg")
	hookContent := `#!/bin/bash
# coco-ext commit-msg hook
# 1. 烂 commit message 时同步优化 message
# 2. 失败时保留原始 message，不阻塞 commit
# 3. 输出本次优化耗时

MSG_FILE="$1"
if [ -z "$MSG_FILE" ] || [ ! -f "$MSG_FILE" ]; then
    exit 0
fi

FIRST_LINE=$(sed -n '/^[[:space:]]*#/d;/^[[:space:]]*$/d;p;q' "$MSG_FILE" | tr -d '[:space:]')
if [ -z "$FIRST_LINE" ] || [ ${#FIRST_LINE} -ge 10 ]; then
    exit 0
fi

RAW_FIRST_LINE=$(sed -n '/^[[:space:]]*#/d;/^[[:space:]]*$/d;p;q' "$MSG_FILE")
case "$RAW_FIRST_LINE" in
    Merge*|Revert*)
        exit 0
        ;;
esac

if ! git diff --cached --quiet --exit-code; then
    :
else
    exit 0
fi

START_TIME=$(date +%s)

echo "⚠ commit message 太简短，正在优化..."
echo ""
echo "📝 优化进度:"
echo "   [1/3] 生成规范 commit message..."
echo "   [2/3] 调用 AI 生成 message..."

mkdir -p .livecoding/logs
LOG_FILE=".livecoding/logs/gcmsg-commitmsg-$(date +%Y%m%d%H%M%S).log"

coco-ext gcmsg --staged --commit-msg-file "$MSG_FILE" 2>&1 | tee "$LOG_FILE"
EXIT_CODE=${PIPESTATUS[0]}
END_TIME=$(date +%s)
ELAPSED_SECONDS=$((END_TIME - START_TIME))

if [ $EXIT_CODE -eq 0 ]; then
    echo "   [3/3] commit message 已更新 ✓"
    echo ""
    echo "⏱ 本次优化耗时: ${ELAPSED_SECONDS}s"
else
    echo ""
    echo "⚠ 优化失败，继续使用原始 commit message"
    echo "日志: $LOG_FILE"
    echo "⏱ 本次优化耗时: ${ELAPSED_SECONDS}s"
fi

exit 0
`

	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		return fmt.Errorf("写入 hook 失败: %w", err)
	}

	color.Green("✓ commit-msg hook 已安装")
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

	// 获取用户目录
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %w", err)
	}

	userSkillsDir := filepath.Join(home, ".trae", "skills")
	if err := os.MkdirAll(userSkillsDir, 0755); err != nil {
		return fmt.Errorf("创建用户 skills 目录失败: %w", err)
	}

	entries, err := fs.ReadDir(embeddedSkillsFS, embeddedSkillsRoot)
	if err != nil {
		return fmt.Errorf("读取内置 skills 失败: %w", err)
	}

	installedSkills := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		srcRoot := filepath.ToSlash(filepath.Join(embeddedSkillsRoot, skillName))
		dstRoot := filepath.Join(userSkillsDir, skillName)
		if err := copyEmbeddedDir(srcRoot, dstRoot); err != nil {
			color.Yellow("⚠ 复制 %s 失败: %v", skillName, err)
			continue
		}
		installedSkills = append(installedSkills, skillName)
		color.Green("✓ 同步 skill: %s", skillName)
	}

	if err := writeSkillsManifest(userSkillsDir, installedSkills); err != nil {
		return fmt.Errorf("写入 skills manifest 失败: %w", err)
	}

	color.Green("✓ skills 同步完成")
	return nil
}

func copyEmbeddedDir(srcRoot, dstRoot string) error {
	return fs.WalkDir(embeddedSkillsFS, srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return os.MkdirAll(dstRoot, 0755)
		}

		dstPath := filepath.Join(dstRoot, filepath.FromSlash(relPath))
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		data, err := embeddedSkillsFS.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, 0644)
	})
}

type skillsManifest struct {
	InstalledBy string    `json:"installed_by"`
	Version     string    `json:"version"`
	InstalledAt time.Time `json:"installed_at"`
	Skills      []string  `json:"skills"`
}

func writeSkillsManifest(userSkillsDir string, skills []string) error {
	manifest := skillsManifest{
		InstalledBy: "coco-ext",
		Version:     version,
		InstalledAt: time.Now(),
		Skills:      skills,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(userSkillsDir, skillsManifestFileName)
	return os.WriteFile(manifestPath, data, 0644)
}

func readSkillsManifest(userSkillsDir string) (*skillsManifest, error) {
	manifestPath := filepath.Join(userSkillsDir, skillsManifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest skillsManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
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
	Long:  "卸载 git pre-commit/commit-msg hook 和 skills（仅删除从 coco-ext 安装的部分）",
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
		if err := removeCommitMsgHook(repoRoot); err != nil {
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

func removeCommitMsgHook(repoRoot string) error {
	color.Cyan("正在卸载 commit-msg hook...")
	hooksDir := filepath.Join(repoRoot, ".git", "hooks")
	hookPath := filepath.Join(hooksDir, "commit-msg")
	if _, err := os.Stat(hookPath); err == nil {
		if err := os.Remove(hookPath); err != nil {
			return fmt.Errorf("删除 commit-msg hook 失败: %w", err)
		}
		color.Green("✓ commit-msg hook 已卸载")
	} else {
		color.Yellow("⚠ commit-msg hook 不存在，跳过")
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

func removeLegacyPostCommitHook(repoRoot string) error {
	hooksDir := filepath.Join(repoRoot, ".git", "hooks")
	hookPath := filepath.Join(hooksDir, "post-commit")
	if _, err := os.Stat(hookPath); err == nil {
		if err := os.Remove(hookPath); err != nil {
			return fmt.Errorf("删除 legacy post-commit hook 失败: %w", err)
		}
		color.Green("✓ legacy post-commit hook 已移除")
	}
	return nil
}

func removeLegacyPrePushHook(repoRoot string) error {
	hooksDir := filepath.Join(repoRoot, ".git", "hooks")
	hookPath := filepath.Join(hooksDir, "pre-push")
	if _, err := os.Stat(hookPath); err == nil {
		if err := os.Remove(hookPath); err != nil {
			return fmt.Errorf("删除 legacy pre-push hook 失败: %w", err)
		}
		color.Green("✓ legacy pre-push hook 已移除")
	}
	return nil
}

func removeSkills() error {
	color.Cyan("正在卸载 skills...")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户目录失败: %w", err)
	}

	userSkillsDir := filepath.Join(home, ".trae", "skills")
	if _, err := os.Stat(userSkillsDir); os.IsNotExist(err) {
		color.Yellow("⚠ 用户 skills 目录不存在，跳过")
		return nil
	}

	manifest, err := readSkillsManifest(userSkillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			color.Yellow("⚠ 未找到 skills manifest，跳过")
			return nil
		}
		return fmt.Errorf("读取 skills manifest 失败: %w", err)
	}

	for _, skillName := range manifest.Skills {
		targetPath := filepath.Join(userSkillsDir, skillName)
		if _, err := os.Stat(targetPath); err == nil {
			if err := os.RemoveAll(targetPath); err != nil {
				color.Yellow("⚠ 删除 %s 失败: %v", skillName, err)
			} else {
				color.Green("✓ 删除 skill: %s", skillName)
			}
		} else {
			color.Yellow("⚠ %s 不存在，跳过", skillName)
		}
	}

	manifestPath := filepath.Join(userSkillsDir, skillsManifestFileName)
	if err := os.Remove(manifestPath); err == nil {
		color.Green("✓ 删除 skills manifest")
	} else if !os.IsNotExist(err) {
		color.Yellow("⚠ 删除 skills manifest 失败: %v", err)
	}

	color.Green("✓ skills 卸载完成")
	return nil
}
