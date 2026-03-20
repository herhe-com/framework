package ollama

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/herhe-com/framework/contracts/ai"
	"github.com/herhe-com/framework/facades"
)

type Client struct {
	host   string `valid:"required"`
	model  string
	prefix string
}

func NewClient(name string) (*Client, error) {

	c := &Client{
		host:   facades.Cfg.GetString("ai.ollama." + name + ".host"),
		model:  facades.Cfg.GetString("ai.ollama." + name + ".model"),
		prefix: facades.Cfg.GetString("ai.ollama." + name + ".prefix"),
	}

	if err := facades.Validator.Struct(c); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) Chat(request *ai.ChatRequest) (*ai.ChatResponse, error) {
	if request.Model == "" {
		request.Model = c.model
	}

	if c.prefix != "" {
		if len(request.Messages) > 0 {
			request.Messages[0].Content = c.prefix + request.Messages[0].Content
		}
	}

	reqBody := map[string]any{
		"model":    request.Model,
		"messages": request.Messages,
		"stream":   false,
	}
	if request.Temperature > 0 {
		reqBody["temperature"] = request.Temperature
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
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

func (c *Client) Stream(request *ai.ChatRequest) (chan *ai.StreamResponse, error) {
	if request.Model == "" {
		request.Model = c.model
	}

	if c.prefix != "" {
		if len(request.Messages) > 0 {
			request.Messages[0].Content = c.prefix + request.Messages[0].Content
		}
	}

	reqBody := map[string]any{
		"model":    request.Model,
		"messages": request.Messages,
		"stream":   true,
	}
	if request.Temperature > 0 {
		reqBody["temperature"] = request.Temperature
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
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
				ch <- &ai.StreamResponse{Done: true, Error: err}
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

			ch <- streamResp

			if ollamaResp.Done {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- &ai.StreamResponse{Done: true, Error: err}
		}
	}()

	return ch, nil
}

func (c *Client) Embedding(request *ai.EmbeddingRequest) (*ai.EmbeddingResponse, error) {
	if request.Model == "" {
		request.Model = c.model
	}

	reqBody := map[string]any{
		"model": request.Model,
		"input": request.Input,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.host+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
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

func (c *Client) Models() ([]ai.Model, error) {
	req, err := http.NewRequest("GET", c.host+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
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

func (c *Client) Dri() string {
	return ai.DriverOllama
}
