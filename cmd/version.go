package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// 通过 ldflags 注入
var (
	version   = "dev"
	gitCommit = "unknown"
	buildDate = "unknown"
)

// SetVersionInfo 设置版本信息（由 main.go 调用）
func SetVersionInfo(v, commit, date string) {
	version = v
	gitCommit = commit
	buildDate = date
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("coco-repo %s (commit: %s, built: %s, %s, %s/%s)\n",
			version, gitCommit, buildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
