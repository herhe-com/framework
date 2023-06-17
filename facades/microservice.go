package facades

import (
	"github.com/bsm/redislock"
	"github.com/bwmarrin/snowflake"
)

var Locker *redislock.Client

var Snowflake *snowflake.Node
