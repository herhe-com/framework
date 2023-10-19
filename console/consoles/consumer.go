package consoles

import (
	"errors"
	"fmt"
	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/console"
	"github.com/herhe-com/framework/contracts/util"
	"github.com/herhe-com/framework/facades"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
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

func (that *ConsumerProvider) username() (username string, err error) {

	prompt := promptui.Prompt{
		Label: "用户名",
		Validate: func(str string) error {

			if ok, _ := regexp.MatchString(util.PatternOfUsername, str); !ok {
				return errors.New("格式错误")
			}

			return nil
		},
	}

	if username, err = prompt.Run(); err != nil {
		return
	}

	return username, nil
}

func (that *ConsumerProvider) password() (password string, err error) {

	prompt := promptui.Prompt{
		Label: "密码",
		Validate: func(str string) error {

			if ok, _ := regexp.MatchString(util.PatternOfPassword, str); !ok {
				return errors.New("格式错误")
			}

			return nil
		},
	}

	if password, err = prompt.Run(); err != nil {
		return
	}

	return password, nil
}

func (that *ConsumerProvider) nickname() (nickname string, err error) {

	prompt := promptui.Prompt{
		Label: "昵称",
		Validate: func(str string) error {

			s := strings.TrimSpace(str)

			if s == "" {
				return errors.New("昵称不能为空")
			} else if s != str {
				return errors.New("不能包含空字符")
			}

			return nil
		},
	}

	if nickname, err = prompt.Run(); err != nil {
		return
	}

	return nickname, nil
}
