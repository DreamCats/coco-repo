package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-ext/internal/generator"
	internalgit "github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/prd"
)

var prdPlanTaskID string

var prdPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "基于 refined PRD 和 context 生成 design.md 与 plan.md",
	Long:  "先检查 context，再读取 prd-refined.md 做本地调研、复杂度打分，并生成 design.md 与 plan.md。",
	RunE:  runPRDPlan,
}

func init() {
	prdCmd.AddCommand(prdPlanCmd)
	prdPlanCmd.Flags().StringVar(&prdPlanTaskID, "task", "", "指定 task id；默认读取最近一个 task")
}

func runPRDPlan(cmd *cobra.Command, args []string) error {
	startedAt := time.Now()
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}
	if !internalgit.IsGitRepo(repoRoot) {
		return fmt.Errorf("当前目录不是 git 仓库")
	}

	taskID, err := prd.ResolveTaskID(repoRoot, prdPlanTaskID)
	if err != nil {
		return err
	}

	color.Cyan("🧭 PRD Plan")
	color.Cyan("   task_id: %s", taskID)
	color.Cyan("   [1/4] 正在检查 context...")
	if _, err := prd.LoadContextSnapshot(repoRoot); err != nil {
		return err
	}
	color.Green("   [1/4] context 已就绪 ✓")

	color.Cyan("   [2/4] 正在做本地调研...")
	researchStartedAt := time.Now()
	if _, err := prd.PreparePlanBuild(repoRoot, taskID); err != nil {
		return err
	}
	color.Green("   [2/4] 本地调研完成 ✓")
	color.Cyan("      调研耗时: %s", formatDurationSeconds(time.Since(researchStartedAt)))

	color.Cyan("   [3/4] 正在检查并连接 coco daemon...")
	connectStartedAt := time.Now()
	gen, err := generator.New(repoRoot)
	if err != nil {
		color.Yellow("⚠ 连接 coco daemon 失败，使用本地 plan: %v", err)
		artifacts, genErr := prd.GeneratePlan(repoRoot, taskID, time.Now())
		if genErr != nil {
			return genErr
		}
		clearPlanProgressLine()
		color.Cyan("   [4/4] 产物已写入 task 目录")
		color.Green("✓ plan 完成（本地 fallback）")
		color.Green("  design.md: %s", artifacts.DesignPath)
		color.Green("  plan.md: %s", artifacts.PlanPath)
		color.Green("⏱ 本次 plan 总耗时: %s", formatDurationSeconds(time.Since(startedAt)))
		return nil
	}
	defer gen.Close()
	color.Green("   [3/4] coco daemon 已连接 ✓")
	color.Cyan("      连接耗时: %s", formatDurationSeconds(time.Since(connectStartedAt)))

	color.Cyan("   [4/4] 正在生成 AI 方案...")
	generateStartedAt := time.Now()
	streamStarted := false
	discardedAIOutput := false
	var streamBuffer strings.Builder
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
				fmt.Printf("\r\033[K   AI 生成中，已耗时: %s", formatDurationSeconds(time.Since(generateStartedAt)))
			}
		}
	}()
	artifacts, err := prd.GeneratePlanWithAI(gen, repoRoot, taskID, time.Now(), func(chunk string) {
		streamBuffer.WriteString(chunk)
		if streamStarted {
			fmt.Print(chunk)
			return
		}

		filtered := prd.ExtractPlanStream(streamBuffer.String())
		if filtered == "" || !strings.HasPrefix(filtered, "=== PLAN ===") {
			return
		}

		streamStarted = true
		close(firstChunkShown)
		clearPlanProgressLine()
		fmt.Println("   AI 输出（流式 plan）:")
		fmt.Print(filtered)
	})
	close(stopTicker)
	if streamStarted {
		fmt.Println()
	} else {
		fmt.Print("\r")
	}
	if err != nil {
		return err
	}
	clearPlanProgressLine()
	if !artifacts.UsedAI && streamStarted {
		discardedAIOutput = true
	}
	color.Cyan("      AI 生成耗时: %s", formatDurationSeconds(time.Since(generateStartedAt)))
	if discardedAIOutput {
		color.Yellow("      上述 AI 输出未通过校验，已回退为本地 plan。")
	}

	color.Cyan("   [4/4] 产物已写入 task 目录")
	if artifacts.UsedAI {
		color.Green("✓ plan 完成（AI）")
	} else {
		color.Green("✓ plan 完成（本地 fallback）")
	}
	color.Green("  design.md: %s", artifacts.DesignPath)
	color.Green("  plan.md: %s", artifacts.PlanPath)
	color.Green("⏱ 本次 plan 总耗时: %s", formatDurationSeconds(time.Since(startedAt)))

	return nil
}

func clearPlanProgressLine() {
	fmt.Print("\r\033[K")
}
