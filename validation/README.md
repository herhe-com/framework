# Validation 组件

请求验证组件，基于 go-playground/validator，提供多语言支持的验证功能。

## 功能特性

- 基于 go-playground/validator
- 多语言错误消息支持（14 种语言）
- 自定义验证规则
- 可配置的字段标签
- 自动错误消息翻译

## 支持的语言

- zh: 中文
- en: 英文
- ja: 日文
- ar: 阿拉伯语
- es: 西班牙语
- fr: 法语
- id: 印尼语
- it: 意大利语
- lv: 拉脱维亚语
- nl: 荷兰语
- pt: 葡萄牙语
- ru: 俄语
- tr: 土耳其语
- vi: 越南语

## 使用方法

### 基础验证

```go
import "github.com/herhe-com/framework/facades"

type CreateUserRequest struct {
    Username string `json:"username" validate:"required,min=3,max=20"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=6"`
    Age      int    `json:"age" validate:"required,gte=18,lte=100"`
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
    
    // 处理请求
    http.Success(c, nil)
}
```

### 内置验证规则

```go
type User struct {
    // 必填
    Username string `validate:"required"`
    
    // 邮箱
    Email string `validate:"required,email"`
    
    // 长度限制
    Password string `validate:"required,min=6,max=20"`
    
    // 数值范围
    Age int `validate:"gte=18,lte=100"`
    
    // 枚举值
    Gender string `validate:"oneof=male female other"`
    
    // URL
    Website string `validate:"url"`
    
    // 日期
    Birthday string `validate:"datetime=2006-01-02"`
    
    // 正则表达式
    Phone string `validate:"regexp=^1[3-9]\\d{9}$"`
    
    // 数组长度
    Tags []string `validate:"min=1,max=5"`
    
    // 嵌套验证
    Address Address `validate:"required"`
}

type Address struct {
    Street  string `validate:"required"`
    City    string `validate:"required"`
    ZipCode string `validate:"required,len=6"`
}
```

### 字段验证

```go
// 验证单个字段
email := "test@example.com"
err := facades.Validator.Var(email, "required,email")
if err != nil {
    fmt.Println("邮箱格式不正确")
}

// 验证多个字段
username := "john"
err = facades.Validator.Var(username, "required,min=3,max=20,alphanum")
```

### 自定义验证规则

```go
// 在 validation/rules.go 中定义自定义规则
package validation

import "github.com/go-playground/validator/v10"

type CustomRule struct{}

func (r *CustomRule) Key() string {
    return "custom"
}

func (r *CustomRule) Message() string {
    return "{0} 不符合自定义规则"
}

func (r *CustomRule) Validate(fl validator.FieldLevel) bool {
    // 自定义验证逻辑
    value := fl.Field().String()
    return len(value) > 0 && value != "forbidden"
}

// 使用自定义规则
type Request struct {
    Field string `validate:"custom"`
}
```

### 条件验证

```go
type UpdateUserRequest struct {
    Email    string `validate:"omitempty,email"`
    Password string `validate:"omitempty,min=6"`
    Age      int    `validate:"omitempty,gte=18"`
}

// omitempty: 字段为空时跳过验证
```

### 跨字段验证

```go
type ChangePasswordRequest struct {
    OldPassword     string `validate:"required,min=6"`
    NewPassword     string `validate:"required,min=6,nefield=OldPassword"`
    ConfirmPassword string `validate:"required,eqfield=NewPassword"`
}

// nefield: 不等于指定字段
// eqfield: 等于指定字段
// gtfield: 大于指定字段
// ltfield: 小于指定字段
```

### 数组和切片验证

```go
type BatchRequest struct {
    IDs    []int    `validate:"required,min=1,max=100,dive,gt=0"`
    Emails []string `validate:"required,dive,email"`
    Users  []User   `validate:"required,dive"`
}

// dive: 深入验证数组元素
```

### 映射验证

```go
type ConfigRequest struct {
    Settings map[string]string `validate:"required,dive,keys,required,endkeys,required"`
}

// keys: 验证键
// endkeys: 结束键验证
// 之后验证值
```

## 错误处理

### 获取验证错误

```go
if err := facades.Validator.Struct(req); err != nil {
    // 类型断言为 ValidationErrors
    if validationErrors, ok := err.(validator.ValidationErrors); ok {
        for _, fieldError := range validationErrors {
            fmt.Printf("字段: %s\n", fieldError.Field())
            fmt.Printf("标签: %s\n", fieldError.Tag())
            fmt.Printf("值: %v\n", fieldError.Value())
            fmt.Printf("参数: %s\n", fieldError.Param())
        }
    }
}
```

### 自定义错误消息

```go
// 在 validation/error.go 中自定义错误格式
func FormatError(err error, labels map[string]string) string {
    if validationErrors, ok := err.(validator.ValidationErrors); ok {
        var messages []string
        
        for _, fieldError := range validationErrors {
            field := fieldError.Field()
            if label, ok := labels[field]; ok {
                field = label
            }
            
            message := translateError(fieldError, field)
            messages = append(messages, message)
        }
        
        return strings.Join(messages, "; ")
    }
    
    return err.Error()
}
```

### 字段标签映射

```go
// 定义字段标签
labels := map[string]string{
    "Username": "用户名",
    "Email":    "邮箱",
    "Password": "密码",
}

// 使用标签格式化错误
if err := facades.Validator.Struct(req); err != nil {
    message := FormatError(err, labels)
    http.BadRequest(c, message)
}
```

## 常用验证规则

### 字符串验证

```go
type StringValidation struct {
    Required   string `validate:"required"`           // 必填
    MinMax     string `validate:"min=3,max=20"`       // 长度范围
    Len        string `validate:"len=10"`             // 固定长度
    Alpha      string `validate:"alpha"`              // 只包含字母
    Alphanum   string `validate:"alphanum"`           // 字母和数字
    Numeric    string `validate:"numeric"`            // 数字字符串
    Email      string `validate:"email"`              // 邮箱
    URL        string `validate:"url"`                // URL
    URI        string `validate:"uri"`                // URI
    Base64     string `validate:"base64"`             // Base64
    Contains   string `validate:"contains=abc"`       // 包含子串
    StartsWith string `validate:"startswith=prefix"`  // 前缀
    EndsWith   string `validate:"endswith=suffix"`    // 后缀
    UUID       string `validate:"uuid"`               // UUID
    JSON       string `validate:"json"`               // JSON
}
```

### 数值验证

```go
type NumberValidation struct {
    Required int     `validate:"required"`        // 必填
    Min      int     `validate:"min=0"`           // 最小值
    Max      int     `validate:"max=100"`         // 最大值
    Range    int     `validate:"gte=0,lte=100"`   // 范围
    Positive int     `validate:"gt=0"`            // 正数
    Negative int     `validate:"lt=0"`            // 负数
    OneOf    int     `validate:"oneof=1 2 3"`     // 枚举
    Float    float64 `validate:"gte=0.0,lte=1.0"` // 浮点数范围
}
```

### 日期时间验证

```go
type DateValidation struct {
    DateTime string    `validate:"datetime=2006-01-02 15:04:05"` // 日期时间格式
    Date     string    `validate:"datetime=2006-01-02"`          // 日期格式
    Time     string    `validate:"datetime=15:04:05"`            // 时间格式
    After    time.Time `validate:"gtfield=StartDate"`            // 晚于某个日期
    Before   time.Time `validate:"ltfield=EndDate"`              // 早于某个日期
}
```

### 网络验证

```go
type NetworkValidation struct {
    IP       string `validate:"ip"`        // IP 地址
    IPv4     string `validate:"ipv4"`      // IPv4
    IPv6     string `validate:"ipv6"`      // IPv6
    MAC      string `validate:"mac"`       // MAC 地址
    Hostname string `validate:"hostname"`  // 主机名
    FQDN     string `validate:"fqdn"`      // 完全限定域名
}
```

### 文件验证

```go
type FileValidation struct {
    FilePath string `validate:"file"`      // 文件存在
    DirPath  string `validate:"dir"`       // 目录存在
}
```

## 配置

在配置文件中设置验证选项：

```yaml
validation:
  default_language: zh  # 默认语言
  field_labels:         # 字段标签映射
    username: 用户名
    email: 邮箱
    password: 密码
```

## 最佳实践

1. **使用结构体标签**：清晰定义验证规则
2. **提供友好的错误消息**：使用字段标签映射
3. **组合验证规则**：使用多个规则组合验证
4. **自定义规则**：为特定业务逻辑创建自定义规则
5. **条件验证**：使用 `omitempty` 处理可选字段
6. **嵌套验证**：验证嵌套结构体
7. **数组验证**：使用 `dive` 验证数组元素

## 依赖项

- go-playground/validator/v10（验证库）
- go-playground/universal-translator（翻译器）
- go-playground/locales（语言包）

## 文件结构

```
validation/
├── application.go    # 验证器初始化
├── rules.go         # 自定义验证规则
├── error.go         # 错误消息格式化
└── provider.go      # 服务提供者
```

## 访问方式

```go
import "github.com/herhe-com/framework/facades"

// 验证结构体
err := facades.Validator.Struct(data)

// 验证字段
err := facades.Validator.Var(value, "required,email")
```
