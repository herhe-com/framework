package validation

import "github.com/herhe-com/framework/contracts/service"

type ServiceProvider struct {
	service.Provider
}

func (p *ServiceProvider) Register() (err error) {

	NewApplication()

	return nil
}

func (p *ServiceProvider) Boot() (err error) {
	return nil
}
