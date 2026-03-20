# Queue 组件

消息队列抽象层，提供统一的消息队列接口，支持多种消息队列服务。

## 功能特性

- 统一的消息队列接口
- 多驱动支持（RabbitMQ）
- 生产者/消费者模式
- 延迟消息支持
- 死信队列（DLQ）
- 消息重试机制
- 多通道支持

## 支持的驱动

- **RabbitMQ**: 成熟的消息队列系统

## 配置

```yaml
queue:
  driver: rabbitmq
  
  rabbitmq:
    default:
      host: localhost
      port: 5672
      username: guest
      password: guest
      vhost: /
      exchange: default-exchange
      queue: default-queue
      routing: default-routing
```

## 使用方法

### 发送消息

```go
import "github.com/herhe-com/framework/facades"

// 发送简单消息
message := map[string]any{
    "user_id": 123,
    "action":  "send_email",
    "email":   "user@example.com",
}

data, _ := json.Marshal(message)
err := facades.Queue.Producer("exchange", "routing-key", data)
```

### 延迟消息

```go
import "time"

// 发送延迟消息（5 分钟后处理）
err := facades.Queue.Producer(
    "exchange",
    "routing-key",
    data,
    queue.WithDelay(5 * time.Minute),
)
```

### 消息 TTL

```go
// 设置消息过期时间（10 分钟）
err := facades.Queue.Producer(
    "exchange",
    "routing-key",
    data,
    queue.WithTTL(10 * time.Minute),
)
```

### 消费消息

```go
// 定义消息处理函数
handler := func(body []byte) error {
    var message map[string]any
    if err := json.Unmarshal(body, &message); err != nil {
        return err
    }
    
    // 处理消息
    fmt.Printf("Processing message: %v\n", message)
    
    // 返回 nil 表示成功，消息将被确认
    // 返回 error 表示失败，消息将被重新入队
    return nil
}

// 启动消费者
err := facades.Queue.Consumer("queue-name", handler)
```

### 消费者选项

```go
// 带重试的消费者
err := facades.Queue.Consumer(
    "queue-name",
    handler,
    queue.WithRetries(3),                    // 最多重试 3 次
    queue.WithRetryDelay(1 * time.Second),   // 重试间隔 1 秒
)
```

## 应用场景

### 异步任务处理

```go
// 发送邮件任务
type EmailTask struct {
    To      string `json:"to"`
    Subject string `json:"subject"`
    Body    string `json:"body"`
}

func SendEmailAsync(to, subject, body string) error {
    task := EmailTask{
        To:      to,
        Subject: subject,
        Body:    body,
    }
    
    data, _ := json.Marshal(task)
    return facades.Queue.Producer("tasks", "email", data)
}

// 消费者
func EmailWorker() {
    handler := func(body []byte) error {
        var task EmailTask
        json.Unmarshal(body, &task)
        
        // 发送邮件
        return sendEmail(task.To, task.Subject, task.Body)
    }
    
    facades.Queue.Consumer("email-queue", handler)
}
```

### 订单处理

```go
type OrderTask struct {
    OrderID string `json:"order_id"`
    Action  string `json:"action"`
}

func ProcessOrder(orderID string) error {
    task := OrderTask{
        OrderID: orderID,
        Action:  "process",
    }
    
    data, _ := json.Marshal(task)
    return facades.Queue.Producer("orders", "process", data)
}

// 消费者
func OrderWorker() {
    handler := func(body []byte) error {
        var task OrderTask
        json.Unmarshal(body, &task)
        
        // 处理订单
        return processOrderLogic(task.OrderID)
    }
    
    facades.Queue.Consumer("order-queue", handler)
}
```

### 数据同步

```go
type SyncTask struct {
    Type   string `json:"type"`
    ID     string `json:"id"`
    Action string `json:"action"`
}

func SyncData(dataType, id, action string) error {
    task := SyncTask{
        Type:   dataType,
        ID:     id,
        Action: action,
    }
    
    data, _ := json.Marshal(task)
    return facades.Queue.Producer("sync", dataType, data)
}

// 消费者
func SyncWorker() {
    handler := func(body []byte) error {
        var task SyncTask
        json.Unmarshal(body, &task)
        
        switch task.Type {
        case "user":
            return syncUser(task.ID, task.Action)
        case "product":
            return syncProduct(task.ID, task.Action)
        default:
            return fmt.Errorf("unknown type: %s", task.Type)
        }
    }
    
    facades.Queue.Consumer("sync-queue", handler)
}
```

### 日志收集

```go
type LogEntry struct {
    Level     string    `json:"level"`
    Message   string    `json:"message"`
    Timestamp time.Time `json:"timestamp"`
    Context   map[string]any `json:"context"`
}

func LogAsync(level, message string, context map[string]any) error {
    entry := LogEntry{
        Level:     level,
        Message:   message,
        Timestamp: time.Now(),
        Context:   context,
    }
    
    data, _ := json.Marshal(entry)
    return facades.Queue.Producer("logs", level, data)
}

// 消费者
func LogWorker() {
    handler := func(body []byte) error {
        var entry LogEntry
        json.Unmarshal(body, &entry)
        
        // 写入日志存储
        return writeToLogStorage(entry)
    }
    
    facades.Queue.Consumer("log-queue", handler)
}
```

### 图片处理

```go
type ImageTask struct {
    URL    string   `json:"url"`
    Sizes  []string `json:"sizes"`
    UserID string   `json:"user_id"`
}

func ProcessImageAsync(url string, sizes []string, userID string) error {
    task := ImageTask{
        URL:    url,
        Sizes:  sizes,
        UserID: userID,
    }
    
    data, _ := json.Marshal(task)
    return facades.Queue.Producer("images", "process", data)
}

// 消费者
func ImageWorker() {
    handler := func(body []byte) error {
        var task ImageTask
        json.Unmarshal(body, &task)
        
        // 下载原图
        img, err := downloadImage(task.URL)
        if err != nil {
            return err
        }
        
        // 生成不同尺寸
        for _, size := range task.Sizes {
            resized := resizeImage(img, size)
            uploadImage(resized, task.UserID, size)
        }
        
        return nil
    }
    
    facades.Queue.Consumer("image-queue", handler)
}
```

## 错误处理

### 重试机制

```go
handler := func(body []byte) error {
    // 处理消息
    if err := processMessage(body); err != nil {
        // 返回错误，消息将被重新入队
        return err
    }
    
    // 返回 nil，消息将被确认
    return nil
}

// 配置重试
facades.Queue.Consumer(
    "queue-name",
    handler,
    queue.WithRetries(3),
    queue.WithRetryDelay(2 * time.Second),
)
```

### 死信队列

```go
// 配置死信队列
// 当消息重试次数超过限制时，将被发送到死信队列
err := facades.Queue.Producer(
    "exchange",
    "routing-key",
    data,
    queue.WithDLQ("dead-letter-exchange", "dead-letter-routing"),
)

// 监控死信队列
func DeadLetterWorker() {
    handler := func(body []byte) error {
        // 记录失败的消息
        log.Printf("Dead letter message: %s", string(body))
        
        // 可以选择人工处理或丢弃
        return nil
    }
    
    facades.Queue.Consumer("dead-letter-queue", handler)
}
```

### 优雅关闭

```go
func main() {
    // 启动消费者
    go EmailWorker()
    go OrderWorker()
    
    // 等待退出信号
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    // 停止消费者，等待当前消息处理完成
    log.Println("Shutting down workers...")
    time.Sleep(5 * time.Second)
}
```

## 接口定义

```go
type Driver interface {
    // Producer 发送消息
    Producer(exchange, routing string, body []byte, options ...ProducerOption) error
    
    // Consumer 消费消息
    Consumer(queue string, handler ConsumerHandler, options ...ConsumerOption) error
}

type ConsumerHandler func(body []byte) error

type ProducerOption func(*ProducerConfig)
type ConsumerOption func(*ConsumerConfig)
```

## 生产者选项

```go
// 延迟消息
queue.WithDelay(duration time.Duration)

// 消息 TTL
queue.WithTTL(duration time.Duration)

// 死信队列
queue.WithDLQ(exchange, routing string)

// 消息优先级
queue.WithPriority(priority uint8)
```

## 消费者选项

```go
// 重试次数
queue.WithRetries(count int)

// 重试延迟
queue.WithRetryDelay(duration time.Duration)

// 并发数
queue.WithConcurrency(count int)

// 预取数量
queue.WithPrefetch(count int)
```

## 最佳实践

1. **消息幂等性**：确保消息处理是幂等的，避免重复处理
2. **错误处理**：合理处理错误，决定是否重试
3. **消息大小**：避免发送过大的消息，考虑使用引用
4. **监控**：监控队列长度和消费速度
5. **死信队列**：配置死信队列处理失败消息
6. **优雅关闭**：确保消费者优雅关闭，处理完当前消息

## 性能优化

### 批量发送

```go
messages := [][]byte{msg1, msg2, msg3}

for _, msg := range messages {
    facades.Queue.Producer("exchange", "routing", msg)
}
```

### 并发消费

```go
// 启动多个消费者
for i := 0; i < 5; i++ {
    go facades.Queue.Consumer(
        "queue-name",
        handler,
        queue.WithConcurrency(10),
    )
}
```

### 预取优化

```go
// 增加预取数量，提高吞吐量
facades.Queue.Consumer(
    "queue-name",
    handler,
    queue.WithPrefetch(100),
)
```

## 监控和调试

### 队列状态

```go
// 获取队列信息（需要 RabbitMQ Management API）
// - 队列长度
// - 消费者数量
// - 消息速率
```

### 日志记录

```go
handler := func(body []byte) error {
    start := time.Now()
    
    log.Printf("Processing message: %s", string(body))
    
    err := processMessage(body)
    
    duration := time.Since(start)
    log.Printf("Message processed in %v, error: %v", duration, err)
    
    return err
}
```

## 依赖项

- wagslane/go-rabbitmq（RabbitMQ 客户端）
- Config facade（配置管理）

## 文件结构

```
queue/
├── application.go    # 队列应用实现
├── provider.go       # 服务提供者
└── rabbitmq/        # RabbitMQ 驱动实现
```

## 访问方式

```go
import "github.com/herhe-com/framework/facades"

// 发送消息
facades.Queue.Producer("exchange", "routing", data)

// 消费消息
facades.Queue.Consumer("queue", handler)

// 切换通道
rabbitmq := facades.Queue.Channel("rabbitmq")
```
