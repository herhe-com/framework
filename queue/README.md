# Queue 组件

`queue` 提供 RabbitMQ 队列封装。当前只实现了 RabbitMQ 驱动。

## 配置

`queue.ServiceProvider` 会读取 `queue.default` 选择默认队列连接名，并按 `queue.connections.<name>` 初始化默认队列驱动。每个连接实例都需要自己的 `driver` 字段：

```yaml
queue:
  default: default
  connections:
    default:
      driver: rabbitmq
      host: 127.0.0.1
      port: 5672
      username: guest
      password: guest
      vhost: /
      error: basic_error
```

注意：必须保留 `connections` 这一层。`queue.default` 只保存连接名，example 基础项目如果写成 `queue.host`，则 `NewDriver("rabbitmq", "default")` 读不到配置。

## 使用

发送消息：

```go
body, err := json.Marshal(map[string]any{
	"user_id": 123,
	"action":  "send_email",
})
if err != nil {
	return err
}

err = facades.Queue.Producer(
	body,
	"basic",
	"basic_email",
	[]string{"email"},
	0,
	0,
)
```

消费消息：

```go
handler := func(data []byte) error {
	var message map[string]any
	if err := json.Unmarshal(data, &message); err != nil {
		return err
	}

	return nil
}

err := facades.Queue.Consumer(
	handler,
	"basic",
	"basic_email",
	"email",
	false,
	0,
	3,
)
```

切换通道：

```go
rabbitmqReport, err := facades.Queue.Channel("rabbitmq", "report")
if err != nil {
	return err
}
```

## 接口

```go
type Queue interface {
	Driver
	Channel(channel string, name string) (Driver, error)
}

type Driver interface {
	Producer(body []byte, exchange, queue string, routes []string, delay, ttl int64, headers ...rabbitmq.Table) error
	Consumer(handler func(data []byte) error, exchange, queue, route string, delay bool, ttl int64, retry int) error
	Close() error
}
```

参数说明：

- `delay`: 延迟秒数，大于 0 时使用 `x-delayed-message`。
- `ttl`: 秒数，大于 0 时设置 TTL 和死信交换机。
- `retry`: 消费失败后的重试次数。
- `headers`: 额外 RabbitMQ headers。

## 注意事项

- 当前没有 `queue.WithDelay`、`queue.WithTTL`、`queue.WithRetries` 这类 option API。
- `Producer` 每次调用会创建 publisher；高频场景需要评估连接和 publisher 生命周期成本。
- 如果消费失败且超过重试次数，会把错误信息投递到 `error` 配置的队列，默认 `basic_error`。
- 如果 example 基础项目没有注册 `queue.ServiceProvider`，队列配置即使存在也不会实际初始化。
