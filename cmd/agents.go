package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var agentsForce bool

const cocoExtSectionMarker = "## Coco-Ext 行为约束"

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "生成 AGENTS.md 行为约束文件",
	Long:  "在仓库根目录生成或更新 AGENTS.md，定义 AI 编码时的行为约束准则。",
	RunE:  runAgents,
}

func init() {
	rootCmd.AddCommand(agentsCmd)
	agentsCmd.Flags().BoolVarP(&agentsForce, "force", "f", false, "强制重新生成 AGENTS.md（会覆盖已有的 Coco-Ext 约束）")
}

func runAgents(cmd *cobra.Command, args []string) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取当前目录失败: %w", err)
	}

	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err != nil {
		return fmt.Errorf("当前目录不是 git 仓库: %s", repoRoot)
	}

	agentsPath := filepath.Join(repoRoot, "AGENTS.md")

	existingContent, err := os.ReadFile(agentsPath)
	hasExisting := err == nil

	if hasExisting && !agentsForce {
		// 检查是否已存在 Coco-Ext section
		if strings.Contains(string(existingContent), cocoExtSectionMarker) {
			color.Yellow("AGENTS.md 已存在，且包含 Coco-Ext 行为约束")
			color.Yellow("使用 --force 强制重新生成（或手动编辑 AGENTS.md）")
			return nil
		}
	}

	// 生成新内容
	newSection := fmt.Sprintf(`%s

本部分由 coco-ext agents 生成，定义 Coco-Ext 场景下的行为约束。

### 1. 用户已修改的代码：不要覆盖

如果发现用户对某段代码做了修改（即使是微小的调整），不要在后续迭代中直接覆盖。
**正确做法**：先说明改动范围，询问用户确认。

### 2. 有疑问要先问

遇到以下情况，不要猜、不要假设，先问用户：
- 需求不明确
- 术语不理解
- 逻辑不清晰
- 上下文矛盾

**正确做法**："我理解你的意思是...，确认一下..." 或 "这个地方不太清楚，能否解释一下..."

### 3. 不要脑补信息

如果必要信息不足，明确说明：
- 你理解的是什么
- 哪些是不确定的部分
- 需要用户确认什么

**禁止**：假设用户的意图、猜测业务逻辑、自行填补信息空白。

### 4. 不要擅自做决定

以下类型的决定必须先征求用户意见：
- 架构改动
- 技术选型
- 核心逻辑变更
- 大规模重构

**正确做法**：提出 2-3 个方案，说明利弊，让用户选择或确认。

### 5. 附加规则

- **修改前说明**：每次修改前简要说清楚改动范围
- **危险操作确认**：删除文件、重写核心模块、大规模删除等操作必须明确确认
- **用户拒绝后不再提**：如果用户明确拒绝某个建议，记录后不再重复提起
- **不确定性声明**：当你不确定时，明确说"我不确定..."，而不是装作知道

---`, cocoExtSectionMarker)

	var finalContent string
	if hasExisting && !agentsForce {
		// 追加到现有文件
		finalContent = strings.TrimSpace(string(existingContent)) + "\n\n" + newSection + "\n"
	} else {
		// 新建文件
		header := `# AGENTS.md

本文件定义了 AI 编码时的行为约束准则，所有 AI 协作必须遵守。
`
		finalContent = header + newSection + "\n"
	}

	if err := os.WriteFile(agentsPath, []byte(finalContent), 0600); err != nil {
		return fmt.Errorf("写入 AGENTS.md 失败: %w", err)
	}

	if hasExisting && !agentsForce {
		color.Green("✓ AGENTS.md 已更新，追加了 Coco-Ext 行为约束")
	} else {
		color.Green("✓ AGENTS.md 已生成: %s", agentsPath)
	}
	color.Yellow("提示：请 review 文件内容，按需调整约束条款")

	return nil
}
