# Auth 组件

认证授权组件，提供 JWT 令牌管理、基于 Casbin 的 RBAC 权限控制、令牌黑名单和临时令牌功能。

## 功能特性

- JWT 令牌生成、验证和刷新
- 令牌黑名单管理（基于 Redis）
- 临时令牌支持
- Casbin RBAC 权限控制
- 权限树管理和过滤
- 多平台权限支持（Platform、Clique、Store、Region）
- 原子性刷新令牌操作（使用 Lua 脚本）

## 完整配置示例

```yaml
app:
  name: your-app-name    # 应用名称，用于生成黑名单 key 和 issuer

jwt:
  secret: your-secret-key-here    # JWT 签名密钥（必填，建议使用强随机字符串）
  sub: default                    # JWT 主题标识，用于生成 issuer
  lifetime: 2                     # 令牌生命周期（小时），默认 2 小时
  leeway: 5                       # 令牌刷新容错时间（秒），默认 5 秒

auth:
  casbin:
    table: casbin_rule    # Casbin 规则表名

database:
  driver: mysql           # 数据库驱动
  mysql:
    prefix: ""            # 表前缀
```

## JWT 令牌管理

### 生成令牌

```go
import "github.com/herhe-com/framework/auth"

// 创建 JWT 令牌
token := auth.NewJWToken()
accessToken, refreshToken, err := token.Create(auth.Claims{
    ID:       "user-123",
    Username: "john",
    Platform: "admin",
    Clique:   "clique-1",
    Store:    "store-1",
})
```

### 验证令牌

```go
// 验证访问令牌
claims, err := token.Check(accessToken)
if err != nil {
    // 令牌无效或已过期
}

// 使用刷新令牌获取新的访问令牌
newAccessToken, err := token.Refresh(refreshToken)
```

### 令牌配置

```yaml
jwt:
  secret: your-secret-key    # JWT 签名密钥（必填）
  sub: default               # JWT 主题标识，用于生成 issuer
  lifetime: 2                # 令牌生命周期（小时），默认 2 小时
  leeway: 5                  # 令牌刷新容错时间（秒），默认 5 秒
```

### 令牌黑名单

```go
// 将令牌加入黑名单（登出）
err := token.Blacklist(accessToken)

// 检查令牌是否在黑名单中
isBlacklisted := token.IsBlacklisted(accessToken)
```

黑名单特性：
- 基于 Redis 存储
- 自动过期（最长 7 天）
- 支持访问令牌和刷新令牌

## 临时令牌

用于短期访问场景，如邮件验证、密码重置等。

```go
// 生成临时令牌
tempToken, err := auth.GenerateTemporaryToken("user-123", 3600) // 1 小时有效

// 验证临时令牌
userID, err := auth.ValidateTemporaryToken(tempToken)
```

## Casbin 权限控制

### 基础使用

```go
import "github.com/herhe-com/framework/facades"

// 检查权限
allowed, err := facades.Casbin.Enforce("user-123", "article", "read")
if allowed {
    // 用户有权限
}

// 添加权限策略
facades.Casbin.AddPolicy("user-123", "article", "write")

// 添加角色
facades.Casbin.AddRoleForUser("user-123", "admin")

// 删除权限
facades.Casbin.RemovePolicy("user-123", "article", "write")
```

### 配置 Casbin

在配置文件中指定 Casbin 相关配置：

```yaml
auth:
  casbin:
    table: casbin_rule    # Casbin 规则表名，默认 casbin_rule

database:
  driver: mysql           # 数据库驱动
  mysql:
    prefix: ""            # 表前缀
```

Casbin 模型文件路径固定为 `conf/casbin.conf`，需要在项目根目录的 `conf` 文件夹中创建该文件。

## 权限树管理

权限树用于组织和过滤层级化的权限结构。

```go
// 定义权限树
permissions := []auth.Permission{
    {
        ID:       "1",
        ParentID: "0",
        Name:     "用户管理",
        Children: []auth.Permission{
            {ID: "1-1", ParentID: "1", Name: "用户列表"},
            {ID: "1-2", ParentID: "1", Name: "添加用户"},
        },
    },
}

// 过滤权限树
tree := auth.NewTree(permissions)
filtered := tree.Filter([]string{"1-1", "1-2"})
```

## 请求上下文辅助

从 HTTP 请求上下文中提取认证信息。

```go
import (
    "github.com/cloudwego/hertz/pkg/app"
    "github.com/herhe-com/framework/auth"
)

func Handler(ctx context.Context, c *app.RequestContext) {
    // 获取当前用户 ID
    userID := auth.ID(c)
    
    // 获取用户名
    username := auth.Username(c)
    
    // 获取平台代码
    platform := auth.Platform(c)
    
    // 获取完整的 Claims
    claims := auth.Claims(c)
}
```

## 核心类型

### Claims

```go
type Claims struct {
    ID       string   // 用户 ID
    Username string   // 用户名
    Platform string   // 平台代码
    Clique   string   // 集团 ID
    Store    string   // 门店 ID
    Region   string   // 区域 ID
    Modules  []Module // 模块权限
}
```

### Permission

```go
type Permission interface {
    GetID() string
    GetParentID() string
    GetName() string
    GetChildren() []Permission
    SetChildren([]Permission)
}
```

## 中间件集成

在 Hertz 路由中使用 JWT 中间件：

```go
import (
    "github.com/cloudwego/hertz/pkg/app/server"
    "github.com/herhe-com/framework/auth"
)

func main() {
    h := server.Default()
    
    // JWT 认证中间件
    h.Use(auth.JWTMiddleware())
    
    // 需要认证的路由
    h.GET("/profile", func(ctx context.Context, c *app.RequestContext) {
        userID := auth.ID(c)
        // 处理请求
    })
}
```

## 依赖项

- GORM（数据库访问）
- Redis（令牌黑名单）
- Casbin（权限控制）
- Config facade（配置管理）

## 文件结构

```
auth/
├── application.go    # Casbin 初始化
├── jwt.go           # JWT 令牌管理
├── blacklist.go     # 令牌黑名单
├── temporary.go     # 临时令牌
├── permission.go    # 权限树管理
├── context.go       # 请求上下文辅助
└── provider.go      # 服务提供者
```

## 安全建议

1. 使用强密钥：JWT secret 应使用足够长的随机字符串
2. HTTPS 传输：始终通过 HTTPS 传输令牌
3. 令牌过期：合理设置令牌有效期，访问令牌建议 2 小时以内
4. 刷新令牌：使用刷新令牌机制，避免长期有效的访问令牌
5. 黑名单管理：用户登出时将令牌加入黑名单
6. 权限最小化：遵循最小权限原则分配用户权限
