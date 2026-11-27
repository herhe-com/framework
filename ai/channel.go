package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	einoclaude "github.com/cloudwego/eino-ext/components/model/claude"
	einogemini "github.com/cloudwego/eino-ext/components/model/gemini"
	einoollama "github.com/cloudwego/eino-ext/components/model/ollama"
	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"google.golang.org/genai"

	contractai "github.com/herhe-com/framework/contracts/ai"
)

// NewChannel 按 ai.channels.<name> 构造一个渠道。
//
// 驱动只负责把配置映射到对应厂商的 eino 组件,协议细节(请求体、鉴权、SSE 解析)
// 全部由组件承担;返回的 Driver 再做一层契约转换。
func NewChannel(name string) (contractai.Driver, error) {

	return newChannelOf(configOfChannel(name))
}

// newChannelOf 按一份渠道配置构造渠道,与配置来源解耦,便于按场景直接验证。
func newChannelOf(config channelConfig) (contractai.Driver, error) {

	var (
		chat model.BaseChatModel
		err  error
	)

	switch config.Driver {
	case DriverOpenAI:
		chat, err = newOpenAI(config)
	case DriverOllama:
		chat, err = newOllama(config)
	case DriverAnthropic:
		chat, err = newAnthropic(config)
	case DriverGemini:
		chat, err = newGemini(config)
	case DriverResponses:
		return nil, fmt.Errorf("ai: 渠道 %s 的驱动 responses 暂未实现(eino 没有对应组件),请先用 openai 驱动接入", config.Name)
	case "":
		return nil, fmt.Errorf("ai: 渠道 %s 缺少 driver 配置", config.Name)
	default:
		return nil, fmt.Errorf("ai: 渠道 %s 的驱动 %s 不支持", config.Name, config.Driver)
	}

	if err != nil {
		return nil, fmt.Errorf("ai: 渠道 %s 初始化失败: %w", config.Name, err)
	}

	return &adapter{dri: config.Driver, chat: chat, modelName: config.Model, prefix: config.Prefix, timeout: config.Timeout}, nil
}

// newOpenAI 用 openai 驱动接入 OpenAI 与各类 /chat/completions 兼容网关。
func newOpenAI(config channelConfig) (model.BaseChatModel, error) {

	if strings.TrimSpace(config.APIKey) == "" {
		return nil, fmt.Errorf("api_key 不能为空")
	}

	format, err := responseFormatOf(config)
	if err != nil {
		return nil, err
	}

	return einoopenai.NewChatModel(context.Background(), &einoopenai.ChatModelConfig{
		APIKey:  config.APIKey,
		BaseURL: config.BaseURL,
		Model:   config.Model,
		// 组件的 Timeout 会变成 http.Client.Timeout，而它覆盖到「读完响应体」：
		// 推理模型的长流会被这个总时长直接掐断。流式的生命周期交给 ctx 与 ai.StreamLimit，
		// 非流式的超时由 adapter.Chat 用 ctx 施加，因此这里显式给一个不带超时的 client。
		HTTPClient:      &http.Client{},
		Temperature:     float32Of(config.Temperature),
		TopP:            float32Of(config.TopP),
		MaxTokens:       intOf(config.MaxTokens),
		ReasoningEffort: einoopenai.ReasoningEffortLevel(config.ReasoningEffort),
		ResponseFormat:  format,
	})
}

// responseFormatOf 把渠道配置里的 response_format 翻成 openai 组件的结构化输出约束。
// 只支持 json_object:json_schema 需要传结构体,交给业务侧用提示词约束更实际。
func responseFormatOf(config channelConfig) (*einoopenai.ChatCompletionResponseFormat, error) {

	switch strings.ToLower(strings.TrimSpace(config.ResponseFormat)) {
	case "":
		return nil, nil
	case "json_object":
		return &einoopenai.ChatCompletionResponseFormat{Type: einoopenai.ChatCompletionResponseFormatTypeJSONObject}, nil
	}

	return nil, fmt.Errorf("response_format 只支持 json_object,当前为 %s", config.ResponseFormat)
}

// newOllama 用 ollama 驱动接入本地或自建服务。
func newOllama(config channelConfig) (model.BaseChatModel, error) {

	if strings.TrimSpace(config.Host) == "" {
		return nil, fmt.Errorf("host 不能为空")
	}

	format, err := formatOfOllama(config)
	if err != nil {
		return nil, err
	}

	return einoollama.NewChatModel(context.Background(), &einoollama.ChatModelConfig{
		BaseURL: config.Host,
		Model:   config.Model,
		// 同 openai 驱动:不设 HTTP 超时,避免长流被 http.Client.Timeout 掐断。
		HTTPClient: &http.Client{},
		Options: &einoollama.Options{
			Temperature: float32(config.Temperature),
			TopP:        float32(config.TopP),
			NumPredict:  config.MaxTokens,
		},
		Thinking: thinkOf(config.ReasoningEffort),
		Format:   format,
	})
}

// thinkOf 把 reasoning_effort 翻成 ollama 的思考开关:
// true/false 直接用布尔,其余取值(如 low/medium/high)原样下发给支持等级划分的模型。
func thinkOf(effort string) *einoollama.ThinkValue {

	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "":
		return nil
	case "true":
		return &einoollama.ThinkValue{Value: true}
	case "false":
		return &einoollama.ThinkValue{Value: false}
	}

	return &einoollama.ThinkValue{Value: effort}
}

// formatOfOllama 把 response_format=json_object 映射成 ollama 的 json 输出格式。
func formatOfOllama(config channelConfig) (json.RawMessage, error) {

	switch strings.ToLower(strings.TrimSpace(config.ResponseFormat)) {
	case "":
		return nil, nil
	case "json_object":
		return json.RawMessage(`"json"`), nil
	}

	return nil, fmt.Errorf("response_format 只支持 json_object,当前为 %s", config.ResponseFormat)
}

// newAnthropic 用 anthropic 驱动接入 Anthropic Claude。
func newAnthropic(config channelConfig) (model.BaseChatModel, error) {

	if strings.TrimSpace(config.APIKey) == "" {
		return nil, fmt.Errorf("api_key 不能为空")
	}
	if strings.TrimSpace(config.ResponseFormat) != "" {
		return nil, fmt.Errorf("anthropic 驱动暂不支持 response_format 配置")
	}

	chat := &einoclaude.Config{
		APIKey:      config.APIKey,
		Model:       config.Model,
		MaxTokens:   config.MaxTokens,
		Temperature: float32Of(config.Temperature),
		TopP:        float32Of(config.TopP),
		// claude 组件会把 RequestTimeout 同时作用在 Stream 请求上,长流会被截断;
		// 非流式超时已由 adapter.Chat 通过 ctx 施加,这里使用无超时的 HTTP client。
		HTTPClient: &http.Client{},
	}

	if config.BaseURL != "" {
		baseURL := config.BaseURL
		chat.BaseURL = &baseURL
	}

	return einoclaude.NewChatModel(context.Background(), chat)
}

// newGemini 用 gemini 驱动接入 Google Gemini;组件需要调用方自己建 SDK 客户端。
func newGemini(config channelConfig) (model.BaseChatModel, error) {

	if strings.TrimSpace(config.APIKey) == "" {
		return nil, fmt.Errorf("api_key 不能为空")
	}
	if strings.TrimSpace(config.ResponseFormat) != "" {
		return nil, fmt.Errorf("gemini 驱动暂不支持 response_format 配置")
	}

	clientConfig := &genai.ClientConfig{
		APIKey:     config.APIKey,
		HTTPClient: &http.Client{},
	}
	if strings.TrimSpace(config.BaseURL) != "" {
		clientConfig.HTTPOptions = genai.HTTPOptions{BaseURL: config.BaseURL}
	}

	client, err := genai.NewClient(context.Background(), clientConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 Gemini 客户端失败: %w", err)
	}

	return einogemini.NewChatModel(context.Background(), &einogemini.Config{
		Client:      client,
		Model:       config.Model,
		MaxTokens:   intOf(config.MaxTokens),
		Temperature: float32Of(config.Temperature),
		TopP:        float32Of(config.TopP),
	})
}

// float32Of 把配置里的模型参数转成组件需要的指针:0 表示未设置,交给服务端默认。
func float32Of(value float64) *float32 {

	if value == 0 {
		return nil
	}

	result := float32(value)

	return &result
}

// intOf 把配置里的整数参数转成组件需要的指针:非正数表示未设置。
func intOf(value int) *int {

	if value <= 0 {
		return nil
	}

	return &value
}
