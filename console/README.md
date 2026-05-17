# Console 组件

`console` 基于 Cobra 封装命令行入口。注册 `console.ServiceProvider` 后，框架会创建 `facades.Console`，加载内置命令和 `kernel.consoles` 中的命令，然后执行 `facades.Console.Execute()`。

## 配置

命令通过 Go 代码注册到 `kernel.consoles`：

```go
facades.Cfg.Add("kernel", map[string]any{
	"consoles": []console.Provider{
		&consoles.ServerProvider{},
		&consoles.MigrationProvider{},
	},
})
```

`example` 基础项目的 admin 服务也可以注册自定义命令：

```go
"consoles": []cons.Provider{
	&consoles.MigrationProvider{},
	&consoles.ServerProvider{},
	&consoles.ReloadProvider{},
	&consoles.RestartProvider{},
	&developer.DeveloperProvider{},
}
```

## Provider 接口

```go
type Provider interface {
	Register() Console
}
```

命令数据结构：

```go
type Console struct {
	Cmd      string
	Name     string
	Summary  string
	Consoles []Console
	Run      func(cmd *cobra.Command, args []string)
	Tags     func(cmd *cobra.Command)
}
```

## 自定义命令

```go
package console

import (
	"fmt"

	"github.com/herhe-com/framework/contracts/console"
	"github.com/spf13/cobra"
)

type HelloProvider struct{}

func (*HelloProvider) Register() console.Console {
	return console.Console{
		Cmd:     "hello",
		Name:    "问候",
		Summary: "输出问候信息",
		Tags: func(cmd *cobra.Command) {
			cmd.Flags().String("name", "world", "name to greet")
		},
		Run: func(cmd *cobra.Command, args []string) {
			name, _ := cmd.Flags().GetString("name")
			fmt.Printf("hello %s\n", name)
		},
	}
}
```

## Server 命令

`consoles.ServerProvider` 会读取以下配置启动 Hertz：

```go
facades.Cfg.Add("server", map[string]any{
	"address": "0.0.0.0",
	"port":    "9600",
	"route":   route.Router,
	"middlewares": []app.HandlerFunc{
		middleware.Access(),
		middleware.Cors(),
	},
})
```

运行：

```bash
go run main.go server
```

`server.route` 的类型必须是：

```go
func(route *server.Hertz)
```

## 注意事项

- `facades.Console` 是 `*cobra.Command`，不是带 `Register()` 方法的自定义对象。
- 命令注册发生在 `console.ServiceProvider.Register()` 中，注册后会立即执行 `Execute()`。
- 如果只想启动 HTTP 服务，至少需要注册 `console.ServiceProvider` 和 `consoles.ServerProvider`。
- `kernel.consoles` 当前必须由 Go 代码写入 `[]contracts/console.Provider`，不能只写 YAML 字符串。
