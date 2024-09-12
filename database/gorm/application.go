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

	cfg := gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: facades.Cfg.GetString("database.mysql.prefix"),
		},
		Logger:                 logger.Default.LogMode(logger.Error),
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	}

	if facades.Cfg.GetBool("server.debug") {
		cfg.Logger = logger.Default.LogMode(logger.Info)
		cfg.PrepareStmt = false
	}

	if facades.Gorm, err = gorm.Open(dialectal, &cfg); err != nil {
		return err
	}

	return nil
}
