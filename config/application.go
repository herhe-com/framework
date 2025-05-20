package config

import (
	"bytes"
	"github.com/herhe-com/framework/facades"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
	"os"
)

type Application struct {
	vip *viper.Viper
}

func NewApplication() (err error) {

	app := &Application{
		vip: viper.New(),
	}

	var file []byte

	if _, err = os.Stat(facades.Root + "/conf/env.yaml"); err == nil {

		if file, err = os.ReadFile(facades.Root + "/conf/env.yaml"); err != nil {
			return err
		}
	}

	if len(file) > 0 {

		app.vip.SetConfigType("yaml")

		if err = app.vip.ReadConfig(bytes.NewReader(file)); err != nil {
			return err
		}
	}

	facades.Cfg = app

	return nil
}

func (app *Application) Env(key string, defaultValue ...any) any {

	value := app.Get(key, defaultValue...)

	if cast.ToString(value) == "" {

		if len(defaultValue) > 0 {
			return defaultValue[0]
		}

		return nil
	}

	return value
}

func (app *Application) Add(name string, configuration map[string]any) {
	app.vip.Set(name, configuration)
}

func (app *Application) Set(key string, configuration any) {
	app.vip.Set(key, configuration)
}

func (app *Application) Get(key string, defaultValue ...any) any {

	if !app.vip.IsSet(key) {

		if len(defaultValue) > 0 {
			return defaultValue[0]
		}

		return nil
	}

	return app.vip.Get(key)
}

func (app *Application) GetString(key string, defaultValue ...string) string {

	if !app.vip.IsSet(key) {

		if len(defaultValue) > 0 {
			return defaultValue[0]
		}

		return ""
	}

	return app.vip.GetString(key)
}

func (app *Application) GetStrings(key string, defaultValue ...[]string) []string {

	if !app.vip.IsSet(key) {

		if len(defaultValue) > 0 {
			return defaultValue[0]
		}

		return nil
	}

	return app.vip.GetStringSlice(key)
}

func (app *Application) GetMaps(key string, defaultValue ...map[string]any) map[string]any {

	if !app.vip.IsSet(key) {

		if len(defaultValue) > 0 {
			return defaultValue[0]
		}

		return nil
	}

	return app.vip.GetStringMap(key)
}

func (app *Application) GetInt(key string, defaultValue ...int) int {

	if !app.vip.IsSet(key) {

		if len(defaultValue) > 0 {
			return defaultValue[0]
		}

		return 0
	}

	return app.vip.GetInt(key)
}

func (app *Application) GetInt64(key string, defaultValue ...int64) int64 {

	if !app.vip.IsSet(key) {

		if len(defaultValue) > 0 {
			return defaultValue[0]
		}

		return 0
	}

	return app.vip.GetInt64(key)
}

func (app *Application) GetBool(key string, defaultValue ...bool) bool {

	if !app.vip.IsSet(key) {

		if len(defaultValue) > 0 {
			return defaultValue[0]
		}

		return false
	}

	return app.vip.GetBool(key)
}
