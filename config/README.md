# Config 组件

`config` 基于 Viper 封装配置读取。框架启动时会先初始化 `facades.Cfg`，应用随后可以在各自的 `config/*.go` 的 `init()` 中继续写入模块配置。

## 示例目录

更完整的模块化配置样例见 [examples/config](</Users/orange/Developer/Project/go/src/github.com/herhe-com/framework/examples/config/README.md>)。

## 加载顺序

1. 如果存在 `facades.Root + "/conf/env.yaml"`，读取该 YAML。
2. 如果本地配置不存在，则尝试通过环境变量读取远程配置。
3. 应用代码可继续调用 `facades.Cfg.Add()` 或 `facades.Cfg.Set()` 写入派生配置。

远程配置相关环境变量：

- `HH_CFG_PROVIDER`
- `HH_CFG_ENDPOINT`
- `HH_CFG_PATH`
- `HH_CFG_WATCH`
- `HH_CFG_SECRET`

## 常用读取方法

```go
name := facades.Cfg.GetString("app.name", "UPER")
port := facades.Cfg.GetString("server.port", "9600")
debug := facades.Cfg.GetBool("app.debug", false)
exists := facades.Cfg.IsSet("database.orm.connections.default.host")
```

接口定义以 `contracts/config.Application` 为准：

```go
type Application interface {
	Env(key string, defaultValue ...any) any
	Add(name string, configuration map[string]any)
	Set(key string, configuration any)
	Get(key string, defaultValue ...any) any
	GetString(key string, defaultValue ...string) string
	GetStrings(key string, defaultValue ...[]string) []string
	GetMaps(key string, defaultValue ...map[string]any) map[string]any
	GetInt(key string, defaultValue ...int) int
	GetInt64(key string, defaultValue ...int64) int64
	GetBool(key string, defaultValue ...bool) bool
	IsSet(key string) bool
}
```

## 应用配置模式

`example` 基础项目推荐使用这种模式：本地或远程配置提供原始值，业务 config 包再按框架需要组织配置结构。

```go
func init() {
	cfg := facades.Cfg
	cfg.Add("database", map[string]any{
		"orm": map[string]any{
			"default": "default",
			"connections": map[string]any{
				"default": map[string]any{
					"driver":   "mysql",
					"username": cfg.Env("database.orm.connections.default.username", "root"),
					"password": cfg.Env("database.orm.connections.default.password", ""),
					"host":     cfg.Env("database.orm.connections.default.host", "127.0.0.1"),
					"port":     cfg.Env("database.orm.connections.default.port", "3306"),
					"db":       cfg.Env("database.orm.connections.default.db", "upper"),
					"charset":  cfg.Env("database.orm.connections.default.charset", "utf8mb4_unicode_ci"),
				},
			},
		},
		"redis": map[string]any{
			"default": "default",
			"connections": map[string]any{
				"default": map[string]any{
					"driver":   "redis",
					"host":     cfg.Env("database.redis.connections.default.host", "127.0.0.1"),
					"port":     cfg.Env("database.redis.connections.default.port", "6379"),
					"username": cfg.Env("database.redis.connections.default.username", ""),
					"password": cfg.Env("database.redis.connections.default.password", ""),
					"db":       cfg.Env("database.redis.connections.default.db", 1),
				},
			},
		},
	})
}
```

## 配置结构约定

以下 key 与当前框架实现强相关：

```yaml
app:
  location: Asia/Shanghai
  debug: false

server:
  address: 0.0.0.0
  port: "9600"

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
        charset: utf8mb4_unicode_ci
        prefix: ""
        log_mode: error
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
filesystem:
  default: default
  disks:
    default:
      driver: s3
      access: ""
      secret: ""
      region: us-east-1
      bucket: ""
      domain: ""
      endpoint: ""

queue:
  default: default
  connections:
    default:
      driver: rabbitmq
      host: 127.0.0.1
      port: 5672
      username: guest
      password: guest
      vhost: /

search:
  default: default
  connections:
    default:
      driver: elasticsearch
      host: http://127.0.0.1:9200
      hosts:
        - http://127.0.0.1:9200
      username: elastic
      password: changeme
```

## 注意事项

- `Env()` 不是直接读取操作系统环境变量；它先调用 `Get()`，用于从已加载配置中读取值并处理空值默认值。
- `Add("database", map[string]any{...})` 会覆盖同名顶层配置，应用应集中组织每个顶层配置。
- `kernel.providers` 和 `kernel.consoles` 当前必须由 Go 代码写入接口实例，不能只写 YAML 字符串。
- Redis DB 编号的 key 是 `database.redis.connections.<name>.db`，不是 `database.redis.connections.<name>.database`。
- `database.orm.default` 保存默认 ORM 连接名，实际 ORM 配置放在 `database.orm.connections.<name>`。
- `database.orm.migration.table` 和 `database.orm.migration.dir` 保存迁移命名空间配置。
- `database.orm.connections.<name>.driver`、`database.redis.connections.<name>.db`、`filesystem.disks.<disk>.driver`、`queue.connections.<name>.driver`、`search.connections.<name>.driver` 是当前约定的实例级配置字段。
- `database.redis.default`、`filesystem.default`、`queue.default`、`search.default` 只保存默认实例名。
- ORM 的 `prefix` 也按连接名读取，例如 `database.orm.connections.default.prefix`，`auth` 和 migration 都会使用这个值。
