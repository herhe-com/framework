package openai

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
	apiKey  string `valid:"required"`
	baseURL string
	model   string
	prefix  string
}

func NewClient(name string) (*Client, error) {

	c := &Client{
		apiKey:  facades.Config().GetString("ai.openai." + name + ".api_key"),
		baseURL: facades.Config().GetString("ai.openai." + name + ".base_url"),
		model:   facades.Config().GetString("ai.openai." + name + ".model"),
		prefix:  facades.Config().GetString("ai.openai." + name + ".prefix"),
	}

	if c.baseURL == "" {
		c.baseURL = "https://api.openai.com/v1"
	}

	if err := facades.Validator().Struct(c); err != nil {
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

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := http.DefaultClient.Do(req)
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

func (c *Client) Stream(request *ai.ChatRequest) (chan *ai.StreamResponse, error) {
	if request.Model == "" {
		request.Model = c.model
	}

	if c.prefix != "" {
		if len(request.Messages) > 0 {
			request.Messages[0].Content = c.prefix + request.Messages[0].Content
		}
	}

	request.Stream = true

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

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
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- &ai.StreamResponse{Done: true}
				return
			}

			var streamResp ai.StreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				ch <- &ai.StreamResponse{Done: true, Error: err}
				return
			}

			ch <- &streamResp
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

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := http.DefaultClient.Do(req)
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

func (c *Client) Models() ([]ai.Model, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := http.DefaultClient.Do(req)
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

func (c *Client) Dri() string {
	return ai.DriverOpenAI
}
