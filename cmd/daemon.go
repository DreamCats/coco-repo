package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/DreamCats/coco-acp-sdk/daemon"
	"github.com/DreamCats/coco-ext/internal/config"
)

var daemonCwd string
var daemonIdleTimeout string
var daemonBackground bool

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "coco daemon 管理",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "启动 daemon 服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		if daemonCwd == "" {
			daemonCwd = "."
		}

		var idleTimeout time.Duration
		if daemonIdleTimeout != "" {
			var err error
			idleTimeout, err = time.ParseDuration(daemonIdleTimeout)
			if err != nil {
				return fmt.Errorf("无效的 idle-timeout 值: %w", err)
			}
		}

		// 后台启动
		if daemonBackground {
			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("获取可执行文件路径失败: %w", err)
			}
			startArgs := []string{"daemon", "start", "--cwd", daemonCwd}
			if daemonIdleTimeout != "" {
				startArgs = append(startArgs, "--idle-timeout", daemonIdleTimeout)
			}
			execCmd := exec.Command(exe, startArgs...)
			execCmd.Stdin = nil
			execCmd.Stdout = nil
			execCmd.Stderr = nil
			execCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			if err := execCmd.Start(); err != nil {
				return fmt.Errorf("后台启动 daemon 失败: %w", err)
			}
			// 等待 daemon ready
			time.Sleep(2 * time.Second)
			if daemon.IsRunningAt(configDir) {
				fmt.Printf("daemon已在后台启动 (pid=%d)\n", execCmd.Process.Pid)
			} else {
				fmt.Println("daemon启动中，请稍后用 status 查看")
			}
			return nil
		}

		// 前台启动（阻塞）
		server := daemon.NewServer(configDir, daemonCwd, idleTimeout)
		return server.Run()
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看 daemon 状态",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		if daemon.IsRunningAt(configDir) {
			conn, err := daemon.Dial(".", &daemon.DialOption{ConfigDir: configDir})
			if err != nil {
				return err
			}
			defer conn.Close()

			resp, err := conn.Status()
			if err != nil {
				return err
			}
			fmt.Printf("daemon 运行中 (pid=%d, session=%s, model=%s, uptime=%s)\n",
				resp.PID, resp.SessionID, resp.ModelID, resp.Uptime)
		} else {
			fmt.Println("daemon 未运行")
		}
		return nil
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止 daemon 服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.DefaultConfigDir()
		if !daemon.IsRunningAt(configDir) {
			fmt.Println("daemon 未运行")
			return nil
		}

		conn, err := daemon.Dial(".", &daemon.DialOption{ConfigDir: configDir})
		if err != nil {
			return err
		}
		defer conn.Close()

		if err := conn.Shutdown(); err != nil {
			return err
		}
		fmt.Println("daemon 已停止")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonStartCmd.Flags().StringVar(&daemonCwd, "cwd", "", "工作目录")
	daemonStartCmd.Flags().StringVar(&daemonIdleTimeout, "idle-timeout", "", "空闲超时时间（如 10m）")
	daemonStartCmd.Flags().BoolVarP(&daemonBackground, "background", "d", false, "后台启动")
}
