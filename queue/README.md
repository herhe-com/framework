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
      # 心跳与重连：心跳用于发现半开 TCP；reconnect 既是客户端自愈的重试间隔，
      # 也是驱动重建连接时的退避基准
      heartbeat: 10s
      reconnect: 3s
      reconnect_retries: 10
      # auto: 探测 broker 是否支持 delayed message 插件；也可是 plugin / ttl
      delay_mode: auto

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
- `delay`: 首次生产延迟。与消费失败重试无关；具体实现由 `delay_mode` 决定。
- `delayed`: 旧配置，用于强制声明 `x-delayed-message` exchange；现在由 broker 能力自动判断，此字段被忽略。
- `ttl`: 消息 TTL，并配置死信 exchange。语义是过期/死信，不是重试等待。
- `retry`: 消费失败后的阶梯重试。数组长度是最大重试次数，每一项是对应次数的等待时间，例如 `[1m, 5m, 30m]` 表示最多重试 3 次，间隔 1 分钟 / 5 分钟 / 30 分钟。兼容旧写法 `retry: 3`（重试 3 次、间隔 0，立刻再投）。`retry: false` 显式关闭重试（不重投，失败走 error 流程）。
- `concurrency`: 消费并发数，默认 `10`。
- `prefetch`: 每消费者未确认消息上限，缺省等于 `concurrency`。
- `durable`: 队列是否持久化，缺省 `true`；与 exchange 一样，队列创建后无法修改该标记，非持久化重声明会遇到 `406 PRECONDITION_FAILED`。
- `exchange_kind`: exchange 类型，缺省 `direct`；`delay > 0` 且使用插件时固定为 `x-delayed-message`。
- `exchange_durable`: exchange 是否持久化，缺省 `true`；RabbitMQ 不允许修改已存在 exchange 的该标记，声明成 `false` 会遇到 `406 PRECONDITION_FAILED`。
- `exchange_args`、`queue_args`: 追加的 exchange / 队列声明参数（map），可用于 `x-max-length`、`x-queue-type` 等。
- `persistent`: 消息是否持久化投递，缺省 `false`（等同 `transient`）。
- `error`: 最终失败消息队列，依次回退到连接级 `error` 和 `basic_error`。
- `handler_timeout`: 单次 handler 的最大处理时间，缺省 `10m`，`0` 表示不限制。超时按消费失败处理：进 `retry` 阶梯重投，次数用尽转 `error` 队列（`error_enable: false` 时丢弃），并释放被占住的 worker。超时只能让消息回归队列，Go 无法强杀已在跑的 goroutine，handler 内部仍应设置自己的 context 超时。
- `headers`: 每次生产默认附带的消息头；调用 `Producer` 时传入的 header 会覆盖同名配置。

以上字段同时支持连接级缺省值：`queue.connections.<name>` 下的 `durable`、`exchange_kind`、`exchange_args`、`persistent`、`handler_timeout` 会作为同级队列字段的兜底。

### 连接与断线重连

驱动直接使用 `github.com/rabbitmq/amqp091-go`，不再依赖 `go-rabbitmq`。断线分两层处理：

1. **客户端自愈（毫秒~秒级抖动）**：启用 `amqp.Recovery`，由 amqp091 自动重连并重开 channel、重新声明已声明的 exchange/队列/绑定，并用**同一个交付通道**重新订阅消费者。worker goroutine 不受影响，业务侧无感知。重试次数/间隔来自 `reconnect_retries` / `reconnect`（默认 10 次 × 3s）。
2. **驱动兜底（长时间不可用）**：客户端自愈耗尽后会关闭连接，驱动以 `reconnect` 为基准做指数退避（上限 16 倍，±25% 抖动）重新拨号，成功后重建 publisher channel、延迟等待队列声明和消费者（先等上一代 worker 退出再订阅，避免重复消费）。

其他要点：

- 消费者使用 `basic.qos` + 固定数量 worker（`concurrency`），handler 返回值决定 `ack` / `nack(requeue)` / `nack(discard)`；channel 关闭时未确认消息由 broker 自动重投，语义为 at-least-once。
- 断线期间的 `Producer` 返回 `ErrDisconnected`（重连中）或 `ErrClosed`（已关闭）；消费失败重试路径遇到这两类错误会返回 `nack(requeue)`，等连接恢复后由 broker 重投。
- `Close()` 会先停消费者（等待在处理的 handler 结束），再关闭 publisher channel 和连接。

本地断线演练：

```bash
docker run -d --name rabbitmq-drill -p 5672:5672 rabbitmq:4.4
# 启动业务进程后：
docker pause rabbitmq-drill     # 短断线：日志出现 interrupted → recovered，消费者不重启
docker unpause rabbitmq-drill
docker stop rabbitmq-drill      # 长断线：客户端自愈耗尽 → 日志出现 rebuilding
docker start rabbitmq-drill     # 驱动重连后自动重建 publisher/消费者
```

### 延迟实现（插件 / TTL 自动切换）

延迟有两种实现，驱动默认自动选择，业务代码和队列配置都不用改：

| 模式 | 启用条件 | 延迟载体 |
| --- | --- | --- |
| `plugin` | broker 支持 `x-delayed-message`（RabbitMQ 4.2 及以下 + `rabbitmq_delayed_message_exchange` 插件） | 共享 exchange `_framework.queue.delay` / 业务 exchange 本身 |
| `ttl` | broker 不支持插件（典型是 4.3+，或未安装插件） | 每个延迟一条 TTL 等待队列 + 死信回投 |

`queue.connections.<name>.delay_mode` 可选 `auto`（缺省，自动探测）、`plugin`、`ttl`。

自动探测不是解析版本号，而是**实测能力**：启动后第一次遇到需要延迟的队列（`delay > 0` 或 `retry` 含正数等待）时，在独立的声明 channel 上尝试声明 `x-delayed-message` exchange：成功即 `plugin`，被 broker 以未知 exchange 类型拒绝即 `ttl`。结果按连接缓存并只探测一次，同时打印日志：

```text
[queue.rabbitmq] rabbitmq 4.4.0: delay mode ttl (dead-letter)
```

之所以不用版本号判断：插件是社区维护的，4.2 也可能没装，重新编译的 4.3 也可能有。

#### plugin 模式（4.2 及以下，保持旧行为）

- `delay > 0` 时业务 exchange 本身声明为 `x-delayed-message`（`x-delayed-type=direct`），消息带 `x-delay` 头直接投递，由 broker 延时后路由。
- 阶梯重试走共享 exchange `_framework.queue.delay`，消费者会把自己的队列按 `routes` 绑定上去，业务 exchange 保持原类型。

#### ttl 模式（4.3+，无需插件）

- 每个 `(topic, route, 延迟)` 组合对应一条等待队列 `_framework.queue.delay.<毫秒>.<topic>.<route>`，声明为 `durable` 且带 `x-message-ttl`。
- 生产时消息通过默认 exchange 写入等待队列；TTL 到期后由 `x-dead-letter-exchange` + `x-dead-letter-routing-key` 回投业务 `topic`，消息头与 `x-retry` 原样保留。
- 等待队列在首次生产时自动声明，名称包含毫秒值以保证同一队列内消息的 TTL 完全一致（避免队头阻塞）。名字含非法字符或超长时退化为哈希后缀，防止不同路由目标共用一条参数冲突的队列。
- 等待队列用独立的声明 channel 创建，声明结果按进程缓存；broker 重启或队列被删除时，`mandatory` 发布会被退回并打日志，同时让缓存的声明失效，下次发布自动重建。

ttl 模式下阶梯重试（`retry` 中存在大于 `0` 的等待）走同一套等待队列，按 `x-retry` 选择对应等待时长后回投业务 `topic`，成功路径仍直接走业务 exchange。由于延迟期间消息不在业务队列中，`ttl` 不再受最大重试等待时长约束。

注意：延迟档位来自配置里的 `delay` / `retry`，ttl 模式下每个不同的等待时长会占用一条等待队列；请避免配置大量互不相同的等待时长。延迟消息在等待队列中的持久化沿用队列的 `persistent`（缺省 transient，broker 重启会丢失），与普通投递一致；长延迟场景请把 `persistent` 设为 `true`。

**投递语义（at-most-once）**：ttl 模式把消息发布到等待队列（mandatory publish），`Producer` 在 broker 确认路由前就已返回成功。若等待队列被意外删除、消息被退回，驱动只打印日志并让下一次发布重建队列，**这一条消息就丢了，调用方不会收到错误**。需要确认投递的场景请使用 `delay_mode: plugin`，或由调用方自行校验拓扑。

升级提示：从 4.2 及以下升级到 4.3+ 时插件无法加载，旧集群里已经存在的 `x-delayed-message` 类型 exchange（包括旧版为 `delay` 队列转换的业务 exchange）无法再被声明，需要在升级前清空这些 exchange 并删除它们，驱动随后会按 ttl 模式重建普通 exchange。

### NATS 队列字段

- `topic` 与 `routes` 使用点号组成 subject。例如 `events` + `created` 为 `events.created`。
- `queue`: queue group，同时用于 durable consumer 名称；缺省为队列 key。
- `routes`: subject 后缀列表；也可以使用单个 `route`。没有 route 时直接使用 `topic`，没有 topic 时退回 `queue`。
- `delay` / `delayed`、`ttl`: 启用 JetStream 延迟、消息 TTL 或消费 TTL。`delay` 只影响首次生产。
- `retry`: 与 RabbitMQ 相同，数组长度 = 最大重试次数，元素为阶梯等待。兼容 `retry: 3`；`retry: false` 关闭重试。
- `error`: 最终失败 subject，依次回退到连接级 `error` 和 `basic_error`。
- `stream`、`schedule_prefix`、`retention`、`ack_wait`、`max_ack_pending`、`handler_timeout`: 可按队列覆盖连接级（`queue.connections.<name>`）同名参数。
- `ack_wait`: 单条消息的确认窗口，缺省 `30s`；消费期间驱动会按 `ack_wait / 3` 持续发送 in-progress 心跳，因此处理耗时超过 `ack_wait` 也不会被重复投递。
- `max_ack_pending`: 单个 durable consumer 的未确认消息上限，缺省 `1`。NATS 驱动是串行消费（每个 route 一个 consumer），`concurrency` 对它不生效；调大该值只是提前把消息拉到本地，超过 `ack_wait` 未被处理仍会被重投，长耗时处理建议保持 `1`。
- `handler_timeout`: 单次 handler 的最大处理时间，缺省 `10m`，`0` 表示不限制。超时按消费失败处理：进 `retry` 阶梯重投，次数用尽转 `error` subject（`error_enable: false` 时丢弃）。超时只能让消息回归队列，Go 无法强杀已在跑的 goroutine，handler 内部仍应设置自己的 context 超时。
- `headers`: 默认 NATS headers；运行时 header 会覆盖同名配置。

NATS 延迟消息依赖 JetStream 的消息调度（`allow_msg_schedules`），要求 **nats-server v2.12.0 及以上**。更低版本会接受创建流请求但丢弃该开关，延迟消息随后静默留在调度 subject 上永不投递，所以驱动在消费启动（以及首条延迟投递）时先比对服务端版本，不满足就直接失败、不创建任何流：

```text
nats: server version "2.11.6" is too low for delayed queues (message schedules); nats-server v2.12.0 or later is required
```

没有延迟、TTL 与正等待重试的队列不受版本限制。`delay=0`、`ttl=0` 且没有正等待重试的普通队列使用 core NATS；设置 `delay` / `delayed` / `ttl`，或 `retry` 中存在大于 `0` 的等待时，改用 durable JetStream consumer。

调度投递只对 JetStream 消费者可见，core NATS 订阅者收不到，所以带阶梯重试的队列必须走 JetStream 消费；驱动已按此自动切换，不需要额外配置。延迟时间精确到纳秒（驱动自己写 `@at` 头，nats.go 的 `WithScheduleAt` 会截断到整秒，最多少投递约 1 秒）。

**升级检查项**：手写 `@at` 依赖 `jetstream.ScheduleHeader` 常量与 nats-server 对 `@at` 的 `time.RFC3339Nano` 解析。升级 `github.com/nats-io/nats.go` 或 nats-server 时，请回归验证 `publishDelayed` 的延迟精度；若上游 `WithScheduleAt` 改为保留纳秒，可直接删掉手写 header。

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
