package nps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// getNpsConfDir 获取 NPS 服务端配置目录
func getNpsConfDir(dataDir string, id uint) string {
	return filepath.Join(dataDir, "nps", fmt.Sprintf("server_%d", id))
}

// runNpsServer 以子进程方式运行 NPS 服务端（阻塞直到 ctx 取消或子进程退出）
//
// NPS 库（djylb/nps）内部在多处直接调用 os.Exit()，无法作为库安全嵌入主进程。
// 因此通过重新启动自身可执行文件并传入 --nps-server 子命令，在独立子进程中运行 NPS，
// 子进程退出不会影响主进程。
func runNpsServer(ctx context.Context, confDir string) error {
	// 获取当前可执行文件路径
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		cmd := exec.CommandContext(ctx, exe, "--nps-server", "--nps-conf", confDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("启动 NPS 子进程失败: %w", err)
		}

		// 等待子进程退出
		waitErr := cmd.Wait()

		// ctx 已取消，正常退出
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// 子进程异常退出，5 秒后自动重启
		if waitErr != nil {
			// 退出码 0 也视为异常（NPS 内部 os.Exit(0) 触发）
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
		}
	}
}