package consoles

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/console"
	"github.com/herhe-com/framework/facades"
	"github.com/spf13/cobra"
)

type RestartProvider struct {
}

func (*RestartProvider) Register() console.Console {
	return console.Console{
		Cmd:  "restart",
		Name: "硬重启（完全重启应用）",
		Summary: `停止当前进程并重新启动应用程序（完全重启）

使用方式：
  1. 自动查找并重启：
     ./app restart
     
  2. 指定进程 ID 重启：
     ./app restart -p 12345
     
  3. 通过 PID 文件重启：
     ./app restart -f /var/run/app.pid
     
  4. 以守护进程模式重启：
     ./app restart -d
     
  5. 组合使用：
     ./app restart -f /var/run/app.pid -d

参数说明：
  -p, --process  指定要停止的进程 ID
  -f, --pid      指定 PID 文件路径
  -d, --daemon   以守护进程模式启动新进程
  
重启流程：
  1. 先发送 SIGTERM 信号优雅关闭（等待最多 5 秒）
  2. 如果进程未响应，强制终止（SIGKILL）
  3. 等待端口释放
  4. 启动新的服务进程`,
		Run: func(cmd *cobra.Command, args []string) {

			pidFile, _ := cmd.Flags().GetString("pid")
			pidFlag, _ := cmd.Flags().GetInt("process")
			daemon, _ := cmd.Flags().GetBool("daemon")

			var pid int
			var err error

			// 优先级：--process > --pid > 自动查找
			if pidFlag > 0 {
				pid = pidFlag
			} else if pidFile != "" {
				// 从 PID 文件读取
				data, err := os.ReadFile(pidFile)
				if err != nil {
					color.Warnf("[restart] 无法读取 PID 文件（进程可能未运行）: %s\n", err)
				} else {
					pidStr := strings.TrimSpace(string(data))
					pid, err = strconv.Atoi(pidStr)
					if err != nil {
						color.Warnf("[restart] 无法解析 PID: %s\n", err)
					}
				}
			} else {
				// 自动查找进程
				pid, err = findRunningProcess()
				if err != nil {
					color.Warnf("[restart] %s\n", err)
				}
			}

			// 如果找到了运行中的进程，尝试停止它
			if pid > 0 {
				if err := stopProcess(pid); err != nil {
					color.Errorf("[restart] 停止进程失败: %s\n", err)
					return
				}
			} else {
				color.Infoln("[restart] 未找到运行中的进程，将直接启动新进程")
			}

			// 等待一小段时间确保端口释放
			time.Sleep(1 * time.Second)

			// 获取当前可执行文件路径
			executable, err := os.Executable()
			if err != nil {
				color.Errorf("[restart] 无法获取可执行文件路径: %s\n", err)
				return
			}

			// 解析符号链接
			executable, err = filepath.EvalSymlinks(executable)
			if err != nil {
				color.Errorf("[restart] 无法解析符号链接: %s\n", err)
				return
			}

			// 准备启动新进程
			commandArgs := []string{"server"}
			if len(args) > 0 {
				commandArgs = append(commandArgs, args...)
			}

			color.Infof("[restart] 正在启动新进程: %s %v\n", executable, commandArgs)

			// 创建新进程
			newCmd := exec.Command(executable, commandArgs...)

			if daemon {
				// 守护进程模式：分离标准输入输出
				newCmd.Stdout = nil
				newCmd.Stderr = nil
				newCmd.Stdin = nil
				newCmd.SysProcAttr = &syscall.SysProcAttr{
					Setsid: true,
				}
			} else {
				newCmd.Stdout = os.Stdout
				newCmd.Stderr = os.Stderr
				newCmd.Stdin = os.Stdin
			}

			newCmd.Env = os.Environ()

			// 启动新进程
			if err := newCmd.Start(); err != nil {
				color.Errorf("[restart] 启动新进程失败: %s\n", err)
				return
			}

			color.Successf("[restart] 成功启动新进程 PID=%d\n", newCmd.Process.Pid)

			if daemon {
				color.Infof("[restart] 以守护进程模式运行，进程已分离\n")
				newCmd.Process.Release()
			} else {
				// 等待新进程
				if err := newCmd.Wait(); err != nil {
					color.Errorf("[restart] 新进程异常退出: %s\n", err)
					return
				}
			}
		},
		Tags: func(cmd *cobra.Command) {
			cmd.Flags().StringP("pid", "f", "", "指定 PID 文件路径")
			cmd.Flags().IntP("process", "p", 0, "指定要停止的进程 ID")
			cmd.Flags().BoolP("daemon", "d", false, "以守护进程模式启动新进程")
		},
	}
}

// findRunningProcess 查找当前运行的服务进程
func findRunningProcess() (int, error) {
	executable, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("无法获取可执行文件路径: %v", err)
	}

	execName := filepath.Base(executable)
	appName := facades.Config().GetString("app.name", "")

	// 尝试多种方式查找进程
	commands := []string{
		fmt.Sprintf("pgrep -f '%s.*server'", execName),
		fmt.Sprintf("pgrep -f '%s'", execName),
	}

	if appName != "" {
		commands = append([]string{fmt.Sprintf("pgrep -f '%s.*server'", appName)}, commands...)
	}

	for _, cmdStr := range commands {
		output, err := exec.Command("sh", "-c", cmdStr).Output()
		if err == nil && len(output) > 0 {
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			currentPid := os.Getpid()

			// 排除当前进程，查找其他运行中的进程
			for _, line := range lines {
				if pid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
					if pid != currentPid {
						return pid, nil
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("未找到运行中的服务进程")
}

// stopProcess 停止指定的进程
func stopProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("无法找到进程 PID=%d: %v", pid, err)
	}

	color.Infof("[restart] 正在停止进程 PID=%d...\n", pid)

	// 先尝试优雅关闭
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("发送 SIGTERM 失败: %v", err)
	}

	// 等待进程退出
	maxWait := 5 * time.Second
	checkInterval := 200 * time.Millisecond
	elapsed := time.Duration(0)

	for elapsed < maxWait {
		time.Sleep(checkInterval)
		elapsed += checkInterval

		// 检查进程是否还在运行
		if err := process.Signal(syscall.Signal(0)); err != nil {
			// 进程已经退出
			color.Successf("[restart] 进程 PID=%d 已成功停止\n", pid)
			return nil
		}
	}

	// 进程仍在运行，强制杀死
	color.Warnf("[restart] 进程未响应，强制终止...\n")
	if err := process.Kill(); err != nil {
		return fmt.Errorf("强制终止进程失败: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	color.Successf("[restart] 进程 PID=%d 已被强制终止\n", pid)
	return nil
}
