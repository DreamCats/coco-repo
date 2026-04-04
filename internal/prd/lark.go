package prd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var reWikiToken = regexp.MustCompile(`/wiki/([A-Za-z0-9]+)`)

// CheckLarkCLI 检查 lark-cli 是否已安装
func CheckLarkCLI() error {
	_, err := exec.LookPath("lark-cli")
	if err != nil {
		return fmt.Errorf("lark-cli 未安装，请访问 https://github.com/larksuite/cli 下载安装")
	}
	return nil
}

// FetchLarkDocContent 通过 lark-cli 拉取飞书文档内容，返回 markdown 正文和标题。
// 支持 /wiki/TOKEN 和 /docx?/TOKEN 两种链接格式。
func FetchLarkDocContent(rawURL string) (content string, title string, err error) {
	if err := CheckLarkCLI(); err != nil {
		return "", "", err
	}

	docToken, title, err := resolveDocToken(rawURL)
	if err != nil {
		return "", "", err
	}

	content, fetchTitle, err := fetchDocContent(docToken)
	if err != nil {
		return "", "", err
	}

	// 优先用 fetch 返回的标题（更准确）
	if fetchTitle != "" {
		title = fetchTitle
	}

	return content, title, nil
}

// resolveDocToken 根据 URL 类型解析出真正的 doc_token。
// wiki 链接需要先调 get_node 拿 obj_token，docx 链接直接提取。
func resolveDocToken(rawURL string) (docToken string, title string, err error) {
	// wiki 链接：/wiki/TOKEN → 需要先解析
	if matches := reWikiToken.FindStringSubmatch(rawURL); len(matches) > 1 {
		wikiToken := matches[1]
		return resolveWikiNode(wikiToken)
	}

	// docx/doc 链接：/docx/TOKEN 或 /doc/TOKEN → 直接用
	if matches := reDocToken.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "", nil
	}

	return "", "", fmt.Errorf("无法从链接中提取文档 token: %s", rawURL)
}

// resolveWikiNode 调用 lark-cli wiki spaces get_node 解析 wiki token 为 doc token
func resolveWikiNode(wikiToken string) (docToken string, title string, err error) {
	params := fmt.Sprintf(`{"token":"%s"}`, wikiToken)
	cmd := exec.Command("lark-cli", "wiki", "spaces", "get_node", "--params", params)
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("解析 wiki 节点失败: %w（请确认已执行 lark-cli auth login）", err)
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Node struct {
				ObjToken string `json:"obj_token"`
				ObjType  string `json:"obj_type"`
				Title    string `json:"title"`
			} `json:"node"`
		} `json:"data"`
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(output, &resp); err != nil {
		return "", "", fmt.Errorf("解析 wiki 节点响应失败: %w", err)
	}
	if resp.Code != 0 {
		return "", "", fmt.Errorf("wiki get_node 返回错误: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data.Node.ObjToken == "" {
		return "", "", fmt.Errorf("wiki 节点未返回 obj_token，wiki_token=%s", wikiToken)
	}

	return resp.Data.Node.ObjToken, resp.Data.Node.Title, nil
}

// fetchDocContent 调用 lark-cli docs +fetch 拉取文档 markdown 内容
func fetchDocContent(docToken string) (content string, title string, err error) {
	cmd := exec.Command("lark-cli", "docs", "+fetch", "--doc", docToken)
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("拉取文档内容失败: %w（请确认已执行 lark-cli auth login）", err)
	}

	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			Markdown string `json:"markdown"`
			Title    string `json:"title"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &resp); err != nil {
		return "", "", fmt.Errorf("解析文档内容响应失败: %w", err)
	}
	if !resp.OK {
		return "", "", fmt.Errorf("docs +fetch 返回失败，doc_token=%s", docToken)
	}

	content = strings.TrimSpace(resp.Data.Markdown)
	if content == "" {
		return "", "", fmt.Errorf("文档内容为空，doc_token=%s", docToken)
	}

	return content, resp.Data.Title, nil
}
