package prd

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

const defaultDesignTemplateName = "content-ecommerce"

//go:embed templates/design/*.md.tmpl
var designTemplateFS embed.FS

type DesignInfoItem struct {
	Label string
	Value string
}

type DesignRegionMode struct {
	Region string
	Mode   string
	Note   string
}

type DesignManpowerRow struct {
	PSM      string
	Content  string
	Effort   string
	Owner    string
}

type DesignDiagram struct {
	Title   string
	Type    string
	Content string
}

type DesignTemplateData struct {
	Title               string
	TaskID              string
	Status              string
	ComplexityLevel     string
	ComplexityTotal     int
	InfoItems           []DesignInfoItem
	Why                 string
	ChangePoints        []string
	ScopeFiles          []string
	ExcludedScope       []string
	SystemDesign        []string
	IDLChanges          []string
	StorageChanges      []string
	DependencyChanges   []string
	ExperimentChanges   []string
	ShowMultiRegion     bool
	RegionModes         []DesignRegionMode
	ShowRegionIsolation bool
	RegionIsolation     []string
	ShowCompliance      bool
	ComplianceNotes     []string
	MonitoringNotes     []string
	PerformanceNotes    []string
	QAInputs            []string
	RolloutPlan         []string
	RollbackPlan        []string
	ManpowerRows        []DesignManpowerRow
	PostLaunchNotes     []string
	DesignNotes         []string
	Diagrams            []DesignDiagram
}

func BuildDesignContent(task *TaskStatusReport, context *ContextSnapshot, sections RefinedSections, findings ResearchFinding, assessment ComplexityAssessment) string {
	data := BuildDesignTemplateData(task, context, sections, findings, assessment)
	content, err := RenderDesignTemplate(defaultDesignTemplateName, data)
	if err != nil {
		return BuildFallbackDesignContent(task, context, sections, findings, assessment)
	}
	return content
}

func BuildDesignTemplateData(task *TaskStatusReport, context *ContextSnapshot, sections RefinedSections, findings ResearchFinding, assessment ComplexityAssessment) DesignTemplateData {
	sourceValue := task.Metadata.SourceValue
	if task.Source != nil {
		switch {
		case task.Source.URL != "":
			sourceValue = fmt.Sprintf("[%s](%s)", task.Source.Title, task.Source.URL)
		case task.Source.Path != "":
			sourceValue = task.Source.Path
		case task.Source.Title != "":
			sourceValue = task.Source.Title
		}
	}
	if strings.TrimSpace(sourceValue) == "" {
		sourceValue = task.Metadata.Title
	}

	changePoints := make([]string, 0, 3)
	for _, feature := range sections.Features {
		if strings.TrimSpace(feature) == "" {
			continue
		}
		changePoints = append(changePoints, feature)
		if len(changePoints) >= 3 {
			break
		}
	}
	if len(changePoints) == 0 {
		changePoints = append(changePoints,
			"补齐飞书 PRD 链接场景下的来源信息展示。",
			"统一 task 状态页中的下一步操作提示。",
			"确保 refine / status / plan 链路中的来源体验一致。",
		)
	}

	scopeFiles := findings.CandidateFiles
	if len(scopeFiles) == 0 {
		scopeFiles = []string{"当前未命中候选文件，需要补充 context 或人工确认模块。"}
	}

	excludedScope := []string{
		"不引入新的外部服务依赖或服务端接口。",
		"不在本次需求中实现飞书正文抓取。",
		"不调整与当前需求无关的 review / gcmsg 流程。",
	}

	systemDesign := buildDesignSystemNotes(findings, assessment)
	idlChanges := []string{"本次需求不涉及 IDL、协议字段或公开 API 变更。"}
	storageChanges := []string{"本次需求不涉及数据库、持久化模型或配置中心变更。"}
	dependencyChanges := []string{"不新增下游服务依赖，仅复用现有本地 task/context 产物。"}
	experimentChanges := []string{"本次需求不涉及实验平台或新增埋点。"}
	monitoringNotes := []string{
		"关注 `prd refine` / `prd status` / `prd plan` 在飞书链接场景下的输出是否一致。",
		"关注 task 目录内 `source.json`、`prd.source.md` 与状态提示是否正确落盘。",
	}
	performanceNotes := []string{
		"本次改动以 CLI 文本处理和本地文件落盘为主，不引入线上流量。",
		"继续保持最小范围编译和本地搜索，不做全仓构建或全量测试。",
	}
	qaInputs := buildDesignQAInputs(sections, findings)
	rolloutPlan := []string{
		"通过发布新的 coco-ext 二进制版本生效，无额外发布顺序依赖。",
		"上线后重点验证飞书链接输入、任务状态展示和下一步指引三个关键路径。",
	}
	rollbackPlan := []string{
		"如出现问题，直接回滚到上一版本 coco-ext 二进制。",
		"如 task 产物格式需要回退，仅删除本次新增或变更的 task 目录后重新执行旧版本命令。",
	}
	manpowerRows := []DesignManpowerRow{
		{
			PSM:     "coco-ext.prd",
			Content: "PRD 来源体验优化、任务状态提示完善、方案模板渲染",
			Effort:  estimateManpower(assessment),
			Owner:   "待补充",
		},
	}
	postLaunchNotes := []string{
		"上线后补充真实使用反馈，包括飞书链接场景的命中率与误导提示情况。",
		"评估是否需要进一步支持飞书正文拉取与模板切换配置。",
	}

	designNotes := make([]string, 0, len(findings.Notes))
	for _, note := range findings.Notes {
		designNotes = append(designNotes, note)
	}
	if len(findings.UnmatchedTerms) > 0 {
		designNotes = append(designNotes, "存在 glossary 未命中的术语，需结合实际代码进一步人工确认。")
	}

	return DesignTemplateData{
		Title:           task.Metadata.Title,
		TaskID:          task.TaskID,
		Status:          TaskStatusPlanned,
		ComplexityLevel: assessment.Level,
		ComplexityTotal: assessment.Total,
		InfoItems: []DesignInfoItem{
			{Label: "PRD / Bug单(Hotfix)", Value: sourceValue},
			{Label: "Meego", Value: "N/A"},
			{Label: "PM", Value: "待补充"},
			{Label: "Tech Owner", Value: "待补充"},
			{Label: "Server", Value: "coco-ext"},
			{Label: "FE", Value: "N/A"},
			{Label: "Client", Value: "N/A"},
			{Label: "QA", Value: "待补充"},
			{Label: "客户端跟版版本(如有)", Value: "N/A"},
			{Label: "PPE环境", Value: "N/A"},
		},
		Why:               buildDesignWhy(task, sections),
		ChangePoints:      changePoints,
		ScopeFiles:        scopeFiles,
		ExcludedScope:     excludedScope,
		SystemDesign:      systemDesign,
		IDLChanges:        idlChanges,
		StorageChanges:    storageChanges,
		DependencyChanges: dependencyChanges,
		ExperimentChanges: experimentChanges,
		ShowMultiRegion:   false,
		RegionModes: []DesignRegionMode{
			{Region: "US", Mode: "N/A", Note: "本需求为本地 CLI 能力，不区分区域。"},
			{Region: "UK", Mode: "N/A", Note: "本需求为本地 CLI 能力，不区分区域。"},
			{Region: "EU", Mode: "N/A", Note: "本需求为本地 CLI 能力，不区分区域。"},
			{Region: "JP", Mode: "N/A", Note: "本需求为本地 CLI 能力，不区分区域。"},
			{Region: "SEA", Mode: "N/A", Note: "本需求为本地 CLI 能力，不区分区域。"},
			{Region: "LATAM", Mode: "N/A", Note: "本需求为本地 CLI 能力，不区分区域。"},
		},
		ShowRegionIsolation: false,
		RegionIsolation: []string{
			"本次需求为本地 CLI 体验优化，不涉及区域隔离逻辑。",
		},
		ShowCompliance: false,
		ComplianceNotes: []string{
			"本次需求不涉及新增合规逻辑或 EU 特殊链路。",
		},
		MonitoringNotes:  monitoringNotes,
		PerformanceNotes: performanceNotes,
		QAInputs:         qaInputs,
		RolloutPlan:      rolloutPlan,
		RollbackPlan:     rollbackPlan,
		ManpowerRows:     manpowerRows,
		PostLaunchNotes:  postLaunchNotes,
		DesignNotes:      designNotes,
		Diagrams:         buildDesignDiagrams(task, sections, findings, assessment),
	}
}

func RenderDesignTemplate(name string, data DesignTemplateData) (string, error) {
	templatePath := fmt.Sprintf("templates/design/%s.md.tmpl", name)
	raw, err := designTemplateFS.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("读取 design 模板失败: %w", err)
	}

	tpl, err := template.New(name).Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("解析 design 模板失败: %w", err)
	}

	var b bytes.Buffer
	if err := tpl.Execute(&b, data); err != nil {
		return "", fmt.Errorf("渲染 design 模板失败: %w", err)
	}
	return strings.TrimSpace(b.String()) + "\n", nil
}

func BuildFallbackDesignContent(task *TaskStatusReport, context *ContextSnapshot, sections RefinedSections, findings ResearchFinding, assessment ComplexityAssessment) string {
	var b strings.Builder
	b.WriteString("# Design\n\n")
	b.WriteString(fmt.Sprintf("- task_id: %s\n", task.TaskID))
	b.WriteString(fmt.Sprintf("- title: %s\n", task.Metadata.Title))
	b.WriteString(fmt.Sprintf("- status: %s\n\n", TaskStatusPlanned))
	b.WriteString("## 调研摘要\n\n")
	if sections.Summary != "" {
		b.WriteString(sections.Summary + "\n\n")
	}
	b.WriteString("## Context 检查\n\n")
	b.WriteString(fmt.Sprintf("- glossary: %s\n", context.GlossaryPath))
	b.WriteString(fmt.Sprintf("- architecture: %s\n", context.ArchitecturePath))
	b.WriteString(fmt.Sprintf("- patterns: %s\n", context.PatternsPath))
	if context.GotchasPath != "" {
		b.WriteString(fmt.Sprintf("- gotchas: %s\n", context.GotchasPath))
	}
	b.WriteString("\n## 候选实现范围\n\n")
	for _, file := range findings.CandidateFiles {
		b.WriteString(fmt.Sprintf("- %s\n", file))
	}
	b.WriteString("\n## 复杂度评估\n\n")
	b.WriteString(fmt.Sprintf("- 总分: %d\n", assessment.Total))
	b.WriteString(fmt.Sprintf("- 等级: %s\n", assessment.Level))
	b.WriteString(fmt.Sprintf("- 结论: %s\n", assessment.Conclusion))
	return b.String()
}

func buildDesignWhy(task *TaskStatusReport, sections RefinedSections) string {
	if strings.TrimSpace(sections.Summary) != "" {
		return sections.Summary
	}
	return fmt.Sprintf("为 %s 补齐更清晰的任务来源与下一步指引，减少使用者在 PRD 流程中的理解成本和返工。", task.Metadata.Title)
}

func buildDesignSystemNotes(findings ResearchFinding, assessment ComplexityAssessment) []string {
	notes := make([]string, 0, 6)
	if len(findings.CandidateFiles) > 0 {
		notes = append(notes, "基于本地调研结果，在 PRD workflow 相关命令和内部实现层补齐飞书来源提示与状态展示能力。")
		notes = append(notes, fmt.Sprintf("候选实现文件共 %d 个，优先在现有 prd 子命令和 internal/prd 逻辑中收敛实现。", len(findings.CandidateFiles)))
	} else {
		notes = append(notes, "当前未命中明确候选文件，优先通过 context 和人工确认收敛实现范围。")
	}
	notes = append(notes, fmt.Sprintf("当前复杂度评估为 %s（%d 分），实现时优先保持最小改动范围，避免扩散到无关命令。", assessment.Level, assessment.Total))
	notes = append(notes, "设计上优先复用现有 task/source/status 产物，不新增额外状态机或跨命令耦合。")
	return notes
}

func buildDesignQAInputs(sections RefinedSections, findings ResearchFinding) []string {
	inputs := []string{
		"验证飞书链接输入场景下，task 是否能清晰展示来源文档信息。",
		"验证未同步正文时，用户是否能从 status/产物中清楚知道下一步操作。",
		"验证 text / file / lark_doc 三种 source 类型的表现是否一致且互不回归。",
	}
	for _, question := range sections.OpenQuestions {
		inputs = append(inputs, fmt.Sprintf("人工确认并补测待确认项：%s", question))
	}
	if len(findings.CandidateFiles) > 0 {
		inputs = append(inputs, fmt.Sprintf("重点回归候选实现文件相关路径：%s。", strings.Join(findings.CandidateFiles, "、")))
	}
	return inputs
}

func estimateManpower(assessment ComplexityAssessment) string {
	switch assessment.Level {
	case "简单":
		return "1-2pd"
	case "中等":
		return "3-5pd"
	default:
		return "5pd+"
	}
}

func buildDesignFlowchartMermaid(task *TaskStatusReport, findings ResearchFinding) string {
	var b strings.Builder
	b.WriteString("flowchart TD\n")
	b.WriteString("  A[用户输入 PRD 文本/文件/飞书链接] --> B[coco-ext prd refine]\n")
	b.WriteString("  B --> C[落盘 source.json / prd.source.md]\n")
	b.WriteString("  C --> D{daemon 可用?}\n")
	b.WriteString("  D -- 是 --> E[生成 prd-refined.md]\n")
	b.WriteString("  D -- 否 --> F[写入 fallback refined]\n")
	b.WriteString("  E --> G[coco-ext prd plan]\n")
	b.WriteString("  F --> G\n")
	b.WriteString("  G --> H[读取 context 与 prd-refined]\n")
	b.WriteString("  H --> I[本地调研与复杂度评估]\n")
	if len(findings.CandidateFiles) > 0 {
		b.WriteString("  I --> J[候选实现范围收敛到 prd workflow 文件]\n")
		b.WriteString("  J --> K{AI 方案是否通过校验?}\n")
	} else {
		b.WriteString("  I --> K{AI 方案是否通过校验?}\n")
	}
	b.WriteString("  K -- 是 --> L[写入 design.md / plan.md]\n")
	b.WriteString("  K -- 否 --> M[回退到本地 design / plan]\n")
	b.WriteString("  M --> L\n")
	b.WriteString(fmt.Sprintf("  L --> N[task %s 状态更新为 planned]\n", task.TaskID))
	return b.String()
}

func buildDesignSequenceMermaid(task *TaskStatusReport) string {
	return strings.TrimSpace(fmt.Sprintf(`
sequenceDiagram
  participant U as User
  participant C as coco-ext CLI
  participant T as Task Dir
  participant D as Coco Daemon
  participant M as Model

  U->>C: prd refine
  C->>T: 写 source.json / prd.source.md
  C->>D: 检查并连接 daemon
  alt daemon 可用
    D->>M: prompt refine
    M-->>D: refined PRD
    D-->>C: refined result
    C->>T: 写 prd-refined.md
  else daemon 不可用或结果无效
    C->>T: 写 fallback refined
  end

  U->>C: prd plan --task %s
  C->>T: 读 prd-refined.md / context
  C->>C: 本地调研与复杂度评估
  C->>D: 检查并连接 daemon
  alt AI plan 通过校验
    D->>M: prompt plan
    M-->>D: plan result
    D-->>C: validated plan
    C->>T: 写 design.md / plan.md
  else AI 不可用或结果未通过校验
    C->>T: 写本地 fallback design / plan
  end
  C->>T: 更新 task 状态为 planned
`, task.TaskID))
}

func buildDesignStateMermaid(task *TaskStatusReport) string {
	return strings.TrimSpace(fmt.Sprintf(`
stateDiagram-v2
  [*] --> task_created: prd refine
  task_created --> source_recorded: 写 source.json / prd.source.md
  source_recorded --> refined_ready: refined 成功
  source_recorded --> refined_fallback: refined 失败，写 fallback
  refined_ready --> planned: prd plan 成功
  refined_fallback --> planned: 基于 fallback 继续 plan
  planned --> [*]: task %s
`, task.TaskID))
}

func buildDesignDiagrams(task *TaskStatusReport, sections RefinedSections, findings ResearchFinding, assessment ComplexityAssessment) []DesignDiagram {
	diagrams := make([]DesignDiagram, 0, 3)

	if shouldShowWorkflowFlowchart(findings) {
		diagrams = append(diagrams, DesignDiagram{
			Title:   "调用流程图",
			Type:    "mermaid",
			Content: buildDesignFlowchartMermaid(task, findings),
		})
	}

	if shouldShowStateDiagram(task, sections) {
		diagrams = append(diagrams, DesignDiagram{
			Title:   "任务状态图",
			Type:    "mermaid",
			Content: buildDesignStateMermaid(task),
		})
	}

	if shouldShowSequenceDiagram(findings, assessment) {
		diagrams = append(diagrams, DesignDiagram{
			Title:   "时序图",
			Type:    "mermaid",
			Content: buildDesignSequenceMermaid(task),
		})
	}

	return diagrams
}

func shouldShowWorkflowFlowchart(findings ResearchFinding) bool {
	return len(findings.CandidateFiles) > 0
}

func shouldShowStateDiagram(task *TaskStatusReport, sections RefinedSections) bool {
	if task.Source != nil && task.Source.Type == SourceTypeLarkDoc {
		return true
	}

	joined := strings.Join([]string{
		task.Metadata.Title,
		sections.Summary,
		strings.Join(sections.Features, "\n"),
		strings.Join(sections.OpenQuestions, "\n"),
	}, "\n")
	return containsAny(joined, "来源", "状态", "下一步", "提示", "飞书", "链接")
}

func shouldShowSequenceDiagram(findings ResearchFinding, assessment ComplexityAssessment) bool {
	if assessment.Total >= 4 {
		return true
	}
	for _, file := range findings.CandidateFiles {
		if strings.Contains(file, "prd_refine") || strings.Contains(file, "prd_plan") {
			return true
		}
	}
	return false
}
