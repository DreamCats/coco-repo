package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/DreamCats/coco-repo/internal/config"
)

// FileStatus 单个知识文件的状态
type FileStatus struct {
	Name     string    // 文件名
	Exists   bool      // 是否存在
	ModTime  time.Time // 最后修改时间
	Size     int64     // 文件大小
	Content  string    // 文件内容
}

// Status 知识库整体状态
type Status struct {
	ContextDir string       // .context/ 绝对路径
	Exists     bool         // .context/ 目录是否存在
	Files      []FileStatus // 各文件状态
}

// ReadStatus 读取知识库状态
func ReadStatus(repoRoot string) (*Status, error) {
	contextDir := filepath.Join(repoRoot, config.ContextDir)

	st := &Status{ContextDir: contextDir}

	info, err := os.Stat(contextDir)
	if err != nil {
		st.Exists = false
		return st, nil
	}
	st.Exists = info.IsDir()

	for name := range config.KnowledgeFiles {
		fs := FileStatus{Name: name}
		filePath := filepath.Join(contextDir, name)

		fi, err := os.Stat(filePath)
		if err == nil {
			fs.Exists = true
			fs.ModTime = fi.ModTime()
			fs.Size = fi.Size()
		}

		st.Files = append(st.Files, fs)
	}

	return st, nil
}

// ReadFile 读取单个知识文件内容
func ReadFile(repoRoot, name string) (string, error) {
	filePath := filepath.Join(repoRoot, config.ContextDir, name)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("读取知识文件 %s 失败: %w", name, err)
	}
	return string(data), nil
}

// ReadAll 读取所有知识文件内容
func ReadAll(repoRoot string) (map[string]string, error) {
	result := make(map[string]string)
	contextDir := filepath.Join(repoRoot, config.ContextDir)

	for name := range config.KnowledgeFiles {
		filePath := filepath.Join(contextDir, name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // 文件不存在则跳过
		}
		result[name] = string(data)
	}

	return result, nil
}
