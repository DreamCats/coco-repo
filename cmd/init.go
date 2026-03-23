package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-repo/internal/config"
	"github.com/DreamCats/coco-repo/internal/generator"
	"github.com/DreamCats/coco-repo/internal/knowledge"
	"github.com/DreamCats/coco-repo/internal/scanner"
)

var forceInit bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "初始化仓库知识库",
	Long:  "扫描仓库结构、核心类型、RPC 接口，生成 .context/ 下的知识文件（glossary.md, architecture.md, patterns.md, gotchas.md）。",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "强制覆盖已有的 .context/ 目录")
}

func runInit(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	// 检查是否为 git 仓库
	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err != nil {
		return fmt.Errorf("当前目录不是 git 仓库: %s", repoRoot)
	}

	// 检查 .context/ 是否已存在
	contextDir := filepath.Join(repoRoot, config.ContextDir)
	if _, err := os.Stat(contextDir); err == nil && !forceInit {
		return fmt.Errorf(".context/ 已存在，使用 --force 强制覆盖")
	}

	// 1. 扫描仓库
	color.Cyan("正在扫描仓库结构...")
	scanResult, err := scanner.Scan(repoRoot)
	if err != nil {
		return err
	}
	fmt.Printf("  发现 %d 个包, %d 个导出类型, %d 个 IDL 文件\n",
		len(scanResult.GoPackages), len(scanResult.GoTypes), len(scanResult.RPCFiles))

	// 2. 创建 .context/ 目录
	if err := knowledge.EnsureDir(repoRoot); err != nil {
		return err
	}

	// 3. 连接 coco daemon 生成知识文件
	color.Cyan("正在连接 coco daemon...")
	gen, err := generator.New(repoRoot)
	if err != nil {
		return err
	}
	defer gen.Close()

	summary := scanResult.Summary()

	// 按顺序生成：glossary → architecture → patterns → gotchas
	fileOrder := []string{"glossary.md", "architecture.md", "patterns.md", "gotchas.md"}
	for _, name := range fileOrder {
		desc := config.KnowledgeFiles[name]
		color.Cyan("正在生成 %s（%s）...", name, desc)

		content, err := gen.Generate(name, summary, func(chunk string) {
			fmt.Print(chunk) // 流式输出
		})
		if err != nil {
			color.Red("生成 %s 失败: %v", name, err)
			continue
		}
		fmt.Println() // 换行

		if err := knowledge.WriteFile(repoRoot, name, content); err != nil {
			color.Red("写入 %s 失败: %v", name, err)
			continue
		}
		color.Green("✓ %s 已生成", name)
	}

	fmt.Println()
	color.Green("知识库初始化完成！文件位于 %s/", config.ContextDir)
	color.Yellow("提示：请 review 生成的文件，将 ❓ 标记的条目确认为 ✅")

	return nil
}
