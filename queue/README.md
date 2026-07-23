# Queue 组件

`queue` 提供 RabbitMQ 和 NATS 队列封装。

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

NATS 使用 core NATS subject 和 queue group：

```yaml
queue:
  default: events
  connections:
    events:
      driver: nats
      url: nats://127.0.0.1:4222
      name: framework-queue
      username: ""
      password: ""
      token: ""
      credentials: ""
      error: basic_error
      stream: FRAMEWORK_QUEUE
      schedule_prefix: _framework.queue.schedule
      retention: 1h
```

未设置 `url` 时，也可以用 `host` 和 `port`，默认分别为 `127.0.0.1` 和 `4222`。认证按 `credentials`、`token`、`username/password` 的顺序选择。延迟消息和 Producer/Consumer TTL 要求 NATS 服务端启用 JetStream；延迟消息还要求服务端支持消息调度。

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

err = facades.Queue().Producer(body, queue.ProducerOptions{
	Topic:  "basic",
	Queue:  "basic_email",
	Routes: []string{"email"},
})
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

err := facades.Queue().Consumer(handler, queue.ConsumerOptions{
	Topic: "basic",
	Queue: "basic_email",
	Route: "email",
	Retry: 3,
})
```

切换通道：

```go
rabbitmqReport, err := facades.Queue().Channel("rabbitmq", "report")
if err != nil {
	return err
}

natsEvents, err := facades.Queue().Channel("nats", "events")
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

type Handler func(data []byte) error

type Headers map[string]any

type ProducerOptions struct {
	Topic   string
	Queue   string
	Routes  []string
	Delay   time.Duration
	TTL     time.Duration
	Headers Headers
}

type ConsumerOptions struct {
	Topic   string
	Queue   string
	Route   string
	Delayed bool
	TTL     time.Duration
	Retry   int
}

type Driver interface {
	Producer(body []byte, options ProducerOptions) error
	Consumer(handler Handler, options ConsumerOptions) error
	Close() error
}
```

参数说明：

- `Topic`: 逻辑主题；RabbitMQ 映射为 exchange，NATS 映射为 subject 前缀。
- `Queue`: 队列或消费组；RabbitMQ 映射为 queue，NATS 映射为 queue group 和 durable consumer 名称来源。
- `Route` / `Routes`: 主题下的路由；NATS 会与 `Topic` 用点号拼成 subject。
- `Delay`: 延迟时长；RabbitMQ 使用 `x-delayed-message`，NATS 使用 JetStream 原生消息调度。
- `Delayed`: Consumer 是否订阅延迟队列。
- `TTL`: 消息存活时长。RabbitMQ 到期后走死信交换机；NATS 到期后直接删除消息，不做死信转发。
- `Retry`: 仅当 Consumer handler 返回非 `nil error` 时，重新发布回原队列的最大次数；处理成功不会重试。
- `Headers`: RabbitMQ 会原样传递；NATS 会把值转换为文本 header。

RabbitMQ 和 NATS 使用 `x-retry` header 记录已经完成的业务重试次数。只有 Consumer handler 返回非 `nil error` 才会进入重试流程；初次消费没有该 header，第一次处理失败后重新发布的消息为 `x-retry=1`。RabbitMQ 和 NATS JetStream 只有重新发布成功后才确认原消息；达到 `ConsumerOptions.Retry` 后不再重新发布，而是发送到错误队列。延迟消息重试时会通过内部 `x-delay` header 保留原延迟。core NATS 没有确认机制，重新发布失败时会记录错误并尝试发送错误消息。重新发布失败、错误消息发布失败或 Ack 失败属于基础设施异常，不增加 `x-retry`。

## NATS 映射

- `Topic` 和 `Route` 用点号拼成 subject，例如 `events` + `created` 会发布或订阅 `events.created`；如果 `Topic` 为空，`Route` 可直接传完整 subject。
- `Queue` 是消费者的 queue group；多个同组消费者中只有一个会收到每条消息。
- `Routes` 有多个值时，生产者会向每个 subject 各发布一次。
- `Delay > 0` 时，驱动通过 JetStream `WithScheduleAt` 在服务端持久化调度；对应 Consumer 需要把 `Delayed` 设为 `true`，使用 durable JetStream consumer 接收和确认消息。若同时设置 `TTL`，TTL 从消息实际投递后开始计算。
- Producer 的 `TTL > 0` 通过 JetStream per-message TTL 实现。消息到期后从流中删除，NATS 不提供 RabbitMQ 式的死信转发。
- Consumer 的 `TTL > 0` 通过 JetStream 流的 `MaxAge` 实现，并自动切换为 durable JetStream consumer。同一流上的 TTL 是共享配置，多个 Consumer 设置不同值时以最短 TTL 为准。
- 驱动会自动创建或扩展 `stream` 配置的流，默认名为 `FRAMEWORK_QUEUE`。调度控制 subject 使用 `schedule_prefix`；`TTL=0` 的延迟消息使用 `retention` 作为投递后的保留时间，默认 `1h`。
- `Delay=0` 且 `TTL=0` 的普通消息和消费者仍使用 core NATS；Consumer 设置 `Delayed` 或 `TTL` 时，`Queue` 不能为空。
- NATS 的业务重试次数只由 `x-retry` 判断，每次失败都会产生一条重新入队的新消息；JetStream 的服务端重投递只用于重新发布或错误消息发布失败等基础设施异常。
- 最终失败消息会发布到 `error` 配置的 subject，默认 `basic_error`。

## 注意事项

- 延迟、TTL、重试和 headers 统一通过 `ProducerOptions` / `ConsumerOptions` 配置。
- RabbitMQ 的 `Producer` 每次调用会创建 publisher；高频场景需要评估连接和 publisher 生命周期成本。
- 如果消费失败且超过重试次数，会把错误信息投递到 `error` 配置的 RabbitMQ 队列或 NATS subject，默认 `basic_error`。
- 如果 example 基础项目没有注册 `queue.ServiceProvider`，队列配置即使存在也不会实际初始化。
