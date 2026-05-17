# Facades 组件

`facades` 提供全局单例访问器。框架中的服务 provider 会把初始化后的组件写入这些变量，业务代码再通过 `facades.DB`、`facades.Cfg`、`facades.Storage` 等访问能力。

## 可用 Facade

| 名称 | 类型 | 初始化来源 |
| --- | --- | --- |
| `Cfg` | `contracts/config.Application` | `config.ServiceProvider` |
| `DB` | `contracts/database.DB` | `database/orm.ServiceProvider` |
| `Redis` | `contracts/database.Redis` | `database/redis.ServiceProvider` |
| `Mongo` | `contracts/mongodb.Mongo` | `database/mongodb.ServiceProvider` |
| `Storage` | `contracts/filesystem.Storage` | `filesystem.ServiceProvider` |
| `Queue` | `contracts/queue.Queue` | `queue.ServiceProvider` |
| `Search` | `contracts/search.Search` | `search.ServiceProvider` |
| `AI` | `contracts/ai.AI` | `ai.ServiceProvider` |
| `Validator` | `*validator.Validate` | `validation.ServiceProvider` |
| `Console` | `*cobra.Command` | `console.ServiceProvider` |
| `Casbin` | `*casbin.Enforcer` | `auth.ServiceProvider` |
| `Locker` | `*redsync.Redsync` | `microservice/locker.ServiceProvider` |
| `Snowflake` | `*snowflake.Node` | `microservice/snowflake.ServiceProvider` |
| `Root` | `string` | `foundation.init()` |

## 常用示例

配置：

```go
name := facades.Cfg.GetString("app.name", "UPER")
debug := facades.Cfg.GetBool("app.debug", false)
```

数据库：

```go
db := facades.DB.Default()
reportDB, err := facades.DB.Drivers("mysql", "report")
```

Redis：

```go
redis := facades.Redis.Default()
cacheRedis, err := facades.Redis.Channel("cache")
```

文件存储：

```go
err := facades.Storage.Put("uploads/file.txt", reader, size)

s3Public, err := facades.Storage.Disk("s3", "public")
```

队列：

```go
err := facades.Queue.Producer(body, "basic", "basic_email", []string{"email"}, 0, 0)

err = facades.Queue.Consumer(handler, "basic", "basic_email", "email", false, 0, 3)
```

校验器：

```go
err := facades.Validator.Struct(request)
```

权限：

```go
allowed, err := facades.Casbin.Enforce(user, resource, action)
```

## 初始化顺序

业务代码必须在对应 provider 注册后再使用 facade。`example` 基础项目推荐启动顺序如下：

1. `foundation` 初始化 `facades.Cfg`。
2. 业务 `config/*.go` 通过 `init()` 写入 `kernel.providers`。
3. `foundation.Application{}.Boot()` 注册 `orm`、`redis`、`filesystem`、`validation`、`auth`、`console` 等 provider。
4. 路由和业务 handler 中使用 `facades.*`。

## 风险和约束

- Facade 是全局变量，测试时需要小心隔离状态。
- 未注册 provider 时直接调用会出现 nil panic，例如 `facades.DB.Default()`。
- 多服务共用同一进程时，不同服务写入同名配置会互相覆盖。
- `Console` 是 Cobra 根命令，命令注册由 `console.ServiceProvider` 完成，不应调用不存在的 `facades.Console.Register()`。
