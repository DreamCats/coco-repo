package config

import (
	"os"
	"path/filepath"
)

const (
	// DefaultModel 默认使用的 AI 模型
	DefaultModel = "Doubao-Seed-2.0-Code"

	// ContextDir 知识库目录（在仓库根目录下）
	ContextDir = ".livecoding/context"

	// ReviewOutputDir Review 报告输出目录
	ReviewOutputDir = ".livecoding/review"
)

// DefaultConfigDir 返回用户级配置目录
func DefaultConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "coco-ext")
}

// KnowledgeFiles 知识文件列表及说明
var KnowledgeFiles = map[string]string{
	"glossary.md":     "业务术语 <-> 代码标识符映射",
	"architecture.md": "仓库架构概览（目录结构、分层、技术栈）",
	"patterns.md":     "代码模式（Handler/Service/Converter 骨架）",
	"gotchas.md":      "踩坑记录、隐式约定、容易出错的点",
}
