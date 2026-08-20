# Queue 组件

`queue` 提供 RabbitMQ 和 NATS 队列封装。连接配置只描述消息服务器，具体的 topic、queue、routes、delay、ttl 等参数放在 `queue.queues.<key>`，业务代码只传队列 key。

## 配置

```yaml
queue:
  default: rabbit

  connections:
    rabbit:
      driver: rabbitmq
      host: 127.0.0.1
      port: 5672
      username: guest
      password: guest
      vhost: /
      error: basic_error

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

  queues:
    email:
      enable: true
      connection: rabbit
      topic: basic
      queue: basic_email
      routes: [email]
      delay: 0s
      ttl: 10m
      retry: [1m, 5m, 30m]
      concurrency: 10
      error_enable: true
      error: basic_error
      headers:
        source: framework

    audit:
      enable: true
      connection: events
      topic: events
      queue: audit_workers
      routes: [created, updated]
      delayed: false
      delay: 0s
      ttl: 1h
      retry: [1m, 5m]
      error_enable: true
      stream: FRAMEWORK_AUDIT
      schedule_prefix: _framework.queue.schedule
      retention: 2h
```

- `enable` 控制单个队列，缺省为 `true`。关闭后 `Producer` 和 `Consumer` 返回可通过 `errors.Is(err, queue.ErrDisabled)` 判断的错误。
- `connection` 指向 `queue.connections.<name>`；不设置时使用 `queue.default`。
- `error_enable` 控制消费者重试耗尽后是否发送失败消息，缺省为 `true`；设为 `false` 时仍按 handler 返回值应答原消息。
- `connections` 只保存服务器连接参数。RabbitMQ/NATS 专属队列字段从 `queue.queues.<key>` 读取。
- 时长字段支持 Go duration，例如 `500ms`、`30s`、`10m`、`1h`；纯数字按秒解释。
- 服务连接按首次使用惰性创建；关闭的队列不会触发连接。

### RabbitMQ 队列字段

- `topic`: exchange，缺省为 `queue`。
- `queue`: 队列名，缺省为队列 key。
- `routes`: routing key 列表；也可以使用单个 `route`。缺省为队列名。
- `delay`: 首次生产延迟；大于 `0` 时业务 exchange 使用 `x-delayed-message`。与消费失败重试无关。
- `delayed`: 强制消费者使用延迟 exchange；通常配置了 `delay` 后无需单独设置。
- `ttl`: 消息 TTL，并配置死信 exchange。语义是过期/死信，不是重试等待。
- `retry`: 消费失败后的阶梯重试。数组长度是最大重试次数，每一项是对应次数的等待时间，例如 `[1m, 5m, 30m]` 表示最多重试 3 次，间隔 1 分钟 / 5 分钟 / 30 分钟。兼容旧写法 `retry: 3`（重试 3 次、间隔 0，立刻再投）。`retry: false` 显式关闭重试（不重投，失败走 error 流程）。
- `concurrency`: 消费并发数，默认 `10`。
- `error`: 最终失败消息队列，依次回退到连接级 `error` 和 `basic_error`。
- `headers`: 每次生产默认附带的消息头；调用 `Producer` 时传入的 header 会覆盖同名配置。
- `publisher_options`、`publish_options`、`consumer_options`: Go 配置中可传对应的 `go-rabbitmq` option 函数；连接级默认值仍放在 `queue.connections.<name>.default`。

阶梯重试（`retry` 中存在大于 `0` 的等待）通过共享 delay exchange `_framework.queue.delay`（`x-delayed-message`）回投业务队列，**不会**把业务 `topic` 改成 delayed 类型。成功路径仍走业务 exchange。需要 RabbitMQ 安装 delayed message 插件。若配置了 `ttl`，请保证它不会短于最大重试等待，否则消息可能在延迟等待中过期。

### NATS 队列字段

- `topic` 与 `routes` 使用点号组成 subject。例如 `events` + `created` 为 `events.created`。
- `queue`: queue group，同时用于 durable consumer 名称；缺省为队列 key。
- `routes`: subject 后缀列表；也可以使用单个 `route`。没有 route 时直接使用 `topic`，没有 topic 时退回 `queue`。
- `delay` / `delayed`、`ttl`: 启用 JetStream 延迟、消息 TTL 或消费 TTL。`delay` 只影响首次生产。
- `retry`: 与 RabbitMQ 相同，数组长度 = 最大重试次数，元素为阶梯等待。兼容 `retry: 3`；`retry: false` 关闭重试。
- `error`: 最终失败 subject，依次回退到连接级 `error` 和 `basic_error`。
- `stream`、`schedule_prefix`、`retention`: 可按队列覆盖连接级 JetStream 参数。
- `headers`: 默认 NATS headers；运行时 header 会覆盖同名配置。

NATS 延迟消息要求服务端支持 JetStream 消息调度。`delay=0` 且 `ttl=0` 的普通队列使用 core NATS；设置 `delayed` 或 `ttl` 时使用 durable JetStream consumer。消费失败后的阶梯重试若等待大于 `0`，会通过 JetStream schedule 回投业务 subject（即使该队列平时走 core 消费）。

## 使用

发送消息只需要队列 key：

```go
body, err := json.Marshal(map[string]any{
	"user_id": 123,
	"action":  "send_email",
})
if err != nil {
	return err
}

err = facades.Queue().Producer(body, "email", queue.Headers{
	"trace-id": "request-1",
})
```

消费消息：

```go
err := facades.Queue().Consumer(func(data []byte) (any, error) {
	var message map[string]any
	if err := json.Unmarshal(data, &message); err != nil {
		return nil, err
	}

	return nil, nil
}, "email")
```

通过 `consumer` 命令统一启动消费者时，在 `queue.consumes` 中注册 `queue.Consumer`：

```go
type EmailConsumer struct{}

func (*EmailConsumer) Key() string {
	return "email"
}

func (*EmailConsumer) Prepare() error {
	return nil
}

func (*EmailConsumer) Handle(data []byte) (any, error) {
	return nil, nil
}

facades.Config().Set("queue.consumes", []queue.Consumer{
	&EmailConsumer{},
})
```

这里的 `queue.Consumer` 来自框架根 `queue` 包；也可以直接使用 `contracts/queue.Consumer`。

命令会根据 `Key()` 自动读取 `queue.queues.<key>.connection`，未配置时使用 `queue.default`；连接成功后先执行 `Prepare()`，再在协程中启动 `Handle()` 消费，并在退出时关闭连接。

handler 的第一个返回值是驱动应答码，`nil` 表示默认应答：RabbitMQ 默认为 `rabbitmq.Ack`，NATS JetStream 默认为 `nats.Ack`。

RabbitMQ 可返回 `github.com/herhe-com/framework/queue/rabbitmq` 导出的 `Ack`、`NackDiscard` 或 `NackRequeue`：

```go
return rabbitmq.NackRequeue, nil
```

NATS JetStream 可返回 `nats.Ack`、`nats.Nak` 或 `nats.Term`。core NATS 沉默忽略应答码，因为协议本身没有消费确认。

handler 返回非 `nil error` 时进入配置的重试流程：读取消息头 `x-retry`（已完成次数），若仍小于 `len(retry)`，则等待 `retry[x-retry]` 后重新投递并把 `x-retry` 加 1；次数耗尽后按 `error_enable` 写入失败队列。应答码在重新发布或错误消息发布成功后决定原消息如何确认。重新发布/错误消息发布失败时驱动会优先要求服务端重投。

## 接口

```go
type Queue interface {
	Driver
	Channel(name string) (Driver, error)
}

type Handler func(data []byte) (response any, err error)

type Consumer interface {
	Key() string
	Prepare() error
	Handle(data []byte) (response any, err error)
}

type Headers map[string]any

type Driver interface {
	Producer(body []byte, key string, headers ...Headers) error
	Consumer(handler Handler, key string) error
	Close() error
}
```

`Channel(name)` 仍可显式取得某个连接驱动，但驱动的 `Producer` / `Consumer` 同样接收 `queue.queues.<key>`，且会校验该 key 的 `connection` 是否与当前驱动一致。
