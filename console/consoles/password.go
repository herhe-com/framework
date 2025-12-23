package consoles

import (
	"errors"
	"strings"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/auth"
	"github.com/herhe-com/framework/contracts/console"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

type PasswordProvider struct {
}

func (p *PasswordProvider) Register() console.Console {

	return console.Console{
		Cmd:     "password",
		Name:    "生成密码",
		Summary: "该生成的密码不会涉及数据变更，只是打印输出",
		Run: func(cmd *cobra.Command, args []string) {

			var err error = nil
			var prompt = promptui.Prompt{}
			var password = ""

			prompt = promptui.Prompt{
				Label: "密码",
				Validate: func(password string) error {

					pwd := strings.TrimSpace(password)

					if pwd == "" {
						return errors.New("密码不能为空")
					} else if pwd != password {
						return errors.New("输入的密码前后不能包含空字符")
					}

					return nil
				},
			}

			if password, err = prompt.Run(); err != nil {
				return
			}

			color.Successf("\n\n明文：%s\n密码：%s\n\n", password, auth.Password(password))
		},
	}
}
