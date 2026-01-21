package consoles

import (
	"strings"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/auth"
	"github.com/herhe-com/framework/contracts/console"
	"github.com/pterm/pterm"
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

			password, err := pterm.DefaultInteractiveTextInput.
				WithDefaultText("密码").
				WithOnInterruptFunc(func() {
					color.Errorln("输入取消")
				}).
				Show()

			if err != nil {
				return
			}

			pwd := strings.TrimSpace(password)
			if pwd == "" {
				color.Errorln("密码不能为空")
				return
			} else if pwd != password {
				color.Errorln("输入的密码前后不能包含空字符")
				return
			}

			color.Successf("\n\n明文：%s\n密码：%s\n\n", password, auth.Password(password))
		},
	}
}
