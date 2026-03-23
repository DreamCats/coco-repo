package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-repo/internal/config"
	"github.com/DreamCats/coco-repo/internal/knowledge"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看知识库状态",
	Long:  "显示 .livecoding/context/ 下各知识文件的状态、条目数量和覆盖率。",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	st, err := knowledge.ReadStatus(repoRoot)
	if err != nil {
		return err
	}

	if !st.Exists {
		color.Yellow(".livecoding/context/ 目录不存在，请先运行 coco-repo init")
		return nil
	}

	fmt.Println()
	color.Cyan(".livecoding/context/ 知识库状态")
	fmt.Println("──────────────────────────────")

	existCount := 0
	var latestMod time.Time

	for _, f := range st.Files {
		desc := config.KnowledgeFiles[f.Name]
		if f.Exists {
			existCount++
			if f.ModTime.After(latestMod) {
				latestMod = f.ModTime
			}
			ago := time.Since(f.ModTime).Round(time.Minute)
			color.Green("  ✓ %-20s %s  (更新于 %s 前)", f.Name, desc, formatDuration(ago))
		} else {
			color.Red("  ✗ %-20s %s  (未生成)", f.Name, desc)
		}
	}

	fmt.Println("──────────────────────────────")
	total := len(config.KnowledgeFiles)
	pct := existCount * 100 / total
	fmt.Printf("  覆盖率: %d/%d (%d%%)\n", existCount, total, pct)
	fmt.Println()

	return nil
}

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%d 分钟", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d 小时", int(d.Hours()))
	}
	return fmt.Sprintf("%d 天", int(d.Hours()/24))
}
