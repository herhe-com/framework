package queue

import (
	"github.com/herhe-com/framework/contracts/queue"
	"github.com/herhe-com/framework/contracts/service"
	"github.com/herhe-com/framework/facades"
)

type ServiceProvider struct {
	service.Provider
}

func (that *ServiceProvider) Register() error {
	application, err := NewQueueWithError()
	if err != nil {
		return err
	}

	facades.Register[queue.Queue](application)
	return nil
}

func (that *ServiceProvider) Boot() error {
	return nil
}
