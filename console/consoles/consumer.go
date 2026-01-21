package consoles

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/console"
	"github.com/herhe-com/framework/facades"
	"github.com/spf13/cobra"
)

type ConsumerProvider struct {
}

func (that *ConsumerProvider) Register() console.Console {

	return console.Console{
		Cmd:  "consumer",
		Name: "消费队列",
		Run: func(cmd *cobra.Command, args []string) {

			consumes, _ := facades.Cfg.Get("queue.consumes").([]func())

			for _, consume := range consumes {
				go consume()
			}

			color.Successf("\n\n消费队列运行成功\n\n")

			// block main thread - wait for shutdown signal
			signs := make(chan os.Signal, 1)
			done := make(chan bool, 1)

			signal.Notify(signs, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				sig := <-signs
				fmt.Println(sig)
				done <- true
			}()

			<-done
			color.Warnln("\n\n消费队列已停止运行\n\n")
		},
	}
}
