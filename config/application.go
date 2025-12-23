package config

import (
	"bytes"
	"os"
	"time"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/facades"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

type Application struct {
	vip *viper.Viper
}

func NewApplication() (err error) {

	app := &Application{
		vip: viper.New(),
	}

	app.vip.SetConfigType("yaml")

	if _, err = os.Stat(facades.Root + "/conf/env.yaml"); err == nil {

		var file []byte

		if file, err = os.ReadFile(facades.Root + "/conf/env.yaml"); err != nil {
			return err
		}

		if len(file) > 0 {
			if err = app.vip.ReadConfig(bytes.NewReader(file)); err != nil {
				return err
			}
		}
	} else {
		// 从环境变量中获取配置

		provider := os.Getenv("HH_CFG_PROVIDER")
		endpoint := os.Getenv("HH_CFG_ENDPOINT")
		path := os.Getenv("HH_CFG_PATH")
		watch := os.Getenv("HH_CFG_WATCH")
		secret := os.Getenv("HH_CFG_SECRET")

		if watch == "" {
			watch = "true"
		}

		if provider != "" && endpoint != "" && path != "" {

			if secret != "" {
				if err = app.vip.AddSecureRemoteProvider(provider, endpoint, path, secret); err != nil {
					return err
				}
			} else if err = app.vip.AddRemoteProvider(provider, endpoint, path); err != nil {
				return err
			}

			if err = app.vip.ReadRemoteConfig(); err != nil {
				return err
			}

			if watch == "true" {

				go func() {
					for {
						time.Sleep(time.Second * 5) // delay after each request

						// currently, only tested with etcd support
						err = app.vip.WatchRemoteConfig()

						if err != nil {
							color.Errorf("unable to read remote config: %v", err)
							continue
						}
					}
				}()
			}
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

func (app *Application) IsSet(key string) bool {
	return app.IsSet(key)
}
