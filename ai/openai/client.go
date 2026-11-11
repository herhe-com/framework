package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/herhe-com/framework/contracts/ai"
	"github.com/herhe-com/framework/facades"
	"github.com/spf13/cast"
)

// defaultBaseURL is the endpoint used when base_url is not configured.
const defaultBaseURL = "https://api.openai.com/v1"

type Client struct {
	apiKey   string `validate:"required"`
	baseURL  string `validate:"required"`
	defaults ai.ModelDefaults
	// http bounds a whole request, response body included. Stream uses stream
	// instead: a long SSE response would otherwise be cut off mid-flight by
	// that timeout, so its lifetime belongs to the caller's context.
	http   *http.Client
	stream *http.Client
	prefix string
}

func NewClient(name string) (*Client, error) {

	timeout := time.Duration(facades.Config().GetInt("ai.openai."+name+".timeout", 300)) * time.Second

	c := &Client{
		apiKey:  facades.Config().GetString("ai.openai." + name + ".api_key"),
		baseURL: facades.Config().GetString("ai.openai." + name + ".base_url"),
		prefix:  facades.Config().GetString("ai.openai." + name + ".prefix"),
		// 模型参数默认值，请求未设置（0 或空字符串）时生效
		defaults: ai.ModelDefaults{
			Model:           facades.Config().GetString("ai.openai." + name + ".model"),
			Temperature:     cast.ToFloat64(facades.Config().Get("ai.openai." + name + ".temperature")),
			TopP:            cast.ToFloat64(facades.Config().Get("ai.openai." + name + ".top_p")),
			MaxTokens:       facades.Config().GetInt("ai.openai."+name+".max_tokens", 0),
			ReasoningEffort: facades.Config().GetString("ai.openai." + name + ".reasoning_effort"),
		},
		http:   &http.Client{Timeout: timeout},
		stream: &http.Client{},
	}

	// base_url keeps its historical default: required and a default value are
	// not mutually exclusive, the default just has to be filled before
	// validation runs.
	if c.baseURL == "" {
		c.baseURL = defaultBaseURL
	}

	if err := facades.Validator().Struct(c); err != nil {
		return nil, err
	}

	return c, nil
}

func (r *Client) Chat(c context.Context, request *ai.ChatRequest) (*ai.ChatResponse, error) {

	r.defaults.Apply(request)

	ai.ApplyPrefix(request.Messages, r.prefix)

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(c, "POST", r.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error: %s", string(body))
	}

	var response ai.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &response, nil
}

func (r *Client) Stream(c context.Context, request *ai.ChatRequest) (chan *ai.StreamResponse, error) {

	r.defaults.Apply(request)

	ai.ApplyPrefix(request.Messages, r.prefix)

	request.Stream = true

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(c, "POST", r.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.stream.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("api error: %s", string(body))
	}

	ch := make(chan *ai.StreamResponse)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		// 单行事件可能包含整段回复，默认 64KB 上限不足，放大到 16MB。
		scanner.Buffer(make([]byte, 0, 64*1024), 16<<20)

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ai.Send(c, ch, &ai.StreamResponse{Done: true})
				return
			}

			var streamResp ai.StreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				ai.Send(c, ch, &ai.StreamResponse{Done: true, Error: fmt.Errorf("decode stream: %w", err)})
				return
			}

			if !ai.Send(c, ch, &streamResp) {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			ai.Send(c, ch, &ai.StreamResponse{Done: true, Error: err})
		}
	}()

	return ch, nil
}

func (r *Client) Embedding(c context.Context, request *ai.EmbeddingRequest) (*ai.EmbeddingResponse, error) {
	if request.Model == "" {
		request.Model = r.defaults.Model
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(c, "POST", r.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error: %s", string(body))
	}

	var response ai.EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &response, nil
}

func (r *Client) Models(c context.Context) ([]ai.Model, error) {
	req, err := http.NewRequestWithContext(c, "GET", r.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error: %s", string(body))
	}

	var response struct {
		Data []ai.Model `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return response.Data, nil
}

func (r *Client) Dri() string {
	return "openai"
}
