package foundation

import (
	"os"
	"time"

	"github.com/dromara/carbon/v2"
	"github.com/gookit/color"
	"github.com/herhe-com/framework/config"
	"github.com/herhe-com/framework/contracts/service"
	"github.com/herhe-com/framework/facades"
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

	if loc, err := time.LoadLocation(facades.Config().GetString("app.location")); err == nil {
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
	config := facades.Config()

	if !config.IsSet("kernel.providers") {
		return nil
	}

	providers, ok := config.Get("kernel.providers").([]service.Provider)
	if ok {
		return providers
	}

	color.Errorf("kernel.providers must be []service.Provider")
	os.Exit(1)

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
		if provider == nil {
			color.Errorf("register service provider error: provider cannot be nil")
			os.Exit(1)
		}

		if err := provider.Register(); err != nil {
			color.Errorf("register service provider error: %v", err)
			os.Exit(1)
		}
	}
}

func (a *Application) BootServiceProviders(providers []service.Provider) {

	for _, provider := range providers {
		if provider == nil {
			color.Errorf("boot service provider error: provider cannot be nil")
			os.Exit(1)
		}

		if err := provider.Boot(); err != nil {
			color.Errorf("boot service provider error: %v", err)
			os.Exit(1)
		}
	}
}

func (a *Application) setRootPath() {

	root, _ := os.Getwd()

	facades.Register[facades.RootPath](facades.RootPath(root))
}
