# Foundation 组件

应用程序引导和服务提供者管理组件，负责框架的初始化和生命周期管理。

## 功能特性

- 服务提供者注册和启动
- 应用程序生命周期管理
- 根路径管理
- 时区配置
- 服务提供者模式实现

## 核心概念

### 服务提供者

服务提供者是框架的核心机制，用于组织和管理服务的初始化逻辑。

#### 生命周期

```
init() → Register() → Boot()
```

1. **init()**: 包初始化阶段
2. **Register()**: 注册服务到容器
3. **Boot()**: 启动服务，执行初始化逻辑

### 服务提供者接口

```go
package service

type Provider interface {
    // Register 注册服务
    Register()
    
    // Boot 启动服务
    Boot()
}
```

## 使用方法

### 应用程序启动

```go
package main

import "github.com/herhe-com/framework/foundation"

func main() {
    // 引导应用程序
    foundation.Boot()
    
    // 应用程序逻辑
    // ...
}
```

### 创建服务提供者

```go
package myservice

import (
    "github.com/herhe-com/framework/contracts/service"
    "github.com/herhe-com/framework/facades"
)

type Provider struct{}

// Register 注册服务
func (p *Provider) Register() {
    // 注册服务到容器
    // 例如：初始化配置、创建实例等
}

// Boot 启动服务
func (p *Provider) Boot() {
    // 执行启动逻辑
    // 例如：连接数据库、初始化缓存等
    
    // 可以访问其他已注册的服务
    db := facades.DB.Default()
    // ...
}
```

### 注册服务提供者

在配置文件中注册服务提供者：

```yaml
kernel:
  providers:
    - github.com/herhe-com/framework/config.Provider
    - github.com/herhe-com/framework/database/orm.Provider
    - github.com/herhe-com/framework/database/redis.Provider
    - github.com/herhe-com/framework/filesystem.Provider
    - github.com/herhe-com/framework/validation.Provider
    - yourproject/services/myservice.Provider
```

## 内置服务提供者

### Config Provider

配置管理服务提供者，最先注册和启动。

```go
// foundation/application.go
app.Register(&config.Provider{})
```

### 可配置的提供者

从配置文件读取并注册的提供者：

```go
providers := facades.Cfg.Get("kernel.providers").([]service.Provider)
for _, provider := range providers {
    app.Register(provider)
}
```

## 根路径管理

### 设置根路径

```go
import "github.com/herhe-com/framework/foundation"

// 设置项目根路径
foundation.SetRoot("/path/to/project")
```

### 获取根路径

```go
import "github.com/herhe-com/framework/facades"

// 获取根路径
rootPath := facades.Root

// 构建完整路径
configPath := filepath.Join(facades.Root, "conf", "env.yaml")
```

## 时区配置

Foundation 组件会在启动时设置时区：

```go
// 设置为本地时区
time.Local = time.Local

// 或从配置读取
timezone := facades.Cfg.GetString("app.timezone", "UTC")
loc, _ := time.LoadLocation(timezone)
time.Local = loc
```

## 应用程序结构

### 典型的 main.go

```go
package main

import (
    "github.com/cloudwego/hertz/pkg/app/server"
    "github.com/herhe-com/framework/foundation"
    "github.com/herhe-com/framework/facades"
)

func main() {
    // 引导框架
    foundation.Boot()
    
    // 创建 HTTP 服务器
    h := server.Default(
        server.WithHostPorts(
            fmt.Sprintf(":%d", facades.Cfg.GetInt("app.port", 8080)),
        ),
    )
    
    // 注册路由
    RegisterRoutes(h)
    
    // 启动服务器
    h.Spin()
}
```

### 项目结构

```
project/
├── main.go              # 应用入口
├── conf/
│   └── env.yaml        # 配置文件
├── app/
│   ├── http/
│   │   ├── controllers/
│   │   └── middleware/
│   ├── models/
│   └── services/
│       └── provider.go  # 自定义服务提供者
└── routes/
    └── api.go          # 路由定义
```

## 服务提供者示例

### 数据库服务提供者

```go
package database

import (
    "github.com/herhe-com/framework/contracts/service"
    "github.com/herhe-com/framework/facades"
    "gorm.io/gorm"
)

type Provider struct{}

func (p *Provider) Register() {
    // 注册数据库配置
}

func (p *Provider) Boot() {
    // 初始化数据库连接
    db := initDatabase()
    
    // 设置到 facade
    facades.DB = db
    
    // 运行迁移
    if facades.Cfg.GetBool("database.auto_migrate") {
        runMigrations(db)
    }
}

func initDatabase() *gorm.DB {
    // 数据库初始化逻辑
    return nil
}

func runMigrations(db *gorm.DB) {
    // 迁移逻辑
}
```

### 缓存服务提供者

```go
package cache

import (
    "github.com/herhe-com/framework/contracts/service"
    "github.com/herhe-com/framework/facades"
)

type Provider struct{}

func (p *Provider) Register() {
    // 注册缓存配置
}

func (p *Provider) Boot() {
    // 初始化 Redis 连接
    redis := facades.Redis.Default()
    
    // 测试连接
    if err := redis.Ping(context.Background()).Err(); err != nil {
        log.Fatalf("Redis connection failed: %v", err)
    }
    
    log.Println("Cache service initialized")
}
```

### 队列服务提供者

```go
package queue

import (
    "github.com/herhe-com/framework/contracts/service"
    "github.com/herhe-com/framework/facades"
)

type Provider struct{}

func (p *Provider) Register() {
    // 注册队列配置
}

func (p *Provider) Boot() {
    // 初始化队列连接
    queue := facades.Queue.Default()
    
    // 启动消费者
    go startConsumers(queue)
    
    log.Println("Queue service initialized")
}

func startConsumers(queue queue.Queue) {
    // 启动消费者逻辑
}
```

## 启动顺序

1. 包初始化（`init()` 函数）
2. 设置根路径
3. 注册 Config Provider
4. 启动 Config Provider
5. 读取配置的服务提供者列表
6. 注册所有服务提供者
7. 按顺序启动所有服务提供者
8. 应用程序就绪

## 依赖注入

虽然 Go 没有内置的依赖注入容器，但可以通过服务提供者模式实现类似功能：

```go
// 在 Register 阶段创建实例
func (p *Provider) Register() {
    myService := &MyService{
        config: facades.Cfg,
    }
    
    // 存储到全局变量或 facade
    facades.MyService = myService
}

// 在 Boot 阶段注入依赖
func (p *Provider) Boot() {
    facades.MyService.SetDatabase(facades.DB.Default())
    facades.MyService.SetCache(facades.Redis.Default())
}
```

## 错误处理

### 启动失败处理

```go
func (p *Provider) Boot() {
    db, err := initDatabase()
    if err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }
    
    facades.DB = db
}
```

### 优雅关闭

```go
func main() {
    foundation.Boot()
    
    // 设置信号处理
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    
    // 启动服务器
    go h.Spin()
    
    // 等待退出信号
    <-quit
    
    // 清理资源
    cleanup()
}

func cleanup() {
    // 关闭数据库连接
    if db := facades.DB.Default(); db != nil {
        sqlDB, _ := db.DB()
        sqlDB.Close()
    }
    
    // 关闭 Redis 连接
    if redis := facades.Redis.Default(); redis != nil {
        redis.Close()
    }
    
    log.Println("Application shutdown complete")
}
```

## 最佳实践

1. 服务提供者应该只负责服务的注册和启动
2. 在 Register 阶段创建实例，在 Boot 阶段注入依赖
3. 按照依赖顺序注册服务提供者
4. 使用配置文件管理服务提供者列表
5. 在 Boot 阶段检查必需的依赖是否已初始化
6. 实现优雅关闭，清理资源

## 文件结构

```
foundation/
├── application.go    # 应用程序引导逻辑
└── path.go          # 根路径管理
```

## 依赖项

- Config facade（配置管理）
- Service contracts（服务提供者接口）
