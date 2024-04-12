package crontab

import (
	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/crontab"
	"github.com/herhe-com/framework/facades"
	"github.com/robfig/cron/v3"
	"time"
)

type Application struct {
	client *cron.Cron
}

func (app *Application) configured() []crontab.Crontab {

	if cs, ok := facades.Cfg.Get("server.crontab").([]crontab.Crontab); ok {
		return cs
	}

	return nil
}

func (app *Application) register() {
	app.Register(app.configured())
}

func (app *Application) Register(crontab []crontab.Crontab) {

	for _, item := range crontab {

		_, err := app.client.AddFunc(item.Rule(), item.Func)

		if err != nil {
			color.Errorf("\n定时任务「%s」运行失败：%v\n", item.Name(), err)
		} else {
			color.Successf("\n定时任务「%s」运行成功\n", item.Name())
		}
	}
}

func (app *Application) Init() {

	app.client = cron.New(cron.WithLocation(time.Local))

	app.register()
}

func (app *Application) Start() {

	app.client.Start()
}

func (app *Application) Stop() {
	app.client.Stop()
}
