# Facades 组件

`facades` 提供一个类型索引的服务注册表。Provider 使用 `facades.Register[T]()` 注册服务，业务或框架内部通过 `facades.Get[T]()`、`facades.MustGet[T]()` 或薄访问器获取服务。

旧的全局变量式写法已经移除，例如不再使用 `facades.Cfg.GetString(...)`，改为 `facades.Config().GetString(...)` 或 `facades.MustGet[config.Application]().GetString(...)`。

## 核心 API

注册服务：

```go
facades.Register[config.Application](app)
facades.Register[database.DB](db)
facades.Register[filesystem.Storage](storage)
```

获取服务：

```go
cfg := facades.MustGet[config.Application]()
db, ok := facades.Get[database.DB]()
```

访问器：

```go
name := facades.Config().GetString("app.name", "UPER")
db := facades.Database().Default()
storage := facades.Storage()
```

注册接口实现时需要显式指定接口类型，避免按具体类型注册后无法按接口取回：

```go
facades.Register[database.DB](ormDatabase) // 推荐
facades.Register(ormDatabase)              // 不推荐，可能注册为 *orm.Database
```

## 可用服务

| 访问器 | 注册类型 | 初始化来源 |
| --- | --- | --- |
| `Config()` | `contracts/config.Application` | `config.ServiceProvider` |
| `Database()` | `contracts/database.DB` | `database/orm.ServiceProvider` |
| `Redis()` | `contracts/database.Redis` | `database/redis.ServiceProvider` |
| `Mongo()` | `contracts/mongodb.Mongo` | `database/mongodb.ServiceProvider` |
| `Storage()` | `contracts/filesystem.Storage` | `filesystem.ServiceProvider` |
| `Queue()` | `contracts/queue.Queue` | `queue.ServiceProvider` |
| `Search()` | `contracts/search.Search` | `search.ServiceProvider` |
| `AI()` | `contracts/ai.AI` | `ai.ServiceProvider` |
| `Validator()` | `*validator.Validate` | `validation.ServiceProvider` |
| `Console()` | `*cobra.Command` | `console.ServiceProvider` |
| `Casbin()` | `*casbin.Enforcer` | `auth.ServiceProvider` |
| `Locker()` | `*redsync.Redsync` | `microservice/locker.ServiceProvider` |
| `Snowflake()` | `*snowflake.Node` | `microservice/snowflake.ServiceProvider` |
| `Root()` | `facades.RootPath` | `foundation.init()` |

## 可选依赖

某些能力是可选的，例如 Redis。需要容错时使用 `Get[T]()` 或封装访问器：

```go
if redis, ok := facades.OptionalRedis(); ok {
    redis.Default().Del(ctx, key)
}
```

## 初始化顺序

1. `foundation` 注册 `facades.RootPath`。
2. `config.ServiceProvider` 注册 `contracts/config.Application`。
3. 业务配置中的 `kernel.providers` 注册 ORM、Redis、Filesystem、Validation、Auth、Console 等 provider。
4. 路由和业务 handler 中通过 `facades.*()` 或 `facades.MustGet[T]()` 使用服务。

## 风险和约束

- 注册表仍然是进程级全局状态，测试时需要用 `SetContainer(&facades.Services{})` 隔离。
- 未注册服务时调用 `MustGet[T]()` 或访问器会 panic。
- Provider 内部应使用 `facades.Register[T]()`，不要新增每个服务专属的 `Set/Get`。
