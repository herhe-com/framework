# Herhe Framework

这是一个基于 Cloudwego Hertz、GORM、Viper、Casbin 等组件封装的 Go 应用基础框架。它采用类似 Laravel 的组织方式：`foundation` 负责启动生命周期，`config` 负责配置聚合，业务能力通过 `service.Provider` 注册到 `facades` 全局访问器。

## 当前实现

框架启动分两段完成：

1. 导入 `foundation` 包时，`init()` 会设置 `facades.Root`，并注册基础配置服务。
2. 应用入口显式创建 `foundation.Application{}` 并调用 `Boot()`，随后根据 `facades.Cfg.Get("kernel.providers")` 中的 provider 实例注册业务服务。

`example` 基础项目的推荐接入方式如下：

```go
func Boot() {
	application := foundation.Application{}
	application.Boot()

	config.Boot()
}
```

基础项目可以在 `server/admin/config/kernel.go` 或 `server/web/config/kernel.go` 的 `init()` 中，通过代码把 provider 实例写入配置：

```go
cfg := facades.Cfg
cfg.Add("kernel", map[string]any{
	"providers": []service.Provider{
		&orm.ServiceProvider{},
		&redis.ServiceProvider{},
		&filesystem.ServiceProvider{},
		&validation.ServiceProvider{},
		&console.ServiceProvider{},
	},
})
```

这意味着当前框架并不支持“仅在 YAML 中写 provider 包路径后自动实例化”。如果要从 YAML 驱动 provider，需要额外实现 registry 或反射装配机制。

## 主要模块

- `foundation`: 应用根路径、时区、provider 注册和启动。
- `config`: 基于 Viper 的本地/远程配置读取，支持运行时 `Add` 和 `Set`。
- `facades`: 全局单例访问器，例如 `Cfg`、`DB`、`Redis`、`Storage`、`Queue`、`Validator`。
- `database`: GORM、Redis、MongoDB 连接管理。
- `filesystem`: S3、OSS、COS、MinIO、Qiniu 的统一存储接口。
- `auth`: JWT、Casbin 权限、token 黑名单、临时 token。
- `console`: Cobra 命令封装，内置 server、migration、password 等命令。
- `http`: Hertz 响应和中间件。
- `validation`: validator/v10 和多语言翻译。

详细的模块化配置样例见 [examples/config](</Users/orange/Developer/Project/go/src/github.com/herhe-com/framework/examples/config/README.md>)。

## 结合 example 基础项目看到的问题

1. Provider 装配依赖 Go 代码注入接口实例，文档中“YAML 配置包路径即可注册”的写法不符合当前实现。
2. 框架大量依赖全局 facade，业务层调用很方便，但单元测试需要额外隔离全局状态。
3. 启动失败会直接 `os.Exit(1)`，适合 CLI/服务入口，但不利于库式复用；更理想的方向是让启动链路返回 error。
4. 配置 key 必须和框架实现完全一致，例如 ORM 使用 `database.orm.default` 选择默认连接名、`database.orm.connections.default.driver` 定义默认连接驱动、`database.orm.connections.default.prefix` 定义默认前缀，Redis 使用 `database.redis.default` 选择默认连接名、`database.redis.connections.default.db` 定义默认 DB，文件系统使用 `filesystem.default` 选择默认磁盘名。
5. 队列和搜索配置使用 `queue.default`、`search.default` 选择默认连接名，再用 `queue.connections.default.driver`、`search.connections.default.driver` 这类实例级驱动字段定义具体连接；example 基础项目如果少了 `default` 层，启用对应 `ServiceProvider` 时会初始化失败。
6. 当前框架测试覆盖仍然偏少，核心配置、启动、驱动分发、权限和中间件都应补回归测试。

## 验证

```bash
go test ./...
```
