# Database 组件

`database` 提供 GORM 和 Redis 的连接管理。关系型数据库和 Redis 会通过各自的 `ServiceProvider` 初始化到 `facades.DB` 和 `facades.Redis`。

## GORM

支持驱动：

- `mysql`
- `sqlite`
- `postgresql`
- `sqlserver`

配置示例：

```yaml
database:
  orm:
    default: default
    connections:
      default:
        driver: mysql
        username: root
        password: ""
        host: 127.0.0.1
        port: "3306"
        db: upper
        charset: utf8mb4
        prefix: ""
        log_mode: error
      report:
        driver: mysql
        username: root
        password: ""
        host: 127.0.0.1
        port: "3306"
        db: upper_report
        charset: utf8mb4
        prefix: rpt_
        log_mode: error
      postgres:
        driver: postgresql
        username: postgres
        password: ""
        host: 127.0.0.1
        port: "5432"
        db: upper
        sslmode: disable
        timezone: Asia/Shanghai
        prefix: ""
        log_mode: error
      sqlserver:
        driver: sqlserver
        username: sa
        password: ""
        host: 127.0.0.1
        port: "1433"
        db: upper
        prefix: ""
        log_mode: error
      sqlite:
        driver: sqlite
        path: /database/default.db
```

使用：

```go
db := facades.DB.Default()

mysqlDefault, err := facades.DB.Drivers("mysql")
mysqlReport, err := facades.DB.Drivers("mysql", "report")
postgresDefault, err := facades.DB.Drivers("postgresql")
sqlserverDefault, err := facades.DB.Drivers("sqlserver")
```

注意：接口方法名是 `Drivers(driver string, names ...string)`，不是 `Channel()`。

## Redis

Redis 配置位于 `database.redis` 下，`database.redis.default` 只保存默认连接名，实际配置位于 `database.redis.connections.<name>`：

```yaml
database:
  redis:
    default: default
    connections:
      default:
        driver: redis
        host: 127.0.0.1
        port: "6379"
        username: ""
        password: ""
        db: 0
      cache:
        driver: redis
        host: 127.0.0.1
        port: "6379"
        db: 1
```

使用：

```go
ctx := context.Background()

redis := facades.Redis.Default()
redis.Set(ctx, "key", "value", 0)

cacheRedis, err := facades.Redis.Channel("cache")
```

- 注意：当前实现读取的是 `database.redis.connections.<name>.db`，不是 `database.redis.connections.<name>.database`。如果配置写成 `database`，会落到默认 DB `1`。
- `database.orm.default` 只保存默认 ORM 连接名，实际连接配置位于 `database.orm.connections.<name>`。
- `database.orm.migration.table` 和 `database.orm.migration.dir` 保存迁移命名空间配置。
- `database.orm.connections.<name>.prefix` 里的 `prefix` 按连接名读取，例如 `database.orm.connections.default.prefix`，`auth` 和 migration 都会复用这个值。
- `database.redis.default` 只保存默认 Redis 连接名，实际连接配置位于 `database.redis.connections.<name>`。

## Provider

典型注册顺序：

```go
facades.Cfg.Add("kernel", map[string]any{
	"providers": []service.Provider{
		&orm.ServiceProvider{},
		&redis.ServiceProvider{},
	},
})
```

`orm.ServiceProvider` 会设置 `facades.DB`，`redis.ServiceProvider` 会设置 `facades.Redis`。

## 结合 example 基础项目的注意点

- example 基础项目中的业务逻辑通常会直接使用 `facades.DB.Default().WithContext(ctx)`，因此数据库 provider 必须早于 auth、console server 等依赖数据库的 provider。
- 登录限流、JWT 黑名单等能力依赖 Redis；如果启用相关功能，Redis provider 也必须启动成功。
- 当前框架没有统一连接池配置读取，README 不应声明 `pool.max_idle_conns` 等字段已经生效。
- 初始化时会真实连接数据库/Redis；缺失配置或服务不可达会导致启动失败。
