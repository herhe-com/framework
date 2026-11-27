package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	einoModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	contractai "github.com/herhe-com/framework/contracts/ai"
)

// ErrOfResponseFormat 表示请求里按请求指定了 response_format。
//
// eino 的组件把 response_format 放在构造参数里,没有按请求覆盖的选项,
// 因此结构化输出统一在渠道配置(ai.channels.<name>.response_format)里声明。
var ErrOfResponseFormat = errors.New("ai: response_format 需在渠道配置里声明(ai.channels.<name>.response_format),不支持按请求下发")

// adapter 把一个 eino 的对话模型映射成框架的 Driver 契约。
//
// 各家的协议、鉴权、SSE 解析、结构化输出都交给 eino 组件,这里只做契约转换:
// 新增一家厂商只需要在 channel.go 里多一个构造分支,不必再写一遍 HTTP 与流式解析。
type adapter struct {
	dri       string                  // 驱动名
	chat      einoModel.BaseChatModel // eino 对话模型
	modelName string                  // 渠道配置的默认模型名
	prefix    string                  // 系统提示词前缀
	timeout   time.Duration           // 非流式调用的期限；流式由流式超时口径控制
}

var _ contractai.Driver = (*adapter)(nil)

func (r *adapter) Dri() string {
	return r.dri
}

func (r *adapter) Model() string {
	return r.modelName
}

func (r *adapter) modelOf(request *contractai.ChatRequest) string {
	if request != nil && request.Model != "" {
		return request.Model
	}

	return r.modelName
}

// Chat 走一次非流式调用。
//
// 非流式没有分片可看门,用渠道的 timeout 给整次调用戴上期限;流式的空闲超时与总时长上限
// 由调用方的 StreamLimit 负责,两者不重叠。
func (r *adapter) Chat(c context.Context, request *contractai.ChatRequest) (*contractai.ChatResponse, error) {

	if r.timeout > 0 {
		var cancel context.CancelFunc
		c, cancel = context.WithTimeout(c, r.timeout)
		defer cancel()
	}

	messages, options, err := r.prepare(request)
	if err != nil {
		return nil, err
	}

	message, err := r.chat.Generate(c, messages, options...)
	if err != nil {
		return nil, err
	}

	return responseOf(r.modelOf(request), message), nil
}

// Stream 走一次流式调用,把 eino 的 StreamReader 转成框架的分片通道。
// 调用方取消 ctx 后 Recv 会返回错误,生产者随即退出并关闭通道。
func (r *adapter) Stream(c context.Context, request *contractai.ChatRequest) (chan *contractai.StreamResponse, error) {

	messages, options, err := r.prepare(request)
	if err != nil {
		return nil, err
	}

	stream, err := r.chat.Stream(c, messages, options...)
	if err != nil {
		return nil, err
	}

	modelName := r.modelOf(request)
	ch := make(chan *contractai.StreamResponse)

	go func() {
		defer close(ch)
		defer stream.Close()

		for {
			message, err := stream.Recv()

			if errors.Is(err, io.EOF) {
				contractai.Send(c, ch, &contractai.StreamResponse{Model: modelName, Done: true})
				return
			}
			if err != nil {
				contractai.Send(c, ch, &contractai.StreamResponse{Model: modelName, Done: true, Error: err})
				return
			}

			// finish_reason 只在末片有意义(stop/length/tool_calls…),调用方靠它判断是否被截断。
			finishReason := ""
			if message.ResponseMeta != nil {
				finishReason = message.ResponseMeta.FinishReason
			}

			item := &contractai.StreamResponse{
				Model: modelName,
				Usage: usageOf(message.ResponseMeta),
				Choices: []contractai.StreamChoice{{
					Delta: contractai.MessageDelta{
						Content:   message.Content,
						Reasoning: message.ReasoningContent,
					},
					FinishReason: finishReason,
				}},
			}
			if !contractai.Send(c, ch, item) {
				return
			}
		}
	}()

	return ch, nil
}

// prepare 把契约请求翻成 eino 的入参与按请求覆盖的选项。
// 模型参数为 0 时不传选项,由渠道构造时的默认值生效。
func (r *adapter) prepare(request *contractai.ChatRequest) ([]*schema.Message, []einoModel.Option, error) {

	if request.ResponseFormat != nil {
		return nil, nil, ErrOfResponseFormat
	}

	source := make([]contractai.Message, len(request.Messages))
	copy(source, request.Messages)
	if len(source) > 0 && len(source[0].Parts) > 0 {
		parts := make([]contractai.Part, len(source[0].Parts))
		copy(parts, source[0].Parts)
		source[0].Parts = parts
	}
	contractai.ApplyPrefix(source, r.prefix)

	messages := make([]*schema.Message, 0, len(source))
	for _, message := range source {
		messages = append(messages, messageOf(message))
	}

	options := make([]einoModel.Option, 0, 5)
	if request.Model != "" {
		options = append(options, einoModel.WithModel(request.Model))
	}
	if request.Temperature != 0 {
		options = append(options, einoModel.WithTemperature(float32(request.Temperature)))
	}
	if request.TopP != 0 {
		options = append(options, einoModel.WithTopP(float32(request.TopP)))
	}
	if request.MaxTokens != 0 {
		options = append(options, einoModel.WithMaxTokens(request.MaxTokens))
	}
	if request.ReasoningEffort != "" {
		if r.dri != DriverOpenAI {
			return nil, nil, fmt.Errorf("ai: 驱动 %s 不支持按请求指定 reasoning_effort,请在渠道配置里声明", r.dri)
		}
		options = append(options, einoopenai.WithReasoningEffort(einoopenai.ReasoningEffortLevel(request.ReasoningEffort)))
	}

	return messages, options, nil
}

// messageOf 把契约消息翻成 eino 消息;带内容块的消息按多模态输入承载。
func messageOf(message contractai.Message) *schema.Message {

	out := &schema.Message{Role: schema.RoleType(message.Role), Content: message.Content}

	if len(message.Parts) == 0 {
		return out
	}

	parts := make([]schema.MessageInputPart, 0, len(message.Parts))
	for _, part := range message.Parts {
		switch part.Type {
		case contractai.PartOfText:
			parts = append(parts, schema.MessageInputPart{Type: schema.ChatMessagePartTypeText, Text: part.Text})
		case contractai.PartOfImageURL:
			if part.ImageURL == nil {
				continue
			}
			url := part.ImageURL.Url
			parts = append(parts, schema.MessageInputPart{
				Type:  schema.ChatMessagePartTypeImageURL,
				Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &url}},
			})
		case contractai.PartOfFile:
			if part.File == nil {
				continue
			}
			url := part.File.FileData
			parts = append(parts, schema.MessageInputPart{
				Type: schema.ChatMessagePartTypeFileURL,
				File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{URL: &url}, Name: part.File.Name},
			})
		}
	}

	if len(parts) > 0 {
		out.Content = ""
		out.UserInputMultiContent = parts
	}

	return out
}

// responseOf 把 eino 的响应消息翻成契约响应。
func responseOf(modelName string, message *schema.Message) *contractai.ChatResponse {

	response := &contractai.ChatResponse{
		Model: modelName,
		Choices: []contractai.Choice{{
			Message: contractai.Message{
				Role:      string(message.Role),
				Content:   message.Content,
				Reasoning: message.ReasoningContent,
			},
		}},
	}

	if message.ResponseMeta == nil {
		return response
	}

	response.Choices[0].FinishReason = message.ResponseMeta.FinishReason

	if usage := usageOf(message.ResponseMeta); usage != nil {
		response.Usage = *usage
	}

	return response
}

// usageOf 把 eino 的用量翻成契约用量;没有用量时返回 nil,避免把 0 当成真实用量。
func usageOf(meta *schema.ResponseMeta) *contractai.Usage {

	if meta == nil || meta.Usage == nil {
		return nil
	}

	return &contractai.Usage{
		PromptTokens:     meta.Usage.PromptTokens,
		CompletionTokens: meta.Usage.CompletionTokens,
		ReasoningTokens:  meta.Usage.CompletionTokensDetails.ReasoningTokens,
		TotalTokens:      meta.Usage.TotalTokens,
	}
}
