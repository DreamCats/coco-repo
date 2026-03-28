package cmd

import (
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "仓库上下文知识库管理",
	Long:  "通过 AI 自动扫描分析仓库，生成并维护 .livecoding/context/ 目录下的业务知识文件。",
}

func init() {
	rootCmd.AddCommand(contextCmd)
}
