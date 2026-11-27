package ai

import (
	"time"

	"github.com/spf13/cast"

	"github.com/herhe-com/framework/facades"
)

// channelConfig 是 ai.channels.<name> 里一个渠道的配置。
//
// 字段按驱动取用:openai/anthropic/gemini 用 api_key,openai/anthropic 另有 base_url,
// ollama 用 host;model、prefix、timeout、temperature、top_p、max_tokens、reasoning_effort
// 对所有驱动生效。请求里没设置的模型参数用这里的值补齐。
type channelConfig struct {
	Name            string
	Driver          string
	APIKey          string
	BaseURL         string
	Host            string
	Model           string
	Prefix          string
	Timeout         time.Duration
	Temperature     float64
	TopP            float64
	MaxTokens       int
	ReasoningEffort string
	ResponseFormat  string
}

// configOfChannel 读取一个渠道的配置;渠道不存在时各字段为空,由驱动校验报错。
func configOfChannel(name string) channelConfig {

	path := "ai.channels." + name
	cfg := facades.Config()

	return channelConfig{
		Name:            name,
		Driver:          cfg.GetString(path + ".driver"),
		APIKey:          cfg.GetString(path + ".api_key"),
		BaseURL:         cfg.GetString(path + ".base_url"),
		Host:            cfg.GetString(path + ".host"),
		Model:           cfg.GetString(path + ".model"),
		Prefix:          cfg.GetString(path + ".prefix"),
		Timeout:         time.Duration(cfg.GetInt(path+".timeout", 300)) * time.Second,
		Temperature:     cast.ToFloat64(cfg.Get(path + ".temperature")),
		TopP:            cast.ToFloat64(cfg.Get(path + ".top_p")),
		MaxTokens:       cfg.GetInt(path+".max_tokens", 0),
		ReasoningEffort: cfg.GetString(path + ".reasoning_effort"),
		ResponseFormat:  cfg.GetString(path + ".response_format"),
	}
}
