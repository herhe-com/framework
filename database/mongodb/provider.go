package mongodb

import (
	"github.com/herhe-com/framework/contracts/mongodb"
	"github.com/herhe-com/framework/contracts/service"
	"github.com/herhe-com/framework/facades"
)

type ServiceProvider struct {
	service.Provider
}

func (p *ServiceProvider) Register() (err error) {

	application, err := NewApplication()

	if err != nil {
		return err
	}

	facades.Register[mongodb.Mongo](application)

	return nil
}

func (p *ServiceProvider) Boot() error {
	return nil
}
