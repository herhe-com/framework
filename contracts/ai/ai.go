package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type AI interface {
	Driver
	Channel(driver string, name string) (Driver, error)
}

type Driver interface {
	Chat(c context.Context, request *ChatRequest) (*ChatResponse, error)
	Stream(c context.Context, request *ChatRequest) (chan *StreamResponse, error)
	Embedding(c context.Context, request *EmbeddingRequest) (*EmbeddingResponse, error)
	Models(c context.Context) ([]Model, error)
	Dri() string
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

// ModelDefaults 是驱动从 ai.<driver>.<name>.* 配置读出的模型参数默认值。
//
// 各驱动的配置路径不同,所以读取留在驱动层;往请求里补齐的逻辑只有这一份。
type ModelDefaults struct {
	Model           string
	Temperature     float64
	TopP            float64
	MaxTokens       int
	ReasoningEffort string
}

// Apply 用默认值补齐请求未设置(0 或空字符串)的字段,已设置的值原样保留。
func (d ModelDefaults) Apply(request *ChatRequest) {
	if request.Model == "" {
		request.Model = d.Model
	}

	if request.Temperature == 0 {
		request.Temperature = d.Temperature
	}

	if request.TopP == 0 {
		request.TopP = d.TopP
	}

	if request.MaxTokens == 0 {
		request.MaxTokens = d.MaxTokens
	}

	if request.ReasoningEffort == "" {
		request.ReasoningEffort = d.ReasoningEffort
	}
}

// FileDataUrlPrefix Data Url 中 base64 段的起始标记,例如 data:application/pdf;base64,<内容>。
const (
	FileDataUrlPrefix = ";base64,"
)

// ChatRequest 聊天请求。
//
// Temperature/TopP/MaxTokens 为 0、ReasoningEffort 为空字符串时,表示请求未设置,
// 由驱动用 ai.<driver>.<name>.* 配置的默认值补齐;配置也没填时该字段不下发,
// 交给服务端默认行为。补齐全过程由 ModelDefaults.Apply 完成。
type ChatRequest struct {
	Model           string    `json:"model"`
	Messages        []Message `json:"messages"`
	Temperature     float64   `json:"temperature,omitempty"`
	TopP            float64   `json:"top_p,omitempty"`
	MaxTokens       int       `json:"max_tokens,omitempty"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"`
	Stream          bool      `json:"stream,omitempty"`
}

// Message 聊天消息。
//
// Content 为纯文本内容;Parts 为多模态内容块,通过 MarshalJSON 序列化为数组,
// 兼容各大模型服务商的 OpenAI 协议扩展。Content 保持原样,确保纯文本用法不变。
// TextPart、ImagePart、FilePart 用于构造带内容块的消息,也支持手工组装。
type Message struct {
	Role    string
	Content string
	Parts   []Part
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
	if first.Content != "" {
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
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}

	content, parts, err := decodeContent(w.Content)
	if err != nil {
		return err
	}

	r.Role, r.Content, r.Parts = w.Role, content, parts

	return nil
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

// EmbeddingRequest 向量请求。
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
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

// Usage 用量。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamResponse 流式响应。
type StreamResponse struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
	Done    bool           `json:"done"`
	Error   error          `json:"error,omitempty"`
}

// UnmarshalJSON 兼容 string 与 object 两种错误形态（部分服务商会把业务错误作为流事件下发）。
func (r *StreamResponse) UnmarshalJSON(data []byte) error {

	type wire struct {
		ID      string          `json:"id"`
		Model   string          `json:"model"`
		Choices []StreamChoice  `json:"choices"`
		Done    bool            `json:"done"`
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
type MessageDelta struct {
	Role    string
	Content string
	Parts   []Part
}

// UnmarshalJSON 兼容字符串与数组两种内容格式,数组会同时回填 Parts 与 Content。
func (r *MessageDelta) UnmarshalJSON(data []byte) error {
	var w struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}

	content, parts, err := decodeContent(w.Content)
	if err != nil {
		return err
	}

	r.Role, r.Content, r.Parts = w.Role, content, parts

	return nil
}

// EmbeddingResponse 向量响应。
type EmbeddingResponse struct {
	Model string      `json:"model"`
	Data  []Embedding `json:"data"`
	Usage Usage       `json:"usage"`
}

// Embedding 向量结果。
type Embedding struct {
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// Model 模型定义。
type Model struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	OwnedBy string `json:"owned_by"`
}
