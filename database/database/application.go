package database

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/gookit/color"
	"github.com/herhe-com/framework/facades"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

const DriverMySQL string = "mysql"
const DriverSQLite string = "sqlite"
const DriverPostgreSQL string = "postgresql"

type Database struct {
	driver  *gorm.DB
	drivers map[string]*gorm.DB
}

func NewApplication() (*Database, error) {

	defaultDriver := facades.Cfg.GetString("database.driver", DriverMySQL)

	driver, name, err := NewDriver(defaultDriver, "default")

	if err != nil {
		color.Errorf("[database] %s", err)
		return nil, err
	}

	drivers := make(map[string]*gorm.DB)
	drivers[defaultDriver+":"+name] = driver

	return &Database{
		drivers: drivers,
		driver:  driver,
	}, nil
}

func NewDriver(driver string, name string) (*gorm.DB, string, error) {

	switch driver {
	case DriverMySQL:
		return newMysqlClient(name)
	case DriverSQLite:
		return newSQLiteClient(name)
	case DriverPostgreSQL:
		return newPostgreSQLClient(name)
	}

	return nil, "", fmt.Errorf("invalid driver: %s", driver)
}

func newMysqlClient(name string) (*gorm.DB, string, error) {

	var username, password, host, port, prefix, db, charset string

	username = facades.Cfg.GetString("database.mysql." + name + ".username")
	password = facades.Cfg.GetString("database.mysql." + name + ".password")
	host = facades.Cfg.GetString("database.mysql." + name + ".host")
	port = facades.Cfg.GetString("database.mysql."+name+".port", "3306")
	prefix = facades.Cfg.GetString("database.mysql."+name+".prefix", "")
	db = facades.Cfg.GetString("database.mysql." + name + ".db")
	charset = facades.Cfg.GetString("database.mysql."+name+".charset", "utf8mb4_unicode_ci")
	log := facades.Cfg.GetString("database.mysql."+name+".log_mode", "error")

	if username == "" || password == "" || host == "" || db == "" {
		return nil, "", errors.New("invalid database config: mysql")
	}

	logMode := logger.Error

	switch log {
	case "error":
		logMode = logger.Error
	case "info":
		logMode = logger.Info
	case "warn":
		logMode = logger.Warn
	case "silent":
		logMode = logger.Silent
	}

	dialectal := mysql.Open(dns(username, password, host, port, prefix, db, charset))

	config := gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: facades.Cfg.GetString(prefix),
		},
		Logger:                 logger.Default.LogMode(logMode),
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	}

	if facades.Cfg.GetBool("app.debug") {
		config.PrepareStmt = false
	}

	open, err := gorm.Open(dialectal, &config)

	if err != nil {
		return nil, "", err
	}

	return open, name, nil
}

func newSQLiteClient(name string) (*gorm.DB, string, error) {

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

	if facades.Cfg.GetBool("app.debug") {
		config.Logger = logger.Default.LogMode(logger.Info)
		config.PrepareStmt = false
	}

	open, err := gorm.Open(dialectal, &config)

	if err != nil {
		return nil, "", err
	}

	return open, name, nil
}

func newPostgreSQLClient(name string) (*gorm.DB, string, error) {

	var username, password, host, port, prefix, db, sslmode, timezone string

	username = facades.Cfg.GetString("database.postgresql." + name + ".username")
	password = facades.Cfg.GetString("database.postgresql." + name + ".password")
	host = facades.Cfg.GetString("database.postgresql." + name + ".host")
	port = facades.Cfg.GetString("database.postgresql."+name+".port", "5432")
	prefix = facades.Cfg.GetString("database.postgresql."+name+".prefix", "")
	db = facades.Cfg.GetString("database.postgresql." + name + ".db")
	sslmode = facades.Cfg.GetString("database.postgresql."+name+".sslmode", "disable")
	timezone = facades.Cfg.GetString("database.postgresql."+name+".timezone", "Asia/Shanghai")
	log := facades.Cfg.GetString("database.postgresql."+name+".log_mode", "error")

	if username == "" || password == "" || host == "" || db == "" {
		return nil, "", errors.New("invalid database config: postgresql")
	}

	logMode := logger.Error

	switch log {
	case "error":
		logMode = logger.Error
	case "info":
		logMode = logger.Info
	case "warn":
		logMode = logger.Warn
	case "silent":
		logMode = logger.Silent
	}

	dialectal := postgres.Open(postgreDSN(username, password, host, port, db, sslmode, timezone))

	config := gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: facades.Cfg.GetString(prefix),
		},
		Logger:                 logger.Default.LogMode(logMode),
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	}

	if facades.Cfg.GetBool("app.debug") {
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
