package database

import "gorm.io/gorm"

type DB interface {
	Default() *gorm.DB

	Drivers(name string) (*gorm.DB, error)
}
