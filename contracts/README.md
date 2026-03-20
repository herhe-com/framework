# Contracts 组件

框架的接口定义层，为所有组件提供统一的契约和类型定义。

## 功能特性

- 定义所有组件的接口契约
- 提供类型安全的接口定义
- 解耦接口定义与具体实现
- 支持多驱动实现

## 目录结构

```
contracts/
├── ai/              # AI 服务接口
├── auth/            # 认证授权接口
├── captcha/         # 验证码接口
├── config/          # 配置管理接口
├── console/         # 命令行接口
├── crontab/         # 定时任务接口
├── database/        # 数据库接口
├── filesystem/      # 文件存储接口
├── global/          # 全局通用类型
├── http/            # HTTP 请求响应接口
├── mongodb/         # MongoDB 接口
├── queue/           # 消息队列接口
├── search/          # 搜索引擎接口
├── service/         # 服务提供者接口
└── validation/      # 验证规则接口
```

## 核心接口

### Service Provider

所有服务提供者必须实现的接口：

```go
package service

type Provider interface {
    // Register 注册服务
    Register()
    
    // Boot 启动服务
    Boot()
}
```

### Storage Driver

文件存储驱动接口：

```go
package filesystem

type Driver interface {
    Put(key string, file io.Reader, size int64) error
    Get(key string) ([]byte, error)
    Delete(key string) error
    Copy(src, dst string) error
    Move(src, dst string) error
    Exists(key string) bool
    Size(key string) (int64, error)
    List(prefix string) ([]string, error)
    TemporaryUrl(key string, ttl time.Duration) (string, error)
    PresignedUploadUrl(key string, ttl time.Duration) (string, error)
}
```

### Database

数据库访问接口：

```go
package database

type DB interface {
    // Default 获取默认数据库连接
    Default() *gorm.DB
    
    // Channel 获取指定通道的数据库连接
    Channel(channel string) *gorm.DB
}
```

### Queue Driver

消息队列驱动接口：

```go
package queue

type Driver interface {
    // Producer 发送消息
    Producer(exchange, routing string, body []byte, options ...ProducerOption) error
    
    // Consumer 消费消息
    Consumer(queue string, handler ConsumerHandler, options ...ConsumerOption) error
}
```

### Search Driver

搜索引擎驱动接口：

```go
package search

type Driver interface {
    // Index 创建索引
    Index(index string) error
    
    // Save 保存文档
    Save(index, id string, document any) error
    
    // Delete 删除文档
    Delete(index, id string) error
    
    // Search 搜索文档
    Search(index string, query any, page, size int) (*SearchResult, error)
    
    // Document 获取文档
    Document(index, id string) (any, error)
    
    // Ping 检查连接
    Ping() error
}
```

### AI Driver

AI 服务驱动接口：

```go
package ai

type Driver interface {
    // Chat 对话聊天
    Chat(request ChatRequest) (*ChatResponse, error)
    
    // ChatStream 流式对话
    ChatStream(request ChatRequest) (<-chan StreamResponse, error)
    
    // Embedding 文本嵌入
    Embedding(request EmbeddingRequest) ([][]float64, error)
    
    // Models 获取可用模型列表
    Models() ([]Model, error)
}
```

### Config

配置管理接口：

```go
package config

type Application interface {
    Get(key string) any
    GetString(key string, defaultValue ...string) string
    GetInt(key string, defaultValue ...int) int
    GetBool(key string, defaultValue ...bool) bool
    GetMaps(keys ...string) map[string]any
    Set(key string, value any)
    Add(key string, value any)
}
```

### Console

命令行接口：

```go
package console

type Console interface {
    // Command 返回 Cobra 命令
    Command() *cobra.Command
    
    // Subcommands 返回子命令列表
    Subcommands() []Console
}
```

### Validation Rule

验证规则接口：

```go
package validation

type Rule interface {
    // Key 返回规则键名
    Key() string
    
    // Message 返回错误消息
    Message() string
    
    // Validate 执行验证
    Validate(fl validator.FieldLevel) bool
}
```

## 使用方式

### 实现接口

```go
package mydriver

import "github.com/herhe-com/framework/contracts/filesystem"

type MyStorage struct {
    // 字段
}

// 实现 Driver 接口
func (s *MyStorage) Put(key string, file io.Reader, size int64) error {
    // 实现逻辑
    return nil
}

func (s *MyStorage) Get(key string) ([]byte, error) {
    // 实现逻辑
    return nil, nil
}

// ... 实现其他方法
```

### 类型断言

```go
import "github.com/herhe-com/framework/contracts/filesystem"

var driver filesystem.Driver = &MyStorage{}

// 使用接口方法
err := driver.Put("file.txt", reader, size)
```

### 接口组合

```go
// 组合多个接口
type AdvancedStorage interface {
    filesystem.Driver
    Compress(key string) error
    Decompress(key string) error
}
```

## 全局类型

### Response

标准 HTTP 响应结构：

```go
package global

type Response struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    any    `json:"data,omitempty"`
}
```

### Pagination

分页结构：

```go
package global

type Pagination struct {
    Page     int   `json:"page"`
    PageSize int   `json:"page_size"`
    Total    int64 `json:"total"`
}
```

## 设计原则

### 接口隔离

每个接口应该只包含必要的方法，避免臃肿的接口：

```go
// 好的设计
type Reader interface {
    Read(key string) ([]byte, error)
}

type Writer interface {
    Write(key string, data []byte) error
}

// 避免
type Storage interface {
    Read(key string) ([]byte, error)
    Write(key string, data []byte) error
    Delete(key string) error
    List() []string
    // ... 太多方法
}
```

### 依赖倒置

高层模块依赖接口，而不是具体实现：

```go
// 好的设计
func ProcessFile(storage filesystem.Driver, key string) error {
    data, err := storage.Get(key)
    // 处理数据
    return nil
}

// 避免
func ProcessFile(s3 *S3, key string) error {
    // 直接依赖具体实现
}
```

### 单一职责

每个接口应该只负责一个功能领域：

```go
// 好的设计
type Authenticator interface {
    Authenticate(credentials Credentials) (User, error)
}

type Authorizer interface {
    Authorize(user User, resource string) bool
}

// 避免
type AuthService interface {
    Authenticate(credentials Credentials) (User, error)
    Authorize(user User, resource string) bool
    SendEmail(to string, subject string) error  // 不相关的功能
}
```

## 扩展接口

### 添加新接口

1. 在 `contracts/` 下创建新的包目录
2. 定义接口和相关类型
3. 在具体实现包中实现接口
4. 在 `facades/` 中添加全局访问器（如需要）

示例：

```go
// contracts/cache/cache.go
package cache

type Cache interface {
    Get(key string) (any, error)
    Set(key string, value any, ttl time.Duration) error
    Delete(key string) error
}

// cache/redis.go
package cache

import "github.com/herhe-com/framework/contracts/cache"

type Redis struct {
    // 实现
}

func (r *Redis) Get(key string) (any, error) {
    // 实现
}

// facades/cache.go
package facades

import "github.com/herhe-com/framework/contracts/cache"

var Cache cache.Cache
```

## 最佳实践

1. 接口应该小而专注
2. 使用接口而不是具体类型作为参数
3. 返回具体类型，接受接口类型
4. 接口命名应该清晰表达其用途
5. 避免在接口中定义字段
6. 使用接口组合而不是继承

## 依赖关系

Contracts 包不依赖任何其他框架包，但被所有实现包依赖：

```
contracts/  (无依赖)
    ↑
    |
    |-- filesystem/  (实现 contracts/filesystem)
    |-- database/    (实现 contracts/database)
    |-- queue/       (实现 contracts/queue)
    └-- ...
```

## 版本兼容性

接口变更应该遵循以下原则：

1. 添加新方法是破坏性变更（需要所有实现更新）
2. 添加新接口是安全的
3. 修改方法签名是破坏性变更
4. 删除方法是破坏性变更

建议使用接口版本化：

```go
type StorageV1 interface {
    Get(key string) ([]byte, error)
}

type StorageV2 interface {
    StorageV1
    GetWithMetadata(key string) ([]byte, Metadata, error)
}
```
