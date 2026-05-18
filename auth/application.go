package auth

import (
	"errors"

	"github.com/casbin/casbin/v3"
	adapter "github.com/casbin/gorm-adapter/v3"
	"github.com/herhe-com/framework/database/orm"
	"github.com/herhe-com/framework/facades"
)

func NewApplication() error {

	if facades.Database().Default() == nil {
		return errors.New("请先初始化数据库")
	}

	connectionName := facades.Config().GetString("auth.casbin.database", orm.DefaultName())
	prefix := orm.ConnectionPrefix(connectionName)
	table := facades.Config().GetString("auth.casbin.table")

	a, err := adapter.NewAdapterByDBUseTableName(facades.Database().Default(), prefix, table)
	if err != nil {
		return err
	}

	enforcer, err := casbin.NewEnforcer(facades.Root()+"/conf/casbin.conf", a)
	if err != nil {
		return err
	}

	facades.Register[*casbin.Enforcer](enforcer)

	return toTrees()
}
