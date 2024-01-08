package consoles

import (
	"github.com/cloudwego/hertz/pkg/app"
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

			hp := net.JoinHostPort(facades.Cfg.GetString("server.address"), facades.Cfg.GetString("server.port"))

			options := []config.Option{
				server.WithHostPorts(hp),
			}

			if facades.Validator != nil {
				options = append(options, server.WithCustomValidator(facades.Validator))
			}

			if facades.Cfg.GetBool("server.debug") {
				options = append(options, server.WithExitWaitTime(1))
			} else {
				options = append(options, server.WithDisablePrintRoute(true))
			}

			if option, ok := facades.Cfg.Get("server.options").([]config.Option); ok {
				options = append(options, option...)
			}

			serv := server.Default(options...)

			if middlewares, ok := facades.Cfg.Get("server.middlewares").([]app.HandlerFunc); ok {
				serv.Use(middlewares...)
			}

			if route, ok := facades.Cfg.Get("server.route").(func(route *server.Hertz)); ok {
				route(serv)
			}

			if handle, ok := facades.Cfg.Get("server.handle").(func(s *server.Hertz)); ok {
				handle(serv)
			}

			serv.Spin()
		},
	}
}
