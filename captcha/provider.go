package captcha

import (
	contractcaptcha "github.com/herhe-com/framework/contracts/captcha"
	"github.com/herhe-com/framework/contracts/service"
	"github.com/herhe-com/framework/facades"
)

// ServiceProvider registers the captcha application.
type ServiceProvider struct {
	service.Provider
}

// Register registers the captcha application in the service container.
func (r *ServiceProvider) Register() error {
	application, err := NewCaptchaWithError()
	if err != nil {
		return err
	}

	facades.Register[contractcaptcha.Application](application)
	return nil
}

// Boot boots the captcha service provider.
func (r *ServiceProvider) Boot() error {
	return nil
}
