package database

import "gorm.io/gorm"

type Database interface {
	Drivers(driver string, names ...string) (*gorm.DB, error)
	Default() *gorm.DB
}
