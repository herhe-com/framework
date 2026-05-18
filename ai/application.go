package ai

import (
	"fmt"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/ai/ollama"
	"github.com/herhe-com/framework/ai/openai"
	contractai "github.com/herhe-com/framework/contracts/ai"
	"github.com/herhe-com/framework/facades"
)

type AI struct {
	contractai.Driver
	drivers map[string]contractai.Driver
}

func NewAI() *AI {

	defaultDriver := facades.Config().GetString("ai.driver")

	if defaultDriver == "" {
		color.Errorln("[ai] please set default driver")
		return nil
	}

	driver, err := NewDriver(defaultDriver, "default")

	if err != nil {
		color.Errorf("[ai] %s", err)
		return nil
	}

	drivers := make(map[string]contractai.Driver)
	key := fmt.Sprintf("%s_%s", defaultDriver, "default")
	drivers[key] = driver

	return &AI{
		drivers: drivers,
		Driver:  driver,
	}
}

func NewDriver(driver string, name string) (contractai.Driver, error) {

	switch driver {
	case DriverOpenAI:
		return openai.NewClient(name)
	case DriverOllama:
		return ollama.NewClient(name)
	case DriverClaude:
		return nil, fmt.Errorf("claude driver not implemented yet")
	case DriverGemini:
		return nil, fmt.Errorf("gemini driver not implemented yet")
	case DriverQianwen:
		return nil, fmt.Errorf("qianwen driver not implemented yet")
	case DriverZhipu:
		return nil, fmt.Errorf("zhipu driver not implemented yet")
	case DriverDeepSeek:
		return nil, fmt.Errorf("deepseek driver not implemented yet")
	}

	return nil, fmt.Errorf("invalid driver: %s", driver)
}

func (r *AI) Channel(driver string, name string) (contractai.Driver, error) {

	key := fmt.Sprintf("%s_%s", driver, name)

	if dri, exist := r.drivers[key]; exist {
		return dri, nil
	}

	dri, err := NewDriver(driver, name)
	if err != nil {
		return nil, err
	}

	r.drivers[key] = dri

	return dri, nil
}
