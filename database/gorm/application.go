package gorm

import (
	"github.com/herhe-com/framework/facades"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func NewApplication() (err error) {

	dialectal := mysql.Open(dns())

	if facades.Gorm, err = gorm.Open(dialectal, &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: facades.Cfg.GetString("database.mysql.prefix"),
		},
		Logger:                 logger.Default.LogMode(logger.Info),
		SkipDefaultTransaction: true,
		PrepareStmt:            !facades.Cfg.GetBool("app.debug"),
	}); err != nil {
		return err
	}

	return nil
}
