# Facades 组件

全局单例访问器，提供 Laravel 风格的 Facade 模式，简化组件访问。

## 功能特性

- 全局单例访问
- 简化的 API 调用
- 统一的访问接口
- 延迟初始化支持

## 可用的 Facades

### 配置管理

```go
import "github.com/herhe-com/framework/facades"

// 访问配置
facades.Cfg.GetString("app.name")
facades.Cfg.GetInt("app.port", 8080)
facades.Cfg.GetBool("app.debug")
```

### 数据库

```go
// GORM 数据库
db := facades.DB.Default()
mysqlDB := facades.DB.Channel("mysql")

// Redis
redis := facades.Redis.Default()
cacheRedis := facades.Redis.Channel("cache")

// MongoDB
mongo := facades.Mongo.Default()
analyticsDB := facades.Mongo.Channel("analytics")

// Casbin 权限
allowed, _ := facades.Casbin.Enforce("user", "resource", "action")
```

### 文件存储

```go
// 默认存储
facades.Storage.Put("file.txt", reader, size)
facades.Storage.Get("file.txt")

// 指定磁盘
s3 := facades.Storage.Disk("s3")
s3.Put("file.txt", reader, size)
```

### 消息队列

```go
// 发送消息
facades.Queue.Producer("exchange", "routing", data)

// 消费消息
facades.Queue.Consumer("queue", handler)

// 指定通道
rabbitmq := facades.Queue.Channel("rabbitmq")
```

### 搜索引擎

```go
// 索引文档
facades.Search.Save("users", "1", userData)

// 搜索
results, _ := facades.Search.Search("users", query, 1, 10)

// 指定驱动
es := facades.Search.Driver("elasticsearch")
```

### AI 服务

```go
// 对话
response, _ := facades.AI.Chat(ai.ChatRequest{
    Model: "gpt-4",
    Messages: []ai.Message{
        {Role: "user", Content: "Hello"},
    },
})

// 指定驱动
ollama := facades.AI.Driver("ollama")
```

### 验证器

```go
// 验证结构体
err := facades.Validator.Struct(request)

// 验证字段
err := facades.Validator.Var(email, "required,email")
```

### 命令行

```go
// 注册命令
facades.Console.Register(myCommand)

// 执行命令
facades.Console.Execute()
```

### 微服务工具

```go
// Snowflake ID 生成
id := facades.Snowflake.Generate()

// 分布式锁
lock := facades.Locker.NewMutex("resource-key")
lock.Lock()
defer lock.Unlock()
```

### 路径工具

```go
// 获取项目根路径
rootPath := facades.Root
```

## Facade 列表

| Facade | 类型 | 说明 |
|--------|------|------|
| `Cfg` | `config.Application` | 配置管理 |
| `DB` | `database.DB` | 数据库（GORM） |
| `Redis` | `database.Redis` | Redis 客户端 |
| `Mongo` | `mongodb.Mongo` | MongoDB 客户端 |
| `Casbin` | `*casbin.Enforcer` | 权限控制 |
| `Storage` | `filesystem.Storage` | 文件存储 |
| `Queue` | `queue.Queue` | 消息队列 |
| `Search` | `search.Search` | 搜索引擎 |
| `AI` | `ai.AI` | AI 服务 |
| `Validator` | `*validator.Validate` | 验证器 |
| `Console` | `console.Console` | 命令行 |
| `Snowflake` | `*snowflake.Node` | ID 生成器 |
| `Locker` | `*redsync.Redsync` | 分布式锁 |
| `Root` | `string` | 根路径 |

## 使用示例

### 完整的 CRUD 示例

```go
package main

import (
    "github.com/herhe-com/framework/facades"
)

type User struct {
    ID       uint   `gorm:"primarykey"`
    Username string `json:"username"`
    Email    string `json:"email"`
}

func main() {
    // 创建用户
    user := User{
        Username: "john",
        Email:    "john@example.com",
    }
    facades.DB.Default().Create(&user)
    
    // 缓存用户数据
    facades.Redis.Default().Set(
        context.Background(),
        fmt.Sprintf("user:%d", user.ID),
        user,
        1*time.Hour,
    )
    
    // 上传用户头像
    file, _ := os.Open("avatar.jpg")
    defer file.Close()
    
    fileInfo, _ := file.Stat()
    facades.Storage.Put(
        fmt.Sprintf("avatars/%d.jpg", user.ID),
        file,
        fileInfo.Size(),
    )
    
    // 索引用户到搜索引擎
    facades.Search.Save("users", fmt.Sprintf("%d", user.ID), user)
}
```

### 多驱动切换

```go
// 文件存储 - 不同的存储后端
s3Storage := facades.Storage.Disk("s3")
ossStorage := facades.Storage.Disk("oss")
minioStorage := facades.Storage.Disk("minio")

// 数据库 - 不同的数据库连接
mysqlDB := facades.DB.Channel("mysql")
postgresDB := facades.DB.Channel("postgres")

// Redis - 不同的 Redis 实例
defaultRedis := facades.Redis.Default()
cacheRedis := facades.Redis.Channel("cache")
sessionRedis := facades.Redis.Channel("session")

// 搜索引擎 - 不同的搜索后端
elasticsearch := facades.Search.Driver("elasticsearch")
meilisearch := facades.Search.Driver("meilisearch")

// AI 服务 - 不同的 AI 提供商
openai := facades.AI.Driver("openai")
ollama := facades.AI.Driver("ollama")
```

### 配置驱动的应用

```go
// 根据配置选择存储
storageDriver := facades.Cfg.GetString("filesystem.default", "s3")
storage := facades.Storage.Disk(storageDriver)

// 根据配置选择数据库
dbDriver := facades.Cfg.GetString("database.default", "mysql")
db := facades.DB.Channel(dbDriver)

// 根据配置选择搜索引擎
searchDriver := facades.Cfg.GetString("search.default", "elasticsearch")
search := facades.Search.Driver(searchDriver)
```

## 初始化

Facades 在应用启动时通过服务提供者自动初始化：

```go
// foundation/application.go
func Boot() {
    // 注册基础提供者
    app.Register(&config.Provider{})
    
    // 注册配置的提供者
    providers := facades.Cfg.Get("kernel.providers").([]service.Provider)
    for _, provider := range providers {
        app.Register(provider)
    }
    
    // 启动所有提供者
    app.Boot()
}
```

## 自定义 Facade

创建自定义 Facade：

```go
// 1. 定义接口
package contracts

type Cache interface {
    Get(key string) (any, error)
    Set(key string, value any, ttl time.Duration) error
}

// 2. 实现接口
package cache

type Redis struct {
    client *redis.Client
}

func (r *Redis) Get(key string) (any, error) {
    return r.client.Get(context.Background(), key).Result()
}

func (r *Redis) Set(key string, value any, ttl time.Duration) error {
    return r.client.Set(context.Background(), key, value, ttl).Err()
}

// 3. 创建 Facade
package facades

import "yourproject/contracts"

var Cache contracts.Cache

// 4. 在服务提供者中初始化
package cache

type Provider struct{}

func (p *Provider) Register() {
    // 初始化逻辑
}

func (p *Provider) Boot() {
    facades.Cache = &Redis{
        client: redis.NewClient(&redis.Options{
            Addr: "localhost:6379",
        }),
    }
}
```

## 注意事项

### 初始化顺序

Facades 必须在使用前初始化。确保在 `main` 函数中调用 `foundation.Boot()`：

```go
func main() {
    foundation.Boot()
    
    // 现在可以使用 Facades
    facades.DB.Default()
}
```

### 空指针检查

在使用 Facade 前检查是否已初始化：

```go
if facades.DB.Default() == nil {
    log.Fatal("Database not initialized")
}
```

### 并发安全

所有 Facades 都是并发安全的，可以在多个 goroutine 中使用：

```go
go func() {
    facades.Redis.Default().Set(ctx, "key1", "value1", 0)
}()

go func() {
    facades.Redis.Default().Set(ctx, "key2", "value2", 0)
}()
```

### 测试中的 Facades

在测试中可以替换 Facades：

```go
func TestMyFunction(t *testing.T) {
    // 保存原始 Facade
    originalDB := facades.DB
    
    // 使用 mock
    facades.DB = &MockDB{}
    
    // 测试逻辑
    
    // 恢复原始 Facade
    facades.DB = originalDB
}
```

## 优势

1. 简化的 API 调用
2. 全局访问，无需传递依赖
3. 易于测试和 mock
4. 统一的访问模式
5. 延迟初始化支持

## 文件结构

```
facades/
├── app.go           # Console facade
├── config.go        # Config facade
├── database.go      # DB, Redis, Casbin facades
├── mongodb.go       # MongoDB facade
├── filesystem.go    # Storage facade
├── queue.go         # Queue facade
├── search.go        # Search facade
├── ai.go           # AI facade
├── auth.go         # Validator facade
├── microservice.go  # Snowflake, Locker facades
└── path.go         # Root path
```

## 最佳实践

1. 始终在应用启动时初始化 Facades
2. 在使用前检查 Facade 是否为 nil
3. 使用配置驱动的驱动选择
4. 在测试中使用 mock 替换 Facades
5. 避免在包初始化时使用 Facades
6. 使用具体的驱动/通道而不是总是使用默认值
