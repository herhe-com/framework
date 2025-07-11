package facades

import (
	"github.com/herhe-com/framework/contracts/database"
	"github.com/redis/go-redis/v9"
)

var Database database.Database

var Redis *redis.Client
