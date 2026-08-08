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
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

const DriverMySQL string = "mysql"
const DriverSQLite string = "sqlite"
const DriverPostgreSQL string = "postgresql"
const DriverSQLServer string = "sqlserver"

type Database struct {
	driver  *gorm.DB
	mu      sync.RWMutex
	drivers map[string]*gorm.DB
}

func NewApplication() (*Database, error) {
	defaultName := DefaultName()
	driver, err := NewDriver(defaultName)

	if err != nil {
		color.Errorf("[database] %s", err)
		return nil, err
	}

	drivers := make(map[string]*gorm.DB)
	drivers[defaultName] = driver

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

// NewDriver creates an ORM driver from the given connection's configuration.
func NewDriver(name string) (*gorm.DB, error) {
	driver := DriverOf(name)
	if driver == "" {
		return nil, fmt.Errorf("please set driver for connection: %s", name)
	}

	switch driver {
	case DriverMySQL:
		db, _, err := newMysqlClient(name)
		return db, err
	case DriverSQLite:
		db, _, err := newSQLiteClient(name)
		return db, err
	case DriverPostgreSQL:
		db, _, err := newPostgreSQLClient(name)
		return db, err
	case DriverSQLServer:
		db, _, err := newSQLServerClient(name)
		return db, err
	}

	return nil, fmt.Errorf("invalid driver: %s", driver)
}

func defaultDatabaseName() string {
	return facades.Config().GetString("database.orm.default", "default")
}

// DriverOf returns the driver configured for the given ORM connection name.
func DriverOf(name string) string {
	configKey := fmt.Sprintf("database.orm.connections.%s", name)
	cfg, _ := facades.Config().Get(configKey).(map[string]any)
	driver, _ := cfg["driver"].(string)

	return driver
}

func ConnectionPrefix(name string) string {
	return ormConnectionString(name, "prefix", "")
}

func ormConnectionKey(name, field string) string {
	return "database.orm.connections." + name + "." + field
}

func ormConnectionString(name, field, defaultValue string) string {
	if value := facades.Config().GetString(ormConnectionKey(name, field)); value != "" {
		return value
	}

	return defaultValue
}

func mysqlCharset(name string) string {
	return ormConnectionString(name, "charset", "utf8mb4")
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
	charset = mysqlCharset(name)
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

	dialectal := mysql.Open(mysqlDSN(username, password, host, port, db, charset))

	config := gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: prefix,
		},
		Logger:                 logger.Default.LogMode(logMode),
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	}

	if facades.Config().GetBool("app.debug") {
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

	db := ormConnectionString(name, "path", "default.db")

	path := facades.Root() + db

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

	if facades.Config().GetBool("app.debug") {
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

	if facades.Config().GetBool("app.debug") {
		config.PrepareStmt = false
	}

	open, err := gorm.Open(dialectal, &config)

	if err != nil {
		return nil, "", err
	}

	return open, name, nil
}

func newSQLServerClient(name string) (*gorm.DB, string, error) {

	var username, password, host, port, prefix, db string

	if configDriver := DriverOf(name); configDriver != "" && configDriver != DriverSQLServer {
		return nil, "", fmt.Errorf("invalid database config: sqlserver driver %s", configDriver)
	}

	username = ormConnectionString(name, "username", "")
	password = ormConnectionString(name, "password", "")
	host = ormConnectionString(name, "host", "")
	port = ormConnectionString(name, "port", "1433")
	prefix = ormConnectionString(name, "prefix", "")
	db = ormConnectionString(name, "db", "")
	log := ormConnectionString(name, "log_mode", "error")

	if username == "" || password == "" || host == "" || db == "" {
		return nil, "", errors.New("invalid database config: sqlserver")
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

	dialectal := sqlserver.Open(sqlserverDSN(username, password, host, port, db))

	config := gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: prefix,
		},
		Logger:                 logger.Default.LogMode(logMode),
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	}

	if facades.Config().GetBool("app.debug") {
		config.PrepareStmt = false
	}

	open, err := gorm.Open(dialectal, &config)

	if err != nil {
		return nil, "", err
	}

	return open, name, nil
}

func (r *Database) Drivers(name string) (*gorm.DB, error) {
	r.mu.RLock()
	if dri, exist := r.drivers[name]; exist {
		r.mu.RUnlock()
		return dri, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if dri, exist := r.drivers[name]; exist {
		return dri, nil
	}

	dri, err := NewDriver(name)

	if err != nil {
		return nil, err
	}

	r.drivers[name] = dri

	return dri, nil
}

func (r *Database) Default() *gorm.DB {
	return r.driver
}
