# Config 组件

集中式配置管理组件，基于 Viper 实现，支持本地和远程配置。

## 功能特性

- YAML 配置文件支持
- 远程配置支持（etcd、consul）
- 自动监听配置变更
- 类型安全的配置读取
- 默认值支持
- 动态配置添加

## 使用方法

### 基础配置读取

```go
import "github.com/herhe-com/framework/facades"

// 读取字符串配置
dbDriver := facades.Cfg.GetString("database.driver", "mysql")

// 读取整数配置
port := facades.Cfg.GetInt("app.port", 8080)

// 读取布尔配置
debug := facades.Cfg.GetBool("app.debug", false)

// 读取任意类型
value := facades.Cfg.Get("custom.config")
```

### 读取复杂配置

```go
// 读取 map 配置
s3Config := facades.Cfg.Get("filesystem.s3").(map[string]any)
access := s3Config["access"].(string)

// 读取数组配置
hosts := facades.Cfg.Get("redis.hosts").([]string)

// 使用 GetMaps 读取多个配置
configs := facades.Cfg.GetMaps("database", "redis", "cache")
```

### 动态设置配置

```go
// 设置单个配置
facades.Cfg.Set("app.name", "MyApp")

// 添加配置（支持嵌套）
facades.Cfg.Add("custom", map[string]any{
    "key1": "value1",
    "key2": 123,
})
```

## 配置文件

默认配置文件位置：`conf/env.yaml`

```yaml
app:
  name: MyApplication
  debug: true
  port: 8080

database:
  driver: mysql
  host: localhost
  port: 3306
  database: mydb
  username: root
  password: secret

redis:
  host: localhost
  port: 6379
  password: ""
  database: 0

filesystem:
  default: s3
  s3:
    access: your-access-key
    secret: your-secret-key
    region: us-east-1
    bucket: my-bucket
```

## 远程配置

通过环境变量启用远程配置支持：

### 环境变量

- `HH_CFG_PROVIDER`: 配置提供者（etcd、consul）
- `HH_CFG_ENDPOINT`: 配置服务器地址
- `HH_CFG_PATH`: 配置路径
- `HH_CFG_WATCH`: 是否监听配置变更（true/false）
- `HH_CFG_SECRET`: 配置加密密钥（可选）

### 使用 etcd

```bash
export HH_CFG_PROVIDER=etcd
export HH_CFG_ENDPOINT=http://localhost:2379
export HH_CFG_PATH=/config/myapp
export HH_CFG_WATCH=true
```

### 使用 consul

```bash
export HH_CFG_PROVIDER=consul
export HH_CFG_ENDPOINT=http://localhost:8500
export HH_CFG_PATH=config/myapp
export HH_CFG_WATCH=true
```

### 配置监听

启用 `HH_CFG_WATCH=true` 后，配置变更会自动同步（每 5 秒检查一次）：

```go
// 配置会自动更新，无需重启应用
dbHost := facades.Cfg.GetString("database.host")
```

## 接口定义

```go
type Application interface {
    // Get 获取配置值
    Get(key string) any
    
    // GetString 获取字符串配置
    GetString(key string, defaultValue ...string) string
    
    // GetInt 获取整数配置
    GetInt(key string, defaultValue ...int) int
    
    // GetBool 获取布尔配置
    GetBool(key string, defaultValue ...bool) bool
    
    // GetMaps 获取多个配置
    GetMaps(keys ...string) map[string]any
    
    // Set 设置配置值
    Set(key string, value any)
    
    // Add 添加配置
    Add(key string, value any)
}
```

## 配置最佳实践

### 使用默认值

始终为配置提供合理的默认值：

```go
// 好的做法
port := facades.Cfg.GetInt("app.port", 8080)
timeout := facades.Cfg.GetInt("http.timeout", 30)

// 避免
port := facades.Cfg.GetInt("app.port") // 可能返回 0
```

### 配置分组

按功能模块组织配置：

```yaml
# 数据库配置
database:
  driver: mysql
  host: localhost

# Redis 配置
redis:
  host: localhost
  port: 6379

# 文件存储配置
filesystem:
  default: s3
  s3:
    bucket: my-bucket
```

### 环境特定配置

使用不同的配置文件管理不同环境：

```
conf/
├── env.yaml          # 开发环境
├── env.prod.yaml     # 生产环境
└── env.test.yaml     # 测试环境
```

### 敏感信息

敏感配置应通过环境变量或远程配置管理：

```go
// 从环境变量读取
dbPassword := os.Getenv("DB_PASSWORD")
if dbPassword == "" {
    dbPassword = facades.Cfg.GetString("database.password")
}
```

## 高级用法

### 配置验证

在应用启动时验证必需的配置：

```go
func ValidateConfig() error {
    required := []string{
        "database.host",
        "database.database",
        "redis.host",
    }
    
    for _, key := range required {
        if facades.Cfg.Get(key) == nil {
            return fmt.Errorf("missing required config: %s", key)
        }
    }
    
    return nil
}
```

### 配置热更新

监听配置变更并执行相应操作：

```go
// 注意：需要启用远程配置和 watch 功能
func WatchDatabaseConfig() {
    go func() {
        ticker := time.NewTicker(10 * time.Second)
        defer ticker.Stop()
        
        lastHost := facades.Cfg.GetString("database.host")
        
        for range ticker.C {
            currentHost := facades.Cfg.GetString("database.host")
            if currentHost != lastHost {
                // 配置已变更，重新连接数据库
                ReconnectDatabase()
                lastHost = currentHost
            }
        }
    }()
}
```

### 配置缓存

对于频繁访问的配置，可以缓存到内存：

```go
type AppConfig struct {
    Name    string
    Debug   bool
    Port    int
}

var appConfig *AppConfig

func LoadAppConfig() {
    appConfig = &AppConfig{
        Name:  facades.Cfg.GetString("app.name", "MyApp"),
        Debug: facades.Cfg.GetBool("app.debug", false),
        Port:  facades.Cfg.GetInt("app.port", 8080),
    }
}
```

## 配置加密

对于敏感配置，可以使用加密存储：

```bash
# 设置加密密钥
export HH_CFG_SECRET=your-encryption-key

# 配置会自动加密/解密
```

## 依赖项

- Viper（配置管理库）
- Remote config providers（etcd、consul 客户端）

## 文件结构

```
config/
├── application.go    # 配置应用实现
└── provider.go       # 服务提供者
```

## 故障排查

### 配置未生效

1. 检查配置文件路径是否正确
2. 确认配置键名拼写正确（区分大小写）
3. 验证 YAML 格式是否正确

### 远程配置连接失败

1. 检查 `HH_CFG_ENDPOINT` 是否可访问
2. 验证 `HH_CFG_PATH` 路径是否存在
3. 确认配置服务器（etcd/consul）正常运行

### 配置类型转换错误

```go
// 错误：直接断言可能 panic
value := facades.Cfg.Get("key").(string)

// 正确：检查类型
if value := facades.Cfg.Get("key"); value != nil {
    if str, ok := value.(string); ok {
        // 使用 str
    }
}

// 更好：使用类型安全的方法
value := facades.Cfg.GetString("key", "default")
```
