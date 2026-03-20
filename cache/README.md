# Cache 组件

模型级缓存组件，提供基于 GORM 钩子的自动缓存失效机制。

## 功能特性

- 自动缓存失效（更新/删除时）
- 基于 GORM 钩子的透明缓存
- Redis 后端存储
- 可配置的 TTL
- 支持复合主键
- 缓存优先查询

## 使用方法

### 嵌入缓存模型

在你的 GORM 模型中嵌入 `cache.Model`：

```go
import (
    "github.com/herhe-com/framework/cache"
    "gorm.io/gorm"
)

type User struct {
    cache.Model        // 嵌入缓存模型
    ID          uint   `gorm:"primarykey"`
    Username    string
    Email       string
}
```

### 缓存优先查询

使用 `FindByID` 函数进行缓存优先查询：

```go
import (
    "github.com/herhe-com/framework/cache"
    "github.com/herhe-com/framework/facades"
)

var user User

// 首先从缓存查询，缓存未命中则从数据库查询并缓存
err := cache.FindByID(facades.DB.Default(), &user, 123)
if err != nil {
    // 处理错误
}
```

### 自动缓存失效

当模型更新或删除时，缓存会自动失效：

```go
// 更新用户 - 缓存自动失效
db.Model(&user).Update("username", "newname")

// 删除用户 - 缓存自动失效
db.Delete(&user)
```

## 工作原理

### GORM 钩子

`cache.Model` 实现了以下 GORM 钩子：

- `AfterUpdate`: 更新后清除缓存
- `AfterDelete`: 删除后清除缓存

### 缓存键生成

缓存键格式：`cache:model:{table_name}:{primary_key}`

示例：
- 单主键：`cache:model:users:123`
- 复合主键：`cache:model:orders:123_456`

### 缓存 TTL

默认 TTL 可以通过配置文件设置：

```yaml
cache:
  ttl: 3600  # 秒，默认 1 小时
```

## 高级用法

### 手动清除缓存

```go
import "github.com/herhe-com/framework/cache"

// 清除指定模型的缓存
err := cache.Clear(db, &user)
```

### 自定义缓存键

如果需要自定义缓存键生成逻辑，可以实现自己的缓存工具函数：

```go
import (
    "fmt"
    "github.com/herhe-com/framework/facades"
)

func CustomCacheKey(modelName string, id interface{}) string {
    return fmt.Sprintf("custom:cache:%s:%v", modelName, id)
}

// 使用自定义键
key := CustomCacheKey("users", 123)
facades.Redis.Default().Set(ctx, key, data, ttl)
```

## 注意事项

### 复合主键

组件自动检测复合主键并生成相应的缓存键。确保你的模型正确定义了主键：

```go
type OrderItem struct {
    cache.Model
    OrderID   uint `gorm:"primaryKey"`
    ProductID uint `gorm:"primaryKey"`
    Quantity  int
}
```

### 事务支持

在事务中使用缓存时，缓存失效会在事务提交后触发：

```go
db.Transaction(func(tx *gorm.DB) error {
    // 更新操作
    if err := tx.Model(&user).Update("status", "active").Error; err != nil {
        return err
    }
    // 事务提交后，缓存自动失效
    return nil
})
```

### 批量操作

批量更新或删除操作不会自动触发缓存失效，需要手动清除：

```go
// 批量更新
db.Model(&User{}).Where("status = ?", "inactive").Update("status", "active")

// 手动清除相关缓存
// 根据业务需求清除特定缓存或使用缓存标签
```

## 性能优化

### 缓存命中率

监控缓存命中率以优化缓存策略：

```go
// 在应用中添加缓存统计
var cacheHits, cacheMisses int64

// 在 FindByID 调用前后统计
```

### 缓存预热

对于热点数据，可以在应用启动时预热缓存：

```go
func WarmupCache(db *gorm.DB) {
    var users []User
    db.Find(&users)
    
    for _, user := range users {
        cache.FindByID(db, &User{}, user.ID)
    }
}
```

## 依赖项

- GORM（ORM 框架）
- Redis（缓存存储）
- Database facade（数据库访问）

## 文件结构

```
cache/
├── model.go    # 缓存模型和 GORM 钩子
└── util.go     # 缓存工具函数
```

## 最佳实践

1. 只对读多写少的数据使用缓存
2. 合理设置 TTL，避免缓存过期导致的缓存雪崩
3. 对于频繁更新的数据，考虑使用更短的 TTL 或不使用缓存
4. 监控缓存命中率，优化缓存策略
5. 在高并发场景下，考虑使用缓存预热和缓存降级策略
