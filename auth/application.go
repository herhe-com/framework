package auth

import (
	"errors"
	"github.com/casbin/casbin/v2"
	adapter "github.com/casbin/gorm-adapter/v3"
	"github.com/herhe-com/framework/facades"
)

func NewApplication() error {

	if facades.Gorm == nil {
		return errors.New("请先初始化 GORM")
	}

	prefix := facades.Cfg.GetString("database.mysql.prefix")
	table := facades.Cfg.GetString("auth.casbin.table")

	a, err := adapter.NewAdapterByDBUseTableName(facades.Gorm, prefix, table)
	if err != nil {
		return err
	}

	facades.Casbin, err = casbin.NewEnforcer(facades.Root+"/conf/casbin.conf", a)
	if err != nil {
		return err
	}

	return nil
}
