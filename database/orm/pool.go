package orm

import (
	"fmt"
	"strings"
	"time"

	"github.com/herhe-com/framework/facades"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

const (
	defaultORMTimeout         = 3 * time.Second
	defaultORMReadTimeout     = 5 * time.Second
	defaultORMWriteTimeout    = 5 * time.Second
	defaultORMMaxOpenConns    = 50
	defaultORMMaxIdleConns    = 10
	defaultORMConnMaxLifetime = 30 * time.Minute
	defaultORMConnMaxIdleTime = 5 * time.Minute
)

type ormPoolConfig struct {
	maxOpenConns    int
	maxIdleConns    int
	connMaxLifetime time.Duration
	connMaxIdleTime time.Duration
}

func ormConnectionInt(name, field string, defaultValue int) int {
	value := facades.Config().Get(ormConnectionKey(name, field))
	if value == nil {
		return defaultValue
	}

	if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
		return defaultValue
	}

	return cast.ToInt(value)
}

func ormConnectionDuration(name, field string, defaultValue time.Duration) (time.Duration, error) {
	key := ormConnectionKey(name, field)
	value := facades.Config().Get(key)
	if value == nil {
		return defaultValue, nil
	}

	if text, ok := value.(string); ok {
		text = strings.TrimSpace(text)
		if text == "" {
			return defaultValue, nil
		}
		value = text
	}

	duration, err := cast.ToDurationE(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration for %s: %w", key, err)
	}

	return duration, nil
}

func ormPoolConfigOf(name string) (ormPoolConfig, error) {
	connMaxLifetime, err := ormConnectionDuration(name, "pool.conn_max_lifetime", defaultORMConnMaxLifetime)
	if err != nil {
		return ormPoolConfig{}, err
	}

	connMaxIdleTime, err := ormConnectionDuration(name, "pool.conn_max_idle_time", defaultORMConnMaxIdleTime)
	if err != nil {
		return ormPoolConfig{}, err
	}

	return ormPoolConfig{
		maxOpenConns:    ormConnectionInt(name, "pool.max_open_conns", defaultORMMaxOpenConns),
		maxIdleConns:    ormConnectionInt(name, "pool.max_idle_conns", defaultORMMaxIdleConns),
		connMaxLifetime: connMaxLifetime,
		connMaxIdleTime: connMaxIdleTime,
	}, nil
}

func openORM(name string, dialectal gorm.Dialector, config *gorm.Config) (*gorm.DB, error) {
	pool, err := ormPoolConfigOf(name)
	if err != nil {
		return nil, err
	}

	open, err := gorm.Open(dialectal, config)
	if err != nil {
		return nil, err
	}

	sqlDB, err := open.DB()
	if err != nil {
		return nil, fmt.Errorf("get database/sql pool for connection %s: %w", name, err)
	}

	sqlDB.SetMaxOpenConns(pool.maxOpenConns)
	sqlDB.SetMaxIdleConns(pool.maxIdleConns)
	sqlDB.SetConnMaxLifetime(pool.connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(pool.connMaxIdleTime)

	return open, nil
}
