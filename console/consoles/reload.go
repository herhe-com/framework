package consoles

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/console"
	"github.com/herhe-com/framework/facades"
	"github.com/spf13/cobra"
)

type ReloadProvider struct {
}

func (*ReloadProvider) Register() console.Console {
	return console.Console{
		Cmd:  "reload",
		Name: "软重启（发送 SIGHUP 信号）",
		Summary: `向当前运行的进程发送 SIGHUP 信号进行软重启（优雅重载配置和服务）

使用方式：
  1. 自动查找进程：
     ./app reload
     
  2. 指定进程 ID：
     ./app reload -p 12345
     
  3. 通过 PID 文件：
     ./app reload -f /var/run/app.pid

参数说明：
  -p, --process  指定进程 ID
  -f, --pid      指定 PID 文件路径
  
如果不指定任何参数，命令会自动查找正在运行的服务进程。`,
		Run: func(cmd *cobra.Command, args []string) {

			pidFile, _ := cmd.Flags().GetString("pid")
			pidFlag, _ := cmd.Flags().GetInt("process")

			var pid int
			var err error

			// 优先级：--process > --pid > 自动查找
			if pidFlag > 0 {
				pid = pidFlag
			} else if pidFile != "" {
				// 从 PID 文件读取
				data, err := os.ReadFile(pidFile)
				if err != nil {
					color.Errorf("[reload] 无法读取 PID 文件: %s\n", err)
					return
				}
				pidStr := strings.TrimSpace(string(data))
				pid, err = strconv.Atoi(pidStr)
				if err != nil {
					color.Errorf("[reload] 无法解析 PID: %s\n", err)
					return
				}
			} else {
				// 自动查找进程
				pid, err = findProcessByName()
				if err != nil {
					color.Errorf("[reload] %s\n", err)
					color.Infoln("[reload] 提示：使用 --pid 指定 PID 文件路径，或使用 --process 指定进程 ID")
					return
				}
			}

			if pid <= 0 {
				color.Errorln("[reload] 无效的进程 ID")
				return
			}

			// 查找进程
			process, err := os.FindProcess(pid)
			if err != nil {
				color.Errorf("[reload] 无法找到进程 PID=%d: %s\n", pid, err)
				return
			}

			// 验证进程是否存在
			if err := process.Signal(syscall.Signal(0)); err != nil {
				color.Errorf("[reload] 进程 PID=%d 不存在或无权限访问\n", pid)
				return
			}

			// 发送 SIGHUP 信号
			if err := process.Signal(syscall.SIGHUP); err != nil {
				color.Errorf("[reload] 发送 SIGHUP 信号失败: %s\n", err)
				return
			}

			color.Successf("[reload] 成功向进程 PID=%d 发送软重启信号\n", pid)
		},
		Tags: func(cmd *cobra.Command) {
			cmd.Flags().StringP("pid", "f", "", "指定 PID 文件路径")
			cmd.Flags().IntP("process", "p", 0, "指定进程 ID")
		},
	}
}

// findProcessByName 通过进程名查找当前运行的进程 ID
func findProcessByName() (int, error) {
	executable, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("无法获取可执行文件路径: %v", err)
	}

	execName := filepath.Base(executable)
	appName := facades.Cfg.GetString("app.name", "")

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
