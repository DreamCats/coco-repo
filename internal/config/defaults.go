package config

import (
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultModel 默认使用的 AI 模型
	DefaultModel = "Doubao-Seed-2.0-Code"

	// ContextDir 知识库目录（在仓库根目录下）
	ContextDir = ".livecoding/context"

	// ReviewOutputDir Review 报告输出目录
	ReviewOutputDir = ".livecoding/review"

	// ChangelogDir Commit changelog 目录
	ChangelogDir = ".livecoding/changelog"
)

const (
	// DefaultPromptTimeout 默认 AI 请求超时，适用于 gcmsg 等短流程
	DefaultPromptTimeout = 30 * time.Second

	// ContextPromptTimeout context init/update 专用超时，允许更长时间生成知识文件
	ContextPromptTimeout = 5 * time.Minute

	// ReviewPromptTimeout review 专用超时，允许更长时间生成报告
	ReviewPromptTimeout = 3 * time.Minute
)

const (
	// ReviewBackgroundPriority 后台 review 进程 nice 优先级
	ReviewBackgroundPriority = 10
)

const (
	// DefaultDaemonIdleTimeout daemon 空闲自动退出时间
	DefaultDaemonIdleTimeout = 60 * time.Minute
)

// DefaultConfigDir 返回用户级配置目录
func DefaultConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "coco-ext")
}

// DaemonIdleTimeout 返回统一的 daemon 空闲超时配置。
// 可通过 COCO_EXT_DAEMON_IDLE_TIMEOUT 覆盖，格式遵循 time.ParseDuration。
func DaemonIdleTimeout() time.Duration {
	raw := os.Getenv("COCO_EXT_DAEMON_IDLE_TIMEOUT")
	if raw == "" {
		return DefaultDaemonIdleTimeout
	}

	if duration, err := time.ParseDuration(raw); err == nil && duration > 0 {
		return duration
	}

	return DefaultDaemonIdleTimeout
}

// KnowledgeFiles 知识文件列表及说明
var KnowledgeFiles = map[string]string{
	"glossary.md":     "业务术语 <-> 代码标识符映射",
	"architecture.md": "仓库架构概览（目录结构、分层、技术栈）",
	"patterns.md":     "代码模式（Handler/Service/Converter 骨架）",
	"gotchas.md":      "踩坑记录、隐式约定、容易出错的点",
}
