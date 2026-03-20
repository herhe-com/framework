# 依赖注入迁移方案 (uber-go/fx)

> 本文档记录从 Facades 模式迁移到依赖注入的技术方案，供后续评估使用。
> 
> 创建时间：2026-03-21

## 背景

当前项目使用 Facades 模式（类似 Laravel），全局单例访问各种服务（132+ 处调用）。考虑迁移到依赖注入以提升可测试性和代码解耦。

## 技术选型

### 推荐：uber-go/fx

**选择理由：**
- 基于 uber/dig 构建，提供完整的应用框架
- 内置生命周期管理（启动/关闭钩子）
- 自动协调多服务启动顺序
- 内置优雅关闭和信号处理
- 适合长期运行的服务（HTTP/RPC/Cron）

**安装：**
```bash
go get go.uber.org/fx
```

### 备选方案对比

| 特性 | uber/dig | uber/fx | google/wire |
|------|----------|---------|-------------|
| 依赖注入 | ✅ | ✅ | ✅ |
| 生命周期管理 | ❌ 手动 | ✅ 自动 | ❌ |
| 启动顺序控制 | ❌ 手动 | ✅ 自动 | ❌ |
| 优雅关闭 | ❌ 手动 | ✅ 自动 | ❌ |
| 性能 | 运行时反射 | 运行时反射 | 编译时生成 |
| 学习曲线 | 简单 | 中等 | 陡峭 |
| 适用场景 | CLI 工具 | 长期服务 | 性能敏感 |

## 核心改造方案

### 1. 定义依赖接口

```go
// contracts/config/config.go
type Config interface {
    GetString(key string, def ...string) string
    GetInt(key string, def ...int) int
    GetBool(key string, def ...bool) bool
    Get(key string) any
}

// contracts/database/database.go
type Database interface {
    Default() *gorm.DB
    Connection(name string) *gorm.DB
}

// contracts/cache/cache.go
type Cache interface {
    Get(key string) (string, error)
    Set(key string, value any, ttl time.Duration) error
    Delete(key string) error
}
```

### 2. 改造服务结构

#### 当前代码（Facades）
```go
// auth/application.go
func NewAuth() (*Auth, error) {
    if facades.DB.Default() == nil {
        return nil, errors.New("please initialize database first")
    }
    
    defaultDriver := facades.Cfg.GetString("database.driver", "mysql")
    prefix := facades.Cfg.GetString("database." + defaultDriver + ".prefix")
    table := facades.Cfg.GetString("auth.casbin.table")
    
    a, err := adapter.NewAdapterByDBUseTableName(facades.DB.Default(), prefix, table)
    // ...
}
```

#### 改造后（DI）
```go
// auth/application.go
type Auth struct {
    db       *gorm.DB
    config   config.Config
    enforcer *casbin.Enforcer
}

// NewAuth 构造函数，fx 会自动注入依赖
func NewAuth(db *gorm.DB, cfg config.Config) (*Auth, error) {
    if db == nil {
        return nil, errors.New("database is required")
    }
    
    defaultDriver := cfg.GetString("database.driver", "mysql")
    prefix := cfg.GetString("database." + defaultDriver + ".prefix")
    table := cfg.GetString("auth.casbin.table")
    
    a, err := adapter.NewAdapterByDBUseTableName(db, prefix, table)
    if err != nil {
        return nil, err
    }
    
    enforcer, err := casbin.NewEnforcer(model, a)
    if err != nil {
        return nil, err
    }
    
    return &Auth{
        db:       db,
        config:   cfg,
        enforcer: enforcer,
    }, nil
}
```

### 3. 注册服务提供者

```go
// providers.go
package framework

import (
    "go.uber.org/fx"
    
    "github.com/herhe-com/framework/auth"
    "github.com/herhe-com/framework/cache"
    "github.com/herhe-com/framework/config"
    "github.com/herhe-com/framework/database/orm"
)

// Module 框架核心模块
var Module = fx.Options(
    // 基础设施
    fx.Provide(config.NewConfig),      // 提供 config.Config
    fx.Provide(orm.NewDatabase),       // 提供 *gorm.DB
    fx.Provide(cache.NewCache),        // 提供 cache.Cache
    
    // 业务服务
    fx.Provide(auth.NewAuth),          // 提供 *auth.Auth
    fx.Provide(filesystem.NewStorage), // 提供 *filesystem.Storage
)

// DatabaseModule 数据库模块（带生命周期）
var DatabaseModule = fx.Options(
    fx.Provide(orm.NewDatabase),
    fx.Invoke(func(lc fx.Lifecycle, db *gorm.DB) {
        lc.Append(fx.Hook{
            OnStart: func(ctx context.Context) error {
                sqlDB, err := db.DB()
                if err != nil {
                    return err
                }
                return sqlDB.Ping()
            },
            OnStop: func(ctx context.Context) error {
                sqlDB, _ := db.DB()
                return sqlDB.Close()
            },
        })
    }),
)
```

### 4. HTTP Handler 改造

#### 当前代码
```go
// http/middleware/login_limiter.go
func LoginLimiter() app.HandlerFunc {
    return func(c context.Context, ctx *app.RequestContext) {
        maxAttempts := facades.Cfg.GetInt64("auth.login.max_attempts", 5)
        lockMinutes := facades.Cfg.GetInt("auth.login.lock_duration", 15)
        
        // ... 业务逻辑
        
        facades.DB.Default().Create(&loginLog)
    }
}
```

#### 改造后
```go
// http/middleware/login_limiter.go
type LoginLimiter struct {
    db     *gorm.DB
    config config.Config
}

func NewLoginLimiter(db *gorm.DB, cfg config.Config) *LoginLimiter {
    return &LoginLimiter{
        db:     db,
        config: cfg,
    }
}

func (l *LoginLimiter) Middleware() app.HandlerFunc {
    return func(c context.Context, ctx *app.RequestContext) {
        maxAttempts := l.config.GetInt64("auth.login.max_attempts", 5)
        lockMinutes := l.config.GetInt("auth.login.lock_duration", 15)
        
        // ... 业务逻辑
        
        l.db.Create(&loginLog)
    }
}

// 注册
var HTTPModule = fx.Options(
    fx.Provide(NewLoginLimiter),
    fx.Invoke(func(h *server.Hertz, limiter *LoginLimiter) {
        h.Use(limiter.Middleware())
    }),
)
```

### 5. 主程序入口

```go
// main.go
package main

import (
    "go.uber.org/fx"
    
    "github.com/herhe-com/framework"
    "github.com/herhe-com/framework/http"
)

func main() {
    fx.New(
        // 核心模块
        framework.Module,
        framework.DatabaseModule,
        
        // HTTP 模块
        http.Module,
        
        // 启动 HTTP 服务器
        fx.Invoke(func(lc fx.Lifecycle, h *server.Hertz) {
            lc.Append(fx.Hook{
                OnStart: func(ctx context.Context) error {
                    go h.Spin()
                    return nil
                },
                OnStop: func(ctx context.Context) error {
                    return h.Shutdown(ctx)
                },
            })
        }),
    ).Run()  // 阻塞运行，Ctrl+C 触发优雅关闭
}
```

## 测试改进

### 当前测试（Facades）
```go
func TestCreateUser(t *testing.T) {
    // 必须初始化全局 facades
    facades.DB = setupTestDB()
    facades.Cfg = setupTestConfig()
    
    err := CreateUser("test")
    assert.NoError(t, err)
    
    // 全局状态污染，影响其他测试
}
```

### 改造后（DI）
```go
func TestCreateUser(t *testing.T) {
    // 使用 mock，隔离测试
    mockDB := &MockDB{}
    mockConfig := &MockConfig{}
    
    svc := NewUserService(mockDB, mockConfig)
    
    err := svc.CreateUser("test")
    assert.NoError(t, err)
    
    // 验证 mock 调用
    mockDB.AssertCalled(t, "Create", mock.Anything)
}

// 使用 fx 的测试工具
func TestUserServiceIntegration(t *testing.T) {
    var svc *UserService
    
    app := fxtest.New(t,
        fx.Provide(NewTestDB),
        fx.Provide(NewTestConfig),
        fx.Provide(NewUserService),
        fx.Populate(&svc),
    )
    defer app.RequireStart().RequireStop()
    
    err := svc.CreateUser("test")
    assert.NoError(t, err)
}
```

## 迁移策略

### 方案一：完全重构（3-4 周）
- 一次性将所有 132+ 处 facades 调用改为 DI
- 风险高，影响范围大
- 适合有充足测试覆盖的项目

### 方案二：渐进式迁移（推荐，1-2 周）
1. **第一阶段**：保留 facades，添加 DI 支持
   ```go
   // 提供桥接函数
   func NewDatabaseFromFacades() *gorm.DB {
       return facades.DB.Default()
   }
   
   // 新代码使用 DI
   type UserService struct {
       db *gorm.DB
   }
   
   func NewUserService(db *gorm.DB) *UserService {
       return &UserService{db: db}
   }
   ```

2. **第二阶段**：核心模块迁移
   - auth 模块
   - database 模块
   - cache 模块
   - config 模块

3. **第三阶段**：业务模块迁移
   - HTTP handlers
   - RPC services
   - Cron jobs
   - Queue consumers

4. **第四阶段**：移除 facades
   - 确认所有调用已迁移
   - 删除 facades 包
   - 更新文档

### 方案三：混合模式（最务实，1-2 天）
- 基础设施层保留 facades（DB, Config, Cache）
- 业务逻辑层使用 DI
- 新代码强制使用 DI
- 老代码逐步重构

```go
// 混合使用示例
type UserService struct {
    db     *gorm.DB  // DI 注入
    config config.Config  // DI 注入
}

func NewUserService(db *gorm.DB, cfg config.Config) *UserService {
    return &UserService{
        db:     db,
        config: cfg,
    }
}

// 提供桥接
func NewUserServiceFromFacades() *UserService {
    return NewUserService(facades.DB.Default(), facades.Cfg)
}
```

## 工作量评估

基于当前 132+ 处 facades 调用：

| 迁移方案 | 工作量 | 风险 | 收益 |
|---------|--------|------|------|
| 完全重构 | 3-4 周 | 高 | 高 |
| 渐进式迁移 | 1-2 周 | 中 | 高 |
| 混合模式 | 1-2 天 | 低 | 中 |

## 性能影响

- fx 使用运行时反射，启动时有轻微性能开销（毫秒级）
- 运行时性能无影响（依赖已解析）
- 如需极致性能，可考虑 google/wire（编译时生成）

## 参考资源

- [uber-go/fx 官方文档](https://uber-go.github.io/fx/)
- [uber-go/fx GitHub](https://github.com/uber-go/fx)
- [uber-go/dig GitHub](https://github.com/uber-go/dig)
- [google/wire GitHub](https://github.com/google/wire)

## 决策建议

**推荐采用方案三（混合模式）：**
1. 投入产出比最高
2. 风险可控，可随时回退
3. 新代码享受 DI 好处
4. 老代码保持稳定
5. 为未来完全迁移留下空间

**如果决定完全迁移，推荐采用方案二（渐进式）：**
1. 分阶段实施，降低风险
2. 每个阶段可独立验证
3. 团队有时间适应新模式
4. 可根据实际情况调整节奏

## 后续行动

- [ ] 团队讨论，确定迁移方案
- [ ] 评估测试覆盖率，补充关键测试
- [ ] 选择试点模块进行 POC
- [ ] 制定详细迁移计划和时间表
- [ ] 更新开发文档和最佳实践
