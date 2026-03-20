# Database 组件

多数据库支持组件，提供 ORM（GORM）、Redis 和 MongoDB 的统一访问接口。

## 功能特性

- 多数据库驱动支持（MySQL、PostgreSQL、SQLite）
- Redis 多通道支持
- MongoDB 多客户端支持
- 连接池管理
- 预编译语句支持
- 调试模式
- 表前缀支持
- 平台级查询作用域

## 子组件

### ORM (GORM)

基于 GORM 的关系型数据库访问。

#### 配置

```yaml
database:
  driver: mysql
  mysql:
    default:
      driver: mysql
      host: localhost
      port: 3306
      database: mydb
      username: root
      password: secret
      charset: utf8mb4
      prefix: app_
      singular: false
      pool:
        max_idle_conns: 10
        max_open_conns: 100
        conn_max_lifetime: 3600
```

#### 使用方法

```go
import "github.com/herhe-com/framework/facades"

// 获取默认数据库连接
db := facades.DB.Default()

// 获取指定通道的连接
mysqlDB := facades.DB.Channel("mysql")
postgresDB := facades.DB.Channel("postgres")
```

#### 基础操作

```go
// 创建记录
user := User{Username: "john", Email: "john@example.com"}
db.Create(&user)

// 查询记录
var user User
db.First(&user, 1)
db.Where("username = ?", "john").First(&user)

// 更新记录
db.Model(&user).Update("email", "newemail@example.com")
db.Model(&user).Updates(User{Email: "new@example.com", Status: "active"})

// 删除记录
db.Delete(&user)
db.Where("status = ?", "inactive").Delete(&User{})
```

#### 高级查询

```go
// 关联查询
db.Preload("Orders").Find(&users)
db.Preload("Orders.Items").Find(&users)

// 聚合查询
var count int64
db.Model(&User{}).Where("status = ?", "active").Count(&count)

var total float64
db.Model(&Order{}).Select("SUM(amount)").Scan(&total)

// 分组查询
type Result struct {
    Date  string
    Count int64
}
var results []Result
db.Model(&Order{}).
    Select("DATE(created_at) as date, COUNT(*) as count").
    Group("DATE(created_at)").
    Scan(&results)
```

#### 事务

```go
// 自动事务
err := db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&user).Error; err != nil {
        return err
    }
    
    if err := tx.Create(&profile).Error; err != nil {
        return err
    }
    
    return nil
})

// 手动事务
tx := db.Begin()
defer func() {
    if r := recover(); r != nil {
        tx.Rollback()
    }
}()

if err := tx.Create(&user).Error; err != nil {
    tx.Rollback()
    return err
}

if err := tx.Create(&profile).Error; err != nil {
    tx.Rollback()
    return err
}

tx.Commit()
```

#### 平台作用域

```go
// 定义平台作用域
func PlatformScope(platform string) func(db *gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        return db.Where("platform = ?", platform)
    }
}

// 使用作用域
db.Scopes(PlatformScope("admin")).Find(&users)
```

### Redis

Redis 客户端管理，支持多通道。

#### 配置

```yaml
redis:
  default:
    host: localhost
    port: 6379
    password: ""
    database: 0
    pool:
      max_idle: 10
      max_active: 100
  cache:
    host: localhost
    port: 6379
    database: 1
```

#### 使用方法

```go
import (
    "context"
    "github.com/herhe-com/framework/facades"
)

ctx := context.Background()

// 获取默认 Redis 连接
redis := facades.Redis.Default()

// 获取指定通道的连接
cacheRedis := facades.Redis.Channel("cache")
```

#### 基础操作

```go
// 字符串操作
redis.Set(ctx, "key", "value", 0)
val, err := redis.Get(ctx, "key").Result()
redis.Del(ctx, "key")

// 设置过期时间
redis.Set(ctx, "key", "value", 10*time.Minute)
redis.Expire(ctx, "key", 5*time.Minute)

// 哈希操作
redis.HSet(ctx, "user:1", "name", "john")
redis.HSet(ctx, "user:1", "email", "john@example.com")
name := redis.HGet(ctx, "user:1", "name").Val()
redis.HGetAll(ctx, "user:1")

// 列表操作
redis.LPush(ctx, "queue", "task1", "task2")
redis.RPush(ctx, "queue", "task3")
task := redis.LPop(ctx, "queue").Val()

// 集合操作
redis.SAdd(ctx, "tags", "go", "redis", "database")
redis.SMembers(ctx, "tags")
redis.SIsMember(ctx, "tags", "go")

// 有序集合操作
redis.ZAdd(ctx, "leaderboard", redis.Z{Score: 100, Member: "player1"})
redis.ZAdd(ctx, "leaderboard", redis.Z{Score: 200, Member: "player2"})
redis.ZRange(ctx, "leaderboard", 0, -1)
```

#### 高级操作

```go
// 管道
pipe := redis.Pipeline()
pipe.Set(ctx, "key1", "value1", 0)
pipe.Set(ctx, "key2", "value2", 0)
pipe.Incr(ctx, "counter")
_, err := pipe.Exec(ctx)

// 事务
_, err := redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
    pipe.Set(ctx, "key1", "value1", 0)
    pipe.Set(ctx, "key2", "value2", 0)
    return nil
})

// 发布订阅
pubsub := redis.Subscribe(ctx, "channel")
defer pubsub.Close()

ch := pubsub.Channel()
for msg := range ch {
    fmt.Println(msg.Payload)
}

// 发布消息
redis.Publish(ctx, "channel", "message")
```

### MongoDB

MongoDB 客户端管理，支持多客户端。

#### 配置

```yaml
mongodb:
  default:
    uri: mongodb://localhost:27017
    database: mydb
    timeout: 10
  analytics:
    host: localhost
    port: 27017
    username: admin
    password: secret
    database: analytics
```

#### 使用方法

```go
import (
    "context"
    "github.com/herhe-com/framework/facades"
    "go.mongodb.org/mongo-driver/bson"
)

ctx := context.Background()

// 获取默认 MongoDB 客户端
mongo := facades.Mongo.Default()

// 获取指定客户端
analyticsMongo := facades.Mongo.Channel("analytics")

// 获取数据库和集合
db := mongo.Database("mydb")
collection := db.Collection("users")
```

#### 基础操作

```go
// 插入文档
user := bson.M{"name": "john", "email": "john@example.com"}
result, err := collection.InsertOne(ctx, user)

// 插入多个文档
users := []interface{}{
    bson.M{"name": "john", "email": "john@example.com"},
    bson.M{"name": "jane", "email": "jane@example.com"},
}
collection.InsertMany(ctx, users)

// 查询文档
var user bson.M
err := collection.FindOne(ctx, bson.M{"name": "john"}).Decode(&user)

// 查询多个文档
cursor, err := collection.Find(ctx, bson.M{"status": "active"})
defer cursor.Close(ctx)

var users []bson.M
cursor.All(ctx, &users)

// 更新文档
update := bson.M{"$set": bson.M{"email": "newemail@example.com"}}
collection.UpdateOne(ctx, bson.M{"name": "john"}, update)

// 删除文档
collection.DeleteOne(ctx, bson.M{"name": "john"})
collection.DeleteMany(ctx, bson.M{"status": "inactive"})
```

#### 高级查询

```go
// 聚合查询
pipeline := []bson.M{
    {"$match": bson.M{"status": "active"}},
    {"$group": bson.M{
        "_id": "$category",
        "count": bson.M{"$sum": 1},
    }},
    {"$sort": bson.M{"count": -1}},
}

cursor, err := collection.Aggregate(ctx, pipeline)
defer cursor.Close(ctx)

var results []bson.M
cursor.All(ctx, &results)

// 分页查询
opts := options.Find().SetSkip(0).SetLimit(10)
cursor, err := collection.Find(ctx, bson.M{}, opts)

// 排序
opts := options.Find().SetSort(bson.M{"created_at": -1})
cursor, err := collection.Find(ctx, bson.M{}, opts)
```

## 连接池配置

### GORM 连接池

```yaml
database:
  mysql:
    pool:
      max_idle_conns: 10      # 最大空闲连接数
      max_open_conns: 100     # 最大打开连接数
      conn_max_lifetime: 3600 # 连接最大生命周期（秒）
```

### Redis 连接池

```yaml
redis:
  default:
    pool:
      max_idle: 10      # 最大空闲连接数
      max_active: 100   # 最大活跃连接数
      idle_timeout: 300 # 空闲连接超时（秒）
```

## 调试模式

启用调试模式以查看 SQL 查询：

```yaml
app:
  debug: true
```

调试模式下：
- 打印所有 SQL 查询
- 禁用预编译语句
- 显示详细错误信息

## 依赖项

- GORM（ORM 框架）
- go-redis（Redis 客户端）
- mongo-driver（MongoDB 驱动）
- Config facade（配置管理）

## 文件结构

```
database/
├── orm/
│   ├── application.go    # GORM 应用实现
│   ├── dns.go           # DSN 生成
│   ├── log.go           # 日志配置
│   └── provider.go      # 服务提供者
├── redis/
│   ├── application.go    # Redis 应用实现
│   └── provider.go      # 服务提供者
└── mongodb/
    ├── application.go    # MongoDB 应用实现
    └── provider.go      # 服务提供者
```

## 最佳实践

1. 使用连接池管理数据库连接
2. 在生产环境关闭调试模式
3. 使用事务保证数据一致性
4. 合理设置 Redis 过期时间
5. 使用索引优化查询性能
6. 避免 N+1 查询问题（使用 Preload）
7. 定期清理过期数据
