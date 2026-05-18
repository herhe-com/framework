# Contracts 组件

`contracts` 是框架接口定义层。实现包应以这里的接口为准，README 示例也应和这些接口保持一致。

## 目录

```text
contracts/
├── ai/
├── auth/
├── captcha/
├── config/
├── console/
├── crontab/
├── database/
├── filesystem/
├── global/
├── http/
├── queue/
├── search/
├── service/
└── validation/
```

## Service Provider

```go
type Provider interface {
	Boot() error
	Register() error
}
```

## Config

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

## Database

```go
type DB interface {
	Default() *gorm.DB
	Drivers(driver string, names ...string) (*gorm.DB, error)
}
```

Redis：

```go
type Redis interface {
	Default() *redis.Client
	Channel(name string) (*redis.Client, error)
}
```

## Filesystem

```go
type Storage interface {
	Driver
	Disk(driver string, disk string) (Driver, error)
}
```

核心驱动接口包含目录、文件、URL 和上传方法：

```go
type Driver interface {
	Dirs(path string) ([]Pathname, error)
	Files(path string) ([]Pathname, error)
	List(path string) ([]Pathname, error)
	Copy(oldFile, newFile string) error
	Delete(file ...string) error
	DeleteDirectory(directory string) error
	Exists(file string) bool
	MakeDirectory(directory string) error
	Missing(file string) bool
	Move(oldFile, newFile string) error
	Path(file string) string
	Put(file string, content io.Reader, size int64) error
	PutFile(path string, source File) (string, error)
	PutFileAs(path string, source File, name string) (string, error)
	Size(file string) (int64, error)
	TemporaryUrl(file string, time time.Duration) (string, error)
	PresignedUploadUrl(file string, time time.Duration) (string, error)
	Url(file string) string
}
```

## Queue

```go
type Queue interface {
	Driver
	Channel(channel string, name string) (Driver, error)
}

type Driver interface {
	Producer(body []byte, exchange, queue string, routes []string, delay, ttl int64, headers ...rabbitmq.Table) error
	Consumer(handler func(data []byte) error, exchange, queue, route string, delay bool, ttl int64, retry int) error
	Close() error
}
```

## Console

```go
type Provider interface {
	Register() Console
}

type Console struct {
	Cmd      string
	Name     string
	Summary  string
	Consoles []Console
	Run      func(cmd *cobra.Command, args []string)
	Tags     func(cmd *cobra.Command)
}
```

## Auth Claims

```go
type Claims struct {
	jwt.RegisteredClaims
	Refresh bool           `json:"ref,omitempty"`
	Ext     map[string]any `json:"ext,omitempty"`
}
```

## 使用建议

- 新增驱动前先实现对应 contracts，并用编译期断言保证接口完整。
- example 基础项目应以本目录接口和实现包为准，不要依赖旧版 README 示例。
- 如果接口发生变化，需要同步更新实现包 README 和 `facades/README.md`。
