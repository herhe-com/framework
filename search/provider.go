package search

import (
	"github.com/herhe-com/framework/contracts/service"
	"github.com/herhe-com/framework/facades"
)

type ServiceProvider struct {
	service.Provider
}

func (that *ServiceProvider) Register() error {
	search, err := NewSearchWithError()
	if err != nil {
		return err
	}

	facades.Search = search
	return nil
}

func (that *ServiceProvider) Boot() error {
	return nil
}
