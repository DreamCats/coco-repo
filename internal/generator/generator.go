package generator

import (
	"fmt"
	"strings"

	"github.com/DreamCats/coco-acp-sdk/daemon"
	"github.com/DreamCats/coco-ext/internal/config"
)

// Generator 知识文件生成器
type Generator struct {
	conn      *daemon.Conn
	sessionID string
	modelID   string
}

// New 创建生成器，连接 coco daemon
// 每次调用都会创建新的 session，由上游 agent 决策是否复用
func New(repoPath string) (*Generator, error) {
	conn, err := daemon.Dial(repoPath, &daemon.DialOption{
		ConfigDir: config.DefaultConfigDir(),
	})
	if err != nil {
		return nil, fmt.Errorf("连接 coco daemon 失败: %w", err)
	}

	// 创建新 session
	sess, err := conn.NewSession(repoPath)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("创建 session 失败: %w", err)
	}

	// 设置当前使用的 session
	conn.UseSession(sess.SessionID)

	return &Generator{
		conn:      conn,
		sessionID: sess.SessionID,
		modelID:   config.DefaultModel,
	}, nil
}

// Info 获取 daemon 状态信息（PID、SessionID、ModelID、Uptime）
func (g *Generator) Info() (pid int, sessionID, modelID, uptime string, err error) {
	resp, err := g.conn.Status()
	if err != nil {
		return 0, "", "", "", err
	}
	return resp.PID, g.sessionID, g.modelID, resp.Uptime, nil
}

// Close 关闭连接
func (g *Generator) Close() {
	if g.conn != nil {
		g.conn.Close()
	}
}

// Generate 生成单个知识文件内容
func (g *Generator) Generate(name, scanSummary string, onChunk func(string)) (string, error) {
	prompt := GetPrompt(name, scanSummary)
	if prompt == "" {
		return "", fmt.Errorf("未知的知识文件: %s", name)
	}

	var result strings.Builder
	_, err := g.conn.Prompt(
		prompt,
		g.modelID,
		"",
		func(text string) {
			result.WriteString(text)
			if onChunk != nil {
				onChunk(text)
			}
		},
		func(kind, title, status string) {
			// 工具调用回调（暂不处理）
		},
	)
	if err != nil {
		return "", fmt.Errorf("生成 %s 失败: %w", name, err)
	}

	return result.String(), nil
}

// Update 增量更新知识文件
func (g *Generator) Update(name, existingContent, diffContent string, onChunk func(string)) (string, error) {
	prompt := GetUpdatePrompt(name, existingContent, diffContent)

	var result strings.Builder
	_, err := g.conn.Prompt(
		prompt,
		g.modelID,
		"",
		func(text string) {
			result.WriteString(text)
			if onChunk != nil {
				onChunk(text)
			}
		},
		func(kind, title, status string) {},
	)
	if err != nil {
		return "", fmt.Errorf("更新 %s 失败: %w", name, err)
	}

	content := result.String()
	if strings.TrimSpace(content) == "NO_UPDATE" {
		return "", nil // 无需更新
	}

	return content, nil
}

// Prompt 直接发送 prompt，返回完整响应
func (g *Generator) Prompt(prompt string, onChunk func(string)) (string, error) {
	var result strings.Builder
	_, err := g.conn.Prompt(
		prompt,
		g.modelID,
		"",
		func(text string) {
			result.WriteString(text)
			if onChunk != nil {
				onChunk(text)
			}
		},
		func(kind, title, status string) {},
	)
	if err != nil {
		return "", fmt.Errorf("prompt 失败: %w", err)
	}

	return result.String(), nil
}
