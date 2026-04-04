package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/config"
	"github.com/DreamCats/coco-ext/internal/generator"
	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

var prdRefineInput string
var prdRefineTitle string
var prdRefineTaskID string

var prdRefineCmd = &cobra.Command{
	Use:   "refine",
	Short: "读取 PRD 源内容，生成 task 目录并输出 refined PRD",
	Long:  "支持纯文本、本地文件和飞书链接三种输入。当前优先完善 task_id、源文件快照与 prd-refined 产物落盘链路。",
	RunE:  runPRDRefine,
}

func init() {
	prdCmd.AddCommand(prdRefineCmd)
	prdRefineCmd.Flags().StringVar(&prdRefineInput, "prd", "", "PRD 输入：纯文本、本地文件路径或飞书链接")
	prdRefineCmd.Flags().StringVar(&prdRefineTitle, "title", "", "可选标题，优先用于生成 task_id 和任务目录")
	prdRefineCmd.Flags().StringVar(&prdRefineTaskID, "task", "", "可选 task id；不传则自动生成")
	_ = prdRefineCmd.MarkFlagRequired("prd")
}

func runPRDRefine(cmd *cobra.Command, args []string) error {
	startedAt := time.Now()
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}
	if !internalgit.IsGitRepo(repoRoot) {
		return fmt.Errorf("当前目录不是 git 仓库")
	}

	now := time.Now()
	task, err := prd.PrepareRefineTask(repoRoot, prd.RefineInput{
		RawInput:     prdRefineInput,
		Title:        prdRefineTitle,
		ExplicitTask: prdRefineTaskID,
		Now:          now,
	})
	if err != nil {
		return err
	}

	color.Cyan("📝 PRD Refine")
	color.Cyan("   task_id: %s", task.TaskID)
	color.Cyan("   title: %s", task.Title)
	color.Cyan("   source: %s", task.Source.Type)
	color.Cyan("   task dir: %s", task.TaskDir)

	if !task.SupportsRefine {
		pendingContent := prd.BuildPendingRefinedContent(task)
		if err := prd.WriteRefinedContent(task, pendingContent, now, prd.TaskStatusInitialized); err != nil {
			return err
		}

		color.Yellow("⚠ 当前来源已记录，但未能获取正文内容")
		if task.Source.LarkErr != nil {
			color.Yellow("  原因: %v", task.Source.LarkErr)
		}
		color.Yellow("  source.json: %s", task.SourceMetaPath)
		color.Yellow("  prd.source.md: %s", task.SourcePath)
		color.Yellow("  prd-refined.md: %s", task.RefinedPath)
		color.Yellow("  请补充 prd.source.md 正文后，重新执行：")
		color.Yellow("  coco-ext prd refine --task %s --prd %s", task.TaskID, task.SourcePath)
		return nil
	}

	color.Cyan("   [1/3] 正在检查并连接 coco daemon...")
	connectStartedAt := time.Now()
	gen, err := generator.New(repoRoot)
	if err != nil {
		color.Yellow("⚠ 连接 coco daemon 失败，使用本地兜底 refine: %v", err)
		color.Yellow("  建议先执行：coco-ext doctor --fix")
		color.Yellow("  或手动执行：coco-ext daemon start -d --cwd .")
		fallbackContent := prd.BuildFallbackRefinedContent(task.Title, task.Source.Content, err)
		if writeErr := prd.WriteRefinedContent(task, fallbackContent, now, prd.TaskStatusRefined); writeErr != nil {
			return writeErr
		}
		color.Green("✓ 已生成本地兜底 refined PRD")
		color.Green("  source.json: %s", task.SourceMetaPath)
		color.Green("  prd.source.md: %s", task.SourcePath)
		color.Green("  prd-refined.md: %s", task.RefinedPath)
		color.Green("  task.json: %s", task.MetadataPath)
		color.Green("⏱ 本次 refine 总耗时: %s", formatDurationSeconds(time.Since(startedAt)))
		return nil
	}
	defer gen.Close()
	color.Green("   [1/3] coco daemon 已连接 ✓")
	color.Cyan("      连接耗时: %s", formatDurationSeconds(time.Since(connectStartedAt)))

	color.Cyan("   [2/3] 正在生成 refined PRD...")
	generateStartedAt := time.Now()
	var streamBuffer strings.Builder
	prompt := prd.BuildRefinedPrompt(task.Title, task.Source.Content)
	streamStarted := false
	firstChunkShown := make(chan struct{})
	stopTicker := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopTicker:
				return
			case <-firstChunkShown:
				return
			case <-ticker.C:
				fmt.Printf("\r\033[K   生成中，已耗时: %s", formatDurationSeconds(time.Since(generateStartedAt)))
			}
		}
	}()
	refinedContent, err := gen.PromptWithTimeout(prompt, config.ReviewPromptTimeout, func(chunk string) {
		streamBuffer.WriteString(chunk)
		if streamStarted {
			fmt.Print(chunk)
			return
		}

		filtered := prd.ExtractRefinedContent(streamBuffer.String())
		if filtered == "" || !strings.HasPrefix(filtered, "# PRD Refined") {
			return
		}

		streamStarted = true
		close(firstChunkShown)
		clearRefineProgressLine()
		fmt.Println("   AI 输出（流式）:")
		fmt.Print(filtered)
	})
	close(stopTicker)
	if streamStarted {
		fmt.Println()
	} else {
		clearRefineProgressLine()
	}
	if err != nil {
		color.Yellow("⚠ AI refine 失败，使用本地兜底内容: %v", err)
		refinedContent = prd.BuildFallbackRefinedContent(task.Title, task.Source.Content, err)
	} else {
		refinedContent = prd.ExtractRefinedContent(refinedContent)
		if validateErr := prd.ValidateRefinedContent(refinedContent); validateErr != nil {
			color.Yellow("⚠ AI refine 输出未通过校验，使用本地兜底内容: %v", validateErr)
			refinedContent = prd.BuildFallbackRefinedContent(task.Title, task.Source.Content, validateErr)
		}
	}
	clearRefineProgressLine()
	color.Cyan("      生成耗时: %s", formatDurationSeconds(time.Since(generateStartedAt)))

	color.Cyan("   [3/3] 正在写入 task 产物...")
	if err := prd.WriteRefinedContent(task, refinedContent, now, prd.TaskStatusRefined); err != nil {
		return err
	}

	color.Green("✓ refine 完成")
	color.Green("  source.json: %s", task.SourceMetaPath)
	color.Green("  prd.source.md: %s", task.SourcePath)
	color.Green("  prd-refined.md: %s", task.RefinedPath)
	color.Green("  task.json: %s", task.MetadataPath)
	color.Green("⏱ 本次 refine 总耗时: %s", formatDurationSeconds(time.Since(startedAt)))

	if relPath, relErr := filepath.Rel(repoRoot, task.TaskDir); relErr == nil {
		color.Green("  next: coco-ext prd plan --task %s", filepath.Base(relPath))
	}

	return nil
}

func clearRefineProgressLine() {
	fmt.Print("\r\033[K")
}
