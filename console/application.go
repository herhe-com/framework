package console

import (
	"github.com/herhe-com/framework/console/consoles"
	"github.com/herhe-com/framework/contracts/console"
	"github.com/herhe-com/framework/facades"
	"github.com/spf13/cobra"
)

type Application struct {
}

func register() error {

	facades.Console = &cobra.Command{
		Use:     "application",
		Short:   "UPER",
		Version: "1.0.0",
	}

	app := Application{}

	app.registerBasicConsoles()

	app.registerConfiguredConsoles()

	return facades.Console.Execute()
}

func (app *Application) getBasicConsoles() []console.Provider {

	return []console.Provider{
		&consoles.PasswordProvider{},
	}
}

func (app *Application) getConfiguredConsoles() []console.Provider {

	if cons, ok := facades.Cfg.Get("server.consoles").([]console.Provider); ok {
		return cons
	}

	return nil
}

func (app *Application) registerBasicConsoles() {
	app.registerConsoles(app.getBasicConsoles())
}

func (app *Application) registerConfiguredConsoles() {
	app.registerConsoles(app.getConfiguredConsoles())
}

func (app *Application) registerConsoles(providers []console.Provider) {

	cons := make([]console.Console, len(providers))

	for index, item := range providers {
		cons[index] = item.Register()
	}

	app.parseConsoles(facades.Console, cons)
}

func (app *Application) parseConsoles(command *cobra.Command, consoles []console.Console) {

	for _, item := range consoles {

		cmd := &cobra.Command{
			Use:   item.Cmd,
			Short: item.Name,
			Long:  item.Summary,
			Run:   item.Run,
		}

		if item.Tags != nil {
			item.Tags(cmd)
		}

		if len(item.Consoles) > 0 {
			app.parseConsoles(cmd, item.Consoles)
		}

		command.AddCommand(cmd)
	}
}
