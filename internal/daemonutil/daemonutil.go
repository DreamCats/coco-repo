package daemonutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	acpdaemon "github.com/DreamCats/coco-acp-sdk/daemon"
)

const stopWaitTimeout = 3 * time.Second

// RepairBrokenState 在 daemon 不可连接但 pid/sock 残留时，尝试终止旧进程并清理状态文件。
// 返回值 repaired 表示是否执行了清理动作。
func RepairBrokenState(configDir string) (repaired bool, err error) {
	if acpdaemon.IsRunningAt(configDir) {
		return false, nil
	}

	pidPath := filepath.Join(configDir, "daemon.pid")
	sockPath := filepath.Join(configDir, "daemon.sock")

	hasPid := fileExists(pidPath)
	hasSock := fileExists(sockPath)
	if !hasPid && !hasSock {
		return false, nil
	}

	if hasPid {
		pid, readErr := readPID(pidPath)
		if readErr == nil && pid > 0 {
			if stopErr := terminateProcess(pid); stopErr != nil {
				return true, fmt.Errorf("终止异常 daemon 进程失败: %w", stopErr)
			}
		}
	}

	if err := removeIfExists(sockPath); err != nil {
		return true, fmt.Errorf("删除残留 daemon.sock 失败: %w", err)
	}
	if err := removeIfExists(pidPath); err != nil {
		return true, fmt.Errorf("删除残留 daemon.pid 失败: %w", err)
	}

	return true, nil
}

func readPID(pidPath string) (int, error) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return pid, nil
}

func terminateProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil && !isProcessMissing(err) {
		return err
	}

	deadline := time.Now().Add(stopWaitTimeout)
	for time.Now().Before(deadline) {
		if proc.Signal(syscall.Signal(0)) != nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err := proc.Signal(syscall.SIGKILL); err != nil && !isProcessMissing(err) {
		return err
	}
	return nil
}

func isProcessMissing(err error) bool {
	return err == syscall.ESRCH || errors.Is(err, os.ErrProcessDone)
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
