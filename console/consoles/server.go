package consoles

import (
	"net"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/go-playground/validator/v10"
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

			hp := net.JoinHostPort(facades.Config().GetString("server.address", "0.0.0.0"), facades.Config().GetString("server.port", "9600"))

			options := []config.Option{
				server.WithHostPorts(hp),
			}

			if validate, ok := facades.Get[*validator.Validate](); ok {
				options = append(options, server.WithCustomValidatorFunc(func(_ *protocol.Request, req interface{}) error {
					return validate.Struct(req)
				}))
			}

			if facades.Config().GetBool("app.debug") {
				options = append(options, server.WithExitWaitTime(1))
			} else {
				options = append(options, server.WithDisablePrintRoute(true))
			}

			if option, ok := facades.Config().Get("server.options").([]config.Option); ok {
				options = append(options, option...)
			}

			serv := server.Default(options...)

			if middlewares, ok := facades.Config().Get("server.middlewares").([]app.HandlerFunc); ok {
				serv.Use(middlewares...)
			}

			if route, ok := facades.Config().Get("server.route").(func(route *server.Hertz)); ok {
				route(serv)
			}

			if handle, ok := facades.Config().Get("server.handle").(func(s *server.Hertz)); ok {
				handle(serv)
			}

			if facades.Config().GetBool("app.debug") {
				serv.GET("/swagger/*any", swagger.WrapHandler(files.Handler, swagger.DefaultModelsExpandDepth(-1)))
			}

			serv.Spin()
		},
	}
}
