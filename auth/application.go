package auth

import (
	"errors"

	"github.com/casbin/casbin/v3"
	adapter "github.com/casbin/gorm-adapter/v3"
	"github.com/herhe-com/framework/database/orm"
	"github.com/herhe-com/framework/facades"
)

func NewApplication() error {

	if facades.DB.Default() == nil {
		return errors.New("请先初始化数据库")
	}

	defaultDriver := facades.Cfg.GetString("database.driver", orm.DriverMySQL)

	prefix := facades.Cfg.GetString("database." + defaultDriver + ".prefix")
	table := facades.Cfg.GetString("auth.casbin.table")

	a, err := adapter.NewAdapterByDBUseTableName(facades.DB.Default(), prefix, table)
	if err != nil {
		return err
	}

	facades.Casbin, err = casbin.NewEnforcer(facades.Root+"/conf/casbin.conf", a)
	if err != nil {
		return err
	}

	return toTrees()
}
