package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/config"
)

var metricsJSON bool

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "汇总本地 coco-ext 使用指标",
	Long:  "聚合 .livecoding/review 和 .livecoding/tasks 下的结构化产物，输出本地可观测指标。",
	RunE:  runMetrics,
}

type metricsReport struct {
	RepoRoot     string         `json:"repo_root"`
	GeneratedAt  string         `json:"generated_at"`
	Review       reviewMetrics  `json:"review"`
	Lint         lintMetrics    `json:"lint"`
	PRD          prdMetrics     `json:"prd"`
	Events       eventMetrics   `json:"events"`
	Recommendations []string    `json:"recommendations"`
}

type reviewMetrics struct {
	TotalRuns       int            `json:"total_runs"`
	StructuredRuns  int            `json:"structured_runs"`
	RatingCounts    map[string]int `json:"rating_counts"`
	P0Count         int            `json:"p0_count"`
	P1Count         int            `json:"p1_count"`
	P2Count         int            `json:"p2_count"`
	TotalFindings   int            `json:"total_findings"`
	LatestOutputDir string         `json:"latest_output_dir,omitempty"`
}

type lintMetrics struct {
	TotalRuns       int            `json:"total_runs"`
	TotalIssues     int            `json:"total_issues"`
	LinterCounts    map[string]int `json:"linter_counts"`
	LatestOutputDir string         `json:"latest_output_dir,omitempty"`
}

type prdMetrics struct {
	TotalTasks        int            `json:"total_tasks"`
	StatusCounts      map[string]int `json:"status_counts"`
	SourceTypeCounts  map[string]int `json:"source_type_counts"`
	ComplexityCounts  map[string]int `json:"complexity_counts"`
	LatestTaskID      string         `json:"latest_task_id,omitempty"`
}

type eventMetrics struct {
	TotalEvents            int            `json:"total_events"`
	SubmitTotal            int            `json:"submit_total"`
	SubmitSuccess          int            `json:"submit_success"`
	SubmitFailure          int            `json:"submit_failure"`
	SubmitMessageSources   map[string]int `json:"submit_message_sources"`
	GCMsgTotal             int            `json:"gcmsg_total"`
	GCMsgSuccess           int            `json:"gcmsg_success"`
	GCMsgFailure           int            `json:"gcmsg_failure"`
	GCMsgMessageSources    map[string]int `json:"gcmsg_message_sources"`
	ReviewTotal            int            `json:"review_total"`
	ReviewSuccess          int            `json:"review_success"`
	ReviewFailure          int            `json:"review_failure"`
	ReviewRatingCounts     map[string]int `json:"review_rating_counts"`
	LatestEventTime        string         `json:"latest_event_time,omitempty"`
}

type reviewSummaryJSON struct {
	Rating        string `json:"rating"`
	P0Count       int    `json:"p0_count"`
	P1Count       int    `json:"p1_count"`
	P2Count       int    `json:"p2_count"`
	TotalFindings int    `json:"total_findings"`
}

type prdTaskJSON struct {
	TaskID     string `json:"task_id"`
	Status     string `json:"status"`
	SourceType string `json:"source_type"`
}

type localEventJSON struct {
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	Success   bool           `json:"success"`
	Fields    map[string]any `json:"fields"`
}

var rePlanComplexity = regexp.MustCompile(`(?m)^- complexity:\s*([^\s]+)\s*\((\d+)\)\s*$`)

func init() {
	rootCmd.AddCommand(metricsCmd)
	metricsCmd.Flags().BoolVar(&metricsJSON, "json", false, "输出结构化 JSON")
}

func runMetrics(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	report := metricsReport{
		RepoRoot:        repoRoot,
		GeneratedAt:     time.Now().Format(time.RFC3339),
		Review:          collectReviewMetrics(repoRoot),
		Lint:            collectLintMetrics(repoRoot),
		PRD:             collectPRDMetrics(repoRoot),
		Events:          collectEventMetrics(repoRoot),
		Recommendations: buildMetricsRecommendations(repoRoot),
	}

	if metricsJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化 metrics 失败: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	renderMetrics(report)
	return nil
}

func collectReviewMetrics(repoRoot string) reviewMetrics {
	result := reviewMetrics{
		RatingCounts: map[string]int{
			"🟢 可提交":      0,
			"🟡 建议修复后提交": 0,
			"🔴 需修复":      0,
		},
	}

	reviewRoot := filepath.Join(repoRoot, config.ReviewOutputDir)
	entries, err := os.ReadDir(reviewRoot)
	if err != nil {
		return result
	}

	type reviewEntry struct {
		name    string
		modTime time.Time
	}
	reviewDirs := make([]reviewEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		reviewDirs = append(reviewDirs, reviewEntry{name: entry.Name(), modTime: info.ModTime()})
	}

	sort.Slice(reviewDirs, func(i, j int) bool {
		return reviewDirs[i].modTime.After(reviewDirs[j].modTime)
	})
	if len(reviewDirs) > 0 {
		result.LatestOutputDir = filepath.Join(reviewRoot, reviewDirs[0].name)
	}

	for _, dir := range reviewDirs {
		result.TotalRuns++
		summaryPath := filepath.Join(reviewRoot, dir.name, "summary.json")
		data, err := os.ReadFile(summaryPath)
		if err != nil {
			continue
		}
		var summary reviewSummaryJSON
		if err := json.Unmarshal(data, &summary); err != nil {
			continue
		}
		result.StructuredRuns++
		if summary.Rating != "" {
			result.RatingCounts[summary.Rating]++
		}
		result.P0Count += summary.P0Count
		result.P1Count += summary.P1Count
		result.P2Count += summary.P2Count
		result.TotalFindings += summary.TotalFindings
	}

	return result
}

func collectLintMetrics(repoRoot string) lintMetrics {
	result := lintMetrics{
		LinterCounts: map[string]int{},
	}

	lintRoot := filepath.Join(repoRoot, config.LintOutputDir)
	entries, err := os.ReadDir(lintRoot)
	if err != nil {
		return result
	}

	type lintEntry struct {
		name    string
		modTime time.Time
	}
	dirs := make([]lintEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		dirs = append(dirs, lintEntry{name: entry.Name(), modTime: info.ModTime()})
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].modTime.After(dirs[j].modTime)
	})
	if len(dirs) > 0 {
		result.LatestOutputDir = filepath.Join(lintRoot, dirs[0].name)
	}

	for _, dir := range dirs {
		lintPath := filepath.Join(lintRoot, dir.name, "lint.json")
		data, err := os.ReadFile(lintPath)
		if err != nil {
			continue
		}
		var lintResult struct {
			Total     int            `json:"total"`
			LinterMap map[string]int `json:"linter_counts"`
		}
		if err := json.Unmarshal(data, &lintResult); err != nil {
			continue
		}
		result.TotalRuns++
		result.TotalIssues += lintResult.Total
		for linter, count := range lintResult.LinterMap {
			result.LinterCounts[linter] += count
		}
	}

	return result
}

func collectPRDMetrics(repoRoot string) prdMetrics {
	result := prdMetrics{
		StatusCounts:     map[string]int{},
		SourceTypeCounts: map[string]int{},
		ComplexityCounts: map[string]int{},
	}

	tasksRoot := filepath.Join(repoRoot, ".livecoding", "tasks")
	entries, err := os.ReadDir(tasksRoot)
	if err != nil {
		return result
	}

	type taskEntry struct {
		id      string
		modTime time.Time
	}
	taskDirs := make([]taskEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		taskDirs = append(taskDirs, taskEntry{id: entry.Name(), modTime: info.ModTime()})
	}

	sort.Slice(taskDirs, func(i, j int) bool {
		return taskDirs[i].modTime.After(taskDirs[j].modTime)
	})
	if len(taskDirs) > 0 {
		result.LatestTaskID = taskDirs[0].id
	}

	for _, dir := range taskDirs {
		result.TotalTasks++
		taskPath := filepath.Join(tasksRoot, dir.id, "task.json")
		taskData, err := os.ReadFile(taskPath)
		if err == nil {
			var meta prdTaskJSON
			if json.Unmarshal(taskData, &meta) == nil {
				if meta.Status != "" {
					result.StatusCounts[meta.Status]++
				}
				if meta.SourceType != "" {
					result.SourceTypeCounts[meta.SourceType]++
				}
			}
		}

		planPath := filepath.Join(tasksRoot, dir.id, "plan.md")
		planData, err := os.ReadFile(planPath)
		if err != nil {
			continue
		}
		matches := rePlanComplexity.FindStringSubmatch(string(planData))
		if len(matches) > 1 {
			result.ComplexityCounts[strings.TrimSpace(matches[1])]++
		}
	}

	return result
}

func buildMetricsRecommendations(repoRoot string) []string {
	recommendations := make([]string, 0, 4)
	reviewRoot := filepath.Join(repoRoot, config.ReviewOutputDir)
	if _, err := os.Stat(reviewRoot); err != nil {
		recommendations = append(recommendations, "当前仓库还没有 review 结构化产物，建议先执行 `coco-ext review --json`。")
	}

	tasksRoot := filepath.Join(repoRoot, ".livecoding", "tasks")
	if _, err := os.Stat(tasksRoot); err != nil {
		recommendations = append(recommendations, "当前仓库还没有 prd task，建议先执行 `coco-ext prd refine`。")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations,
			"可以基于 `review/report.json` 和 `prd plan` 结果继续建设 metrics dashboard。",
			"可以进一步消费 submit/gcmsg 事件，完善 AI/fallback 成本统计。",
		)
	}
	return recommendations
}

func collectEventMetrics(repoRoot string) eventMetrics {
	result := eventMetrics{
		SubmitMessageSources: map[string]int{},
		GCMsgMessageSources:  map[string]int{},
		ReviewRatingCounts:   map[string]int{},
	}

	eventsPath := filepath.Join(repoRoot, ".livecoding", "metrics", "events.jsonl")
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		return result
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event localEventJSON
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		result.TotalEvents++
		if event.Timestamp > result.LatestEventTime {
			result.LatestEventTime = event.Timestamp
		}

		switch event.Type {
		case "submit":
			result.SubmitTotal++
			if event.Success {
				result.SubmitSuccess++
			} else {
				result.SubmitFailure++
			}
			if source, ok := readStringField(event.Fields, "message_source"); ok && source != "" {
				result.SubmitMessageSources[source]++
			}
		case "gcmsg":
			result.GCMsgTotal++
			if event.Success {
				result.GCMsgSuccess++
			} else {
				result.GCMsgFailure++
			}
			if source, ok := readStringField(event.Fields, "message_source"); ok && source != "" {
				result.GCMsgMessageSources[source]++
			}
		case "review":
			result.ReviewTotal++
			if event.Success {
				result.ReviewSuccess++
			} else {
				result.ReviewFailure++
			}
			if rating, ok := readStringField(event.Fields, "rating"); ok && rating != "" {
				result.ReviewRatingCounts[rating]++
			}
		}
	}

	return result
}

func renderMetrics(report metricsReport) {
	color.Cyan("📈 Coco Metrics")
	color.Cyan("   repo: %s", report.RepoRoot)
	color.Cyan("   generated_at: %s", report.GeneratedAt)
	fmt.Println()

	color.Green("Review")
	color.Green("   total_runs: %d", report.Review.TotalRuns)
	color.Green("   structured_runs: %d", report.Review.StructuredRuns)
	color.Green("   rating: 🟢 %d / 🟡 %d / 🔴 %d",
		report.Review.RatingCounts["🟢 可提交"],
		report.Review.RatingCounts["🟡 建议修复后提交"],
		report.Review.RatingCounts["🔴 需修复"],
	)
	color.Green("   findings: P0=%d P1=%d P2=%d total=%d",
		report.Review.P0Count,
		report.Review.P1Count,
		report.Review.P2Count,
		report.Review.TotalFindings,
	)
	if report.Review.LatestOutputDir != "" {
		color.Green("   latest_output: %s", report.Review.LatestOutputDir)
	}
	fmt.Println()

	color.Green("PRD")
	color.Green("   total_tasks: %d", report.PRD.TotalTasks)
	if report.PRD.LatestTaskID != "" {
		color.Green("   latest_task: %s", report.PRD.LatestTaskID)
	}
	color.Green("   status_counts: %s", formatCountMap(report.PRD.StatusCounts))
	color.Green("   source_type_counts: %s", formatCountMap(report.PRD.SourceTypeCounts))
	color.Green("   complexity_counts: %s", formatCountMap(report.PRD.ComplexityCounts))
	fmt.Println()

	color.Green("Lint")
	color.Green("   total_runs: %d", report.Lint.TotalRuns)
	color.Green("   total_issues: %d", report.Lint.TotalIssues)
	color.Green("   linter_counts: %s", formatCountMap(report.Lint.LinterCounts))
	if report.Lint.LatestOutputDir != "" {
		color.Green("   latest_output: %s", report.Lint.LatestOutputDir)
	}
	fmt.Println()

	color.Green("Events")
	color.Green("   total_events: %d", report.Events.TotalEvents)
	color.Green("   submit: total=%d success=%d failure=%d", report.Events.SubmitTotal, report.Events.SubmitSuccess, report.Events.SubmitFailure)
	color.Green("   submit_message_sources: %s", formatCountMap(report.Events.SubmitMessageSources))
	color.Green("   gcmsg: total=%d success=%d failure=%d", report.Events.GCMsgTotal, report.Events.GCMsgSuccess, report.Events.GCMsgFailure)
	color.Green("   gcmsg_message_sources: %s", formatCountMap(report.Events.GCMsgMessageSources))
	color.Green("   review: total=%d success=%d failure=%d", report.Events.ReviewTotal, report.Events.ReviewSuccess, report.Events.ReviewFailure)
	color.Green("   review_rating_counts: %s", formatCountMap(report.Events.ReviewRatingCounts))
	if report.Events.LatestEventTime != "" {
		color.Green("   latest_event: %s", report.Events.LatestEventTime)
	}
	fmt.Println()

	color.Green("Recommendations")
	for _, item := range report.Recommendations {
		color.Green("   - %s", item)
	}
}

func readStringField(fields map[string]any, key string) (string, bool) {
	if fields == nil {
		return "", false
	}
	value, ok := fields[key]
	if !ok || value == nil {
		return "", false
	}
	str, ok := value.(string)
	return str, ok
}

func formatCountMap(items map[string]int) string {
	if len(items) == 0 {
		return "无"
	}
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, items[key]))
	}
	return strings.Join(parts, ", ")
}
