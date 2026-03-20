# HTTP 组件

HTTP 响应辅助组件，为 Hertz 框架提供标准化的 JSON 响应格式。

## 功能特性

- 标准化的 JSON 响应格式
- 多种响应类型（成功、失败、错误等）
- 验证错误自动翻译
- 文件下载支持
- 泛型支持

## 响应格式

所有 JSON 响应遵循统一格式：

```json
{
  "code": 20000,
  "message": "success",
  "data": {}
}
```

## 响应码规范

| 响应码 | 说明 | 使用场景 |
|--------|------|----------|
| 20000 | 成功 | 请求成功处理 |
| 40000 | 请求错误 | 参数验证失败 |
| 40100 | 未认证 | 未登录或令牌无效 |
| 40300 | 无权限 | 没有访问权限 |
| 40400 | 未找到 | 资源不存在 |
| 50000 | 服务器错误 | 内部错误 |
| 60000 | 业务失败 | 业务逻辑失败 |

## 使用方法

### 成功响应

```go
import (
    "github.com/cloudwego/hertz/pkg/app"
    "github.com/herhe-com/framework/http"
)

func GetUser(ctx context.Context, c *app.RequestContext) {
    user := User{
        ID:       1,
        Username: "john",
        Email:    "john@example.com",
    }
    
    http.Success(c, user)
}

// 响应：
// {
//   "code": 20000,
//   "message": "操作成功",
//   "data": {
//     "id": 1,
//     "username": "john",
//     "email": "john@example.com"
//   }
// }
```

### 自定义成功消息

```go
func CreateUser(ctx context.Context, c *app.RequestContext) {
    // 创建用户逻辑
    
    http.Success(c, user, "用户创建成功")
}
```

### 失败响应

```go
func DeleteUser(ctx context.Context, c *app.RequestContext) {
    if err := deleteUser(id); err != nil {
        http.Fail(c, "删除失败：用户不存在")
        return
    }
    
    http.Success(c, nil)
}

// 响应：
// {
//   "code": 60000,
//   "message": "删除失败：用户不存在"
// }
```

### 验证错误

```go
import "github.com/go-playground/validator/v10"

type CreateUserRequest struct {
    Username string `json:"username" validate:"required,min=3,max=20"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=6"`
}

func CreateUser(ctx context.Context, c *app.RequestContext) {
    var req CreateUserRequest
    if err := c.BindJSON(&req); err != nil {
        http.BadRequest(c, err)
        return
    }
    
    // 验证请求
    if err := facades.Validator.Struct(req); err != nil {
        http.BadRequest(c, err)
        return
    }
    
    // 创建用户逻辑
    http.Success(c, user)
}

// 验证失败响应：
// {
//   "code": 40000,
//   "message": "Username必须至少包含3个字符; Email必须是有效的电子邮件地址"
// }
```

### 未认证响应

```go
func GetProfile(ctx context.Context, c *app.RequestContext) {
    token := c.GetHeader("Authorization")
    if token == "" {
        http.Unauthorized(c, "请先登录")
        return
    }
    
    // 验证令牌
    claims, err := auth.CheckJWToken(token)
    if err != nil {
        http.Unauthorized(c, "令牌无效或已过期")
        return
    }
    
    // 返回用户信息
    http.Success(c, profile)
}

// 响应：
// {
//   "code": 40100,
//   "message": "请先登录"
// }
```

### 无权限响应

```go
func DeleteUser(ctx context.Context, c *app.RequestContext) {
    userID := auth.ID(c)
    
    // 检查权限
    allowed, _ := facades.Casbin.Enforce(userID, "users", "delete")
    if !allowed {
        http.Forbidden(c, "您没有删除用户的权限")
        return
    }
    
    // 删除用户逻辑
    http.Success(c, nil)
}

// 响应：
// {
//   "code": 40300,
//   "message": "您没有删除用户的权限"
// }
```

### 未找到响应

```go
func GetUser(ctx context.Context, c *app.RequestContext) {
    id := c.Param("id")
    
    var user User
    if err := facades.DB.Default().First(&user, id).Error; err != nil {
        http.NotFound(c, "用户不存在")
        return
    }
    
    http.Success(c, user)
}

// 响应：
// {
//   "code": 40400,
//   "message": "用户不存在"
// }
```

### 服务器错误

```go
func ProcessData(ctx context.Context, c *app.RequestContext) {
    if err := processComplexLogic(); err != nil {
        http.ServerError(c, "处理失败，请稍后重试")
        return
    }
    
    http.Success(c, result)
}

// 响应：
// {
//   "code": 50000,
//   "message": "处理失败，请稍后重试"
// }
```

### 文件下载

```go
func DownloadFile(ctx context.Context, c *app.RequestContext) {
    filename := c.Param("filename")
    
    // 从存储获取文件
    data, err := facades.Storage.Get(filename)
    if err != nil {
        http.NotFound(c, "文件不存在")
        return
    }
    
    http.File(c, data, filename)
}
```

### 字符串响应

```go
func HealthCheck(ctx context.Context, c *app.RequestContext) {
    http.String(c, "OK")
}
```

## 泛型支持

使用泛型定义响应数据类型：

```go
type UserResponse struct {
    ID       uint   `json:"id"`
    Username string `json:"username"`
    Email    string `json:"email"`
}

func GetUser(ctx context.Context, c *app.RequestContext) {
    user := UserResponse{
        ID:       1,
        Username: "john",
        Email:    "john@example.com",
    }
    
    http.Success[UserResponse](c, user)
}
```

## 分页响应

```go
type PaginatedResponse struct {
    Items []User `json:"items"`
    Total int64  `json:"total"`
    Page  int    `json:"page"`
    Size  int    `json:"size"`
}

func ListUsers(ctx context.Context, c *app.RequestContext) {
    page := c.DefaultQuery("page", "1")
    size := c.DefaultQuery("size", "10")
    
    var users []User
    var total int64
    
    db := facades.DB.Default()
    db.Model(&User{}).Count(&total)
    db.Offset((page - 1) * size).Limit(size).Find(&users)
    
    response := PaginatedResponse{
        Items: users,
        Total: total,
        Page:  page,
        Size:  size,
    }
    
    http.Success(c, response)
}
```

## 验证错误翻译

HTTP 组件自动翻译验证错误消息：

```go
// 支持的语言
// - zh: 中文
// - en: 英文
// - ja: 日文
// - ar: 阿拉伯语
// - es: 西班牙语
// - fr: 法语
// - id: 印尼语
// - it: 意大利语
// - lv: 拉脱维亚语
// - nl: 荷兰语
// - pt: 葡萄牙语
// - ru: 俄语
// - tr: 土耳其语
// - vi: 越南语

// 根据请求头 Accept-Language 自动选择语言
func CreateUser(ctx context.Context, c *app.RequestContext) {
    var req CreateUserRequest
    c.BindJSON(&req)
    
    if err := facades.Validator.Struct(req); err != nil {
        // 自动翻译为请求的语言
        http.BadRequest(c, err)
        return
    }
}
```

## API 函数

### Success

```go
func Success[T any](c *app.RequestContext, data T, message ...string)
```

成功响应，code: 20000

### Fail

```go
func Fail(c *app.RequestContext, message string)
```

业务失败响应，code: 60000

### BadRequest

```go
func BadRequest(c *app.RequestContext, err error)
```

请求错误响应，code: 40000，自动翻译验证错误

### Unauthorized

```go
func Unauthorized(c *app.RequestContext, message string)
```

未认证响应，code: 40100

### Forbidden

```go
func Forbidden(c *app.RequestContext, message string)
```

无权限响应，code: 40300

### NotFound

```go
func NotFound(c *app.RequestContext, message string)
```

未找到响应，code: 40400

### ServerError

```go
func ServerError(c *app.RequestContext, message string)
```

服务器错误响应，code: 50000

### File

```go
func File(c *app.RequestContext, data []byte, filename string)
```

文件下载响应

### String

```go
func String(c *app.RequestContext, data string)
```

纯文本响应

## 中间件集成

### 错误处理中间件

```go
func ErrorHandler() app.HandlerFunc {
    return func(ctx context.Context, c *app.RequestContext) {
        defer func() {
            if r := recover(); r != nil {
                http.ServerError(c, "服务器内部错误")
            }
        }()
        
        c.Next(ctx)
    }
}

// 使用中间件
h.Use(ErrorHandler())
```

### 认证中间件

```go
func AuthMiddleware() app.HandlerFunc {
    return func(ctx context.Context, c *app.RequestContext) {
        token := c.GetHeader("Authorization")
        if token == "" {
            http.Unauthorized(c, "请先登录")
            c.Abort()
            return
        }
        
        claims, err := auth.CheckJWToken(token)
        if err != nil {
            http.Unauthorized(c, "令牌无效")
            c.Abort()
            return
        }
        
        c.Set("user_id", claims.ID)
        c.Next(ctx)
    }
}
```

## 最佳实践

1. 使用统一的响应格式
2. 提供清晰的错误消息
3. 使用合适的 HTTP 状态码
4. 验证错误自动翻译
5. 敏感信息不要包含在响应中
6. 使用泛型定义响应类型
7. 分页数据包含总数和页码信息

## 依赖项

- Hertz（HTTP 框架）
- validator（验证库）
- Validation facade（验证器）

## 文件结构

```
http/
├── response.go      # 响应辅助函数
└── middleware/      # HTTP 中间件
```
