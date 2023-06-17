package facades

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var Gorm *gorm.DB

var Redis *redis.Client
