# AI 组件

多提供商 AI/LLM 集成层，支持多种大语言模型服务。

## 功能特性

- 多驱动支持（OpenAI、Ollama 等）
- 统一的 API 接口
- 对话聊天功能
- 流式响应支持
- 文本嵌入（Embedding）
- 模型管理

## 支持的驱动

- **OpenAI**: GPT 系列模型
- **Ollama**: 本地部署的开源模型
- 计划支持: Claude, Gemini, 千问, 智谱, DeepSeek

## 使用方法

### 基础配置

在配置文件中设置 AI 服务：

```yaml
ai:
  driver: openai
  openai:
    default:
      api_key: your-api-key
      model: gpt-4
  ollama:
    default:
      base_url: http://localhost:11434
      model: llama2
```

### 对话聊天

```go
import "github.com/herhe-com/framework/facades"

// 发送聊天请求
response, err := facades.AI.Chat(ai.ChatRequest{
    Model: "gpt-4",
    Messages: []ai.Message{
        {Role: "user", Content: "你好"},
    },
})
```

### 流式响应

```go
stream, err := facades.AI.ChatStream(ai.ChatRequest{
    Model: "gpt-4",
    Messages: []ai.Message{
        {Role: "user", Content: "讲个故事"},
    },
})

for chunk := range stream {
    fmt.Print(chunk.Content)
}
```

### 文本嵌入

```go
embeddings, err := facades.AI.Embedding(ai.EmbeddingRequest{
    Model: "text-embedding-ada-002",
    Input: []string{"文本内容"},
})
```

### 切换驱动

```go
// 使用指定的驱动
ollama := facades.AI.Driver("ollama")
response, err := ollama.Chat(request)
```

## 核心类型

### ChatRequest

```go
type ChatRequest struct {
    Model       string
    Messages    []Message
    Temperature float64
    MaxTokens   int
    Stream      bool
}
```

### ChatResponse

```go
type ChatResponse struct {
    ID      string
    Model   string
    Content string
    Usage   Usage
}
```

### EmbeddingRequest

```go
type EmbeddingRequest struct {
    Model string
    Input []string
}
```

## 接口定义

```go
type Driver interface {
    Chat(request ChatRequest) (*ChatResponse, error)
    ChatStream(request ChatRequest) (<-chan StreamResponse, error)
    Embedding(request EmbeddingRequest) ([][]float64, error)
    Models() ([]Model, error)
}
```

## 依赖项

- Config facade（配置管理）
- 各驱动的 SDK（OpenAI SDK、Ollama SDK 等）

## 文件结构

```
ai/
├── application.go      # AI 服务主应用
├── provider.go         # 服务提供者
├── openai/            # OpenAI 驱动实现
└── ollama/            # Ollama 驱动实现
```
