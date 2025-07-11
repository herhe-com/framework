package database

import (
	"errors"
	"fmt"
	"github.com/glebarez/sqlite"
	"github.com/gookit/color"
	"github.com/herhe-com/framework/facades"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"os"
	"path/filepath"
)

const DriverMySQL string = "mysql"
const DriverSQLite string = "sqlite"

type Database struct {
	driver  *gorm.DB
	drivers map[string]*gorm.DB
}

func NewApplication() (*Database, error) {

	defaultDriver := facades.Cfg.GetString("database.driver", DriverMySQL)

	driver, name, err := NewDriver(defaultDriver)

	if err != nil {
		color.Errorln("[database] %s\n", err)
		return nil, err
	}

	drivers := make(map[string]*gorm.DB)
	drivers[defaultDriver+":"+name] = driver

	return &Database{
		drivers: drivers,
		driver:  driver,
	}, nil
}

func NewDriver(driver string, names ...string) (*gorm.DB, string, error) {

	name := "default"

	if len(names) > 0 {
		name = names[0]
	}

	switch driver {
	case DriverMySQL:
		return newMysqlClient(name)
	case DriverSQLite:
		return newSQLiteClient(name)
	}

	return nil, "", fmt.Errorf("invalid driver: %s", driver)
}

func newMysqlClient(names ...string) (*gorm.DB, string, error) {

	name := "default"

	if len(names) > 0 {
		name = names[0]
	}

	var username, password, host, port, prefix, db, charset string

	cfg := facades.Cfg.Get("database.mysql."+name, nil)

	if cfg != nil {

		username = facades.Cfg.GetString("database.mysql." + name + ".username")
		password = facades.Cfg.GetString("database.mysql." + name + ".password")
		host = facades.Cfg.GetString("database.mysql." + name + ".host")
		port = facades.Cfg.GetString("database.mysql."+name+".port", "3306")
		prefix = facades.Cfg.GetString("database.mysql."+name+".prefix", "")
		db = facades.Cfg.GetString("database.mysql." + name + ".db")
		charset = facades.Cfg.GetString("database.mysql."+name+".charset", "utf8mb4_unicode_ci")

	} else if name == "default" {

		cfg = facades.Cfg.Get("database.mysql", nil)

		if cfg != nil {

			username = facades.Cfg.GetString("database.mysql.username")
			password = facades.Cfg.GetString("database.mysql.password")
			host = facades.Cfg.GetString("database.mysql.host")
			port = facades.Cfg.GetString("database.mysql.port", "3306")
			prefix = facades.Cfg.GetString("database.mysql.prefix", "")
			db = facades.Cfg.GetString("database.mysql.db")
			charset = facades.Cfg.GetString("database.mysql.charset", "utf8mb4_unicode_ci")
		}
	}

	if username == "" || password == "" || host == "" || db == "" {
		return nil, "", errors.New("invalid database config: mysql")
	}

	dialectal := mysql.Open(dns(username, password, host, port, prefix, db, charset))

	config := gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: facades.Cfg.GetString(prefix),
		},
		Logger:                 logger.Default.LogMode(logger.Error),
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	}

	if facades.Cfg.GetBool("server.debug") {
		config.Logger = logger.Default.LogMode(logger.Info)
		config.PrepareStmt = false
	}

	open, err := gorm.Open(dialectal, &config)

	if err != nil {
		return nil, "", err
	}

	return open, name, nil
}

func newSQLiteClient(names ...string) (*gorm.DB, string, error) {

	name := "default"

	if len(names) > 0 {
		name = names[0]
	}

	db := facades.Cfg.GetString("database.sqlite."+name, "default.db")

	path := facades.Root + db

	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, "", err
	}

	dialectal := sqlite.Open(path)

	config := gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Error),
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	}

	if facades.Cfg.GetBool("server.debug") {
		config.Logger = logger.Default.LogMode(logger.Info)
		config.PrepareStmt = false
	}

	open, err := gorm.Open(dialectal, &config)

	if err != nil {
		return nil, "", err
	}

	return open, name, nil
}

func (r *Database) Drivers(driver string, names ...string) (*gorm.DB, error) {

	name := "default"

	if len(names) > 0 {
		name = names[0]
	}

	key := driver + ":" + name

	if dri, exist := r.drivers[key]; exist {
		return dri, nil
	}

	dri, _, err := NewDriver(driver, name)

	if err != nil {
		return nil, err
	}

	r.drivers[key] = dri

	return dri, nil
}

func (r *Database) Default() *gorm.DB {
	return r.driver
}
