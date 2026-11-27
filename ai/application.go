package ai

import (
	"sync"

	"github.com/gookit/color"

	contractai "github.com/herhe-com/framework/contracts/ai"
	"github.com/herhe-com/framework/facades"
)

// AI 是模型服务的入口:启动时按 ai.default 建默认渠道,其余渠道取用时再构造。
type AI struct {
	contractai.Driver

	name     string
	mutex    sync.Mutex
	channels map[string]contractai.Driver
}

// NewAI 读取 ai.default 并建默认渠道;没配置渠道时返回 nil,由调用方判定 AI 不可用。
func NewAI() *AI {

	name := facades.Config().GetString("ai.default")
	if name == "" {
		color.Errorln("[ai] please set default channel (ai.default)")
		return nil
	}

	driver, err := NewChannel(name)
	if err != nil {
		color.Errorf("[ai] %s", err)
		return nil
	}

	return &AI{
		Driver:   driver,
		name:     name,
		channels: map[string]contractai.Driver{name: driver},
	}
}

// Channel 返回指定渠道,首次取用时构造并缓存,同一渠道只初始化一次。
func (r *AI) Channel(name string) (contractai.Driver, error) {

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if driver, exists := r.channels[name]; exists {
		return driver, nil
	}

	driver, err := NewChannel(name)
	if err != nil {
		return nil, err
	}

	r.channels[name] = driver

	return driver, nil
}
