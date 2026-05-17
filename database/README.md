# Database 组件

`database` 提供 GORM、Redis 和 MongoDB 的连接管理。关系型数据库和 Redis 会通过各自的 `ServiceProvider` 初始化到 `facades.DB` 和 `facades.Redis`。

## GORM

支持驱动：

- `mysql`
- `sqlite`
- `postgresql`

配置示例：

```yaml
database:
  driver: mysql
  mysql:
    default:
      username: root
      password: ""
      host: 127.0.0.1
      port: "3306"
      db: upper
      charset: utf8mb4_unicode_ci
      prefix: ""
      log_mode: error
  postgresql:
    default:
      username: postgres
      password: ""
      host: 127.0.0.1
      port: "5432"
      db: upper
      sslmode: disable
      timezone: Asia/Shanghai
      prefix: ""
      log_mode: error
  sqlite:
    default: /database/default.db
```

使用：

```go
db := facades.DB.Default()

mysqlDefault, err := facades.DB.Drivers("mysql")
mysqlReport, err := facades.DB.Drivers("mysql", "report")
postgresDefault, err := facades.DB.Drivers("postgresql")
```

注意：接口方法名是 `Drivers(driver string, names ...string)`，不是 `Channel()`。

## Redis

Redis 配置位于 `database.redis` 下：

```yaml
database:
  redis:
    default:
      host: 127.0.0.1
      port: "6379"
      username: ""
      password: ""
      db: 0
    cache:
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

注意：当前实现读取的是 `database.redis.<name>.db`，不是 `database.redis.<name>.database`。如果配置写成 `database`，会落到默认 DB `1`。

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
