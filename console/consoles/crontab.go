package consoles

import (
	"fmt"
	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/console"
	"github.com/herhe-com/framework/crontab"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"syscall"
)

type CrontabProvider struct {
}

func (c *CrontabProvider) Register() console.Console {

	return console.Console{
		Cmd:  "crontab",
		Name: "定时任务",
		Run: func(cmd *cobra.Command, args []string) {

			c.init()

			color.Successf("\n\n定时任务成功\n\n")

			signs := make(chan os.Signal, 1)
			done := make(chan bool, 1)

			signal.Notify(signs, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				sig := <-signs
				fmt.Println(sig)
				done <- true
			}()

			<-done
			color.Warnln("\n\n定时任务已停止运行\n\n")
		},
	}
}

func (c *CrontabProvider) init() {

	cron := crontab.Application{}

	cron.Init()

	cron.Start()
}
