package gorm

import (
	"fmt"
	"github.com/herhe-com/framework/facades"
)

func dns() string {

	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s%s?charset=%s&parseTime=true&loc=Local",
		facades.Cfg.GetString("database.mysql.username"),
		facades.Cfg.GetString("database.mysql.password"),
		facades.Cfg.GetString("database.mysql.host"),
		facades.Cfg.GetString("database.mysql.port"),
		facades.Cfg.GetString("database.mysql.prefix"),
		facades.Cfg.GetString("database.mysql.db"),
		facades.Cfg.GetString("database.mysql.charset"))
}
