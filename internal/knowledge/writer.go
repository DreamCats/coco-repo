package knowledge

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DreamCats/coco-repo/internal/config"
)

// EnsureDir 确保 .context/ 目录存在
func EnsureDir(repoRoot string) error {
	contextDir := filepath.Join(repoRoot, config.ContextDir)
	return os.MkdirAll(contextDir, 0700)
}

// WriteFile 写入单个知识文件
func WriteFile(repoRoot, name, content string) error {
	contextDir := filepath.Join(repoRoot, config.ContextDir)
	filePath := filepath.Join(contextDir, name)
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		return fmt.Errorf("写入知识文件 %s 失败: %w", name, err)
	}
	return nil
}

// WriteAll 写入所有知识文件
func WriteAll(repoRoot string, files map[string]string) error {
	if err := EnsureDir(repoRoot); err != nil {
		return fmt.Errorf("创建 .context/ 目录失败: %w", err)
	}
	for name, content := range files {
		if err := WriteFile(repoRoot, name, content); err != nil {
			return err
		}
	}
	return nil
}
