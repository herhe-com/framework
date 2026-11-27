# AI 组件

基于 [CloudWeGo eino](https://github.com/cloudwego/eino) 的多提供商 AI/LLM 集成层。

## 功能特性

- 不自己维护厂商协议：请求体、鉴权、SSE 解析、结构化输出全部由 eino 的 model 组件承担，框架只做配置读取与契约映射
- 按「渠道」配置：`ai.channels.<name>.driver` 决定协议，同一套代码可同时接入多家
- 统一契约（`contracts/ai`）：对话、流式、结构化输出在一处定义，渠道之间可无缝切换
- 流式调用内置空闲超时与总时长上限（`ai.StreamOf`），不靠 HTTP 客户端超时兜底

## 支持的驱动

| driver | 说明 |
| --- | --- |
| `openai` | OpenAI 及 `/chat/completions` 兼容服务（DeepSeek、通义千问、智谱、vLLM、各类网关） |
| `ollama` | Ollama 本地或自建服务 |
| `anthropic` | Anthropic Claude |
| `gemini` | Google Gemini |
| `responses` | OpenAI Responses API：eino 没有对应组件，暂未实现（构造渠道时报错） |

其余兼容 OpenAI 协议的服务不再单独建驱动，统一用 `openai` + `base_url` 接入。

## 使用方法

### 基础配置

渠道名由自己取，`ai.default` 指向默认使用的那一个：

```yaml
ai:
  default: openai
  channels:
    openai:
      driver: openai
      api_key: your-api-key
      base_url: https://api.openai.com/v1 # 兼容网关改这里
      model: gpt-4o-mini
      temperature: 0.7
    claude:
      driver: anthropic
      api_key: your-anthropic-key
      model: claude-sonnet-4-5
      max_tokens: 4096
```

通用字段：`model`、`prefix`（拼在首条消息最前的系统提示词前缀）、`timeout`（非流式调用的期限，秒；流式不受它约束）、
`temperature`、`top_p`、`max_tokens`、`reasoning_effort`（`openai`/`ollama` 渠道配置生效，`openai` 还支持按请求覆盖）、`response_format`（仅 `openai`/`ollama` 支持）。
`api_key`/`base_url` 用于 `openai`、`anthropic`、`gemini`，`ollama` 用 `host`。
请求里没设置的模型参数用渠道配置补齐；`ai.default` 留空表示不启用 AI。

### 对话聊天

```go
import (
    contractai "github.com/herhe-com/framework/contracts/ai"
    "github.com/herhe-com/framework/facades"
)

response, err := facades.AI().Chat(c, &contractai.ChatRequest{
    Messages: []contractai.Message{
        {Role: contractai.RoleOfSystem, Content: "你是简洁的助手"},
        {Role: contractai.RoleOfUser, Content: "你好"},
    },
})
if err != nil {
    return err
}

fmt.Println(response.Choices[0].Message.Content)         // 正文
fmt.Println(response.Choices[0].Message.Reasoning)       // 推理模型的思考过程，非推理模型为空
fmt.Println(response.Usage.TotalTokens)                  // 用量
```

### 流式响应

`Driver.Stream` 返回分片通道，一般配合 `ai.StreamOf` 使用：超时、取消与流的释放都由它负责。

```go
limit := ai.StreamLimit{
    Idle:     5 * time.Minute,  // 连续这么久没有任何分片即判定卡死（连接建立阶段同样计入）
    Deadline: 10 * time.Minute, // 总时长上限，0 表示不限制
}

err := ai.StreamOf(c, facades.AI(), request, limit, func(item *ai.StreamResponse) error {
    for _, choice := range item.Choices {
        fmt.Print(choice.Delta.Content)   // 正文增量
        _ = choice.Delta.Reasoning        // 思考过程增量
    }
    // 返回错误立即中断流并原样透出，例如识别到模型在循环输出
    return nil
})

var expired *ai.StreamLimitError
if errors.As(err, &expired) {
    // expired.Idle 区分空闲超时（true）与总时长超限（false），expired.Limit 是对应口径
}
```

- 两个维度缺一不可：推理模型的思考过程可能很长，按总时长一刀切会误伤正常输出；只盯空闲又兜不住「一直慢速输出」。
- eino 的组件不提供流式超时语义：它只有 HTTP 客户端超时（`http.Client.Timeout`）这一个开关，而那个超时覆盖到「读完响应体」，会把长流直接掐断。框架因此**不给流式请求设 HTTP 超时**，生命周期只由 `ctx` 与 `StreamLimit` 决定；非流式调用才按渠道的 `timeout` 施加期限。
- 返回的错误优先级：超时中断 > `consume` 的错误 > 流内错误 > 驱动调用错误。
- 错误文案是英文的通用描述，面向用户的提示由调用方按 `StreamLimitError.Idle` 自行组织。

### 结构化输出

`response_format` 在渠道配置里声明（eino 的组件把它放在构造参数里，没有按请求覆盖的选项）：

```yaml
ai:
  default: openai
  channels:
    openai:
      driver: openai
      response_format: json_object # 只要求输出是合法 JSON 对象，字段仍由提示词约定
```

请求级的 `ChatRequest.ResponseFormat` 会被拒绝（`ai.ErrOfResponseFormat`），避免出现「以为下发了其实没下发」。
json_schema 目前未开放：需要把 schema 塞进配置，收益不如直接用提示词约束。

### 切换渠道

```go
driver, err := facades.AI().Channel("claude") // 渠道名，对应 ai.channels.<name>
if err != nil {
    return err
}

response, err := driver.Chat(c, request)
```

同一渠道只初始化一次；默认渠道在启动时按 `ai.default` 构造，配置缺失时 `facades.AI()` 为空。

## 核心类型

### ChatRequest

```go
type ChatRequest struct {
    Model           string
    Messages        []Message
    Temperature     float64         // 0 表示用渠道配置
    TopP            float64         // 0 表示用渠道配置
    MaxTokens       int             // 0 表示用渠道配置
    ReasoningEffort string          // minimal/low/medium/high；空表示用渠道配置（仅 openai 驱动支持按请求覆盖）
    ResponseFormat  *ResponseFormat // 置空即可，非空会被拒绝
}
```

### ChatResponse

```go
type ChatResponse struct {
    ID      string
    Model   string
    Choices []Choice   // 每条含 Message（Content/Reasoning）与 FinishReason
    Usage   Usage      // PromptTokens / CompletionTokens / ReasoningTokens / TotalTokens
}
```

### StreamResponse

```go
type StreamResponse struct {
    Model   string         // 请求模型名或渠道配置的默认模型名
    Choices []StreamChoice // 每片的 Delta.Content / Delta.Reasoning 与 FinishReason
    Done    bool           // 结束分片
    Error   error          // 流内错误
    Usage   *Usage         // 该片用量（一般只在末片回传）
}
```

## 接口定义

```go
type AI interface {
    Driver
    Channel(name string) (Driver, error)
}

type Driver interface {
    Chat(c context.Context, request *ChatRequest) (*ChatResponse, error)
    Stream(c context.Context, request *ChatRequest) (chan *StreamResponse, error)
    Dri() string   // 驱动名
    Model() string // 渠道配置的默认模型名
}
```

## 依赖项

- [CloudWeGo eino](https://github.com/cloudwego/eino) 与 eino-ext 的 model 组件（openai / ollama / claude / gemini）
- Config facade（配置管理）

## 文件结构

```
ai/
├── application.go      # AI 服务主应用：默认渠道与按需渠道
├── channel.go          # 渠道构造：配置 → 各厂商的 eino 组件
├── config.go           # ai.channels.<name> 配置读取
├── adapter.go          # eino 模型 → contracts/ai 契约的映射
├── constants.go        # 驱动名常量
└── provider.go         # 服务提供者
```
