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

			hp := net.JoinHostPort(facades.Cfg.GetString("app.address"), facades.Cfg.GetString("app.port"))

			options := []config.Option{
				server.WithHostPorts(hp),
			}

			if facades.Cfg.GetBool("app.debug") {
				options = append(options, server.WithExitWaitTime(1))
			} else {
				options = append(options, server.WithDisablePrintRoute(true))
			}

			if option, ok := facades.Cfg.Get("app.server.options").([]config.Option); ok {
				options = append(options, option...)
			}

			serv := server.Default(options...)

			if middlewares, ok := facades.Cfg.Get("app.server.middlewares").([]app.HandlerFunc); ok {
				serv.Use(middlewares...)
			}

			if route, ok := facades.Cfg.Get("app.server.route").(func(route *server.Hertz)); ok {
				route(serv)
			}

			if handle, ok := facades.Cfg.Get("app.server.handle").(func(s *server.Hertz)); ok {
				handle(serv)
			}

			serv.Spin()
		},
	}
}
