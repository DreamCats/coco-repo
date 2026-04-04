package prd

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	TaskStatusInitialized = "initialized"
	TaskStatusRefined     = "refined"
	TaskStatusPlanned     = "planned"
)

type SourceType string

const (
	SourceTypeText    SourceType = "text"
	SourceTypeFile    SourceType = "file"
	SourceTypeLarkDoc SourceType = "lark_doc"
)

type TaskMetadata struct {
	TaskID      string     `json:"task_id"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	SourceType  SourceType `json:"source_type"`
	SourceValue string     `json:"source_value"`
}

type SourceMetadata struct {
	Type       SourceType `json:"type"`
	Title      string     `json:"title"`
	Path       string     `json:"path,omitempty"`
	URL        string     `json:"url,omitempty"`
	DocToken   string     `json:"doc_token,omitempty"`
	CapturedAt time.Time  `json:"captured_at"`
}

type RefineInput struct {
	RawInput     string
	Title        string
	ExplicitTask string
	Now          time.Time
}

type RefineSource struct {
	Type       SourceType
	Title      string
	RawInput   string
	Content    string
	Path       string
	URL        string
	DocToken   string
	CapturedAt time.Time
	LarkErr    error // lark-cli 拉取失败时的错误信息，用于在 cmd 层展示提示
}

type RefineTask struct {
	TaskID         string
	Title          string
	TaskDir        string
	SourcePath     string
	RefinedPath    string
	MetadataPath   string
	SourceMetaPath string
	Source         RefineSource
	SupportsRefine bool
}

var (
	reASCIIWord = regexp.MustCompile(`[a-zA-Z0-9]+`)
	reDocToken  = regexp.MustCompile(`/docx?/([A-Za-z0-9]+)`)
	reMDTitle   = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
	reMDFront   = regexp.MustCompile(`(?m)^title:\s*(.+?)\s*$`)
	reSlugDash  = regexp.MustCompile(`-+`)
	reRefinedH1 = regexp.MustCompile(`(?m)^#\s+PRD Refined\s*$`)
	reSpacing   = regexp.MustCompile(`[ \t]+`)
)

var zhSlugKeywords = []struct {
	Needle string
	Token  string
}{
	{Needle: "讲解卡", Token: "popcard"},
	{Needle: "倒计时", Token: "countdown"},
	{Needle: "拍卖", Token: "auction"},
	{Needle: "直播间", Token: "live-room"},
	{Needle: "商品", Token: "product"},
	{Needle: "订单", Token: "order"},
	{Needle: "优惠券", Token: "coupon"},
	{Needle: "福袋", Token: "luckybag"},
	{Needle: "小黄车", Token: "cart"},
	{Needle: "弹窗", Token: "popup"},
	{Needle: "卡片", Token: "card"},
}

// PrepareRefineTask 准备 refine 所需的 task 目录、源文件和元信息。
func PrepareRefineTask(repoRoot string, input RefineInput) (*RefineTask, error) {
	source, err := resolveSource(input)
	if err != nil {
		return nil, err
	}

	taskID := strings.TrimSpace(input.ExplicitTask)
	if taskID == "" {
		taskID = BuildTaskID(source.Title, source.RawInput, input.Now)
	}

	taskDir := filepath.Join(repoRoot, ".livecoding", "tasks", taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return nil, fmt.Errorf("创建 task 目录失败: %w", err)
	}

	task := &RefineTask{
		TaskID:         taskID,
		Title:          source.Title,
		TaskDir:        taskDir,
		SourcePath:     filepath.Join(taskDir, "prd.source.md"),
		RefinedPath:    filepath.Join(taskDir, "prd-refined.md"),
		MetadataPath:   filepath.Join(taskDir, "task.json"),
		SourceMetaPath: filepath.Join(taskDir, "source.json"),
		Source:         source,
		SupportsRefine: strings.TrimSpace(source.Content) != "",
	}

	if err := writeSourceArtifacts(task, input.Now); err != nil {
		return nil, err
	}

	return task, nil
}

// BuildTaskID 生成任务目录名。
func BuildTaskID(title, sourceKey string, now time.Time) string {
	datePart := now.Format("20060102")
	slug := buildTaskSlug(title)
	suffix := shortHash(strings.Join([]string{title, sourceKey, now.Format(time.RFC3339Nano)}, "|"), 4)
	return fmt.Sprintf("%s-%s-%s", datePart, slug, suffix)
}

// BuildRefinedPrompt 构造 PRD refine 的 AI prompt。
func BuildRefinedPrompt(sourceTitle, sourceContent string) string {
	return fmt.Sprintf(`你是一名严谨的产品需求梳理助手。请根据给定 PRD 原文，输出一份适合进入后续代码调研的 refined PRD。

要求：
1. 输出使用中文 Markdown。
2. 你只能基于下面提供的 PRD 原文工作，不要查看仓库、代码、已有实现，也不要提及这些动作。
3. 不要解释你在做什么，不要输出任何思考过程、前言、说明、分析或“让我先看看”之类的话。
4. 输出必须直接从 # PRD Refined 开始，前面不能有任何额外文字。
3. 结构必须包含以下一级标题：
   - # PRD Refined
   - ## 需求概述
   - ## 功能点
   - ## 边界条件
   - ## 交互与展示
   - ## 验收标准
   - ## 业务规则
   - ## 待确认问题
5. 如果原文缺失信息，在“待确认问题”中列出，不要编造。
6. 保持内容紧凑，尽量把原文信息结构化整理出来。

PRD 标题：%s

PRD 原文：

%s
`, sourceTitle, sourceContent)
}

// ExtractRefinedContent 提取真正的 refined 正文，去掉模型的前言和过程性输出。
func ExtractRefinedContent(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return ""
	}

	if loc := reRefinedH1.FindStringIndex(normalized); loc != nil {
		return strings.TrimSpace(normalized[loc[0]:])
	}
	return ""
}

// ValidateRefinedContent 校验 refined PRD 是否满足最小结构要求。
func ValidateRefinedContent(content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("refined PRD 为空")
	}
	if !strings.HasPrefix(content, "# PRD Refined") {
		return fmt.Errorf("refined PRD 缺少 # PRD Refined 标题")
	}

	requiredSections := []string{
		"## 需求概述",
		"## 功能点",
		"## 边界条件",
		"## 交互与展示",
		"## 验收标准",
		"## 业务规则",
		"## 待确认问题",
	}
	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			return fmt.Errorf("refined PRD 缺少必要章节: %s", section)
		}
	}
	return nil
}

// BuildFallbackRefinedContent 在 AI 不可用时生成结构化的本地兜底内容。
func BuildFallbackRefinedContent(sourceTitle, sourceContent string, cause error) string {
	causeText := "未知原因"
	if cause != nil {
		causeText = cause.Error()
	}

	return fmt.Sprintf(`# PRD Refined

> 状态：fallback
> 原因：%s

## 需求概述

- 标题：%s
- 当前版本未能完成 AI refine，以下内容基于原始 PRD 做最小结构化整理。

## 功能点

- 请基于原始 PRD 拆分主要功能点。

## 边界条件

- 请补充异常场景、空状态和结束态。

## 交互与展示

- 请补充 UI 位置、状态变化和特殊展示要求。

## 验收标准

- 请补充“如何算完成”和“如何验证通过”。

## 业务规则

- 请补充适用范围、过滤条件、端侧差异等规则。

## 待确认问题

- 当前为本地兜底稿，建议在 daemon 可用后重新执行 refine。

## 原始 PRD

%s
`, causeText, sourceTitle, strings.TrimSpace(sourceContent))
}

// BuildPendingRefinedContent 为尚未拉取正文的 Lark 文档生成占位 refined 内容。
func BuildPendingRefinedContent(task *RefineTask) string {
	return fmt.Sprintf("# PRD Refined\n\n"+
		"> 状态：待补充源内容\n"+
		"> task_id: %s\n"+
		"> source: %s\n\n"+
		"## 说明\n\n"+
		"当前任务已创建，并已记录飞书文档来源，但这版 `coco-ext prd refine` 还未内置飞书文档拉取能力。\n\n"+
		"请将 PRD 正文补充到以下文件后，再重新执行 refine：\n\n"+
		"- %s\n\n"+
		"## 来源信息\n\n"+
		"- 标题：%s\n"+
		"- 飞书链接：%s\n"+
		"- doc token：%s\n\n"+
		"## 待确认问题\n\n"+
		"- 需要补充 PRD 正文后，才能继续做结构化 refine。\n",
		task.TaskID, task.Source.URL, task.SourcePath, task.Title, task.Source.URL, task.Source.DocToken)
}

func resolveSource(input RefineInput) (RefineSource, error) {
	raw := strings.TrimSpace(input.RawInput)
	if raw == "" {
		return RefineSource{}, fmt.Errorf("--prd 不能为空")
	}

	if isLikelyURL(raw) {
		u, err := url.Parse(raw)
		if err != nil {
			return RefineSource{}, fmt.Errorf("无效的 URL: %w", err)
		}

		title := strings.TrimSpace(input.Title)

		// 尝试通过 lark-cli 拉取飞书文档内容
		var content string
		larkContent, larkTitle, larkErr := FetchLarkDocContent(raw)
		if larkErr == nil {
			content = larkContent
			if title == "" && larkTitle != "" {
				title = larkTitle
			}
		}

		if title == "" {
			title = inferTitleFromURL(u)
		}
		if title == "" {
			title = "未命名需求"
		}

		return RefineSource{
			Type:       SourceTypeLarkDoc,
			Title:      title,
			RawInput:   raw,
			Content:    content,
			URL:        raw,
			DocToken:   extractDocToken(raw),
			CapturedAt: input.Now,
			LarkErr:    larkErr,
		}, nil
	}

	if stat, err := os.Stat(raw); err == nil && !stat.IsDir() {
		data, err := os.ReadFile(raw)
		if err != nil {
			return RefineSource{}, fmt.Errorf("读取 PRD 文件失败: %w", err)
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			return RefineSource{}, fmt.Errorf("PRD 文件为空")
		}

		title := strings.TrimSpace(input.Title)
		if title == "" {
			title = inferTitleFromContent(content)
		}
		if title == "" {
			title = trimTitle(filepath.Base(raw))
		}

		return RefineSource{
			Type:       SourceTypeFile,
			Title:      title,
			RawInput:   raw,
			Content:    content,
			Path:       raw,
			CapturedAt: input.Now,
		}, nil
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = inferTitleFromContent(normalizeTextInput(raw))
	}
	if title == "" {
		title = "未命名需求"
	}

	normalizedContent := normalizeTextInput(raw)

	return RefineSource{
		Type:       SourceTypeText,
		Title:      title,
		RawInput:   raw,
		Content:    normalizedContent,
		CapturedAt: input.Now,
	}, nil
}

func writeSourceArtifacts(task *RefineTask, now time.Time) error {
	sourceMeta := SourceMetadata{
		Type:       task.Source.Type,
		Title:      task.Title,
		Path:       task.Source.Path,
		URL:        task.Source.URL,
		DocToken:   task.Source.DocToken,
		CapturedAt: task.Source.CapturedAt,
	}
	if err := writeJSONFile(task.SourceMetaPath, sourceMeta); err != nil {
		return fmt.Errorf("写入 source.json 失败: %w", err)
	}

	sourceMarkdown := buildSourceMarkdown(task.Source)
	if err := os.WriteFile(task.SourcePath, []byte(sourceMarkdown), 0644); err != nil {
		return fmt.Errorf("写入 prd.source.md 失败: %w", err)
	}

	taskMeta := TaskMetadata{
		TaskID:      task.TaskID,
		Title:       task.Title,
		Status:      TaskStatusInitialized,
		CreatedAt:   now,
		UpdatedAt:   now,
		SourceType:  task.Source.Type,
		SourceValue: firstNonEmpty(task.Source.URL, task.Source.Path, task.Source.RawInput),
	}
	if err := writeJSONFile(task.MetadataPath, taskMeta); err != nil {
		return fmt.Errorf("写入 task.json 失败: %w", err)
	}

	return nil
}

func WriteRefinedContent(task *RefineTask, content string, now time.Time, status string) error {
	if err := os.WriteFile(task.RefinedPath, []byte(strings.TrimSpace(content)+"\n"), 0644); err != nil {
		return fmt.Errorf("写入 prd-refined.md 失败: %w", err)
	}

	meta := TaskMetadata{
		TaskID:      task.TaskID,
		Title:       task.Title,
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
		SourceType:  task.Source.Type,
		SourceValue: firstNonEmpty(task.Source.URL, task.Source.Path, task.Source.RawInput),
	}
	return writeJSONFile(task.MetadataPath, meta)
}

func buildSourceMarkdown(source RefineSource) string {
	var lines []string
	lines = append(lines, "# PRD Source")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("- title: %s", source.Title))
	lines = append(lines, fmt.Sprintf("- source_type: %s", source.Type))
	lines = append(lines, fmt.Sprintf("- captured_at: %s", source.CapturedAt.Format(time.RFC3339)))
	if source.Path != "" {
		lines = append(lines, fmt.Sprintf("- path: %s", source.Path))
	}
	if source.URL != "" {
		lines = append(lines, fmt.Sprintf("- url: %s", source.URL))
	}
	if source.DocToken != "" {
		lines = append(lines, fmt.Sprintf("- doc_token: %s", source.DocToken))
	}
	lines = append(lines, "", "---", "")

	if strings.TrimSpace(source.Content) == "" {
		lines = append(lines, "当前版本尚未自动拉取该来源的正文内容。")
		lines = append(lines, "请将 PRD 正文粘贴到本文件后，重新执行 `coco-ext prd refine --task <task_id> --prd <本文件路径>`。")
		return strings.Join(lines, "\n") + "\n"
	}

	lines = append(lines, source.Content)
	return strings.Join(lines, "\n") + "\n"
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func buildTaskSlug(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "task"
	}

	asciiTokens := reASCIIWord.FindAllString(strings.ToLower(title), -1)
	filtered := make([]string, 0, len(asciiTokens))
	for _, token := range asciiTokens {
		if len(token) < 2 {
			continue
		}
		filtered = append(filtered, token)
	}
	if len(filtered) > 0 {
		if len(filtered) > 4 {
			filtered = filtered[:4]
		}
		return trimTaskSlug(strings.Join(filtered, "-"))
	}

	mapped := make([]string, 0, 4)
	for _, item := range zhSlugKeywords {
		if strings.Contains(title, item.Needle) {
			mapped = append(mapped, item.Token)
		}
	}
	if len(mapped) > 0 {
		sort.Strings(mapped)
		mapped = dedupeStrings(mapped)
		if len(mapped) > 4 {
			mapped = mapped[:4]
		}
		return trimTaskSlug(strings.Join(mapped, "-"))
	}

	return "task"
}

func trimTaskSlug(slug string) string {
	slug = strings.ToLower(slug)
	slug = reSlugDash.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "task"
	}
	const maxLen = 32
	if len(slug) > maxLen {
		slug = strings.Trim(slug[:maxLen], "-")
	}
	if slug == "" {
		return "task"
	}
	return slug
}

func shortHash(raw string, n int) string {
	sum := sha1.Sum([]byte(raw))
	encoded := hex.EncodeToString(sum[:])
	if n <= 0 || n > len(encoded) {
		return encoded
	}
	return encoded[:n]
}

func inferTitleFromContent(content string) string {
	if matches := reMDTitle.FindStringSubmatch(content); len(matches) > 1 {
		return trimTitle(matches[1])
	}
	if matches := reMDFront.FindStringSubmatch(content); len(matches) > 1 {
		return trimTitle(matches[1])
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return trimTitle(line)
	}
	return ""
}

func normalizeTextInput(raw string) string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts = append(parts, line)
	}
	joined := strings.Join(parts, " ")
	joined = reSpacing.ReplaceAllString(joined, " ")
	return strings.TrimSpace(joined)
}

func trimTitle(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "#")
	raw = strings.TrimSpace(raw)
	const maxLen = 80
	runes := []rune(raw)
	if len(runes) > maxLen {
		raw = strings.TrimSpace(string(runes[:maxLen]))
	}
	return raw
}

func inferTitleFromURL(u *url.URL) string {
	if token := extractDocToken(u.String()); token != "" {
		return "lark-doc-" + token
	}
	return strings.Trim(u.Hostname(), " ")
}

func extractDocToken(rawURL string) string {
	// 优先匹配 /docx?/TOKEN
	if matches := reDocToken.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1]
	}
	// 其次匹配 /wiki/TOKEN
	if matches := reWikiToken.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func isLikelyURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func dedupeStrings(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

func firstNonEmpty(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return item
		}
	}
	return ""
}
