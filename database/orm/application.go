package orm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

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
	mu      sync.RWMutex
	drivers map[string]*gorm.DB
}

func NewApplication() (*Database, error) {
	defaultName := DefaultName()
	defaultDriver := resolveDatabaseDriver("", defaultName)

	driver, name, err := NewDriver("", defaultName)

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

// DefaultName returns the configured default ORM connection name.
func DefaultName() string {
	return defaultDatabaseName()
}

// DefaultDriver returns the configured default ORM connection driver.
func DefaultDriver() string {
	return DriverOf(DefaultName())
}

func NewDriver(driver string, name string) (*gorm.DB, string, error) {
	driver = resolveDatabaseDriver(driver, name)

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

func defaultDatabaseName() string {
	return facades.Cfg.GetString("database.orm.default", "default")
}

// DriverOf returns the driver configured for the given ORM connection name.
func DriverOf(name string) string {
	if driver := ormConnectionString(name, "driver", ""); driver != "" {
		return driver
	}

	if name == DefaultName() {
		return facades.Cfg.GetString("database.driver")
	}

	return ""
}

func ConnectionPrefix(name string) string {
	return ormConnectionString(name, "prefix", "")
}

func resolveDatabaseDriver(driver, name string) string {
	if driver != "" {
		return driver
	}

	if value := DriverOf(name); value != "" {
		return value
	}

	if name == DefaultName() {
		for _, candidate := range []string{DriverMySQL, DriverPostgreSQL, DriverSQLite} {
			if facades.Cfg.GetString("database."+candidate+".default.driver") != "" {
				return candidate
			}
		}
	}

	return driver
}

func ormConnectionKey(name, field string) string {
	return "database.orm.connections." + name + "." + field
}

func legacyORMConnectionKey(name, field string) string {
	return "database.orm." + name + "." + field
}

func ormConnectionString(name, field, defaultValue string) string {
	if value := facades.Cfg.GetString(ormConnectionKey(name, field)); value != "" {
		return value
	}

	if value := facades.Cfg.GetString(legacyORMConnectionKey(name, field)); value != "" {
		return value
	}

	return defaultValue
}

func newMysqlClient(name string) (*gorm.DB, string, error) {

	var username, password, host, port, prefix, db, charset string

	if configDriver := DriverOf(name); configDriver != "" && configDriver != DriverMySQL {
		return nil, "", fmt.Errorf("invalid database config: mysql driver %s", configDriver)
	}

	username = ormConnectionString(name, "username", "")
	password = ormConnectionString(name, "password", "")
	host = ormConnectionString(name, "host", "")
	port = ormConnectionString(name, "port", "3306")
	prefix = ormConnectionString(name, "prefix", "")
	db = ormConnectionString(name, "db", "")
	charset = ormConnectionString(name, "charset", "utf8mb4_unicode_ci")
	log := ormConnectionString(name, "log_mode", "error")

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
			TablePrefix: prefix,
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
	if configDriver := DriverOf(name); configDriver != "" && configDriver != DriverSQLite {
		return nil, "", fmt.Errorf("invalid database config: sqlite driver %s", configDriver)
	}

	db := ormConnectionString(name, "path", facades.Cfg.GetString("database.sqlite."+name, "default.db"))

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

	if configDriver := DriverOf(name); configDriver != "" && configDriver != DriverPostgreSQL {
		return nil, "", fmt.Errorf("invalid database config: postgresql driver %s", configDriver)
	}

	username = ormConnectionString(name, "username", "")
	password = ormConnectionString(name, "password", "")
	host = ormConnectionString(name, "host", "")
	port = ormConnectionString(name, "port", "5432")
	prefix = ormConnectionString(name, "prefix", "")
	db = ormConnectionString(name, "db", "")
	sslmode = ormConnectionString(name, "sslmode", "disable")
	timezone = ormConnectionString(name, "timezone", "Asia/Shanghai")
	log := ormConnectionString(name, "log_mode", "error")

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
			TablePrefix: prefix,
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

	r.mu.RLock()
	if dri, exist := r.drivers[key]; exist {
		r.mu.RUnlock()
		return dri, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

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
