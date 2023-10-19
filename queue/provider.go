package queue

import (
	"github.com/herhe-com/framework/contracts/service"
	"github.com/herhe-com/framework/facades"
)

type ServiceProvider struct {
	service.Provider
}

func (that *ServiceProvider) Register() error {
	facades.Queue = NewQueue()
	return nil
}

func (that *ServiceProvider) Boot() error {
	return nil
}
