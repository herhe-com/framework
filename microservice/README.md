# Microservice 组件

微服务工具组件，提供分布式系统所需的基础设施，包括分布式 ID 生成和分布式锁。

## 子组件

### Snowflake - 分布式 ID 生成器

基于 Twitter Snowflake 算法的分布式唯一 ID 生成器。

#### 功能特性

- 全局唯一 ID
- 时间有序
- 高性能（每秒可生成数百万 ID）
- 节点隔离
- 64 位整数

#### ID 结构

```
0 - 0000000000 0000000000 0000000000 0000000000 0 - 00000 - 00000 - 000000000000
    |                                               |       |       |
    符号位(1bit)                                    时间戳  节点ID  序列号
                                                   (41bit) (10bit) (12bit)
```

- 1 位：符号位，始终为 0
- 41 位：时间戳（毫秒级），可使用约 69 年
- 10 位：节点 ID，支持 1024 个节点
- 12 位：序列号，每毫秒可生成 4096 个 ID

#### 配置

```yaml
snowflake:
  node: 1  # 节点 ID (0-1023)
```

#### 使用方法

```go
import "github.com/herhe-com/framework/facades"

// 生成 ID
id := facades.Snowflake.Generate()
fmt.Printf("Generated ID: %d\n", id)

// 批量生成
ids := make([]int64, 100)
for i := 0; i < 100; i++ {
    ids[i] = facades.Snowflake.Generate().Int64()
}
```

#### 在模型中使用

```go
type User struct {
    ID        int64  `gorm:"primarykey" json:"id"`
    Username  string `json:"username"`
    CreatedAt time.Time
}

func CreateUser(username string) (*User, error) {
    user := &User{
        ID:       facades.Snowflake.Generate().Int64(),
        Username: username,
    }
    
    if err := facades.DB.Default().Create(user).Error; err != nil {
        return nil, err
    }
    
    return user, nil
}
```

#### 解析 ID

```go
// 从 ID 中提取时间戳
id := facades.Snowflake.Generate()
timestamp := (id.Int64() >> 22) + 1288834974657 // Snowflake epoch
createdAt := time.Unix(timestamp/1000, (timestamp%1000)*1000000)

// 提取节点 ID
nodeID := (id.Int64() >> 12) & 0x3FF

// 提取序列号
sequence := id.Int64() & 0xFFF
```

### Locker - 分布式锁

基于 Redis 的分布式锁实现，使用 Redsync 库。

#### 功能特性

- 分布式互斥锁
- 自动过期
- 可重入锁支持
- 锁续期
- 多 Redis 实例支持

#### 配置

使用现有的 Redis 配置：

```yaml
redis:
  default:
    host: localhost
    port: 6379
    password: ""
    database: 0
```

#### 使用方法

##### 基础锁操作

```go
import (
    "context"
    "github.com/herhe-com/framework/facades"
)

// 创建互斥锁
mutex := facades.Locker.NewMutex("resource-key")

// 获取锁
if err := mutex.Lock(); err != nil {
    // 获取锁失败
    return err
}
defer mutex.Unlock()

// 执行临界区代码
// ...
```

##### 带超时的锁

```go
import "time"

mutex := facades.Locker.NewMutex(
    "resource-key",
    redsync.WithExpiry(10*time.Second),
    redsync.WithTries(3),
    redsync.WithRetryDelay(100*time.Millisecond),
)

if err := mutex.Lock(); err != nil {
    return err
}
defer mutex.Unlock()
```

##### 尝试获取锁

```go
mutex := facades.Locker.NewMutex("resource-key")

// 尝试获取锁，不阻塞
if err := mutex.TryLock(); err != nil {
    // 锁已被占用
    return errors.New("resource is locked")
}
defer mutex.Unlock()
```

##### 带上下文的锁

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

mutex := facades.Locker.NewMutex("resource-key")

if err := mutex.LockContext(ctx); err != nil {
    // 超时或获取锁失败
    return err
}
defer mutex.Unlock()
```

#### 应用场景

##### 防止重复提交

```go
func ProcessOrder(orderID string) error {
    lockKey := fmt.Sprintf("order:process:%s", orderID)
    mutex := facades.Locker.NewMutex(lockKey)
    
    if err := mutex.TryLock(); err != nil {
        return errors.New("订单正在处理中")
    }
    defer mutex.Unlock()
    
    // 处理订单逻辑
    return processOrderLogic(orderID)
}
```

##### 限流控制

```go
func RateLimitedOperation(userID string) error {
    lockKey := fmt.Sprintf("ratelimit:%s", userID)
    mutex := facades.Locker.NewMutex(
        lockKey,
        redsync.WithExpiry(1*time.Second),
    )
    
    if err := mutex.TryLock(); err != nil {
        return errors.New("操作过于频繁，请稍后再试")
    }
    defer mutex.Unlock()
    
    // 执行操作
    return performOperation(userID)
}
```

##### 定时任务互斥

```go
func ScheduledTask() {
    lockKey := "cron:daily-report"
    mutex := facades.Locker.NewMutex(
        lockKey,
        redsync.WithExpiry(10*time.Minute),
    )
    
    if err := mutex.TryLock(); err != nil {
        // 其他节点正在执行
        return
    }
    defer mutex.Unlock()
    
    // 生成报表
    generateDailyReport()
}
```

##### 库存扣减

```go
func DeductInventory(productID string, quantity int) error {
    lockKey := fmt.Sprintf("inventory:%s", productID)
    mutex := facades.Locker.NewMutex(lockKey)
    
    if err := mutex.Lock(); err != nil {
        return err
    }
    defer mutex.Unlock()
    
    // 检查库存
    var product Product
    db := facades.DB.Default()
    if err := db.First(&product, productID).Error; err != nil {
        return err
    }
    
    if product.Stock < quantity {
        return errors.New("库存不足")
    }
    
    // 扣减库存
    product.Stock -= quantity
    return db.Save(&product).Error
}
```

##### 缓存更新

```go
func UpdateCache(key string, data any) error {
    lockKey := fmt.Sprintf("cache:update:%s", key)
    mutex := facades.Locker.NewMutex(lockKey)
    
    if err := mutex.Lock(); err != nil {
        return err
    }
    defer mutex.Unlock()
    
    // 更新数据库
    if err := updateDatabase(data); err != nil {
        return err
    }
    
    // 更新缓存
    redis := facades.Redis.Default()
    return redis.Set(context.Background(), key, data, 1*time.Hour).Err()
}
```

## 最佳实践

### Snowflake ID

1. **节点 ID 分配**：为每个服务实例分配唯一的节点 ID
2. **时钟同步**：确保服务器时钟同步（使用 NTP）
3. **ID 类型**：使用 `int64` 存储 ID
4. **数据库主键**：可直接用作数据库主键
5. **避免回拨**：处理时钟回拨问题

```go
// 检测时钟回拨
func checkClockBackward(lastTimestamp int64) error {
    current := time.Now().UnixMilli()
    if current < lastTimestamp {
        return fmt.Errorf("clock moved backwards")
    }
    return nil
}
```

### 分布式锁

1. **锁粒度**：使用细粒度的锁键，避免锁竞争
2. **超时设置**：合理设置锁过期时间，防止死锁
3. **错误处理**：处理获取锁失败的情况
4. **及时释放**：使用 `defer` 确保锁被释放
5. **避免嵌套**：避免在持有锁时获取其他锁

```go
// 好的做法
mutex := facades.Locker.NewMutex(
    fmt.Sprintf("user:%d:operation", userID),
    redsync.WithExpiry(5*time.Second),
)

// 避免
mutex := facades.Locker.NewMutex("global-lock") // 锁粒度太粗
```

## 性能考虑

### Snowflake

- 单节点每秒可生成约 400 万个 ID
- 无网络开销，性能极高
- 适合高并发场景

### 分布式锁

- 获取锁需要网络往返
- 使用 Redis 管道可提高性能
- 考虑使用本地缓存减少锁竞争

## 故障处理

### Snowflake 时钟回拨

```go
// 处理时钟回拨
func generateIDWithRetry() (int64, error) {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        id := facades.Snowflake.Generate()
        if id.Int64() > 0 {
            return id.Int64(), nil
        }
        time.Sleep(time.Millisecond)
    }
    return 0, errors.New("failed to generate ID")
}
```

### 分布式锁超时

```go
// 锁续期
func longRunningTask() error {
    mutex := facades.Locker.NewMutex(
        "long-task",
        redsync.WithExpiry(30*time.Second),
    )
    
    if err := mutex.Lock(); err != nil {
        return err
    }
    defer mutex.Unlock()
    
    // 启动续期 goroutine
    done := make(chan bool)
    go func() {
        ticker := time.NewTicker(10 * time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                mutex.Extend()
            case <-done:
                return
            }
        }
    }()
    
    // 执行长时间任务
    err := performLongTask()
    
    close(done)
    return err
}
```

## 依赖项

- bwmarrin/snowflake（Snowflake ID 生成）
- go-redsync（分布式锁）
- Redis（锁存储）
- Config facade（配置管理）

## 文件结构

```
microservice/
├── snowflake/
│   ├── application.go    # Snowflake 实现
│   └── provider.go       # 服务提供者
└── locker/
    ├── application.go    # 分布式锁实现
    └── provider.go       # 服务提供者
```

## 访问方式

```go
import "github.com/herhe-com/framework/facades"

// Snowflake ID 生成
id := facades.Snowflake.Generate()

// 分布式锁
mutex := facades.Locker.NewMutex("key")
```
