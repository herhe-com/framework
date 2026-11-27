package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// AI 是模型服务的入口:默认渠道加按渠道名取用。
type AI interface {
	Driver
	// Channel 返回指定渠道,渠道由 ai.channels.<name> 定义;同一渠道只初始化一次。
	Channel(name string) (Driver, error)
}

// Driver 是一个已配置好的模型渠道。
//
// 渠道的 driver 字段决定走哪家协议,底层统一由 eino 的 model 组件实现:
// 框架只负责读取配置、把契约映射成 eino 的入参,不再自己维护各家的 HTTP/SSE 协议。
type Driver interface {
	Chat(c context.Context, request *ChatRequest) (*ChatResponse, error)
	Stream(c context.Context, request *ChatRequest) (chan *StreamResponse, error)
	// Dri 返回驱动名:openai/ollama/anthropic/gemini。
	Dri() string
	// Model 返回渠道配置的默认模型名,供日志在模型未回传名称时兜底。
	Model() string
}

// 内容块类型常量。
const (
	PartOfText     = "text"      // 文字内容
	PartOfImageURL = "image_url" // 图片内容,支持 Data Url 或外链
	PartOfFile     = "file"      // 文件内容,例如 PDF,使用 Data Url 承载
)

// 消息角色常量。
const (
	RoleOfSystem    = "system"
	RoleOfUser      = "user"
	RoleOfAssistant = "assistant"
)

// 思考程度取值常量,对应 reasoning_effort,网关自定义取值同样支持原样下发。
const (
	ReasoningOfMinimal = "minimal" // 不思考或最少思考(仅部分模型支持)
	ReasoningOfLow     = "low"     // 低,速度优先
	ReasoningOfMedium  = "medium"  // 中
	ReasoningOfHigh    = "high"    // 高,质量优先
)

// 输出格式取值常量,对应 response_format.type。
const (
	ResponseOfText   = "text"        // 不约束,与不下发 response_format 等价
	ResponseOfJSON   = "json_object" // 只要求输出是合法 JSON 对象,字段仍由模型自行决定
	ResponseOfSchema = "json_schema" // 按 JSON Schema 约束字段结构、必填项与取值范围
)

// FileDataUrlPrefix Data Url 中 base64 段的起始标记,例如 data:application/pdf;base64,<内容>。
const (
	FileDataUrlPrefix = ";base64,"
)

// ChatRequest 聊天请求。
//
// Temperature/TopP/MaxTokens 为 0、ReasoningEffort 为空字符串时,表示请求未设置,
// 用 ai.channels.<name> 里配置的渠道默认值;配置也没填时不下发,交给服务端默认行为。
//
// 默认值在渠道构造时注入 eino 组件,按请求覆盖走 eino 的 model.Option,框架不做二次补齐。
//
// ResponseFormat 为 nil 时不下发,输出格式完全由提示词决定。
type ChatRequest struct {
	Model           string          `json:"model"`
	Messages        []Message       `json:"messages"`
	Temperature     float64         `json:"temperature,omitempty"`
	TopP            float64         `json:"top_p,omitempty"`
	MaxTokens       int             `json:"max_tokens,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
	ResponseFormat  *ResponseFormat `json:"response_format,omitempty"`
	Stream          bool            `json:"stream,omitempty"`
}

// ResponseFormat 约束模型输出格式,对应 OpenAI 协议的 response_format。
//
// type=json_object 只保证输出是合法 JSON,字段是否齐全仍取决于提示词;
// type=json_schema 由解码侧保证结构,但保证不了“药名必须逐字来自输入”这类语义约束,
// 业务规则仍需写在提示词里。服务端不认该参数时(部分网关)输出会退回纯文本,
// 调用方仍要按“可能解析失败”处理。
type ResponseFormat struct {
	Type       string        `json:"type"`
	JSONSchema *SchemaFormat `json:"json_schema,omitempty"`
}

// SchemaFormat 是 json_schema 模式的描述,对应 OpenAI 协议的 json_schema 字段。
type SchemaFormat struct {
	Name   string          `json:"name"`             // schema 名称,同一服务内唯一
	Strict bool            `json:"strict,omitempty"` // 严格模式:所有字段都要在 required 里声明,且不允许额外字段
	Schema json.RawMessage `json:"schema"`           // JSON Schema 原文
}

// ResponseFormatOfJSON 返回“只要求合法 JSON 对象”的输出约束。
func ResponseFormatOfJSON() *ResponseFormat {
	return &ResponseFormat{Type: ResponseOfJSON}
}

// ResponseFormatOfSchema 用 JSON Schema 构造结构化输出约束,strict 为 true 时启用严格模式。
//
// schema 必须是合法的 JSON 对象字面量(不能是数组或空值),构造阶段就校验,
// 避免把坏 schema 发给模型后在服务端才报错。严格模式下多数服务商要求
// 所有字段都列进 required 并显式声明 additionalProperties:false。
func ResponseFormatOfSchema(name string, schema []byte, strict bool) (*ResponseFormat, error) {

	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("response format: schema name is required")
	}

	var object json.RawMessage
	if err := json.Unmarshal(schema, &object); err != nil {
		return nil, fmt.Errorf("response format: invalid json schema: %w", err)
	}
	if trimmed := bytes.TrimSpace(object); len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, fmt.Errorf("response format: json schema must be an object")
	}

	return &ResponseFormat{
		Type: ResponseOfSchema,
		JSONSchema: &SchemaFormat{
			Name:   strings.TrimSpace(name),
			Strict: strict,
			Schema: object,
		},
	}, nil
}

// Message 聊天消息。
//
// Content 为纯文本内容;Parts 为多模态内容块,通过 MarshalJSON 序列化为数组,
// 兼容各大模型服务商的 OpenAI 协议扩展。Content 保持原样,确保纯文本用法不变。
// TextPart、ImagePart、FilePart 用于构造带内容块的消息,也支持手工组装。
//
// Reasoning 是推理模型返回的思考过程,仅在解析响应时回填,发送消息时不会下发。
type Message struct {
	Role      string
	Content   string
	Reasoning string
	Parts     []Part
}

// Part 多模态内容块。
//
// 按服务商协议不同,文字块的内容字段落 Text,图片块落 ImageURL,文件块落 File。
type Part struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
	File     *File     `json:"file,omitempty"`
}

// ImageURL 图片内容块载荷。
type ImageURL struct {
	Url string `json:"url"`
}

// File 文件内容块载荷。
type File struct {
	Name     string `json:"filename,omitempty"`
	FileData string `json:"file_data"`
}

// TextPart 构造文字内容块。
func TextPart(text string) Part {
	return Part{Type: PartOfText, Text: text}
}

// ImagePart 构造图片内容块,支持 Data Url 或外链地址。
func ImagePart(url string) Part {
	return Part{Type: PartOfImageURL, ImageURL: &ImageURL{Url: url}}
}

// FilePart 构造文件内容块,Data Url 格式,例如 application/pdf。
func FilePart(filename string, dataUrl string) Part {
	return Part{Type: PartOfFile, File: &File{Name: filename, FileData: dataUrl}}
}

// DataUrl 二进制内容转 Data Url。
func DataUrl(mime string, raw []byte) string {
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw)
}

// ApplyPrefix 给首条消息注入系统前缀，兼容纯文本与多模态两种消息形态。
func ApplyPrefix(messages []Message, prefix string) {
	if prefix == "" || len(messages) == 0 {
		return
	}

	first := &messages[0]
	if first.Content != "" || len(first.Parts) == 0 {
		first.Content = prefix + first.Content
		return
	}

	for i := range first.Parts {
		if first.Parts[i].Type == PartOfText {
			first.Parts[i].Text = prefix + first.Parts[i].Text
			return
		}
	}

	first.Parts = append([]Part{{Type: PartOfText, Text: prefix}}, first.Parts...)
}

// StringOfDataUrl 提取 Data Url 中的 base64 承载内容。
//
// 不带 Data Url 头部时原样返回,便于各驱动接入自有协议。
func StringOfDataUrl(url string) string {
	if idx := strings.Index(url, FileDataUrlPrefix); idx >= 0 {
		return url[idx+len(FileDataUrlPrefix):]
	}
	return url
}

// MarshalJSON 纯文本消息序列化为字符串,带内容块时序列化为数组。
func (r Message) MarshalJSON() ([]byte, error) {
	if len(r.Parts) == 0 {
		type wire struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		return json.Marshal(wire{Role: r.Role, Content: r.Content})
	}

	type wire struct {
		Role    string `json:"role"`
		Content []Part `json:"content"`
	}
	return json.Marshal(wire{Role: r.Role, Content: r.Parts})
}

// UnmarshalJSON 兼容字符串与数组两种内容格式,数组会同时回填 Parts 与 Content。
func (r *Message) UnmarshalJSON(data []byte) error {
	var w struct {
		Role             string          `json:"role"`
		Content          json.RawMessage `json:"content"`
		ReasoningContent string          `json:"reasoning_content"`
		Reasoning        string          `json:"reasoning"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}

	content, parts, err := decodeContent(w.Content)
	if err != nil {
		return err
	}

	r.Role, r.Content, r.Parts = w.Role, content, parts
	r.Reasoning = reasoningOf(w.ReasoningContent, w.Reasoning)

	return nil
}

// reasoningOf 取思考过程:多数网关用 reasoning_content,少数用 reasoning。
func reasoningOf(content string, reasoning string) string {
	if content != "" {
		return content
	}

	return reasoning
}

// decodeContent 解析消息内容,兼容字符串与数组两种形态。
//
// 字符串直接作为纯文本;数组解析为内容块,并把其中的文字块以换行拼接后
// 作为纯文本回填,使只读 Content 的调用方在两种形态下行为一致。
func decodeContent(raw json.RawMessage) (string, []Part, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", nil, nil
	}

	if raw[0] == '"' {
		var content string
		if err := json.Unmarshal(raw, &content); err != nil {
			return "", nil, err
		}

		return content, nil, nil
	}

	var parts []Part
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", nil, err
	}

	texts := make([]string, 0, len(parts))
	for i := range parts {
		if parts[i].Type == PartOfText && parts[i].Text != "" {
			texts = append(texts, parts[i].Text)
		}
	}

	return strings.Join(texts, "\n"), parts, nil
}

// ChatResponse 聊天响应。
type ChatResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice 完成内容。
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage 用量,取自服务端返回的官方 usage。
//
// 推理模型的思考和正文一起计入 CompletionTokens,ReasoningTokens 是其中的思考部分,
// 用于单独观察「想」花了多少;服务端不回传明细时为 0。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`     // 输入 token
	CompletionTokens int `json:"completion_tokens"` // 输出 token(含推理)
	ReasoningTokens  int `json:"reasoning_tokens"`  // 输出中的推理 token;服务端不回传时为 0
	TotalTokens      int `json:"total_tokens"`      // 合计 token
}

// StreamResponse 流式响应。
type StreamResponse struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
	Done    bool           `json:"done"`
	Error   error          `json:"error,omitempty"`
	// Usage 是该片的用量,一般只有末片带(None 表示这一片没有回传用量);
	// 需要整条流的用量时取最后一个非空的 Usage。
	Usage *Usage `json:"usage,omitempty"`
}

// UnmarshalJSON 兼容 string 与 object 两种错误形态（部分服务商会把业务错误作为流事件下发）。
func (r *StreamResponse) UnmarshalJSON(data []byte) error {

	type wire struct {
		ID      string          `json:"id"`
		Model   string          `json:"model"`
		Choices []StreamChoice  `json:"choices"`
		Done    bool            `json:"done"`
		Usage   *Usage          `json:"usage"`
		Error   json.RawMessage `json:"error"`
	}

	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}

	r.ID = w.ID
	r.Model = w.Model
	r.Choices = w.Choices
	r.Done = w.Done
	r.Usage = w.Usage

	if len(w.Error) != 0 && !bytes.Equal(bytes.TrimSpace(w.Error), []byte("null")) {
		// 对象形态 {"message": ...} 来自 OpenAI 兼容网关,字符串形态来自 ollama 等
		// 驱动:后者必须按 JSON 字符串解析,否则错误文本会带上引号。
		var object struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(w.Error, &object); err == nil && object.Message != "" {
			r.Error = fmt.Errorf("api error: %s", object.Message)
		} else {
			r.Error = fmt.Errorf("api error: %s", errorText(w.Error))
		}
	}

	return nil
}

// errorText 提取错误负载的文本:JSON 字符串去掉引号,其余形态原样透出。
func errorText(raw json.RawMessage) string {
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}

	return string(raw)
}

// StreamChoice 流式完成内容。
type StreamChoice struct {
	Index        int          `json:"index"`
	Delta        MessageDelta `json:"delta"`
	FinishReason string       `json:"finish_reason"`
}

// Send 向流式读取者发送事件。
//
// 消费端退出或请求被取消时返回 false,生产者据此终止,避免 goroutine 泄漏。
func Send(c context.Context, ch chan *StreamResponse, value *StreamResponse) bool {
	select {
	case ch <- value:
		return true
	case <-c.Done():
		return false
	}
}

// MessageDelta 流式增量消息,与 Message 一样兼容字符串与数组内容。
//
// Reasoning 是推理模型在正文之前输出的思考过程增量,与 Content 分开下发,
// 每个分片都需要原样追加;非推理模型或网关不透传时始终为空。
type MessageDelta struct {
	Role      string
	Content   string
	Reasoning string
	Parts     []Part
}

// UnmarshalJSON 兼容字符串与数组两种内容格式,数组会同时回填 Parts 与 Content。
func (r *MessageDelta) UnmarshalJSON(data []byte) error {
	var w struct {
		Role             string          `json:"role"`
		Content          json.RawMessage `json:"content"`
		ReasoningContent string          `json:"reasoning_content"`
		Reasoning        string          `json:"reasoning"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}

	content, parts, err := decodeContent(w.Content)
	if err != nil {
		return err
	}

	r.Role, r.Content, r.Parts = w.Role, content, parts
	r.Reasoning = reasoningOf(w.ReasoningContent, w.Reasoning)

	return nil
}
