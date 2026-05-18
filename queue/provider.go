package queue

import (
	"github.com/herhe-com/framework/contracts/service"
	"github.com/herhe-com/framework/facades"
)

type ServiceProvider struct {
	service.Provider
}

func (that *ServiceProvider) Register() error {
	queue, err := NewQueueWithError()
	if err != nil {
		return err
	}

	facades.Queue = queue
	return nil
}

func (that *ServiceProvider) Boot() error {
	return nil
}
