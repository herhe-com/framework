package database

import (
	"github.com/herhe-com/framework/contracts/service"
	"github.com/herhe-com/framework/facades"
)

type ServiceProvider struct {
	service.Provider
}

func (p *ServiceProvider) Register() (err error) {

	database, err := NewApplication()

	if err != nil {
		return err
	}

	facades.DB = database

	return nil
}

func (p *ServiceProvider) Boot() error {
	return nil
}
