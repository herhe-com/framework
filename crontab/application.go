package crontab

import (
	"strings"
	"time"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/crontab"
	"github.com/herhe-com/framework/facades"
	"github.com/robfig/cron/v3"
)

type Application struct {
	client *cron.Cron
}

func (app *Application) configured() []crontab.Crontab {

	if cs, ok := facades.Config().Get("crontab.functions").([]crontab.Crontab); ok {
		return cs
	}

	return nil
}

func (app *Application) register() {
	app.Register(app.configured())
}

func (app *Application) Register(tasks []crontab.Crontab) {

	for _, task := range tasks {
		key := strings.TrimSpace(task.Key())
		name := strings.TrimSpace(task.Name())
		if name == "" {
			name = key
		}
		if key == "" {
			color.Errorf("\n定时任务「%s」配置键不能为空\n", name)
			continue
		}

		configKey := "crontab.tasks." + key
		if !facades.Config().GetBool(configKey+".enable", true) {
			continue
		}
		rule := strings.TrimSpace(facades.Config().GetString(configKey + ".rule"))
		if rule == "" {
			color.Errorf("\n定时任务「%s」未配置运行频率：%s.rule\n", name, configKey)
			continue
		}

		_, err := app.client.AddFunc(rule, task.Func)

		if err != nil {
			color.Errorf("\n定时任务「%s」注册失败：%v\n", name, err)
		} else {
			color.Successf("\n定时任务「%s」注册成功\n", name)
		}
	}
}

func (app *Application) Init() {

	app.client = cron.New(
		cron.WithLocation(time.Local),
		// 上一轮未退出时跳过本轮，避免每分钟再叠一批循环。
		cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
	)

	app.register()
}

func (app *Application) Start() {
	app.client.Start()
}

func (app *Application) Restart() {

	app.client.Stop()

	app.client.Start()
}

func (app *Application) Stop() {
	app.client.Stop()
}
