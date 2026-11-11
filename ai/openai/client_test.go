package openai

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

func TestChatAndStreamOfClient(t *testing.T) {

	var received ai.ChatRequest

	server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)

		w.Header().Set("Content-Type", "application/json")

		// 流式：内容为特定文字时回 SSE 事件；大行超过默认 64KB 缓冲上限，验证自定义 Scanner 缓冲
		if len(received.Messages) > 0 && received.Messages[0].Content == "流式" {
			big := strings.Repeat("段", 70_000)
			fmt.Fprintln(w, `data: {"id":"1","model":"test","choices":[{"index":0,"delta":{"role":"assistant","content":"你好"}}]}`)
			fmt.Fprintln(w)
			fmt.Fprintf(w, "data: {\"id\":\"1\",\"model\":\"test\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"%s\"}}]}\n\n", big)
			fmt.Fprintln(w, `data: {"error":{"message":"上游出错了"}}`)
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: [DONE]`)
			return
		}

		reply := `{"id":"2","model":"test","choices":[{"index":0,"message":{"role":"assistant","content":[{"type":"text","text":"看图："},{"type":"image_url","image_url":{"url":"x.png"}}]},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`
		_, _ = w.Write([]byte(reply))
	}))
	defer server.Close()

	client := &Client{apiKey: "key", baseURL: server.URL, http: server.Client(), stream: server.Client()}

	// Chat：请求侧纯文本与多模态消息一并序列化，响应内容为数组时回填 Parts
	_, _ = client.Chat(context.Background(), &ai.ChatRequest{Messages: []ai.Message{
		{Role: ai.RoleOfUser, Content: "你好"},
		{Role: ai.RoleOfUser, Parts: []ai.Part{ai.TextPart("看图"), ai.ImagePart("data:image/png;base64,AAA")}},
	}})

	if len(received.Messages) != 2 {
		t.Fatalf("received messages: got %d", len(received.Messages))
	}
	if received.Messages[0].Content != "你好" {
		t.Errorf("first content: got %s", received.Messages[0].Content)
	}
	if len(received.Messages[1].Parts) != 2 {
		t.Errorf("parts should be array: got %+v", received.Messages[1].Parts)
	}

	chat, err := client.Chat(context.Background(), &ai.ChatRequest{Messages: []ai.Message{{Role: ai.RoleOfUser, Content: "看图"}}})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if reply := chat.Choices[0].Message; reply.Content != "看图：" || len(reply.Parts) != 2 || reply.Parts[1].ImageURL.Url != "x.png" {
		t.Errorf("chat reply: got %s / %+v", reply.Content, reply.Parts)
	}

	// Stream：delta 逐段下发、大行解析、流内错误事件与 Done 收尾
	channel, err := client.Stream(context.Background(), &ai.ChatRequest{Messages: []ai.Message{{Role: ai.RoleOfUser, Content: "流式"}}})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	total := strings.Builder{}
	errorGot := false

	for chunk := range channel {
		if chunk.Error != nil {
			if !strings.Contains(chunk.Error.Error(), "上游出错了") {
				t.Errorf("stream error: got %v", chunk.Error)
			}
			errorGot = true
			break
		}
		for _, choice := range chunk.Choices {
			total.WriteString(choice.Delta.Content)
		}
	}

	if !errorGot {
		t.Errorf("stream should stop on the in-band error event")
	}
	if !strings.HasPrefix(total.String(), "你好") {
		t.Errorf("stream prefix: got %.10s", total.String())
	}
	if length := len([]rune(total.String())); length != 70_002 {
		t.Errorf("stream total length: got %d", length)
	}
}

func TestStreamIsNotBoundedByRequestTimeout(t *testing.T) {
	server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(nethttp.StatusOK)
		if flusher, ok := w.(nethttp.Flusher); ok {
			flusher.Flush()
		}

		// 慢于 http client 的超时，只有 Stream 使用不带超时的 client 才能收完。
		time.Sleep(100 * time.Millisecond)

		fmt.Fprintln(w, `data: {"id":"1","model":"test","choices":[{"index":0,"delta":{"content":"ok"}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: [DONE]`)
	}))
	defer server.Close()

	client := &Client{
		apiKey:  "key",
		baseURL: server.URL,
		http:    &nethttp.Client{Timeout: 20 * time.Millisecond},
		stream:  &nethttp.Client{},
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
