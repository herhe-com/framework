package consoles

import (
	"fmt"
	"net"

	"github.com/cloudwego/kitex/server"
	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/console"
	"github.com/herhe-com/framework/facades"
	"github.com/spf13/cobra"
)

type ServiceProvider struct {
}

func (p *ServiceProvider) Register() console.Console {

	return console.Console{
		Cmd:  "service",
		Name: "启动微服务",
		Run: func(cmd *cobra.Command, args []string) {

			addr := facades.Cfg.GetString("service.address", "0.0.0.0")
			port := facades.Cfg.GetString("service.port", "8600")

			options := make([]server.Option, 0)

			address, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%s", addr, port))

			options = append(options, server.WithServiceAddr(address))

			opts, _ := facades.Cfg.Get("service.options").([]server.Option)

			if len(opts) > 0 {
				options = append(options, opts...)
			}

			service, ok := facades.Cfg.Get("service.handle").(func(options ...server.Option) error)

			if !ok {
				color.Errorln("未配置微服务启动项\n")
				return
			}

			if err := service(options...); err != nil {
				color.Errorln("微服务启动失败：%v\n", err)
			}
		},
	}
}
