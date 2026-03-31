package prd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type ArtifactStatus struct {
	Name   string
	Path   string
	Exists bool
}

type TaskStatusReport struct {
	TaskID      string
	TaskDir     string
	Metadata    TaskMetadata
	Source      *SourceMetadata
	Artifacts   []ArtifactStatus
	Missing     []string
	NextCommand string
}

var trackedArtifactNames = []string{
	"source.json",
	"prd.source.md",
	"prd-refined.md",
	"design.md",
	"plan.md",
	"changelog.md",
	"mr.md",
}

// LoadTaskStatus 加载指定 task 的状态信息。
func LoadTaskStatus(repoRoot, taskID string) (*TaskStatusReport, error) {
	taskDir := filepath.Join(repoRoot, ".livecoding", "tasks", taskID)
	info, err := os.Stat(taskDir)
	if err != nil {
		return nil, fmt.Errorf("task 不存在: %s", taskID)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("task 路径不是目录: %s", taskDir)
	}

	meta, err := readTaskMetadata(filepath.Join(taskDir, "task.json"))
	if err != nil {
		return nil, err
	}

	sourceMeta, _ := readSourceMetadata(filepath.Join(taskDir, "source.json"))
	artifacts, missing := collectArtifacts(taskDir)

	return &TaskStatusReport{
		TaskID:      taskID,
		TaskDir:     taskDir,
		Metadata:    *meta,
		Source:      sourceMeta,
		Artifacts:   artifacts,
		Missing:     missing,
		NextCommand: suggestNextCommand(taskID, meta.Status, artifacts),
	}, nil
}

// ResolveTaskID 解析用户指定或最近的 task。
func ResolveTaskID(repoRoot, explicitTaskID string) (string, error) {
	if explicitTaskID != "" {
		return explicitTaskID, nil
	}

	tasksRoot := filepath.Join(repoRoot, ".livecoding", "tasks")
	entries, err := os.ReadDir(tasksRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("未找到任何 task，请先执行 coco-ext prd refine")
		}
		return "", fmt.Errorf("读取 tasks 目录失败: %w", err)
	}

	type taskEntry struct {
		name    string
		modTime time.Time
	}
	tasks := make([]taskEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		tasks = append(tasks, taskEntry{name: entry.Name(), modTime: info.ModTime()})
	}

	if len(tasks) == 0 {
		return "", fmt.Errorf("未找到任何 task，请先执行 coco-ext prd refine")
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].modTime.After(tasks[j].modTime)
	})
	return tasks[0].name, nil
}

func readTaskMetadata(path string) (*TaskMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("缺少 task.json: %s", path)
		}
		return nil, fmt.Errorf("读取 task.json 失败: %w", err)
	}

	var meta TaskMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("解析 task.json 失败: %w", err)
	}
	return &meta, nil
}

func readSourceMetadata(path string) (*SourceMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var meta SourceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func collectArtifacts(taskDir string) ([]ArtifactStatus, []string) {
	artifacts := make([]ArtifactStatus, 0, len(trackedArtifactNames))
	missing := make([]string, 0, len(trackedArtifactNames))
	for _, name := range trackedArtifactNames {
		path := filepath.Join(taskDir, name)
		_, err := os.Stat(path)
		exists := err == nil
		artifacts = append(artifacts, ArtifactStatus{
			Name:   name,
			Path:   path,
			Exists: exists,
		})
		if !exists {
			missing = append(missing, name)
		}
	}
	return artifacts, missing
}

func suggestNextCommand(taskID, status string, artifacts []ArtifactStatus) string {
	hasRefined := hasArtifact(artifacts, "prd-refined.md")
	hasDesign := hasArtifact(artifacts, "design.md")
	hasPlan := hasArtifact(artifacts, "plan.md")
	hasMR := hasArtifact(artifacts, "mr.md")

	switch {
	case !hasRefined:
		return fmt.Sprintf("coco-ext prd refine --task %s --prd .livecoding/tasks/%s/prd.source.md", taskID, taskID)
	case !hasDesign || !hasPlan:
		return fmt.Sprintf("coco-ext prd plan --task %s", taskID)
	case status == TaskStatusPlanned:
		return "人工 review design.md / plan.md；approve / codegen 尚未实现。"
	case status == "approved":
		return fmt.Sprintf("coco-ext prd codegen --task %s", taskID)
	case !hasMR:
		return fmt.Sprintf("coco-ext prd mr --task %s", taskID)
	default:
		return "当前 task 无明确下一步，建议人工确认状态。"
	}
}

func hasArtifact(artifacts []ArtifactStatus, name string) bool {
	for _, artifact := range artifacts {
		if artifact.Name == name {
			return artifact.Exists
		}
	}
	return false
}
