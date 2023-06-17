package consoles

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/herhe-com/framework/contracts/console"
	"github.com/herhe-com/framework/facades"
	"github.com/spf13/cobra"
	"net"
)

type ServerProvider struct {
}

func (*ServerProvider) Register() console.Console {
	return console.Console{
		Cmd:  "server",
		Name: "启动程序",
		Run: func(cmd *cobra.Command, args []string) {

			host := net.JoinHostPort(facades.Cfg.GetString("app.address"), facades.Cfg.GetString("app.port"))

			options := []config.Option{
				server.WithHostPorts(host),
			}

			if facades.Cfg.GetBool("app.debug") {
				options = append(options, server.WithExitWaitTime(1))
			}

			facades.Server = server.Default(options...)
		},
	}
}
