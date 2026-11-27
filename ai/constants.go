package ai

// 驱动名。渠道的 driver 字段取以下值,决定走哪家协议。
//
// 其余兼容 OpenAI 协议的服务(DeepSeek、通义千问、智谱、vLLM、各类网关等)
// 统一用 openai 驱动 + base_url 接入,不再各建一个驱动。
const (
	DriverOpenAI    = "openai"    // OpenAI 及 /chat/completions 兼容服务
	DriverOllama    = "ollama"    // Ollama 本地或自建服务
	DriverResponses = "responses" // OpenAI Responses API(eino 暂无对应组件,暂未实现)
	DriverAnthropic = "anthropic" // Anthropic Claude
	DriverGemini    = "gemini"    // Google Gemini
)
