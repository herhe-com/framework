package consoles

import (
	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/console"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

type SecretProvider struct {
}

func (s *SecretProvider) Register() console.Console {

	return console.Console{
		Cmd:     "secret",
		Name:    "生成密钥",
		Summary: "该操作并不会影响现有的数据及文件，请将生成后的密钥复制到需要使用的位置",
		Run: func(cmd *cobra.Command, args []string) {
			color.Successf("\n\n密钥：%s\n\n", lo.RandomString(32, lo.AlphanumericCharset))
		},
	}
}
