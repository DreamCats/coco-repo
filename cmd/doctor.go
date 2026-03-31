package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	acpdaemon "github.com/DreamCats/coco-acp-sdk/daemon"
	"github.com/DreamCats/coco-ext/internal/config"
	internalgit "github.com/DreamCats/coco-ext/internal/git"
)

type doctorStatus string

const (
	doctorStatusPass doctorStatus = "PASS"
	doctorStatusWarn doctorStatus = "WARN"
	doctorStatusFail doctorStatus = "FAIL"
	doctorStatusSkip doctorStatus = "SKIP"
)

const (
	doctorCheckRepository = "repository"
	doctorCheckWorkspace  = "workspace"
	doctorCheckHooks      = "hooks"
	doctorCheckTooling    = "tooling"
	doctorCheckSkills     = "skills"
	doctorCheckDaemon     = "daemon"
	doctorCheckLogs       = "logs"
)

type doctorCheckResult struct {
	Name    string       `json:"name"`
	Status  doctorStatus `json:"status"`
	Summary string       `json:"summary"`
	Details []string     `json:"details,omitempty"`
	FixHint string       `json:"fix_hint,omitempty"`
}

type doctorCounts struct {
	Pass int `json:"pass"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
	Skip int `json:"skip"`
}

type doctorReport struct {
	RepoRoot    string              `json:"repo_root"`
	Version     string              `json:"version"`
	GeneratedAt time.Time           `json:"generated_at"`
	Fixed       bool                `json:"fixed"`
	FixActions  []string            `json:"fix_actions,omitempty"`
	FixErrors   []string            `json:"fix_errors,omitempty"`
	Results     []doctorCheckResult `json:"results"`
	Counts      doctorCounts        `json:"counts"`
}

type hookExpectation struct {
	Name        string
	Marker      string
	Description string
}

var doctorJSON bool
var doctorVerbose bool
var doctorFix bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "诊断 coco-ext 当前环境、hooks、skills、daemon 和日志状态",
	Long:  "检查当前仓库与本机上的 coco-ext 运行环境，输出问题摘要，并可选执行低风险修复。",
	RunE:  runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "以 JSON 输出诊断结果")
	doctorCmd.Flags().BoolVarP(&doctorVerbose, "verbose", "v", false, "输出更详细的诊断信息")
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "尝试修复 hooks、skills、workspace 和 daemon 的常见问题")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	report := collectDoctorReport(repoRoot)
	if doctorFix {
		applyDoctorFixes(repoRoot, &report)
		report.Results = runDoctorChecks(repoRoot)
		report.Counts = summarizeDoctorCounts(report.Results)
		report.Fixed = true
	}

	if doctorJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}

	renderDoctorReport(report, doctorVerbose)
	return nil
}

func collectDoctorReport(repoRoot string) doctorReport {
	results := runDoctorChecks(repoRoot)
	return doctorReport{
		RepoRoot:    repoRoot,
		Version:     version,
		GeneratedAt: time.Now(),
		Results:     results,
		Counts:      summarizeDoctorCounts(results),
	}
}

func runDoctorChecks(repoRoot string) []doctorCheckResult {
	results := make([]doctorCheckResult, 0, 7)

	isGitRepo := internalgit.IsGitRepo(repoRoot)
	results = append(results, checkRepository(repoRoot, isGitRepo))
	results = append(results, checkWorkspace(repoRoot, isGitRepo))
	results = append(results, checkHooks(repoRoot, isGitRepo))
	results = append(results, checkTooling())
	results = append(results, checkSkills())
	results = append(results, checkDaemon(repoRoot))
	results = append(results, checkLogs(repoRoot, isGitRepo))

	return results
}

func summarizeDoctorCounts(results []doctorCheckResult) doctorCounts {
	var counts doctorCounts
	for _, result := range results {
		switch result.Status {
		case doctorStatusPass:
			counts.Pass++
		case doctorStatusWarn:
			counts.Warn++
		case doctorStatusFail:
			counts.Fail++
		case doctorStatusSkip:
			counts.Skip++
		}
	}
	return counts
}

func checkRepository(repoRoot string, isGitRepo bool) doctorCheckResult {
	if !isGitRepo {
		return doctorCheckResult{
			Name:    doctorCheckRepository,
			Status:  doctorStatusFail,
			Summary: "当前目录不是 git 仓库",
			Details: []string{fmt.Sprintf("cwd: %s", repoRoot)},
			FixHint: "请切换到目标仓库后重新执行 coco-ext doctor。",
		}
	}

	return doctorCheckResult{
		Name:    doctorCheckRepository,
		Status:  doctorStatusPass,
		Summary: "Git 仓库可用",
		Details: []string{fmt.Sprintf("repo: %s", repoRoot)},
	}
}

func checkWorkspace(repoRoot string, isGitRepo bool) doctorCheckResult {
	if !isGitRepo {
		return doctorCheckResult{
			Name:    doctorCheckWorkspace,
			Status:  doctorStatusSkip,
			Summary: "非 git 仓库，跳过 workspace 检查",
		}
	}

	livecodingDir := filepath.Join(repoRoot, ".livecoding")
	logDir := filepath.Join(livecodingDir, "logs")
	reviewDir := filepath.Join(livecodingDir, "review")
	contextDir := filepath.Join(repoRoot, config.ContextDir)

	missing := make([]string, 0, 4)
	details := []string{
		fmt.Sprintf("livecoding: %s", livecodingDir),
		fmt.Sprintf("logs: %s", logDir),
		fmt.Sprintf("review: %s", reviewDir),
		fmt.Sprintf("context: %s", contextDir),
	}

	for _, path := range []string{livecodingDir, logDir, reviewDir} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, filepath.Base(path))
		}
	}
	if _, err := os.Stat(contextDir); os.IsNotExist(err) {
		missing = append(missing, ".livecoding/context")
	}

	if len(missing) == 0 {
		return doctorCheckResult{
			Name:    doctorCheckWorkspace,
			Status:  doctorStatusPass,
			Summary: "workspace 目录结构完整",
			Details: details,
		}
	}

	return doctorCheckResult{
		Name:    doctorCheckWorkspace,
		Status:  doctorStatusWarn,
		Summary: fmt.Sprintf("workspace 目录不完整，缺少 %d 项", len(missing)),
		Details: append(details, "missing: "+strings.Join(missing, ", ")),
		FixHint: "执行 coco-ext doctor --fix 可自动创建缺失目录。",
	}
}

func checkHooks(repoRoot string, isGitRepo bool) doctorCheckResult {
	if !isGitRepo {
		return doctorCheckResult{
			Name:    doctorCheckHooks,
			Status:  doctorStatusSkip,
			Summary: "非 git 仓库，跳过 hooks 检查",
		}
	}

	hooksDir := filepath.Join(repoRoot, ".git", "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return doctorCheckResult{
			Name:    doctorCheckHooks,
			Status:  doctorStatusFail,
			Summary: ".git/hooks 目录不存在",
			FixHint: "执行 coco-ext install --hooks 重装 hooks。",
		}
	}

	expectations := []hookExpectation{
		{Name: "commit-msg", Marker: "# coco-ext commit-msg hook", Description: "自动优化简短 commit message"},
		{Name: "pre-commit", Marker: "# coco-ext pre-commit hook", Description: "goimports 格式化暂存的 Go 文件"},
	}

	details := make([]string, 0, len(expectations)+2)
	hasWarn := false
	hasFail := false

	for _, expected := range expectations {
		hookPath := filepath.Join(hooksDir, expected.Name)
		data, err := os.ReadFile(hookPath)
		if os.IsNotExist(err) {
			hasFail = true
			details = append(details, fmt.Sprintf("%s: missing", expected.Name))
			continue
		}
		if err != nil {
			hasFail = true
			details = append(details, fmt.Sprintf("%s: read failed: %v", expected.Name, err))
			continue
		}

		info, err := os.Stat(hookPath)
		if err != nil {
			hasFail = true
			details = append(details, fmt.Sprintf("%s: stat failed: %v", expected.Name, err))
			continue
		}

		if !strings.Contains(string(data), expected.Marker) {
			hasWarn = true
			details = append(details, fmt.Sprintf("%s: exists but marker mismatch", expected.Name))
		} else if info.Mode()&0111 == 0 {
			hasWarn = true
			details = append(details, fmt.Sprintf("%s: exists but not executable", expected.Name))
		} else {
			details = append(details, fmt.Sprintf("%s: ok", expected.Name))
		}
	}

	for _, legacyName := range []string{"pre-push", "post-commit"} {
		legacyPath := filepath.Join(hooksDir, legacyName)
		if _, err := os.Stat(legacyPath); err == nil {
			hasWarn = true
			details = append(details, fmt.Sprintf("%s: legacy hook still exists", legacyName))
		}
	}

	switch {
	case hasFail:
		return doctorCheckResult{
			Name:    doctorCheckHooks,
			Status:  doctorStatusFail,
			Summary: "hooks 不完整",
			Details: details,
			FixHint: "执行 coco-ext doctor --fix 或 coco-ext install --hooks。",
		}
	case hasWarn:
		return doctorCheckResult{
			Name:    doctorCheckHooks,
			Status:  doctorStatusWarn,
			Summary: "hooks 存在异常或残留 legacy hook",
			Details: details,
			FixHint: "执行 coco-ext doctor --fix 或 coco-ext install --hooks。",
		}
	default:
		return doctorCheckResult{
			Name:    doctorCheckHooks,
			Status:  doctorStatusPass,
			Summary: "hooks 状态正常",
			Details: details,
		}
	}
}

func checkTooling() doctorCheckResult {
	details := make([]string, 0, 4)
	hasWarn := false
	hasFail := false

	if isCommandAvailable("git") {
		details = append(details, "git: ok")
	} else {
		hasFail = true
		details = append(details, "git: missing")
	}

	if isCommandAvailable("goimports") {
		details = append(details, "goimports: ok")
	} else {
		hasWarn = true
		details = append(details, "goimports: missing")
	}

	exePath, err := os.Executable()
	if err != nil {
		hasFail = true
		details = append(details, fmt.Sprintf("coco-ext executable: resolve failed: %v", err))
	} else {
		details = append(details, fmt.Sprintf("coco-ext: %s", exePath))
	}

	switch {
	case hasFail:
		return doctorCheckResult{
			Name:    doctorCheckTooling,
			Status:  doctorStatusFail,
			Summary: "基础工具不可用",
			Details: details,
			FixHint: "请先修复 git / coco-ext 可执行路径问题。",
		}
	case hasWarn:
		return doctorCheckResult{
			Name:    doctorCheckTooling,
			Status:  doctorStatusWarn,
			Summary: "基础工具部分缺失",
			Details: details,
			FixHint: "建议安装 goimports：go install golang.org/x/tools/cmd/goimports@latest",
		}
	default:
		return doctorCheckResult{
			Name:    doctorCheckTooling,
			Status:  doctorStatusPass,
			Summary: "基础工具可用",
			Details: details,
		}
	}
}

func checkSkills() doctorCheckResult {
	home, err := os.UserHomeDir()
	if err != nil {
		return doctorCheckResult{
			Name:    doctorCheckSkills,
			Status:  doctorStatusFail,
			Summary: "获取用户目录失败",
			Details: []string{err.Error()},
			FixHint: "请确认当前用户环境可正常访问 HOME 目录。",
		}
	}

	userSkillsDir := filepath.Join(home, ".trae", "skills")
	if _, err := os.Stat(userSkillsDir); os.IsNotExist(err) {
		return doctorCheckResult{
			Name:    doctorCheckSkills,
			Status:  doctorStatusWarn,
			Summary: "用户 skills 目录不存在",
			Details: []string{fmt.Sprintf("skills_dir: %s", userSkillsDir)},
			FixHint: "执行 coco-ext doctor --fix 或 coco-ext install --skills。",
		}
	}

	manifest, err := readSkillsManifest(userSkillsDir)
	if err != nil {
		return doctorCheckResult{
			Name:    doctorCheckSkills,
			Status:  doctorStatusWarn,
			Summary: "skills manifest 不存在或不可解析",
			Details: []string{
				fmt.Sprintf("skills_dir: %s", userSkillsDir),
				fmt.Sprintf("manifest: %v", err),
			},
			FixHint: "执行 coco-ext doctor --fix 或 coco-ext install --skills。",
		}
	}

	expectedEntries, err := fs.ReadDir(embeddedSkillsFS, embeddedSkillsRoot)
	if err != nil {
		return doctorCheckResult{
			Name:    doctorCheckSkills,
			Status:  doctorStatusFail,
			Summary: "读取内置 skills 失败",
			Details: []string{err.Error()},
			FixHint: "请重新安装 coco-ext。",
		}
	}

	expectedSkills := make(map[string]bool, len(expectedEntries))
	for _, entry := range expectedEntries {
		if entry.IsDir() {
			expectedSkills[entry.Name()] = true
		}
	}

	details := []string{
		fmt.Sprintf("skills_dir: %s", userSkillsDir),
		fmt.Sprintf("manifest version: %s", manifest.Version),
	}
	hasWarn := false
	for _, skillName := range manifest.Skills {
		skillDir := filepath.Join(userSkillsDir, skillName)
		skillFile := filepath.Join(skillDir, "SKILL.md")
		if _, ok := expectedSkills[skillName]; !ok {
			hasWarn = true
			details = append(details, fmt.Sprintf("%s: listed in manifest but not built-in", skillName))
			continue
		}
		if _, err := os.Stat(skillFile); err != nil {
			hasWarn = true
			details = append(details, fmt.Sprintf("%s: missing SKILL.md", skillName))
			continue
		}
		details = append(details, fmt.Sprintf("%s: ok", skillName))
	}

	for skillName := range expectedSkills {
		if !containsString(manifest.Skills, skillName) {
			hasWarn = true
			details = append(details, fmt.Sprintf("%s: built-in skill missing from manifest", skillName))
		}
	}

	if hasWarn {
		return doctorCheckResult{
			Name:    doctorCheckSkills,
			Status:  doctorStatusWarn,
			Summary: "skills 安装不完整或 manifest 已过期",
			Details: details,
			FixHint: "执行 coco-ext doctor --fix 或 coco-ext install --skills。",
		}
	}

	return doctorCheckResult{
		Name:    doctorCheckSkills,
		Status:  doctorStatusPass,
		Summary: "skills 状态正常",
		Details: details,
	}
}

func checkDaemon(repoRoot string) doctorCheckResult {
	configDir := config.DefaultConfigDir()
	details := []string{
		fmt.Sprintf("config_dir: %s", configDir),
		fmt.Sprintf("default_model: %s", config.DefaultModel),
		fmt.Sprintf("prompt_timeout: %s", config.DefaultPromptTimeout),
		fmt.Sprintf("review_timeout: %s", config.ReviewPromptTimeout),
		fmt.Sprintf("daemon_idle_timeout: %s", config.DaemonIdleTimeout()),
	}

	if !acpdaemon.IsRunningAt(configDir) {
		return doctorCheckResult{
			Name:    doctorCheckDaemon,
			Status:  doctorStatusWarn,
			Summary: "daemon 未运行",
			Details: details,
			FixHint: "执行 coco-ext doctor --fix 或 coco-ext daemon start -d --cwd .",
		}
	}

	conn, err := acpdaemon.Dial(repoRoot, &acpdaemon.DialOption{ConfigDir: configDir})
	if err != nil {
		return doctorCheckResult{
			Name:    doctorCheckDaemon,
			Status:  doctorStatusFail,
			Summary: "daemon 已运行，但连接失败",
			Details: append(details, fmt.Sprintf("dial error: %v", err)),
			FixHint: "执行 coco-ext doctor --fix，或手动 coco-ext daemon stop && coco-ext daemon start -d --cwd .",
		}
	}
	defer conn.Close()

	resp, err := conn.Status()
	if err != nil {
		return doctorCheckResult{
			Name:    doctorCheckDaemon,
			Status:  doctorStatusFail,
			Summary: "daemon 已运行，但 status 调用失败",
			Details: append(details, fmt.Sprintf("status error: %v", err)),
			FixHint: "执行 coco-ext doctor --fix，或手动重启 daemon。",
		}
	}

	details = append(details,
		fmt.Sprintf("pid: %d", resp.PID),
		fmt.Sprintf("session: %s", resp.SessionID),
		fmt.Sprintf("model: %s", resp.ModelID),
		fmt.Sprintf("uptime: %s", resp.Uptime),
	)

	return doctorCheckResult{
		Name:    doctorCheckDaemon,
		Status:  doctorStatusPass,
		Summary: "daemon 运行正常",
		Details: details,
	}
}

func checkLogs(repoRoot string, isGitRepo bool) doctorCheckResult {
	if !isGitRepo {
		return doctorCheckResult{
			Name:    doctorCheckLogs,
			Status:  doctorStatusSkip,
			Summary: "非 git 仓库，跳过日志检查",
		}
	}

	logDir := filepath.Join(repoRoot, ".livecoding", "logs")
	entries, err := os.ReadDir(logDir)
	if os.IsNotExist(err) {
		return doctorCheckResult{
			Name:    doctorCheckLogs,
			Status:  doctorStatusPass,
			Summary: "未发现本地日志目录",
			Details: []string{fmt.Sprintf("logs_dir: %s", logDir)},
		}
	}
	if err != nil {
		return doctorCheckResult{
			Name:    doctorCheckLogs,
			Status:  doctorStatusWarn,
			Summary: "读取日志目录失败",
			Details: []string{fmt.Sprintf("logs_dir: %s", logDir), err.Error()},
		}
	}

	type logInfo struct {
		Name    string
		Path    string
		ModTime time.Time
	}

	logs := make([]logInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		logs = append(logs, logInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(logDir, entry.Name()),
			ModTime: info.ModTime(),
		})
	}

	if len(logs) == 0 {
		return doctorCheckResult{
			Name:    doctorCheckLogs,
			Status:  doctorStatusPass,
			Summary: "日志目录存在，但暂无日志文件",
			Details: []string{fmt.Sprintf("logs_dir: %s", logDir)},
		}
	}

	sort.Slice(logs, func(i, j int) bool {
		return logs[i].ModTime.After(logs[j].ModTime)
	})

	limit := 5
	if len(logs) < limit {
		limit = len(logs)
	}

	keywords := []string{
		"prompt 超时",
		"连接 coco daemon 失败",
		"Review 生成失败",
		"写入 hook 失败",
		"启动异步 review 失败",
	}

	details := make([]string, 0, limit+1)
	warnings := make([]string, 0)
	for i := 0; i < limit; i++ {
		log := logs[i]
		details = append(details, fmt.Sprintf("%s (%s)", log.Name, log.ModTime.Format("2006-01-02 15:04:05")))
		data, err := os.ReadFile(log.Path)
		if err != nil {
			continue
		}
		content := string(data)
		for _, keyword := range keywords {
			if strings.Contains(content, keyword) {
				warnings = append(warnings, fmt.Sprintf("%s: %s", log.Name, keyword))
				break
			}
		}
	}

	if len(warnings) == 0 {
		return doctorCheckResult{
			Name:    doctorCheckLogs,
			Status:  doctorStatusPass,
			Summary: "最近日志未发现明显错误",
			Details: details,
		}
	}

	return doctorCheckResult{
		Name:    doctorCheckLogs,
		Status:  doctorStatusWarn,
		Summary: fmt.Sprintf("最近日志发现 %d 条可疑错误线索", len(warnings)),
		Details: append(details, warnings...),
		FixHint: "可先查看最新日志；若涉及 daemon/hook/skills，可执行 coco-ext doctor --fix。",
	}
}

func applyDoctorFixes(repoRoot string, report *doctorReport) {
	fixActions := make([]string, 0, 4)
	fixErrors := make([]string, 0, 4)

	isGitRepo := internalgit.IsGitRepo(repoRoot)

	for _, result := range report.Results {
		switch result.Name {
		case doctorCheckWorkspace:
			if result.Status == doctorStatusWarn && isGitRepo {
				if err := ensureWorkspaceLayout(repoRoot); err != nil {
					fixErrors = append(fixErrors, "workspace: "+err.Error())
				} else {
					fixActions = append(fixActions, "已创建缺失的 .livecoding 目录结构")
				}
			}
		case doctorCheckHooks:
			if (result.Status == doctorStatusWarn || result.Status == doctorStatusFail) && isGitRepo {
				if err := repairHooks(repoRoot); err != nil {
					fixErrors = append(fixErrors, "hooks: "+err.Error())
				} else {
					fixActions = append(fixActions, "已重装 commit-msg / pre-commit hooks，并清理 legacy hooks")
				}
			}
		case doctorCheckSkills:
			if result.Status == doctorStatusWarn || result.Status == doctorStatusFail {
				if err := syncSkills(); err != nil {
					fixErrors = append(fixErrors, "skills: "+err.Error())
				} else {
					fixActions = append(fixActions, "已重新同步内置 skills")
				}
			}
		case doctorCheckDaemon:
			if result.Status == doctorStatusWarn || result.Status == doctorStatusFail {
				if err := ensureDaemonRunning(repoRoot); err != nil {
					fixErrors = append(fixErrors, "daemon: "+err.Error())
				} else {
					fixActions = append(fixActions, "已尝试后台启动 daemon")
				}
			}
		}
	}

	report.FixActions = fixActions
	report.FixErrors = fixErrors
}

func ensureWorkspaceLayout(repoRoot string) error {
	paths := []string{
		filepath.Join(repoRoot, ".livecoding"),
		filepath.Join(repoRoot, ".livecoding", "logs"),
		filepath.Join(repoRoot, config.ReviewOutputDir),
		filepath.Join(repoRoot, config.ContextDir),
	}
	for _, path := range paths {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	return nil
}

func repairHooks(repoRoot string) error {
	if err := removeLegacyPrePushHook(repoRoot); err != nil {
		return err
	}
	if err := removeLegacyPostCommitHook(repoRoot); err != nil {
		return err
	}
	if err := installPreCommitHook(repoRoot); err != nil {
		return err
	}
	if err := installCommitMsgHook(repoRoot); err != nil {
		return err
	}
	return nil
}

func ensureDaemonRunning(repoRoot string) error {
	configDir := config.DefaultConfigDir()
	if acpdaemon.IsRunningAt(configDir) {
		conn, err := acpdaemon.Dial(repoRoot, &acpdaemon.DialOption{ConfigDir: configDir})
		if err == nil {
			conn.Close()
			return nil
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, "daemon", "start", "--cwd", repoRoot, "--background", "--idle-timeout", config.DaemonIdleTimeout().String())
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}

	time.Sleep(2 * time.Second)
	if !acpdaemon.IsRunningAt(configDir) {
		return fmt.Errorf("daemon 启动后仍未就绪")
	}
	return nil
}

func renderDoctorReport(report doctorReport, verbose bool) {
	fmt.Println()
	color.Cyan("coco-ext doctor")
	fmt.Println("──────────────────────────────")
	fmt.Printf("repo: %s\n", report.RepoRoot)
	fmt.Printf("version: %s\n", report.Version)
	fmt.Printf("generated: %s\n", report.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	for _, result := range report.Results {
		renderDoctorResult(result, verbose)
	}

	if report.Fixed {
		fmt.Println("修复动作")
		fmt.Println("──────────────────────────────")
		if len(report.FixActions) == 0 && len(report.FixErrors) == 0 {
			fmt.Println("  无需修复或未执行修复动作")
		}
		for _, action := range report.FixActions {
			color.Green("  ✓ %s", action)
		}
		for _, fixErr := range report.FixErrors {
			color.Red("  ✗ %s", fixErr)
		}
		fmt.Println()
	}

	fmt.Println("Summary")
	fmt.Println("──────────────────────────────")
	fmt.Printf("PASS %d  WARN %d  FAIL %d  SKIP %d\n", report.Counts.Pass, report.Counts.Warn, report.Counts.Fail, report.Counts.Skip)

	next := buildDoctorNextSteps(report.Results)
	if len(next) > 0 {
		fmt.Println()
		fmt.Println("Next")
		fmt.Println("──────────────────────────────")
		for i, step := range next {
			fmt.Printf("%d. %s\n", i+1, step)
		}
	}
	fmt.Println()
}

func renderDoctorResult(result doctorCheckResult, verbose bool) {
	statusText := "[" + string(result.Status) + "]"
	switch result.Status {
	case doctorStatusPass:
		color.Green("%s %s", statusText, result.Name)
	case doctorStatusWarn:
		color.Yellow("%s %s", statusText, result.Name)
	case doctorStatusFail:
		color.Red("%s %s", statusText, result.Name)
	default:
		color.Cyan("%s %s", statusText, result.Name)
	}
	fmt.Printf("  %s\n", result.Summary)

	if verbose {
		for _, detail := range result.Details {
			fmt.Printf("  - %s\n", detail)
		}
	} else if len(result.Details) > 0 {
		fmt.Printf("  - %s\n", result.Details[0])
	}

	if result.FixHint != "" && result.Status != doctorStatusPass && result.Status != doctorStatusSkip {
		fmt.Printf("  fix: %s\n", result.FixHint)
	}
	fmt.Println()
}

func buildDoctorNextSteps(results []doctorCheckResult) []string {
	steps := make([]string, 0, 4)
	for _, result := range results {
		if result.FixHint == "" {
			continue
		}
		if result.Status != doctorStatusWarn && result.Status != doctorStatusFail {
			continue
		}
		if !containsString(steps, result.FixHint) {
			steps = append(steps, result.FixHint)
		}
	}
	return steps
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
