package git

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DiffInfo 代码变更信息
type DiffInfo struct {
	SourceBranch  string `json:"source_branch"`
	TargetBranch  string `json:"target_branch"`
	CommitMessage string `json:"commit_message"`
	Author        string `json:"author"`
	Additions     int    `json:"additions"`
	Deletions     int    `json:"deletions"`
	FileCount     int    `json:"file_count"`
	Diff          string `json:"diff"`
}

// IsEmpty 判断是否有代码变更
func (d *DiffInfo) IsEmpty() bool {
	return d.Diff == "" || d.FileCount == 0
}

// WriteMeta 将元信息写入 JSON 文件
func (d *DiffInfo) WriteMeta(path string) error {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// GetDiffInfo 获取本地代码变更信息
// fullBranchDiff 为 true 时获取分支整体 diff，为 false 时只获取最后一个 commit 的 diff
func GetDiffInfo(repoRoot, baseBranch string, fullBranchDiff bool) (*DiffInfo, error) {
	// 获取当前分支
	sourceBranch, err := getCurrentBranch(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("获取当前分支失败: %w", err)
	}

	// 获取目标分支（基准分支）
	if baseBranch == "" {
		baseBranch, err = detectDefaultBranch(repoRoot)
		if err != nil {
			return nil, fmt.Errorf("检测默认分支失败: %w", err)
		}
	}

	// 获取 diff
	diff, err := getDiff(repoRoot, baseBranch, sourceBranch, fullBranchDiff)
	if err != nil {
		return nil, fmt.Errorf("获取 diff 失败: %w", err)
	}

	// 解析 diff 统计信息
	additions, deletions, fileCount := parseDiffStats(diff)

	// 获取最新提交信息
	commitMsg, author, err := getLastCommit(repoRoot)
	if err != nil {
		commitMsg = "(无法获取提交信息)"
		author = "(未知)"
	}

	return &DiffInfo{
		SourceBranch:  sourceBranch,
		TargetBranch:  baseBranch,
		CommitMessage: commitMsg,
		Author:        author,
		Additions:     additions,
		Deletions:     deletions,
		FileCount:     fileCount,
		Diff:          diff,
	}, nil
}

// getCurrentBranch 获取当前分支名
func getCurrentBranch(repoRoot string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		// 可能是 detached HEAD 状态，获取 ref
		cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		cmd.Dir = repoRoot
		output, err = cmd.Output()
		if err != nil {
			return "", err
		}
		branch = strings.TrimSpace(string(output))
	}
	return branch, nil
}

// detectDefaultBranch 检测默认分支（main 或 master）
func detectDefaultBranch(repoRoot string) (string, error) {
	// 优先检查本地 main
	cmd := exec.Command("git", "rev-parse", "--verify", "main")
	cmd.Dir = repoRoot
	if err := cmd.Run(); err == nil {
		return "main", nil
	}

	// 检查本地 master
	cmd = exec.Command("git", "rev-parse", "--verify", "master")
	cmd.Dir = repoRoot
	if err := cmd.Run(); err == nil {
		return "master", nil
	}

	// 尝试获取 remote origin 的 HEAD
	cmd = exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err == nil {
		ref := strings.TrimSpace(string(output))
		// refs/remotes/origin/main → main
		parts := strings.Split(ref, "/")
		if len(parts) >= 3 {
			return parts[len(parts)-1], nil
		}
	}

	return "origin/HEAD", nil
}

// getDiff 获取两个分支间的 diff
// fullBranchDiff 为 true 时获取分支整体 diff，为 false 时只获取最后一个 commit 的 diff
func getDiff(repoRoot, baseBranch, sourceBranch string, fullBranchDiff bool) (string, error) {
	// 确保 baseBranch 存在
	cmd := exec.Command("git", "rev-parse", "--verify", baseBranch)
	cmd.Dir = repoRoot
	if err := cmd.Run(); err != nil {
		// 分支不存在，尝试 origin 前缀
		baseBranch = "origin/" + baseBranch
	}

	var diffCmd *exec.Cmd
	if fullBranchDiff {
		// 获取分支整体 diff（从分叉点开始的所有 commits）
		diffCmd = exec.Command("git", "diff", baseBranch+"..."+sourceBranch)
	} else {
		// 只获取最后一个 commit 的 diff
		// 获取 sourceBranch 最新 commit 的 parent 与 最新 commit 之间的 diff
		diffCmd = exec.Command("git", "diff", sourceBranch+"~1", sourceBranch)
	}

	cmd.Dir = repoRoot
	output, err := diffCmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff 失败: %w", err)
	}
	return string(output), nil
}

// getLastCommit 获取最新提交的标题和作者
func getLastCommit(repoRoot string) (string, string, error) {
	cmd := exec.Command("git", "log", "-1", "--format=%s%n%an")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	lines := strings.SplitN(strings.TrimSpace(string(output)), "\n", 2)
	if len(lines) < 2 {
		return lines[0], "", nil
	}
	return lines[0], lines[1], nil
}

// parseDiffStats 解析 diff 统计信息
func parseDiffStats(diff string) (additions, deletions, fileCount int) {
	// 统计 file count（--- 和 +++ 行）
	files := make(map[string]bool)
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "--- a/") || strings.HasPrefix(line, "--- /dev/null") {
			name := strings.TrimPrefix(line, "--- a/")
			if strings.HasPrefix(line, "--- /dev/null") {
				name = strings.TrimPrefix(line, "--- /dev/null")
			}
			files[name] = true
		}
	}
	fileCount = len(files)

	// 统计 additions 和 deletions
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			additions++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deletions++
		}
	}

	return additions, deletions, fileCount
}

// IsModSumOnly 检查是否仅修改了 go.mod/go.sum
func IsModSumOnly(repoRoot string) (bool, error) {
	cmd := exec.Command("git", "diff", "--name-only", "HEAD~1", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(files) == 0 || files[0] == "" {
		return false, nil
	}

	modSumOnly := true
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f != "" && f != "go.mod" && f != "go.sum" && f != "go.mod.lock" {
			modSumOnly = false
			break
		}
	}

	return modSumOnly, nil
}

// IsGitRepo 检查是否是 git 仓库
func IsGitRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	if stat, err := os.Stat(gitPath); err == nil && stat.IsDir() {
		return true
	}
	return false
}
