package prd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/DreamCats/coco-ext/internal/config"
	"github.com/DreamCats/coco-ext/internal/generator"
)

type ContextSnapshot struct {
	GlossaryPath     string
	ArchitecturePath string
	PatternsPath     string
	GotchasPath      string
	GlossaryContent  string
	Architecture     string
	Patterns         string
	Gotchas          string
	GlossaryEntries  []GlossaryEntry
}

type GlossaryEntry struct {
	Business   string
	Identifier string
	Module     string
}

type RefinedSections struct {
	Summary       string
	Features      []string
	Boundaries    []string
	BusinessRules []string
	OpenQuestions []string
	Raw           string
}

type ResearchFinding struct {
	MatchedTerms   []GlossaryEntry
	UnmatchedTerms []string
	CandidateFiles []string
	CandidateDirs  []string
	Notes          []string
}

type ComplexityDimension struct {
	Name   string
	Score  int
	Reason string
}

type ComplexityAssessment struct {
	Dimensions []ComplexityDimension
	Total      int
	Level      string
	Conclusion string
}

type PlanArtifacts struct {
	DesignPath string
	PlanPath   string
	UsedAI     bool
}

type PlanBuild struct {
	Task       *TaskStatusReport
	Context    *ContextSnapshot
	Sections   RefinedSections
	Findings   ResearchFinding
	Assessment ComplexityAssessment
}

var rePlanASCIIWord = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_-]{1,}`)

func PreparePlanBuild(repoRoot, taskID string) (*PlanBuild, error) {
	task, err := LoadTaskStatus(repoRoot, taskID)
	if err != nil {
		return nil, err
	}

	refinedPath := filepath.Join(task.TaskDir, "prd-refined.md")
	refinedContent, err := os.ReadFile(refinedPath)
	if err != nil {
		return nil, fmt.Errorf("读取 prd-refined.md 失败: %w", err)
	}

	context, err := LoadContextSnapshot(repoRoot)
	if err != nil {
		return nil, err
	}

	sections := ParseRefinedSections(string(refinedContent))
	findings := ResearchCodebase(repoRoot, task.Metadata.Title, sections, context)
	assessment := ScoreComplexity(sections, findings)

	return &PlanBuild{
		Task:       task,
		Context:    context,
		Sections:   sections,
		Findings:   findings,
		Assessment: assessment,
	}, nil
}

func GeneratePlan(repoRoot, taskID string, now time.Time) (*PlanArtifacts, error) {
	build, err := PreparePlanBuild(repoRoot, taskID)
	if err != nil {
		return nil, err
	}

	designContent := BuildDesignContent(build.Task, build.Context, build.Sections, build.Findings, build.Assessment)
	planContent := BuildPlanContent(build.Task, build.Sections, build.Findings, build.Assessment)
	return writePlanArtifacts(build.Task, designContent, planContent, now, false)
}

func GeneratePlanWithAI(gen *generator.Generator, repoRoot, taskID string, now time.Time, onChunk func(string)) (*PlanArtifacts, error) {
	build, err := PreparePlanBuild(repoRoot, taskID)
	if err != nil {
		return nil, err
	}

	localDesign := BuildDesignContent(build.Task, build.Context, build.Sections, build.Findings, build.Assessment)
	localPlan := BuildPlanContent(build.Task, build.Sections, build.Findings, build.Assessment)

	if gen == nil {
		return writePlanArtifacts(build.Task, localDesign, localPlan, now, false)
	}

	prompt := BuildPlanPrompt(build)
	raw, err := gen.PromptWithTimeout(prompt, config.ReviewPromptTimeout, onChunk)
	if err != nil {
		return writePlanArtifacts(build.Task, localDesign, localPlan, now, false)
	}

	aiPlan, ok := ExtractPlanOutputs(raw)
	if !ok {
		return writePlanArtifacts(build.Task, localDesign, localPlan, now, false)
	}
	if err := ValidatePlanOutputs(build, aiPlan); err != nil {
		return writePlanArtifacts(build.Task, localDesign, localPlan, now, false)
	}

	return writePlanArtifacts(build.Task, localDesign, aiPlan, now, true)
}

func LoadContextSnapshot(repoRoot string) (*ContextSnapshot, error) {
	contextDir := filepath.Join(repoRoot, ".livecoding", "context")
	required := map[string]*string{
		"glossary.md":     nil,
		"architecture.md": nil,
		"patterns.md":     nil,
	}

	for name := range required {
		path := filepath.Join(contextDir, name)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("context 不完整，缺少 %s；请先执行 `coco-ext context init` 或 `coco-ext context update`", name)
			}
			return nil, fmt.Errorf("读取 context 文件失败: %w", err)
		}
	}

	glossaryPath := filepath.Join(contextDir, "glossary.md")
	architecturePath := filepath.Join(contextDir, "architecture.md")
	patternsPath := filepath.Join(contextDir, "patterns.md")
	gotchasPath := filepath.Join(contextDir, "gotchas.md")

	glossaryContent, err := os.ReadFile(glossaryPath)
	if err != nil {
		return nil, fmt.Errorf("读取 glossary.md 失败: %w", err)
	}
	architectureContent, err := os.ReadFile(architecturePath)
	if err != nil {
		return nil, fmt.Errorf("读取 architecture.md 失败: %w", err)
	}
	patternsContent, err := os.ReadFile(patternsPath)
	if err != nil {
		return nil, fmt.Errorf("读取 patterns.md 失败: %w", err)
	}

	gotchasContent := ""
	if data, err := os.ReadFile(gotchasPath); err == nil {
		gotchasContent = string(data)
	}

	return &ContextSnapshot{
		GlossaryPath:     glossaryPath,
		ArchitecturePath: architecturePath,
		PatternsPath:     patternsPath,
		GotchasPath:      gotchasPath,
		GlossaryContent:  string(glossaryContent),
		Architecture:     string(architectureContent),
		Patterns:         string(patternsContent),
		Gotchas:          gotchasContent,
		GlossaryEntries:  parseGlossaryEntries(string(glossaryContent)),
	}, nil
}

func ParseRefinedSections(content string) RefinedSections {
	sections := splitMarkdownSections(content)
	return RefinedSections{
		Summary:       cleanSectionLines(sections["需求概述"]),
		Features:      extractBulletItems(sections["功能点"]),
		Boundaries:    extractBulletItems(sections["边界条件"]),
		BusinessRules: extractBulletItems(sections["业务规则"]),
		OpenQuestions: extractBulletItems(sections["待确认问题"]),
		Raw:           strings.TrimSpace(content),
	}
}

func ResearchCodebase(repoRoot, title string, sections RefinedSections, context *ContextSnapshot) ResearchFinding {
	searchText := strings.Join([]string{
		title,
		sections.Summary,
		strings.Join(sections.Features, "\n"),
		strings.Join(sections.BusinessRules, "\n"),
	}, "\n")

	matched := make([]GlossaryEntry, 0)
	for _, entry := range context.GlossaryEntries {
		if containsAny(searchText, entry.Business, entry.Identifier) {
			matched = append(matched, entry)
		}
	}

	unmatched := inferUnmatchedTerms(searchText, matched)
	searchTerms := inferSearchTerms(title, sections, matched)
	candidateFiles := make([]string, 0, 8)
	if shouldFocusPRDWorkflow(searchTerms) {
		candidateFiles = preferPRDWorkflowFiles(searchTerms)
	} else {
		candidateFiles = findCandidateFiles(repoRoot, matched, searchTerms)
	}
	if len(candidateFiles) == 0 {
		candidateFiles = heuristicCandidateFiles(repoRoot, searchTerms)
	}
	candidateDirs := summarizeDirs(candidateFiles)

	notes := make([]string, 0, 4)
	if len(matched) == 0 {
		notes = append(notes, "未在 glossary 中命中明显术语，调研可信度较低。")
	}
	if len(candidateFiles) == 0 {
		notes = append(notes, "未通过现有术语映射找到候选代码文件。")
	}
	if len(sections.OpenQuestions) > 0 {
		notes = append(notes, fmt.Sprintf("存在 %d 个待确认问题，说明需求仍有不确定性。", len(sections.OpenQuestions)))
	}

	return ResearchFinding{
		MatchedTerms:   matched,
		UnmatchedTerms: unmatched,
		CandidateFiles: candidateFiles,
		CandidateDirs:  candidateDirs,
		Notes:          notes,
	}
}

func shouldFocusPRDWorkflow(searchTerms []string) bool {
	for _, term := range searchTerms {
		switch strings.ToLower(term) {
		case "prd", "plan", "refine", "status", "source", "task", "lark", "feishu", "doc", "docx", "url":
			return true
		}
	}
	return false
}

func ScoreComplexity(sections RefinedSections, findings ResearchFinding) ComplexityAssessment {
	dimensions := make([]ComplexityDimension, 0, 6)

	fileCount := len(findings.CandidateFiles)
	scopeScore := 0
	scopeReason := "候选改动文件较少，范围集中。"
	switch {
	case fileCount > 5:
		scopeScore = 2
		scopeReason = "候选改动文件超过 5 个，范围偏大。"
	case fileCount > 2:
		scopeScore = 1
		scopeReason = "候选改动文件在 3-5 个之间，范围中等。"
	}
	dimensions = append(dimensions, ComplexityDimension{Name: "改动范围", Score: scopeScore, Reason: scopeReason})

	interfaceScore := 0
	interfaceReason := "未发现明显的接口或协议变更信号。"
	if containsAny(strings.Join(sections.Features, "\n"), "接口", "协议", "请求", "返回", "字段") {
		interfaceScore = 1
		interfaceReason = "需求描述中包含接口/字段类变更信号。"
	}
	if hasPathKeyword(findings.CandidateFiles, "handler", ".proto", ".thrift") {
		interfaceScore = 2
		interfaceReason = "候选文件涉及 handler/IDL，可能影响对外接口。"
	}
	dimensions = append(dimensions, ComplexityDimension{Name: "接口协议", Score: interfaceScore, Reason: interfaceReason})

	dataScore := 0
	dataReason := "未发现复杂数据或持久化变更。"
	if containsAny(strings.Join(sections.Boundaries, "\n"), "状态", "缓存", "数据库", "表", "持久化") {
		dataScore = 1
		dataReason = "边界条件中出现状态/数据类描述。"
	}
	if containsAny(strings.Join(sections.BusinessRules, "\n"), "状态流转", "一致性", "数据同步") {
		dataScore = 2
		dataReason = "业务规则暗示存在复杂状态流转或一致性要求。"
	}
	dimensions = append(dimensions, ComplexityDimension{Name: "数据状态", Score: dataScore, Reason: dataReason})

	questionCount := len(sections.OpenQuestions)
	ruleScore := 0
	ruleReason := "业务规则相对清晰。"
	switch {
	case questionCount > 4:
		ruleScore = 2
		ruleReason = "待确认问题较多，业务规则仍不清晰。"
	case questionCount > 1:
		ruleScore = 1
		ruleReason = "存在少量待确认问题，需要人工确认。"
	}
	dimensions = append(dimensions, ComplexityDimension{Name: "规则清晰度", Score: ruleScore, Reason: ruleReason})

	dependencyScore := 0
	dependencyReason := "候选目录较集中，依赖面可控。"
	switch {
	case len(findings.CandidateDirs) > 2:
		dependencyScore = 2
		dependencyReason = "候选目录跨多个模块，可能需要跨模块协作。"
	case len(findings.CandidateDirs) > 1:
		dependencyScore = 1
		dependencyReason = "候选目录跨两个模块，存在一定依赖关系。"
	}
	dimensions = append(dimensions, ComplexityDimension{Name: "依赖联动", Score: dependencyScore, Reason: dependencyReason})

	verifyScore := 0
	verifyReason := "需求较易验证。"
	if len(findings.UnmatchedTerms) > 0 {
		verifyScore = 1
		verifyReason = "存在 glossary 未命中的术语，调研结果需要额外验证。"
	}
	if len(findings.CandidateFiles) == 0 {
		verifyScore = 2
		verifyReason = "未找到候选文件，当前无法形成可靠实现方案。"
	}
	dimensions = append(dimensions, ComplexityDimension{Name: "验证风险", Score: verifyScore, Reason: verifyReason})

	total := 0
	for _, dimension := range dimensions {
		total += dimension.Score
	}

	level := "简单"
	conclusion := "复杂度较低，可以进入详细编码计划阶段。"
	switch {
	case total > 6:
		level = "复杂"
		conclusion = "复杂度超过阈值，建议先人工拆解或补充上下文，不直接进入自动实现。"
	case total > 3:
		level = "中等"
		conclusion = "复杂度中等，可以生成计划，但需重点关注风险与待确认项。"
	}

	return ComplexityAssessment{
		Dimensions: dimensions,
		Total:      total,
		Level:      level,
		Conclusion: conclusion,
	}
}

func BuildPlanContent(task *TaskStatusReport, sections RefinedSections, findings ResearchFinding, assessment ComplexityAssessment) string {
	var b strings.Builder
	b.WriteString("# Plan\n\n")
	b.WriteString(fmt.Sprintf("- task_id: %s\n", task.TaskID))
	b.WriteString(fmt.Sprintf("- title: %s\n", task.Metadata.Title))
	b.WriteString(fmt.Sprintf("- complexity: %s (%d)\n\n", assessment.Level, assessment.Total))

	if assessment.Total > 6 {
		b.WriteString("## 结论\n\n")
		b.WriteString("- 当前需求被判定为复杂，暂不建议直接进入自动 codegen。\n")
		b.WriteString("- 建议先人工拆分需求、补充上下文或补全 PRD 后再重新执行 `coco-ext prd plan`。\n\n")
	} else {
		b.WriteString("## 实现目标\n\n")
		for _, feature := range sections.Features {
			b.WriteString(fmt.Sprintf("- %s\n", feature))
		}
		if len(sections.Features) == 0 {
			b.WriteString("- 基于 refined PRD 补全实现目标。\n")
		}
		b.WriteString("\n")

		b.WriteString("## 拟改文件\n\n")
		if len(findings.CandidateFiles) == 0 {
			b.WriteString("- 暂未命中候选文件，需要补充 context 或人工指定模块。\n")
		} else {
			for _, file := range findings.CandidateFiles {
				b.WriteString(fmt.Sprintf("- %s：%s\n", file, suggestFileAction(file)))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("## 待确认项\n\n")
	if len(sections.OpenQuestions) == 0 {
		b.WriteString("- 无额外待确认项。\n")
	} else {
		for _, question := range sections.OpenQuestions {
			b.WriteString(fmt.Sprintf("- %s\n", question))
		}
	}
	b.WriteString("\n")

	b.WriteString("## 验证建议\n\n")
	b.WriteString("- 仅编译涉及的 package，不执行全仓 build/test。\n")
	b.WriteString("- 完成实现后建议运行 `coco-ext review` 或 `/livecoding:auto-review`。\n")
	return b.String()
}

func BuildPlanPrompt(build *PlanBuild) string {
	var b strings.Builder
	b.WriteString("你是一名资深技术方案与研发计划助手。系统已经根据固定模板生成 design.md，你只需要基于提供的 PRD refined 内容、本地 context 事实和代码调研结果，输出更完善的 plan.md。\n\n")
	b.WriteString("要求：\n")
	b.WriteString("1. 只能基于提供的信息工作，不要编造未出现的模块、文件或接口。\n")
	b.WriteString("2. 你需要重新进行复杂度打分，并给出简单/中等/复杂结论。\n")
	b.WriteString("3. 如果总分 > 6，plan 中明确写出“不建议自动实现”。\n")
	b.WriteString("4. 输出必须严格使用下面的标记格式：\n")
	b.WriteString("=== PLAN ===\n")
	b.WriteString("# Plan ...\n")
	b.WriteString("5. 不要输出其它前言或解释。\n\n")

	b.WriteString("## PRD Refined\n")
	b.WriteString(build.Sections.Raw)
	b.WriteString("\n\n## Context 摘要\n")
	b.WriteString("- glossary 命中术语：\n")
	if len(build.Findings.MatchedTerms) == 0 {
		b.WriteString("  - 无\n")
	} else {
		for _, entry := range build.Findings.MatchedTerms {
			b.WriteString(fmt.Sprintf("  - %s -> %s (%s)\n", entry.Business, entry.Identifier, entry.Module))
		}
	}
	b.WriteString("- 未命中术语：\n")
	if len(build.Findings.UnmatchedTerms) == 0 {
		b.WriteString("  - 无\n")
	} else {
		for _, term := range build.Findings.UnmatchedTerms {
			b.WriteString(fmt.Sprintf("  - %s\n", term))
		}
	}

	b.WriteString("\n## 本地调研结果\n")
	b.WriteString(fmt.Sprintf("- candidate_files_count: %d\n", len(build.Findings.CandidateFiles)))
	for _, file := range build.Findings.CandidateFiles {
		b.WriteString(fmt.Sprintf("  - %s\n", file))
	}
	b.WriteString(fmt.Sprintf("- candidate_dirs_count: %d\n", len(build.Findings.CandidateDirs)))
	for _, dir := range build.Findings.CandidateDirs {
		b.WriteString(fmt.Sprintf("  - %s\n", dir))
	}
	b.WriteString("- 本地风险备注：\n")
	if len(build.Findings.Notes) == 0 {
		b.WriteString("  - 无\n")
	} else {
		for _, note := range build.Findings.Notes {
			b.WriteString(fmt.Sprintf("  - %s\n", note))
		}
	}

	b.WriteString("\n## 本地基线复杂度评分\n")
	b.WriteString(fmt.Sprintf("- total: %d\n", build.Assessment.Total))
	b.WriteString(fmt.Sprintf("- level: %s\n", build.Assessment.Level))
	for _, dim := range build.Assessment.Dimensions {
		b.WriteString(fmt.Sprintf("  - %s: %d | %s\n", dim.Name, dim.Score, dim.Reason))
	}
	return b.String()
}

func ExtractPlanOutputs(raw string) (plan string, ok bool) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	planMarker := "=== PLAN ==="
	planIndex := strings.Index(normalized, planMarker)
	if planIndex == -1 {
		return "", false
	}
	plan = strings.TrimSpace(normalized[planIndex+len(planMarker):])
	if !strings.HasPrefix(plan, "# Plan") {
		return "", false
	}
	return plan + "\n", true
}

func ExtractPlanStream(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	planMarker := "=== PLAN ==="
	index := strings.Index(normalized, planMarker)
	if index == -1 {
		return ""
	}
	return strings.TrimSpace(normalized[index:])
}

func ValidatePlanOutputs(build *PlanBuild, plan string) error {
	combined := plan
	for _, marker := range []string{"(待生成)", "(待确认)", "未初始化"} {
		if strings.Contains(combined, marker) {
			return fmt.Errorf("AI 输出包含无效占位符: %s", marker)
		}
	}

	if !strings.Contains(plan, build.Task.TaskID) {
		return fmt.Errorf("AI plan 缺少正确 task_id")
	}
	return nil
}

func writePlanArtifacts(task *TaskStatusReport, designContent, planContent string, now time.Time, usedAI bool) (*PlanArtifacts, error) {
	designPath := filepath.Join(task.TaskDir, "design.md")
	planPath := filepath.Join(task.TaskDir, "plan.md")
	if err := os.WriteFile(designPath, []byte(designContent), 0644); err != nil {
		return nil, fmt.Errorf("写入 design.md 失败: %w", err)
	}
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		return nil, fmt.Errorf("写入 plan.md 失败: %w", err)
	}

	if err := updateTaskStatus(task.TaskDir, TaskStatusPlanned, now); err != nil {
		return nil, err
	}
	return &PlanArtifacts{DesignPath: designPath, PlanPath: planPath, UsedAI: usedAI}, nil
}

func updateTaskStatus(taskDir, status string, now time.Time) error {
	metaPath := filepath.Join(taskDir, "task.json")
	meta, err := readTaskMetadata(metaPath)
	if err != nil {
		return err
	}
	meta.Status = status
	meta.UpdatedAt = now
	return writeJSONFile(metaPath, meta)
}

func parseGlossaryEntries(content string) []GlossaryEntry {
	lines := strings.Split(content, "\n")
	entries := make([]GlossaryEntry, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		if strings.Contains(line, "---") || strings.Contains(line, "业务术语") {
			continue
		}
		parts := splitTableLine(line)
		if len(parts) < 4 {
			continue
		}
		business := strings.TrimSpace(parts[0])
		identifier := strings.TrimSpace(parts[1])
		module := strings.TrimSpace(parts[3])
		if business == "" || identifier == "" {
			continue
		}
		entries = append(entries, GlossaryEntry{
			Business:   business,
			Identifier: identifier,
			Module:     module,
		})
	}
	return entries
}

func splitTableLine(line string) []string {
	trimmed := strings.Trim(line, "|")
	rawParts := strings.Split(trimmed, "|")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		parts = append(parts, strings.TrimSpace(part))
	}
	return parts
}

func splitMarkdownSections(content string) map[string]string {
	lines := strings.Split(content, "\n")
	sections := make(map[string]string)
	current := ""
	var currentLines []string
	flush := func() {
		if current == "" {
			return
		}
		sections[current] = strings.TrimSpace(strings.Join(currentLines, "\n"))
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			flush()
			current = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			currentLines = currentLines[:0]
			continue
		}
		if current != "" {
			currentLines = append(currentLines, line)
		}
	}
	flush()
	return sections
}

func cleanSectionLines(section string) string {
	lines := extractBulletItems(section)
	if len(lines) == 0 {
		return strings.TrimSpace(section)
	}
	return strings.Join(lines, "；")
}

func extractBulletItems(section string) []string {
	lines := strings.Split(section, "\n")
	items := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "1. ")
		line = strings.TrimPrefix(line, "2. ")
		line = strings.TrimPrefix(line, "3. ")
		line = strings.TrimPrefix(line, "4. ")
		line = strings.TrimPrefix(line, "5. ")
		if line != "" {
			items = append(items, line)
		}
	}
	return items
}

func inferUnmatchedTerms(searchText string, matched []GlossaryEntry) []string {
	terms := []string{}
	for _, keyword := range []string{"讲解卡", "倒计时", "拍卖", "隐藏", "已结束"} {
		if containsAny(searchText, keyword) && !matchedContainsBusiness(matched, keyword) {
			terms = append(terms, keyword)
		}
	}
	return dedupeTerms(terms)
}

func matchedContainsBusiness(entries []GlossaryEntry, keyword string) bool {
	for _, entry := range entries {
		if entry.Business == keyword {
			return true
		}
	}
	return false
}

func dedupeTerms(items []string) []string {
	seen := map[string]bool{}
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

func containsAny(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if keyword != "" && strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func findCandidateFiles(repoRoot string, matched []GlossaryEntry, terms []string) []string {
	result := make([]string, 0, 8)
	seen := make(map[string]bool)
	preferred := preferPRDWorkflowFiles(terms)

	for _, path := range preferred {
		if seen[path] {
			continue
		}
		seen[path] = true
		result = append(result, path)
		if len(result) >= 6 {
			return result
		}
	}

	if len(result) >= 4 {
		return result
	}

	for _, entry := range matched {
		for _, term := range []string{entry.Identifier, entry.Business} {
			for _, file := range searchFiles(repoRoot, term) {
				if seen[file] {
					continue
				}
				seen[file] = true
				result = append(result, file)
				if len(result) >= 8 {
					return result
				}
			}
		}
	}

	for _, term := range terms {
		for _, file := range searchFiles(repoRoot, term) {
			if seen[file] {
				continue
			}
			seen[file] = true
			result = append(result, file)
			if len(result) >= 8 {
				sort.Strings(result)
				return result
			}
		}
	}
	sort.Strings(result)
	return result
}

func searchFiles(repoRoot, term string) []string {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil
	}

	cmd := exec.Command("rg", "--files-with-matches", "--glob", "*.go", term, ".")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		output = nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ".livecoding/") {
			continue
		}
		files = append(files, line)
	}

	if len(files) < 3 {
		fileCmd := exec.Command("rg", "--files", ".", "--glob", "*.go")
		fileCmd.Dir = repoRoot
		fileOutput, fileErr := fileCmd.Output()
		if fileErr == nil {
			lowerTerm := strings.ToLower(term)
			for _, line := range strings.Split(strings.TrimSpace(string(fileOutput)), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, ".livecoding/") {
					continue
				}
				if strings.Contains(strings.ToLower(line), lowerTerm) {
					files = append(files, line)
				}
			}
		}
	}

	return dedupeAndSort(files)
}

func heuristicCandidateFiles(repoRoot string, searchTerms []string) []string {
	preferred := preferPRDWorkflowFiles(searchTerms)
	if len(preferred) >= 8 {
		return preferred[:8]
	}

	fileCmd := exec.Command("rg", "--files", ".", "--glob", "*.go")
	fileCmd.Dir = repoRoot
	output, err := fileCmd.Output()
	if err != nil {
		return preferred
	}

	priorityKeywords := make([]string, 0, 12)
	for _, term := range searchTerms {
		switch term {
		case "prd", "plan", "refine", "status", "source", "lark", "feishu", "doc", "docx", "url", "task":
			priorityKeywords = append(priorityKeywords, term)
		}
	}
	if len(priorityKeywords) == 0 {
		priorityKeywords = []string{"prd", "plan", "refine", "status", "source"}
	}

	type scoredFile struct {
		path  string
		score int
	}
	scored := make([]scoredFile, 0, 16)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ".livecoding/") {
			continue
		}
		lower := strings.ToLower(line)
		score := 0
		for _, keyword := range priorityKeywords {
			if strings.Contains(lower, keyword) {
				score++
			}
		}
		if strings.Contains(lower, "cmd/prd_") || strings.Contains(lower, "internal/prd/") {
			score += 4
		}
		if strings.Contains(lower, "refine") || strings.Contains(lower, "status") || strings.Contains(lower, "source") {
			score += 2
		}
		if strings.Contains(lower, "cmd/") {
			score++
		}
		if score > 0 {
			scored = append(scored, scoredFile{path: line, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].path < scored[j].path
		}
		return scored[i].score > scored[j].score
	})

	result := make([]string, 0, len(scored))
	seen := make(map[string]bool, len(scored)+len(preferred))
	for _, path := range preferred {
		seen[path] = true
		result = append(result, path)
	}
	for _, item := range scored {
		if seen[item.path] {
			continue
		}
		seen[item.path] = true
		result = append(result, item.path)
		if len(result) >= 8 {
			break
		}
	}
	return result
}

func preferPRDWorkflowFiles(searchTerms []string) []string {
	candidates := []string{
		"./cmd/prd_refine.go",
		"./internal/prd/refine.go",
		"./cmd/prd_status.go",
		"./internal/prd/status.go",
		"./cmd/prd_plan.go",
		"./internal/prd/plan.go",
	}
	if len(searchTerms) == 0 {
		return candidates
	}

	matchedWorkflow := false
	for _, term := range searchTerms {
		switch strings.ToLower(term) {
		case "prd", "plan", "refine", "status", "source", "task", "lark", "feishu", "doc", "docx", "url":
			matchedWorkflow = true
		}
	}
	if matchedWorkflow {
		return candidates
	}
	return nil
}

func inferSearchTerms(title string, sections RefinedSections, matched []GlossaryEntry) []string {
	sourceText := strings.Join([]string{
		title,
		sections.Summary,
		strings.Join(sections.Features, "\n"),
		strings.Join(sections.Boundaries, "\n"),
		strings.Join(sections.BusinessRules, "\n"),
		strings.Join(sections.OpenQuestions, "\n"),
	}, "\n")

	terms := make([]string, 0, 24)
	for _, entry := range matched {
		terms = append(terms, entry.Business, entry.Identifier)
	}

	for _, token := range rePlanASCIIWord.FindAllString(sourceText, -1) {
		token = strings.ToLower(strings.TrimSpace(token))
		switch token {
		case "the", "and", "for", "with", "that", "this", "from":
			continue
		}
		terms = append(terms, token)
	}

	for keyword, expansions := range map[string][]string{
		"飞书":  {"lark", "feishu", "doc", "docx"},
		"文档":  {"doc", "document", "source"},
		"链接":  {"link", "url", "source"},
		"来源":  {"source", "doc_token"},
		"下一步": {"status", "next", "task"},
		"任务":  {"task", "status"},
		"提示":  {"status", "message"},
		"体验":  {"refine", "status", "source"},
		"需求":  {"prd", "refine", "plan"},
	} {
		if strings.Contains(sourceText, keyword) {
			terms = append(terms, expansions...)
		}
	}

	if strings.Contains(strings.ToLower(sourceText), "prd") {
		terms = append(terms, "prd", "refine", "plan", "status")
	}
	if strings.Contains(strings.ToLower(sourceText), "coco-ext") {
		terms = append(terms, "prd_refine", "prd_status", "refine", "status", "source")
	}

	return dedupeAndSort(terms)
}

func dedupeAndSort(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func summarizeDirs(files []string) []string {
	seen := make(map[string]bool)
	dirs := make([]string, 0, len(files))
	for _, file := range files {
		dir := filepath.Dir(file)
		if dir == "." || seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

func hasPathKeyword(files []string, keywords ...string) bool {
	for _, file := range files {
		for _, keyword := range keywords {
			if strings.Contains(file, keyword) {
				return true
			}
		}
	}
	return false
}

func suggestFileAction(file string) string {
	switch {
	case strings.Contains(file, "/handler/"):
		return "评估接口层入参/返回或展示逻辑是否需要调整。"
	case strings.Contains(file, "/service/"):
		return "评估业务逻辑和下游调用是否需要补充。"
	case strings.Contains(file, "/converter/"):
		return "优先检查字段映射和 response 拼装逻辑。"
	case strings.Contains(file, "/model/"):
		return "检查结构体字段或状态定义是否需要扩展。"
	default:
		return "作为候选实现文件，需人工确认是否纳入本次改动范围。"
	}
}
