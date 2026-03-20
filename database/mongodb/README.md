# MongoDB 支持

独立的 MongoDB 驱动包，使用官方 `go.mongodb.org/mongo-driver`。

## 安装

MongoDB 驱动已经包含在项目依赖中。

## 配置

在配置文件中添加 MongoDB 配置：

```yaml
mongodb:
  default:
    # 方式1: 使用连接 URI（推荐）
    uri: "mongodb://username:password@localhost:27017/mydb?authSource=admin"
    
    # 方式2: 使用独立参数
    host: "localhost"
    port: "27017"           # 默认: 27017
    username: "admin"
    password: "password"
    db: "mydb"
    auth_source: "admin"    # 默认: admin
    timeout: 10             # 连接超时（秒），默认: 10
  
  # 额外的连接配置
  analytics:
    host: "localhost"
    port: "27017"
    db: "analytics"
```

## 注册服务提供者

在应用启动时注册 MongoDB 服务提供者：

```go
import "github.com/herhe-com/framework/database/mongodb"

// 在服务提供者列表中添加
providers := []service.Provider{
    &mongodb.ServiceProvider{},
    // ... 其他服务提供者
}
```

## 使用方法

### 基本操作

```go
import (
    "context"
    "time"
    
    "github.com/herhe-com/framework/facades"
    "go.mongodb.org/mongo-driver/bson"
)

// 获取默认 MongoDB 客户端
client := facades.Mongo.Default()

// 获取数据库和集合
database := client.Database("mydb")
collection := database.Collection("users")

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// 插入文档
user := bson.M{
    "name":  "John Doe",
    "email": "john@example.com",
    "age":   30,
}
result, err := collection.InsertOne(ctx, user)

// 查询文档
var foundUser bson.M
err = collection.FindOne(ctx, bson.M{"email": "john@example.com"}).Decode(&foundUser)

// 更新文档
update := bson.M{"$set": bson.M{"age": 31}}
_, err = collection.UpdateOne(ctx, bson.M{"email": "john@example.com"}, update)

// 删除文档
_, err = collection.DeleteOne(ctx, bson.M{"email": "john@example.com"})
```

### 使用多个连接

```go
// 获取指定的 MongoDB 连接
analyticsClient, err := facades.Mongo.Driver("analytics")
if err != nil {
    return err
}

database := analyticsClient.Database("analytics")
collection := database.Collection("events")
```

### 事务

```go
client := facades.Mongo.Default()

session, err := client.StartSession()
if err != nil {
    return err
}
defer session.EndSession(context.Background())

callback := func(sessCtx mongo.SessionContext) (interface{}, error) {
    database := client.Database("mydb")
    usersCollection := database.Collection("users")
    ordersCollection := database.Collection("orders")

    // 插入用户
    user := bson.M{"name": "Jane Doe", "email": "jane@example.com"}
    userResult, err := usersCollection.InsertOne(sessCtx, user)
    if err != nil {
        return nil, err
    }

    // 插入订单
    order := bson.M{
        "user_id": userResult.InsertedID,
        "amount":  100.50,
    }
    _, err = ordersCollection.InsertOne(sessCtx, order)
    return nil, err
}

_, err = session.WithTransaction(context.Background(), callback)
```

### 聚合查询

```go
pipeline := mongo.Pipeline{
    {{Key: "$match", Value: bson.D{{Key: "status", Value: "completed"}}}},
    {{Key: "$group", Value: bson.D{
        {Key: "_id", Value: "$user_id"},
        {Key: "total", Value: bson.D{{Key: "$sum", Value: "$amount"}}},
        {Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
    }}},
    {{Key: "$sort", Value: bson.D{{Key: "total", Value: -1}}}},
}

cursor, err := collection.Aggregate(ctx, pipeline)
if err != nil {
    return err
}
defer cursor.Close(ctx)

var results []bson.M
if err = cursor.All(ctx, &results); err != nil {
    return err
}
```

### 创建索引

```go
indexModel := mongo.IndexModel{
    Keys:    bson.D{{Key: "email", Value: 1}},
    Options: options.Index().SetUnique(true),
}

_, err := collection.Indexes().CreateOne(ctx, indexModel)
```

## 接口定义

```go
type Mongo interface {
    // Default 返回默认的 MongoDB 客户端
    Default() *mongo.Client

    // Driver 返回指定名称的 MongoDB 客户端
    Driver(name string) (*mongo.Client, error)
}
```

## 更多示例

查看 `database/mongodb/example.go` 获取更多使用示例。

## 注意事项

1. MongoDB 是独立的包，不依赖 GORM
2. 始终使用 context 进行超时控制
3. 记得在操作完成后关闭 cursor
4. 事务需要 MongoDB 副本集支持
5. 建议在生产环境使用连接池配置
