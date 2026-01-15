package consoles

import (
	"net"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/herhe-com/framework/contracts/console"
	"github.com/herhe-com/framework/facades"
	"github.com/hertz-contrib/swagger"
	"github.com/spf13/cobra"
	files "github.com/swaggo/files"
)

type ServerProvider struct {
}

func (*ServerProvider) Register() console.Console {
	return console.Console{
		Cmd:  "server",
		Name: "启动程序",
		Run: func(cmd *cobra.Command, args []string) {

			hp := net.JoinHostPort(facades.Cfg.GetString("server.address", "0.0.0.0"), facades.Cfg.GetString("server.port", "9600"))

			options := []config.Option{
				server.WithHostPorts(hp),
			}

			if facades.Validator != nil {
				options = append(options, server.WithCustomValidatorFunc(func(_ *protocol.Request, req interface{}) error {
					return facades.Validator.Struct(req)
				}))
			}

			if facades.Cfg.GetBool("app.debug") {
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

			if facades.Cfg.GetBool("app.debug") {
				serv.GET("/swagger/*any", swagger.WrapHandler(files.Handler, swagger.DefaultModelsExpandDepth(-1)))
			}

			serv.Spin()
		},
	}
}
