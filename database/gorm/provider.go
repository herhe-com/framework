package gorm

import "github.com/herhe-com/framework/contracts/service"

type ServiceProvider struct {
	service.Provider
}

func (p *ServiceProvider) Register() (err error) {

	if err = NewApplication(); err != nil {
		return err
	}

	return nil
}

func (p *ServiceProvider) Boot() error {
	return nil
}
