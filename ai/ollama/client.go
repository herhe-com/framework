package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/herhe-com/framework/contracts/ai"
	"github.com/herhe-com/framework/facades"
	"github.com/spf13/cast"
)

type Client struct {
	host     string `validate:"required"`
	defaults ai.ModelDefaults
	// http bounds a whole request, response body included. Stream uses stream
	// instead: a long response stream would otherwise be cut off mid-flight by
	// that timeout, so its lifetime belongs to the caller's context.
	http   *http.Client
	stream *http.Client
	prefix string
}

func NewClient(name string) (*Client, error) {

	timeout := time.Duration(facades.Config().GetInt("ai.ollama."+name+".timeout", 300)) * time.Second

	c := &Client{
		host:   facades.Config().GetString("ai.ollama." + name + ".host"),
		prefix: facades.Config().GetString("ai.ollama." + name + ".prefix"),
		// 模型参数默认值，请求未设置（0 或空字符串）时生效
		defaults: ai.ModelDefaults{
			Model:           facades.Config().GetString("ai.ollama." + name + ".model"),
			Temperature:     cast.ToFloat64(facades.Config().Get("ai.ollama." + name + ".temperature")),
			TopP:            cast.ToFloat64(facades.Config().Get("ai.ollama." + name + ".top_p")),
			MaxTokens:       facades.Config().GetInt("ai.ollama."+name+".max_tokens", 0),
			ReasoningEffort: facades.Config().GetString("ai.ollama." + name + ".reasoning_effort"),
		},
		http:   &http.Client{Timeout: timeout},
		stream: &http.Client{},
	}

	if err := facades.Validator().Struct(c); err != nil {
		return nil, err
	}

	return c, nil
}

func (r *Client) Chat(c context.Context, request *ai.ChatRequest) (*ai.ChatResponse, error) {

	body, err := r.bodyOf(request, false)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(c, "POST", r.host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error: %s", string(body))
	}

	var ollamaResp struct {
		Model     string     `json:"model"`
		CreatedAt string     `json:"created_at"`
		Message   ai.Message `json:"message"`
		Done      bool       `json:"done"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &ai.ChatResponse{
		ID:    ollamaResp.CreatedAt,
		Model: ollamaResp.Model,
		Choices: []ai.Choice{
			{
				Index:        0,
				Message:      ollamaResp.Message,
				FinishReason: "stop",
			},
		},
	}, nil
}

func (r *Client) Stream(c context.Context, request *ai.ChatRequest) (chan *ai.StreamResponse, error) {

	body, err := r.bodyOf(request, true)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(c, "POST", r.host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

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
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}

			var ollamaResp struct {
				Model     string `json:"model"`
				CreatedAt string `json:"created_at"`
				Message   struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				Done bool `json:"done"`
			}

			if err := json.Unmarshal([]byte(line), &ollamaResp); err != nil {
				ai.Send(c, ch, &ai.StreamResponse{Done: true, Error: err})
				return
			}

			streamResp := &ai.StreamResponse{
				ID:    ollamaResp.CreatedAt,
				Model: ollamaResp.Model,
				Choices: []ai.StreamChoice{
					{
						Index: 0,
						Delta: ai.MessageDelta{
							Role:    ollamaResp.Message.Role,
							Content: ollamaResp.Message.Content,
						},
					},
				},
				Done: ollamaResp.Done,
			}

			if !ai.Send(c, ch, streamResp) {
				return
			}

			if ollamaResp.Done {
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

	reqBody := map[string]any{
		"model": request.Model,
		"input": request.Input,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(c, "POST", r.host+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error: %s", string(body))
	}

	var ollamaResp struct {
		Model      string      `json:"model"`
		Embeddings [][]float64 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	embeddings := make([]ai.Embedding, len(ollamaResp.Embeddings))
	for i, emb := range ollamaResp.Embeddings {
		embeddings[i] = ai.Embedding{
			Index:     i,
			Embedding: emb,
		}
	}

	return &ai.EmbeddingResponse{
		Model: ollamaResp.Model,
		Data:  embeddings,
	}, nil
}

func (r *Client) Models(c context.Context) ([]ai.Model, error) {
	req, err := http.NewRequestWithContext(c, "GET", r.host+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error: %s", string(body))
	}

	var ollamaResp struct {
		Models []struct {
			Name       string `json:"name"`
			Model      string `json:"model"`
			ModifiedAt string `json:"modified_at"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]ai.Model, len(ollamaResp.Models))
	for i, m := range ollamaResp.Models {
		models[i] = ai.Model{
			ID:      m.Name,
			Name:    m.Model,
			OwnedBy: "ollama",
		}
	}

	return models, nil
}

func (r *Client) Dri() string {
	return "ollama"
}

// bodyOf 组装 /api/chat 请求体。Ollama 的采样参数放在 options 里，为 0 时不下发。
func (r *Client) bodyOf(request *ai.ChatRequest, stream bool) ([]byte, error) {

	r.defaults.Apply(request)

	ai.ApplyPrefix(request.Messages, r.prefix)

	wired, err := wireOfMessages(request.Messages)
	if err != nil {
		return nil, err
	}

	reqBody := map[string]any{
		"model":    request.Model,
		"messages": wired,
		"stream":   stream,
	}

	options := map[string]any{}

	if request.Temperature > 0 {
		options["temperature"] = request.Temperature
	}

	if request.TopP > 0 {
		options["top_p"] = request.TopP
	}

	if request.MaxTokens > 0 {
		options["num_predict"] = request.MaxTokens
	}

	if len(options) > 0 {
		reqBody["options"] = options
	}

	// Ollama 的思考开关叫 think，既接受布尔值也接受 low/medium/high 等级。
	if request.ReasoningEffort != "" {
		if think, err := strconv.ParseBool(request.ReasoningEffort); err == nil {
			reqBody["think"] = think
		} else {
			reqBody["think"] = request.ReasoningEffort
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	return body, nil
}

// wiredMessage Ollama /api/chat 的消息报文。
// Ollama 协议不接受内容块数组，多模态内容需转换：图片进 images 字段，文件不支持。
type wiredMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

// wireOfMessages 将多模态消息转换为 Ollama 报文。
func wireOfMessages(messages []ai.Message) ([]wiredMessage, error) {
	wired := make([]wiredMessage, 0, len(messages))

	for i := range messages {
		message := messages[i]

		texts := []string{}
		if message.Content != "" {
			texts = append(texts, message.Content)
		}

		out := wiredMessage{Role: message.Role}
		for j := range message.Parts {
			part := message.Parts[j]

			switch part.Type {
			case ai.PartOfText:
				if part.Text != "" {
					texts = append(texts, part.Text)
				}
			case ai.PartOfImageURL:
				if part.ImageURL == nil || part.ImageURL.Url == "" {
					return nil, fmt.Errorf("invalid image url in message %d", i)
				}
				out.Images = append(out.Images, ai.StringOfDataUrl(part.ImageURL.Url))
			case ai.PartOfFile:
				return nil, fmt.Errorf("ollama does not support file content")
			default:
				return nil, fmt.Errorf("unsupported content `%s` in message %d", part.Type, i)
			}
		}

		out.Content = strings.Join(texts, "\n")
		wired = append(wired, out)
	}

	return wired, nil
}
