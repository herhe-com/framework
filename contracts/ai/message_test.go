package ai

import (
	"encoding/json"
	"testing"
)

func TestMessageMarshalJSON(t *testing.T) {

	tests := []struct {
		name     string
		message  Message
		expected string
	}{
		{name: "text-only", message: Message{Role: "user", Content: "你好"}, expected: `{"role":"user","content":"你好"}`},
		{name: "with-parts", message: Message{Role: "user", Parts: []Part{TextPart("看看这张图"), ImagePart("data:image/png;base64,AAA")}}, expected: `{"role":"user","content":[{"type":"text","text":"看看这张图"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAA"}}]}`},
		{name: "empty-content", message: Message{Role: "system"}, expected: `{"role":"system","content":""}`},
	}

	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			raw, err := json.Marshal(item.message)
			if err != nil {
				t.Fatalf("marshal message: %v", err)
			}
			if string(raw) != item.expected {
				t.Errorf("unexpected marshal:\n got: %s\nwant: %s", raw, item.expected)
			}
		})
	}
}

func TestMessageUnmarshalJSON(t *testing.T) {

	tests := []struct {
		name           string
		input          string
		role           string
		content        string
		parts          int
		firstImage     string
		firstPartIsTex bool
	}{
		{name: "string", input: `{"role":"assistant","content":"回复"}`, role: "assistant", content: "回复"},
		{name: "array", input: `{"role":"assistant","content":[{"type":"text","text":"图1:"},{"type":"image_url","image_url":{"url":"data:image/jpeg;base64,BBB"}},{"type":"text","text":"图2:"}]}`, role: "assistant", content: "图1:\n图2:", parts: 3, firstImage: "data:image/jpeg;base64,BBB"},
		{name: "null-content", input: `{"role":"assistant","content":null}`, role: "assistant"},
	}

	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			var message Message
			if err := json.Unmarshal([]byte(item.input), &message); err != nil {
				t.Fatalf("unmarshal message: %v", err)
			}
			if message.Role != item.role {
				t.Errorf("role: got %s, want %s", message.Role, item.role)
			}
			if message.Content != item.content {
				t.Errorf("content: got %s, want %s", message.Content, item.content)
			}
			if len(message.Parts) != item.parts {
				t.Errorf("parts: got %d, want %d", len(message.Parts), item.parts)
			}
			if item.firstImage != "" && (message.Parts[1].ImageURL == nil || message.Parts[1].ImageURL.Url != item.firstImage) {
				t.Errorf("first image: got %+v, want %s", message.Parts[1].ImageURL, item.firstImage)
			}
		})
	}
}

func TestMessageDeltaUnmarshalJSON(t *testing.T) {

	var delta MessageDelta
	if err := json.Unmarshal([]byte(`{"role":"assistant","content":[{"type":"text","text":"增量"}`+`,{"type":"image_url","image_url":{"url":"x"}}]}`), &delta); err != nil {
		t.Fatalf("unmarshal delta: %v", err)
	}
	if delta.Content != "增量" || len(delta.Parts) != 2 {
		t.Errorf("delta: got content=%s parts=%d", delta.Content, len(delta.Parts))
	}

	delta = MessageDelta{}
	if err := json.Unmarshal([]byte(`{"role":"assistant","content":"纯文本"}`), &delta); err != nil {
		t.Fatalf("unmarshal delta: %v", err)
	}
	if delta.Content != "纯文本" || len(delta.Parts) != 0 {
		t.Errorf("delta: got content=%s parts=%d", delta.Content, len(delta.Parts))
	}
}

func TestApplyPrefix(t *testing.T) {

	messages := []Message{{Role: "system", Content: "你是助手"}}
	ApplyPrefix(messages, "P")
	if messages[0].Content != "P你是助手" {
		t.Errorf("text prefix: got %s", messages[0].Content)
	}

	messages = []Message{{Role: "user", Parts: []Part{TextPart("问题")}}}
	ApplyPrefix(messages, "P")
	if messages[0].Parts[0].Text != "P问题" {
		t.Errorf("part prefix: got %s", messages[0].Parts[0].Text)
	}

	messages = []Message{{Role: "user", Parts: []Part{ImagePart("data:image/png;base64,A")}}}
	ApplyPrefix(messages, "P")
	if len(messages[0].Parts) != 2 || messages[0].Parts[0].Text != "P" {
		t.Errorf("image-only prefix: got %+v", messages[0].Parts)
	}

	messages = []Message{{Role: "system", Content: "x"}}
	ApplyPrefix(messages, "")
	if messages[0].Content != "x" {
		t.Errorf("empty prefix: got %s", messages[0].Content)
	}
}

func TestDataUrl(t *testing.T) {

	raw := []byte("hello")
	if got := DataUrl("application/pdf", raw); got != "data:application/pdf;base64,aGVsbG8=" {
		t.Errorf("dataurl: got %s", got)
	}

	if got := StringOfDataUrl("data:image/png;base64,QUJD"); got != "QUJD" {
		t.Errorf("extract: got %s", got)
	}

	if got := StringOfDataUrl("https://example.com/a.png"); got != "https://example.com/a.png" {
		t.Errorf("plain url: got %s", got)
	}
}

func TestModelDefaultsApply(t *testing.T) {

	defaults := ModelDefaults{
		Model:           "config-model",
		Temperature:     0.2,
		TopP:            0.9,
		MaxTokens:       100,
		ReasoningEffort: ReasoningOfHigh,
	}

	filled := &ChatRequest{}
	defaults.Apply(filled)

	if filled.Model != defaults.Model || filled.Temperature != defaults.Temperature ||
		filled.TopP != defaults.TopP || filled.MaxTokens != defaults.MaxTokens ||
		filled.ReasoningEffort != defaults.ReasoningEffort {
		t.Errorf("配置默认值未补齐：%+v", filled)
	}

	kept := &ChatRequest{
		Model:           "request-model",
		Temperature:     0.7,
		TopP:            0.5,
		MaxTokens:       10,
		ReasoningEffort: ReasoningOfLow,
	}
	defaults.Apply(kept)

	if kept.Model != "request-model" || kept.Temperature != 0.7 || kept.TopP != 0.5 ||
		kept.MaxTokens != 10 || kept.ReasoningEffort != ReasoningOfLow {
		t.Errorf("请求显式值被覆盖：%+v", kept)
	}
}

func TestStreamResponseUnmarshalError(t *testing.T) {

	var response StreamResponse
	if err := json.Unmarshal([]byte(`{"id":"1","error":{"message":"上游出错了"}}`), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Error == nil || response.Error.Error() != "api error: 上游出错了" {
		t.Errorf("object error: got %v", response.Error)
	}

	response = StreamResponse{}
	if err := json.Unmarshal([]byte(`{"error":"boom"}`), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Error == nil || response.Error.Error() != "api error: boom" {
		t.Errorf("string error should drop the quotes: got %v", response.Error)
	}

	response = StreamResponse{}
	if err := json.Unmarshal([]byte(`{"id":"1","done":true,"error":null}`), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Error != nil || !response.Done {
		t.Errorf("null error: got %v (done %v)", response.Error, response.Done)
	}
}
