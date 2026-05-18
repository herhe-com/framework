package filesystem

import (
	"github.com/herhe-com/framework/contracts/service"
	"github.com/herhe-com/framework/facades"
)

type ServiceProvider struct {
	service.Provider
}

func (that *ServiceProvider) Register() error {
	storage, err := NewStorageWithError()
	if err != nil {
		return err
	}

	facades.Storage = storage
	return nil
}

func (that *ServiceProvider) Boot() error {
	return nil
}
