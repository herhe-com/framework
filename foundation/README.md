# Foundation 组件

`foundation` 负责应用启动生命周期：设置项目根路径、初始化基础配置服务、按配置注册业务服务提供者，并设置应用时区。

## 启动流程

当前实现分为两个阶段：

1. `foundation` 包导入时会执行 `init()`，设置 `facades.Root`，并注册/启动基础 `config.ServiceProvider`。
2. 应用入口创建 `foundation.Application{}` 并调用 `Boot()`，此时会读取 `kernel.providers` 中的 provider 实例并执行 `Register()` 和 `Boot()`。

```go
package bootstrap

import (
	"github.com/herhe-com/framework/foundation"

	"your-app/config"
)

func Boot() {
	application := foundation.Application{}
	application.Boot()

	config.Boot()
}
```

## Provider 接口

Provider 必须实现 `contracts/service.Provider`：

```go
type Provider interface {
	Boot() error
	Register() error
}
```

`Register()` 通常用于初始化组件并写入 `facades`，`Boot()` 用于执行依赖其他组件的启动逻辑。

## 注册 Provider

当前框架不会根据 YAML 里的包路径自动创建 provider。应用需要在 Go 代码中把 provider 实例写入 `facades.Cfg`：

```go
package config

import (
	"github.com/herhe-com/framework/contracts/service"
	"github.com/herhe-com/framework/database/orm"
	"github.com/herhe-com/framework/database/redis"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/filesystem"
	"github.com/herhe-com/framework/validation"
)

func init() {
	facades.Cfg.Add("kernel", map[string]any{
		"providers": []service.Provider{
			&orm.ServiceProvider{},
			&redis.ServiceProvider{},
			&filesystem.ServiceProvider{},
			&validation.ServiceProvider{},
		},
	})
}
```

`example` 基础项目的 `server/admin/config/kernel.go` 和 `server/web/config/kernel.go` 推荐使用这种方式。

## 错误处理

如果 provider 的 `Register()` 或 `Boot()` 返回错误，当前实现会打印错误并以 `os.Exit(1)` 退出进程。这适合应用入口和 CLI，但如果未来要把 `foundation` 当作可复用库，建议把启动错误返回给调用方处理。

## 时区

`Application.Boot()` 会读取 `app.location` 并设置 `time.Local` 和 Carbon 的默认时区：

```go
facades.Cfg.Add("app", map[string]any{
	"location": "Asia/Shanghai",
})
```

## 注意事项

- `facades.Root` 来自启动进程的当前工作目录。
- `kernel.providers` 必须是 `[]service.Provider`，不是字符串数组。
- provider 顺序有依赖关系时需要显式排序，例如 `orm` 和 `redis` 应早于依赖数据库或 Redis 的 provider。
- 需要使用 console server 时，应在 `kernel.providers` 中注册 `console.ServiceProvider`，并在 `kernel.consoles` 中注册 `consoles.ServerProvider`。
