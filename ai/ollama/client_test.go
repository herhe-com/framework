package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/herhe-com/framework/contracts/ai"
)

func TestWireOfMessages(t *testing.T) {

	messages := []ai.Message{
		{Role: "system", Content: "你是助手"},
		{Role: "user", Parts: []ai.Part{
			ai.TextPart("看看这张图"),
			ai.ImagePart("data:image/png;base64,AAA"),
			ai.ImagePart("data:image/png;base64,BBB"),
		}},
	}

	wired, err := wireOfMessages(messages)
	if err != nil {
		t.Fatalf("wire messages: %v", err)
	}

	if len(wired) != 2 {
		t.Fatalf("wired: got %d, want 2", len(wired))
	}
	if wired[0].Content != "你是助手" || len(wired[0].Images) != 0 {
		t.Errorf("system: got %+v", wired[0])
	}
	if wired[1].Content != "看看这张图" {
		t.Errorf("user content: got %s", wired[1].Content)
	}
	if len(wired[1].Images) != 2 || wired[1].Images[0] != "AAA" || wired[1].Images[1] != "BBB" {
		t.Errorf("user images: got %+v", wired[1].Images)
	}
}

func TestWireOfMessagesOfUnsupported(t *testing.T) {

	if _, err := wireOfMessages([]ai.Message{{Role: "user", Parts: []ai.Part{ai.FilePart("doc", "data:application/pdf;base64,x")}}}); err == nil {
		t.Error("file part should be rejected")
	} else if !strings.Contains(err.Error(), "file") {
		t.Errorf("error: got %s", err)
	}
}

func TestBodyOfPutsSamplingInOptions(t *testing.T) {

	client := &Client{defaults: ai.ModelDefaults{Model: "config-model", Temperature: 0.2, TopP: 0.9, MaxTokens: 100, ReasoningEffort: ai.ReasoningOfHigh}}

	body, err := client.bodyOf(&ai.ChatRequest{Messages: []ai.Message{{Role: ai.RoleOfUser, Content: "你好"}}}, false)
	if err != nil {
		t.Fatalf("组装请求体失败：%v", err)
	}

	var received struct {
		Model       string         `json:"model"`
		Stream      bool           `json:"stream"`
		Temperature float64        `json:"temperature"`
		Think       any            `json:"think"`
		Options     map[string]any `json:"options"`
	}

	if err := json.Unmarshal(body, &received); err != nil {
		t.Fatalf("解析请求体失败：%v", err)
	}

	if received.Model != "config-model" || received.Stream {
		t.Fatalf("model/stream：%+v", received)
	}

	if received.Temperature != 0 {
		t.Errorf("采样参数不应出现在顶层：%v", received.Temperature)
	}

	if received.Options["temperature"] != 0.2 || received.Options["top_p"] != 0.9 || received.Options["num_predict"] != float64(100) {
		t.Errorf("options：%+v", received.Options)
	}

	if received.Think != ai.ReasoningOfHigh {
		t.Errorf("think：%v", received.Think)
	}

	// 布尔写法转成真正的布尔值，避免 Ollama 解成字符串
	body, err = client.bodyOf(&ai.ChatRequest{
		Messages:        []ai.Message{{Role: ai.RoleOfUser, Content: "你好"}},
		ReasoningEffort: "true",
	}, true)
	if err != nil {
		t.Fatalf("组装请求体失败：%v", err)
	}

	if err := json.Unmarshal(body, &received); err != nil {
		t.Fatalf("解析请求体失败：%v", err)
	}

	if received.Think != true {
		t.Errorf("think 布尔值：%v", received.Think)
	}
}

func TestStreamIsNotBoundedByRequestTimeout(t *testing.T) {
	server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		w.WriteHeader(nethttp.StatusOK)
		if flusher, ok := w.(nethttp.Flusher); ok {
			flusher.Flush()
		}

		// 慢于 http client 的超时，只有 Stream 使用不带超时的 client 才能收完。
		time.Sleep(100 * time.Millisecond)

		fmt.Fprintln(w, `{"model":"test","message":{"role":"assistant","content":"ok"},"done":true}`)
	}))
	defer server.Close()

	client := &Client{
		host:   server.URL,
		http:   &nethttp.Client{Timeout: 20 * time.Millisecond},
		stream: &nethttp.Client{},
	}

	channel, err := client.Stream(context.Background(), &ai.ChatRequest{})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	total := strings.Builder{}
	for chunk := range channel {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		for _, choice := range chunk.Choices {
			total.WriteString(choice.Delta.Content)
		}
	}

	if total.String() != "ok" {
		t.Errorf("stream content: got %q, want ok", total.String())
	}
}
