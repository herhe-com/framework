package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	einoModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	contractai "github.com/herhe-com/framework/contracts/ai"
)

// fakeChat 是 eino 的假对话模型:记录收到的消息,按预设返回生成结果或分片。
type fakeChat struct {
	received []*schema.Message
	lastOpts []einoModel.Option
	chunks   []*schema.Message
	err      error
}

func (f *fakeChat) Generate(_ context.Context, in []*schema.Message, opts ...einoModel.Option) (*schema.Message, error) {

	f.received = in
	f.lastOpts = opts

	if f.err != nil {
		return nil, f.err
	}

	return &schema.Message{
		Role:             schema.Assistant,
		Content:          "你好",
		ReasoningContent: "先想一想",
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: "stop",
			Usage: &schema.TokenUsage{
				PromptTokens:            10,
				CompletionTokens:        20,
				CompletionTokensDetails: schema.CompletionTokensDetails{ReasoningTokens: 15},
				TotalTokens:             30,
			},
		},
	}, nil
}

func (f *fakeChat) Stream(_ context.Context, in []*schema.Message, opts ...einoModel.Option) (*schema.StreamReader[*schema.Message], error) {

	f.received = in
	f.lastOpts = opts

	if f.err != nil {
		return nil, f.err
	}

	return schema.StreamReaderFromArray(f.chunks), nil
}

func (f *fakeChat) GetType() string {
	return "fake"
}

func (f *fakeChat) IsCallbacksEnabled() bool {
	return false
}

func TestAdapterChat(t *testing.T) {

	chat := &fakeChat{}
	driver := &adapter{dri: DriverOpenAI, chat: chat, modelName: "config-model", prefix: "【前缀】"}

	response, err := driver.Chat(context.Background(), &contractai.ChatRequest{
		Messages: []contractai.Message{
			{Role: contractai.RoleOfSystem, Content: "系统提示"},
			{Role: contractai.RoleOfUser, Content: "用户输入"},
		},
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}

	if driver.Dri() != DriverOpenAI || driver.Model() != "config-model" {
		t.Fatalf("渠道信息: %s/%s", driver.Dri(), driver.Model())
	}

	// 前缀只拼到首条消息
	if len(chat.received) != 2 || chat.received[0].Content != "【前缀】系统提示" || chat.received[1].Content != "用户输入" {
		t.Fatalf("入参消息: %+v", chat.received)
	}

	if response.Model != "config-model" || response.Choices[0].Message.Content != "你好" {
		t.Fatalf("响应内容: %+v", response)
	}
	if response.Choices[0].Message.Reasoning != "先想一想" || response.Choices[0].FinishReason != "stop" {
		t.Fatalf("思考过程/结束原因: %+v", response.Choices[0])
	}
	if response.Usage.TotalTokens != 30 || response.Usage.ReasoningTokens != 15 {
		t.Fatalf("用量: %+v", response.Usage)
	}
}

func TestAdapterStream(t *testing.T) {

	chat := &fakeChat{chunks: []*schema.Message{
		{Role: schema.Assistant, Content: "你"},
		{Role: schema.Assistant, Content: "好", ReasoningContent: "思考", ResponseMeta: &schema.ResponseMeta{
			FinishReason: "length",
			Usage:        &schema.TokenUsage{PromptTokens: 7, CompletionTokens: 9, TotalTokens: 16},
		}},
	}}
	driver := &adapter{dri: DriverOpenAI, chat: chat, modelName: "config-model"}

	stream, err := driver.Stream(context.Background(), &contractai.ChatRequest{
		Messages: []contractai.Message{{Role: contractai.RoleOfUser, Content: "说句话"}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	total := strings.Builder{}
	reasoning := strings.Builder{}
	done := false
	finishReason := ""
	var usage *contractai.Usage

	for item := range stream {
		if item.Error != nil {
			t.Fatalf("分片错误: %v", item.Error)
		}
		if item.Model != "config-model" {
			t.Fatalf("分片模型名: %q", item.Model)
		}
		done = done || item.Done
		if item.Usage != nil {
			usage = item.Usage
		}
		for _, choice := range item.Choices {
			total.WriteString(choice.Delta.Content)
			reasoning.WriteString(choice.Delta.Reasoning)
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
		}
	}

	if total.String() != "你好" || reasoning.String() != "思考" {
		t.Fatalf("分片内容: %q / %q", total.String(), reasoning.String())
	}
	if !done {
		t.Fatal("流结束时应有一个 Done 分片")
	}
	if finishReason != "length" {
		t.Fatalf("末片 finish_reason: %q", finishReason)
	}
	if usage == nil || usage.TotalTokens != 16 || usage.PromptTokens != 7 {
		t.Fatalf("末片用量: %+v", usage)
	}
}

func TestAdapterStreamError(t *testing.T) {

	expect := errors.New("网关不可用")
	driver := &adapter{dri: DriverOpenAI, chat: &fakeChat{err: expect}}

	if _, err := driver.Stream(context.Background(), &contractai.ChatRequest{}); !errors.Is(err, expect) {
		t.Fatalf("期望流建立失败直接返回错误, 实际 %v", err)
	}
}

func TestAdapterMultimodal(t *testing.T) {

	chat := &fakeChat{}
	driver := &adapter{dri: DriverOpenAI, chat: chat}

	_, err := driver.Chat(context.Background(), &contractai.ChatRequest{
		Messages: []contractai.Message{{
			Role: contractai.RoleOfUser,
			Parts: []contractai.Part{
				contractai.TextPart("看看这个"),
				contractai.ImagePart("data:image/png;base64,AAA"),
				contractai.FilePart("a.pdf", "data:application/pdf;base64,BBB"),
			},
		}},
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}

	parts := chat.received[0].UserInputMultiContent
	if len(parts) != 3 {
		t.Fatalf("内容块数量: %d", len(parts))
	}
	if parts[0].Type != schema.ChatMessagePartTypeText || parts[0].Text != "看看这个" {
		t.Fatalf("文字块: %+v", parts[0])
	}
	if parts[1].Type != schema.ChatMessagePartTypeImageURL || parts[1].Image == nil || *parts[1].Image.URL != "data:image/png;base64,AAA" {
		t.Fatalf("图片块: %+v", parts[1])
	}
	if parts[2].Type != schema.ChatMessagePartTypeFileURL || parts[2].File == nil || parts[2].File.Name != "a.pdf" {
		t.Fatalf("文件块: %+v", parts[2])
	}
}

func TestAdapterResponseFormatOfRequest(t *testing.T) {

	driver := &adapter{dri: DriverOpenAI, chat: &fakeChat{}}

	_, err := driver.Chat(context.Background(), &contractai.ChatRequest{ResponseFormat: contractai.ResponseFormatOfJSON()})
	if !errors.Is(err, ErrOfResponseFormat) {
		t.Fatalf("期望按请求下发 response_format 被拒绝, 实际 %v", err)
	}
}

func TestResponseFormatOf(t *testing.T) {

	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "empty", value: "", want: ""},
		{name: "json-object", value: "json_object", want: "json_object"},
		{name: "uppercase", value: " JSON_OBJECT ", want: "json_object"},
		{name: "unsupported", value: "json_schema", wantErr: true},
	}

	for _, item := range tests {
		format, err := responseFormatOf(channelConfig{ResponseFormat: item.value})
		if item.wantErr {
			if err == nil {
				t.Fatalf("%s: 期望报错", item.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", item.name, err)
		}
		if item.want == "" {
			if format != nil {
				t.Fatalf("%s: 期望不下发, 实际 %+v", item.name, format)
			}
			continue
		}
		if format == nil || string(format.Type) != item.want {
			t.Fatalf("%s: 期望 %s, 实际 %+v", item.name, item.want, format)
		}
	}
}

func TestThinkOf(t *testing.T) {

	if thinkOf("") != nil {
		t.Fatal("未配置时不应下发思考开关")
	}
	if value := thinkOf("TRUE"); value == nil || value.Value != true {
		t.Fatalf("true: %+v", value)
	}
	if value := thinkOf("false"); value == nil || value.Value != false {
		t.Fatalf("false: %+v", value)
	}
	if value := thinkOf("high"); value == nil || value.Value != "high" {
		t.Fatalf("high: %+v", value)
	}
}

func TestNewChannelOfSwitch(t *testing.T) {

	tests := []struct {
		name    string
		config  channelConfig
		wantErr string
	}{
		{name: "missing-driver", config: channelConfig{Name: "a"}, wantErr: "缺少 driver"},
		{name: "responses", config: channelConfig{Name: "a", Driver: DriverResponses}, wantErr: "responses 暂未实现"},
		{name: "unknown", config: channelConfig{Name: "a", Driver: "foo"}, wantErr: "不支持"},
	}

	for _, item := range tests {
		_, err := newChannelOf(item.config)
		if err == nil || !strings.Contains(err.Error(), item.wantErr) {
			t.Fatalf("%s: 期望包含 %q, 实际 %v", item.name, item.wantErr, err)
		}
	}
}

func TestNewChannelOfConstruction(t *testing.T) {

	tests := []struct {
		name   string
		config channelConfig
		dri    string
	}{
		{name: "openai", dri: DriverOpenAI, config: channelConfig{Name: "a", Driver: DriverOpenAI, APIKey: "k", BaseURL: "http://127.0.0.1:1/v1", Model: "m", Timeout: 5 * time.Second}},
		{name: "ollama", dri: DriverOllama, config: channelConfig{Name: "b", Driver: DriverOllama, Host: "http://127.0.0.1:11434", Model: "llama3"}},
		{name: "anthropic", dri: DriverAnthropic, config: channelConfig{Name: "c", Driver: DriverAnthropic, APIKey: "k", Model: "claude", MaxTokens: 1024}},
		{name: "gemini", dri: DriverGemini, config: channelConfig{Name: "d", Driver: DriverGemini, APIKey: "k", BaseURL: "http://127.0.0.1:1", Model: "gemini-2.5-flash"}},
	}

	for _, item := range tests {
		driver, err := newChannelOf(item.config)
		if err != nil {
			t.Fatalf("%s: %v", item.name, err)
		}
		if driver.Dri() != item.dri || driver.Model() != item.config.Model {
			t.Fatalf("%s: 渠道信息 %s/%s", item.name, driver.Dri(), driver.Model())
		}
	}
}

// blockingChat 一直占着直到 ctx 结束,用于验证非流式调用的期限。
type blockingChat struct {
	fakeChat
}

func (f *blockingChat) Generate(c context.Context, _ []*schema.Message, _ ...einoModel.Option) (*schema.Message, error) {
	<-c.Done()

	return nil, c.Err()
}

func TestAdapterChatTimeout(t *testing.T) {

	driver := &adapter{dri: DriverOpenAI, chat: &blockingChat{}, timeout: 30 * time.Millisecond}

	started := time.Now()
	_, err := driver.Chat(context.Background(), &contractai.ChatRequest{})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("期望非流式按渠道 timeout 中断, 实际 %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("应按 timeout 及时中断, 实际耗时 %v", elapsed)
	}
}

func TestAdapterPrefixDoesNotMutateRequest(t *testing.T) {

	chat := &fakeChat{}
	driver := &adapter{dri: DriverOpenAI, chat: chat, modelName: "config-model", prefix: "【前缀】"}

	textReq := &contractai.ChatRequest{
		Messages: []contractai.Message{
			{Role: contractai.RoleOfSystem, Content: "系统提示"},
			{Role: contractai.RoleOfUser, Content: "用户输入"},
		},
	}

	for range 2 {
		if _, err := driver.Chat(context.Background(), textReq); err != nil {
			t.Fatalf("chat: %v", err)
		}
		if chat.received[0].Content != "【前缀】系统提示" {
			t.Fatalf("预期前缀只拼接一次, 实际 %q", chat.received[0].Content)
		}
	}
	if textReq.Messages[0].Content != "系统提示" {
		t.Fatalf("入参消息被修改: %q", textReq.Messages[0].Content)
	}

	multiReq := &contractai.ChatRequest{
		Messages: []contractai.Message{
			{Role: contractai.RoleOfUser, Parts: []contractai.Part{contractai.TextPart("问题")}},
		},
	}
	for range 2 {
		if _, err := driver.Chat(context.Background(), multiReq); err != nil {
			t.Fatalf("multimodal chat: %v", err)
		}
		if chat.received[0].UserInputMultiContent[0].Text != "【前缀】问题" {
			t.Fatalf("预期多模态前缀只拼接一次, 实际 %q", chat.received[0].UserInputMultiContent[0].Text)
		}
	}
	if multiReq.Messages[0].Parts[0].Text != "问题" {
		t.Fatalf("入参多模态消息被修改: %q", multiReq.Messages[0].Parts[0].Text)
	}
}

func TestAdapterMultimodalClearsContent(t *testing.T) {

	chat := &fakeChat{}
	driver := &adapter{dri: DriverOpenAI, chat: chat}

	_, err := driver.Chat(context.Background(), &contractai.ChatRequest{
		Messages: []contractai.Message{{
			Role:    contractai.RoleOfUser,
			Content: "看看这个",
			Parts: []contractai.Part{
				contractai.TextPart("看看这个"),
				contractai.ImagePart("data:image/png;base64,AAA"),
			},
		}},
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}

	if chat.received[0].Content != "" {
		t.Fatalf("多模态消息应清空 Content, 实际 %q", chat.received[0].Content)
	}
	if len(chat.received[0].UserInputMultiContent) != 2 {
		t.Fatalf("内容块数量: %d", len(chat.received[0].UserInputMultiContent))
	}
}

func TestAdapterModelOverride(t *testing.T) {

	chat := &fakeChat{chunks: []*schema.Message{{Role: schema.Assistant, Content: "hi"}}}
	driver := &adapter{dri: DriverOpenAI, chat: chat, modelName: "default-model"}

	resp, err := driver.Chat(context.Background(), &contractai.ChatRequest{
		Model:    "override-model",
		Messages: []contractai.Message{{Role: contractai.RoleOfUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if resp.Model != "override-model" {
		t.Fatalf("ChatResponse.Model: got %q, want override-model", resp.Model)
	}

	stream, err := driver.Stream(context.Background(), &contractai.ChatRequest{
		Model:    "override-model",
		Messages: []contractai.Message{{Role: contractai.RoleOfUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	for item := range stream {
		if item.Model != "override-model" {
			t.Fatalf("StreamResponse.Model: got %q, want override-model", item.Model)
		}
	}
}

func TestAdapterReasoningEffort(t *testing.T) {

	chat := &fakeChat{}
	openaiDriver := &adapter{dri: DriverOpenAI, chat: chat}
	if _, err := openaiDriver.Chat(context.Background(), &contractai.ChatRequest{
		ReasoningEffort: contractai.ReasoningOfHigh,
		Messages:        []contractai.Message{{Role: contractai.RoleOfUser, Content: "hi"}},
	}); err != nil {
		t.Fatalf("openai reasoning_effort: %v", err)
	}
	if len(chat.lastOpts) != 1 {
		t.Fatalf("期望透传 1 个 option, 实际 %d", len(chat.lastOpts))
	}

	ollamaDriver := &adapter{dri: DriverOllama, chat: &fakeChat{}}
	if _, err := ollamaDriver.Chat(context.Background(), &contractai.ChatRequest{
		ReasoningEffort: contractai.ReasoningOfHigh,
		Messages:        []contractai.Message{{Role: contractai.RoleOfUser, Content: "hi"}},
	}); err == nil {
		t.Fatal("ollama 不支持按请求指定 reasoning_effort, 期望报错")
	}
}

func TestFormatOfOllama(t *testing.T) {

	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "empty", value: "", want: ""},
		{name: "json-object", value: "json_object", want: `"json"`},
		{name: "uppercase", value: " JSON_OBJECT ", want: `"json"`},
		{name: "unsupported", value: "json_schema", wantErr: true},
	}

	for _, item := range tests {
		format, err := formatOfOllama(channelConfig{ResponseFormat: item.value})
		if item.wantErr {
			if err == nil {
				t.Fatalf("%s: 期望报错", item.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", item.name, err)
		}
		if string(format) != item.want {
			t.Fatalf("%s: got %q, want %q", item.name, string(format), item.want)
		}
	}
}

func TestNewChannelOfValidation(t *testing.T) {

	tests := []struct {
		name    string
		config  channelConfig
		wantErr string
	}{
		{name: "openai-missing-key", config: channelConfig{Name: "a", Driver: DriverOpenAI, Model: "m"}, wantErr: "api_key"},
		{name: "ollama-missing-host", config: channelConfig{Name: "b", Driver: DriverOllama, Model: "m"}, wantErr: "host"},
		{name: "anthropic-missing-key", config: channelConfig{Name: "c", Driver: DriverAnthropic, Model: "m"}, wantErr: "api_key"},
		{name: "gemini-missing-key", config: channelConfig{Name: "d", Driver: DriverGemini, Model: "m"}, wantErr: "api_key"},
		{name: "anthropic-response-format", config: channelConfig{Name: "e", Driver: DriverAnthropic, APIKey: "k", Model: "m", ResponseFormat: "json_object"}, wantErr: "response_format"},
		{name: "gemini-response-format", config: channelConfig{Name: "f", Driver: DriverGemini, APIKey: "k", Model: "m", ResponseFormat: "json_object"}, wantErr: "response_format"},
	}

	for _, item := range tests {
		_, err := newChannelOf(item.config)
		if err == nil || !strings.Contains(err.Error(), item.wantErr) {
			t.Fatalf("%s: 期望包含 %q, 实际 %v", item.name, item.wantErr, err)
		}
	}
}
