package ai

const (
	DriverOpenAI   = "openai"
	DriverOllama   = "ollama"
	DriverClaude   = "claude"
	DriverGemini   = "gemini"
	DriverQianwen  = "qianwen"
	DriverZhipu    = "zhipu"
	DriverDeepSeek = "deepseek"
)

type AI interface {
	Driver
	Channel(driver string, name string) (Driver, error)
}

type Driver interface {
	Chat(request *ChatRequest) (*ChatResponse, error)
	Stream(request *ChatRequest) (chan *StreamResponse, error)
	Embedding(request *EmbeddingRequest) (*EmbeddingResponse, error)
	Models() ([]Model, error)
	Dri() string
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type StreamResponse struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
	Done    bool           `json:"done"`
	Error   error          `json:"error,omitempty"`
}

type StreamChoice struct {
	Index        int          `json:"index"`
	Delta        MessageDelta `json:"delta"`
	FinishReason string       `json:"finish_reason"`
}

type MessageDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type EmbeddingResponse struct {
	Model string      `json:"model"`
	Data  []Embedding `json:"data"`
	Usage Usage       `json:"usage"`
}

type Embedding struct {
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type Model struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	OwnedBy string `json:"owned_by"`
}
