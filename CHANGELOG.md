# Changelog

## Unreleased

### ⚠️ Breaking Changes

- **ai**: 配置结构由「厂商 → 渠道」改为「渠道 → 驱动」：`ai.driver` + `ai.<driver>.<name>.*` → `ai.default`（默认渠道名）+ `ai.channels.<name>.driver` + `ai.channels.<name>.*`。迁移示例：`ai.driver: openai`、`ai.openai.default.api_key` → `ai.default: openai`、`ai.channels.openai.driver: openai`、`ai.channels.openai.api_key`。`ai.default` 留空即不启用 AI
- **ai**: `contracts/ai.Driver` 收敛为 `Chat`/`Stream`/`Dri`/`Model`：移除 `Embedding`/`Models` 两个方法与 `ModelDefaults`（默认参数由各渠道注入 eino 组件）；`AI.Channel(driver, name)` 改为 `Channel(name)`（渠道名直接寻址）；驱动常量改为 `DriverOpenAI`/`DriverOllama`/`DriverResponses`/`DriverAnthropic`/`DriverGemini`（`DriverClaude` → `DriverAnthropic`，`DriverQianwen`/`DriverZhipu`/`DriverDeepSeek` 删除）
- **ai**: `ChatRequest.ResponseFormat` 不再按请求下发：eino 的组件把 `response_format` 放在构造参数里，改为渠道配置 `response_format`（当前只支持 `json_object`，仅 `openai`/`ollama` 支持），按请求设置会返回 `ai.ErrOfResponseFormat`
- **ai**: 渠道初始化时显式校验必填凭证：`openai`/`anthropic`/`gemini` 要求配置 `api_key`，`ollama` 要求配置 `host`；此前依赖「缺配置也能启动」的部署会在初始化时收到错误
- **queue**: 延迟消息（RabbitMQ ttl 模式）是 at-most-once：等待队列被意外删除时消息会被 broker 退回并丢弃，`Producer` 已返回成功。需要确认投递请改用 `delay_mode: plugin`。

### 🚀 Features

- **ai**: 用量透出：`Usage` 增加 `ReasoningTokens`（推理模型的思考 token，取 eino 的 `CompletionTokensDetails`），`StreamResponse` 增加 `Usage *Usage`（openai 驱动已默认 `include_usage`，一般只在末片回传）；`ChatResponse.Usage` 一并补齐推理 token
- **ai**: 流式分片透出 `finish_reason`（`StreamChoice.FinishReason`，取 eino 消息的 `ResponseMeta.FinishReason`），调用方可据此识别「输出被 `max_tokens` 截断」（`length`）并放大输出预算重试，而不是用同样的预算再截断一次
- **ai**: 驱动改为基于 CloudWeGo eino 与 eino-ext 的 model 组件实现，不再手写各家的 HTTP/SSE：`openai` 驱动同时覆盖 DeepSeek/千问/智谱等兼容网关，新增 `anthropic`（Claude）与 `gemini`（Gemini）驱动；`responses`（OpenAI Responses API）保留驱动名但暂未实现，eino 没有对应组件
- **ai**: 每个渠道把配置的默认模型参数（model/temperature/top_p/max_tokens/reasoning_effort/response_format）注入组件，请求级参数走 eino 的 `model.Option`（`openai` 驱动支持按请求覆盖 `ReasoningEffort`，其余驱动按请求传入时返回明确错误）；契约与 eino 消息之间的转换集中在 `ai/adapter.go`（含多模态内容块映射与思考过程 `ReasoningContent`）
- **ai**: 新增 `ai.StreamOf` 与 `ai.StreamLimit`/`ai.StreamLimitError`：流式调用的空闲看门狗与总时长上限收敛到框架（HTTP 客户端的 `timeout` 只约束到响应头，读正文时要自己看门），调用方只消费分片、不再自起守护 goroutine；`consume` 返回错误立即中断并原样透出（可识别模型循环输出），中断原因用 `errors.As` 取 `*ai.StreamLimitError` 区分空闲超时与总时长超限
- **ai**: 消息支持多模态内容块 `text`/`image_url`/`file`（`Message.Parts`），`content` 序列化与解析兼容字符串与数组两种格式；新增 `TextPart`/`ImagePart`/`FilePart`/`DataUrl`/`ApplyPrefix` 与 `RoleOf*` 常量，并提供 `ResponseFormatOfJSON`/`ResponseFormatOfSchema`、`ChatRequest.TopP`/`ReasoningEffort` 与 `ReasoningOf*`/`ResponseOf*` 常量定义
- **ai**: `Driver` 接口请求方法均首参 `context.Context`，`StreamResponse` 解析服务端流内错误对象并暴露 `Error`，流式生产者在消费端退出或请求取消时安全终止
- **swagger**: Swagger UI 支持深色模式，右下角按钮在「系统 → 深色 → 浅色」间轮换；系统模式清除本地缓存并跟随 `prefers-color-scheme` 实时切换，手选深色/浅色写入 `localStorage`；UI 处理器改由 `http/swagger` 提供
- **queue**: RabbitMQ 延迟与阶梯重试兼容 4.2 及以下与 4.3+：新增 `queue.connections.<name>.delay_mode`（`auto`/`plugin`/`ttl`），默认按 broker 能力自动探测 `x-delayed-message` 插件；有插件时保持旧行为（业务 exchange 声明为 `x-delayed-message`，重试走共享延迟 exchange），无插件时改为 TTL 等待队列 + 死信回投，业务 exchange 类型保持不变，`delayed` 配置被忽略，`delayed` 与 `ttl` 不再互斥
- **queue**: NATS 连接与消费启动时打印 `[queue.nats]` 运行信息：连接地址（经 `ConnectedUrlRedacted()` 脱敏）与服务端版本、core/JetStream 订阅的 subject、queue group、stream、durable consumer、`ack_wait`、`max_ack_pending`，连接关闭时也打印一行

### 🐛 Bug Fixes

- **ai**: 渠道的 `timeout`（默认 300 秒）不再传给 eino 组件的 HTTP/请求超时：`openai`/`ollama`/`anthropic`/`gemini` 流式请求均不设底层超时（包括不再向 `anthropic` 组件传递 `RequestTimeout`），生命周期只由 `ctx` 与 `ai.StreamLimit` 控制；非流式仍在 `adapter.Chat` 里按 `timeout` 用 `ctx` 施加期限
- **ai**: 流内错误为字符串时（如 ollama 的 `{"error": "..."}`）不再把 JSON 引号带进错误文本；`adapter.prepare` 不再原地改写 `request.Messages`，避免重试时重复拼接 `prefix`；多模态消息传入 eino 时清空已回填的 `Content`，避免与 `UserInputMultiContent` 冲突
- **queue**: RabbitMQ 延迟模式探测只在 broker 不认 `x-delayed-message` 类型（4.3+ 的 406 "unknown exchange type"、旧版本的 503/540）时回退 TTL，网络、权限、参数冲突等其他失败照常返回错误，不再把整个连接的延迟路径静默锁死
- **queue**: NATS 延迟消息要求 nats-server v2.12.0+：消费启动与首条延迟投递前先比对服务端版本（`nats: server version "2.11.6" is too low ...`），低版本直接失败、不创建流；建流后再回读 `allow_msg_schedules` 兜底，不再静默接受调度开关导致消息永不投递
- **queue**: 新增 `contracts/queue.CallHandler`（`ErrHandlerTimeout`、`DefaultHandlerTimeout` 10 分钟），RabbitMQ 与 NATS 共用同一套 handler 超时语义
- **queue**: RabbitMQ 消费新增 `handler_timeout`（队列级，连接级 `queue.connections.<name>.handler_timeout` 兜底），缺省 `10m`、`0` 不限制；超时按消费失败处理、回归 `retry` 阶梯并释放 worker，避免卡死的 handler 永久占住消费者
- **queue**: NATS 消费新增 `handler_timeout`（队列级，连接级 `queue.connections.<name>.handler_timeout` 兜底），缺省 `10m`、`0` 不限制；handler 超时按消费失败处理并回归 `retry` 阶梯，次数用尽进 `error` subject，避免卡死的 handler 永久占住消息
- **queue**: NATS 消费新增 `ack_wait`（缺省 `30s`）与 `max_ack_pending`（缺省 `1`）配置：处理期间按 `ack_wait / 3` 发送 in-progress 心跳，处理耗时超过确认窗口不再被重复投递；未确认消息上限默认 1，避免串行消费下预取的消息排队过期后被重复处理
- **queue**: NATS 带正等待阶梯重试的队列改用 JetStream durable consumer（调度消息只对 JetStream 消费者可见，core 订阅收不到）；延迟 `@at` 改由驱动写头、精确到纳秒，不再被 nats.go 的 RFC3339 截断到整秒
- **queue**: RabbitMQ exchange 与队列默认声明为 `durable`（`exchange_durable`、`durable` 缺省 `true`），避免对已存在的持久化拓扑重新声明为非持久化时报 406 `PRECONDITION_FAILED`；连接级 `durable` 仍是队列级配置的兜底

### 🔨 Refactor

- **queue**: RabbitMQ 驱动改用 `github.com/rabbitmq/amqp091-go` 原生实现，移除 `github.com/wagslane/go-rabbitmq`；启用 amqp091 自动恢复（重开 channel、重声明拓扑、用同一交付通道重订阅消费者）并在其耗尽后用指数退避重建连接；新增 `heartbeat`/`reconnect`/`reconnect_retries`/`prefetch`/`durable`/`persistent`/`exchange_kind`/`exchange_durable`/`exchange_args`/`queue_args` 配置，删除 `publisher_options`/`publish_options`/`consumer_options`（原为 go-rabbitmq 的 option 函数）
- **ai**: 原 `ai/openai`、`ai/ollama` 两个手写客户端删除，改由 `ai/channel.go`（配置 → eino 组件）与 `ai/adapter.go`（eino → 契约）承担，新增一家厂商只需加一个构造分支
- **ai**: 消息内容解析收敛为内部 `decodeContent`，流式发送收敛为 `ai.Send`
- **queue**: RabbitMQ 队列延迟实现由 `delayMode` 单一状态承载，`queueOptions` 的 `plugin`/`delayed` 布尔与 `publisherKey` 的 `ttl`/`plugin` 维度删除，延迟 header 组装收敛为 `delayHeaders`

### ⚙️ Miscellaneous

- 升级全部依赖模块至最新版本（aws-sdk-go-v2、kitex、carbon、goose、nats.go、minio-go、qiniu、go-redis、amqp091-go 等）；`github.com/mfridman/interpolate` 仍锁定在 `v0.0.2`，goose v3.30.0 尚未适配 `v0.2.0` 的 API

## v1.11.6 (2026-10-22)

### 🚀 Features

- **filesystem**: 新增 `TemporaryUpload` 直接上传凭证与 `InitiateMultipartUpload`/`UploadMultipartPart`/`CompleteMultipartUpload`/`AbortMultipartUpload` 分片上传生命周期，s3、oss、cos、minio、qiniu 驱动全部实现 ([458df5e](https://github.com/herhe-com/framework/commit/458df5e))
- **captcha**: 新增场景化配置与中间件 ([27fe8d8](https://github.com/herhe-com/framework/commit/27fe8d8))
- **console**: Swagger UI 展示扩展字段 `showExtensions` ([9ea41b4](https://github.com/herhe-com/framework/commit/9ea41b4))

### 🐛 Bug Fixes

- **captcha**: 拒绝非法资源与滑块输入 ([33b1f88](https://github.com/herhe-com/framework/commit/33b1f88))
- **util**: 验证码数字改用加密安全随机数生成 ([bd54a4c](https://github.com/herhe-com/framework/commit/bd54a4c))

### 🔨 Refactor

- **console**: Swagger UI 迁移到 swaggo/http-swagger 并通过 HTTP adaptor 挂载 ([d06ee16](https://github.com/herhe-com/framework/commit/d06ee16))

### ⚙️ Miscellaneous

- 升级全部依赖模块至最新版本 ([48c60f3](https://github.com/herhe-com/framework/commit/48c60f3))

**完整变更**: https://github.com/herhe-com/framework/compare/v1.11.5...v1.11.6

## v1.11.5 (2026-09-21)

### 🐛 Bug Fixes

- **queue/rabbitmq**: 复用缓存的 Publisher 并安全关闭，`publish` 不再为每条消息重复创建 Publisher，关闭状态改用 `atomic.Bool` 保证并发安全，关闭后的发布/消费返回 `ErrClosed` ([87b63bc](https://github.com/herhe-com/framework/commit/87b63bc))
- **queue/rabbitmq**: 拒绝同时启用 `delayed` 与 `ttl` 选项的队列配置，启动时校验并报错 ([80e69a8](https://github.com/herhe-com/framework/commit/80e69a8))

### 📝 补充说明

- RabbitMQ 驱动新增 `publisherKey` 缓存机制，Publisher 构造选项改为惰性构建（仅缓存未命中时创建），显著降低连接与资源开销
- 新增 `ErrClosed` 错误变量，用于驱动关闭后的操作防护

**完整变更**: https://github.com/herhe-com/framework/compare/v1.11.4...v1.11.5
