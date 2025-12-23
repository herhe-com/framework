package database

import "gorm.io/gorm"

type DB interface {
	Default() *gorm.DB

	Drivers(driver string, names ...string) (*gorm.DB, error)
}
