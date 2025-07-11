package foundation

import (
	"github.com/dromara/carbon/v2"
	"github.com/gookit/color"
	"github.com/herhe-com/framework/config"
	"github.com/herhe-com/framework/contracts/service"
	"github.com/herhe-com/framework/facades"
	"os"
	"time"
)

func init() {

	application := Application{}

	application.setRootPath()

	application.registerBasicServiceProviders()
	application.bootBasicServiceProviders()
}

type Application struct {
}

func (a *Application) Boot() {

	a.SetLocation()

	a.registerConfiguredServiceProviders()
	a.bootConfiguredServiceProviders()
}

func (a *Application) SetLocation() {

	if loc, err := time.LoadLocation(facades.Cfg.GetString("server.location")); err == nil {
		time.Local = loc
		carbon.SetLocation(loc)
	}
}

func (a *Application) getBasicServiceProviders() []service.Provider {
	return []service.Provider{
		&config.ServiceProvider{},
	}
}

func (a *Application) getConfiguredServiceProviders() []service.Provider {

	if providers, ok := facades.Cfg.Get("server.providers").([]service.Provider); ok {
		return providers
	}

	return nil
}

func (a *Application) registerBasicServiceProviders() {
	a.RegisterServiceProviders(a.getBasicServiceProviders())
}

func (a *Application) bootBasicServiceProviders() {
	a.BootServiceProviders(a.getBasicServiceProviders())
}

func (a *Application) registerConfiguredServiceProviders() {
	a.RegisterServiceProviders(a.getConfiguredServiceProviders())
}

func (a *Application) bootConfiguredServiceProviders() {
	a.BootServiceProviders(a.getConfiguredServiceProviders())
}

func (a *Application) RegisterServiceProviders(providers []service.Provider) {

	for _, provider := range providers {
		if err := provider.Register(); err != nil {
			color.Errorf("register service provider error: %v", err)
			os.Exit(0)
		}
	}
}

func (a *Application) BootServiceProviders(providers []service.Provider) {

	for _, provider := range providers {
		if err := provider.Boot(); err != nil {
			color.Errorf("boot service provider error: %v", err)
			os.Exit(0)
		}
	}
}

func (a *Application) setRootPath() {

	root, _ := os.Getwd()

	facades.Root = root
}
