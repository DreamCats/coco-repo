package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// MRInfo MR 信息
type MRInfo struct {
	ProjectID     string `json:"project_id"`
	MRID          int    `json:"mr_id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Author        string `json:"author"`
	SourceBranch  string `json:"source_branch"`
	TargetBranch  string `json:"target_branch"`
	Additions     int    `json:"additions"`
	Deletions     int    `json:"deletions"`
	WebURL        string `json:"web_url"`
}

// GetMRDiff 获取 MR 的 diff 信息
func GetMRDiff(repoRoot string, mrID int) (*MRInfo, string, error) {
	// 获取 GitLab 配置
	gitLabURL, projectPath, token, err := getGitLabConfig(repoRoot)
	if err != nil {
		return nil, "", fmt.Errorf("获取 GitLab 配置失败: %w", err)
	}

	if token == "" {
		return nil, "", fmt.Errorf("未设置 GITLAB_TOKEN 环境变量或 ~/.config/coco-ext/gitlab.json")
	}

	// 获取 MR 信息
	mrInfo, err := fetchMRInfo(gitLabURL, projectPath, mrID, token)
	if err != nil {
		return nil, "", err
	}

	// 获取 MR diff
	diff, err := fetchMRDiff(gitLabURL, projectPath, mrID, token)
	if err != nil {
		return nil, "", err
	}

	return mrInfo, diff, nil
}

// getGitLabConfig 从 git remote 和环境变量获取 GitLab 配置
func getGitLabConfig(repoRoot string) (gitLabURL, projectPath, token string, err error) {
	// 从 git remote 获取 GitLab 地址和项目路径
	gitLabURL, projectPath, err = getGitRemote(repoRoot)
	if err != nil {
		return "", "", "", err
	}

	// 优先从环境变量获取 token
	token = os.Getenv("GITLAB_TOKEN")
	if token != "" {
		return gitLabURL, projectPath, token, nil
	}

	// 其次从配置文件读取
	token, _ = readGitLabToken()

	return gitLabURL, projectPath, token, nil
}

// getGitRemote 获取 git remote 地址和项目路径
func getGitRemote(repoRoot string) (gitLabURL, projectPath string, err error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("获取 git remote 失败: %w", err)
	}

	remoteURL := strings.TrimSpace(string(output))
	// 格式: git@gitlab.example.com:group/project.git 或 https://gitlab.example.com/group/project

	// SSH 格式: git@gitlab.example.com:group/project
	if strings.HasPrefix(remoteURL, "git@") {
		parts := strings.SplitN(strings.TrimPrefix(remoteURL, "git@"), ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("无法解析 git remote URL: %s", remoteURL)
		}
		gitLabURL = "https://" + parts[0]
		projectPath = strings.TrimSuffix(parts[1], ".git")
		return gitLabURL, projectPath, nil
	}

	// HTTPS 格式: https://gitlab.example.com/group/project.git
	if strings.HasPrefix(remoteURL, "https://") || strings.HasPrefix(remoteURL, "http://") {
		u, err := url.Parse(remoteURL)
		if err != nil {
			return "", "", fmt.Errorf("无法解析 git remote URL: %w", err)
		}
		gitLabURL = u.Scheme + "://" + u.Host
		projectPath = strings.TrimSuffix(u.Path, ".git")
		projectPath = strings.TrimPrefix(projectPath, "/")
		return gitLabURL, projectPath, nil
	}

	return "", "", fmt.Errorf("不支持的 git remote URL 格式: %s", remoteURL)
}

// fetchMRInfo 获取 MR 基本信息
func fetchMRInfo(gitLabURL, projectPath string, mrID int, token string) (*MRInfo, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d",
		gitLabURL, url.PathEscape(projectPath), mrID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitLab API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitLab API 返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var mr struct {
		IID           int    `json:"iid"`
		Title         string `json:"title"`
		Description   string `json:"description"`
		Author        struct {
			Username string `json:"username"`
		} `json:"author"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		WebURL       string `json:"web_url"`
		ProjectID    int    `json:"project_id"`
		Additions    int    `json:"additions"`
		Deletions    int    `json:"deletions"`
		ChangesCount string `json:"changes_count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return nil, fmt.Errorf("解析 MR 信息失败: %w", err)
	}

	return &MRInfo{
		ProjectID:    fmt.Sprintf("%d", mr.ProjectID),
		MRID:        mr.IID,
		Title:       mr.Title,
		Description: mr.Description,
		Author:      mr.Author.Username,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		WebURL:      mr.WebURL,
		Additions:   mr.Additions,
		Deletions:   mr.Deletions,
	}, nil
}

// fetchMRDiff 获取 MR 的变更 diff
func fetchMRDiff(gitLabURL, projectPath string, mrID int, token string) (string, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d/changes",
		gitLabURL, url.PathEscape(projectPath), mrID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 GitLab API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitLab API 返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Changes []struct {
			Diff string `json:"diff"`
		} `json:"changes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析 MR diff 失败: %w", err)
	}

	var diffBuilder strings.Builder
	for _, change := range result.Changes {
		diffBuilder.WriteString(change.Diff)
		diffBuilder.WriteString("\n")
	}

	return diffBuilder.String(), nil
}

// WriteMeta 将 MR 信息写入 JSON 文件
func (m *MRInfo) WriteMeta(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// readGitLabToken 从配置文件读取 token
func readGitLabToken() (string, error) {
	home, _ := os.UserHomeDir()
	tokenPath := filepath.Join(home, ".config", "coco-ext", "gitlab.json")

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", err
	}

	var config struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return "", err
	}
	return config.Token, nil
}
