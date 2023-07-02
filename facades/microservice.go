package facades

import (
	"github.com/bwmarrin/snowflake"
	"github.com/go-redsync/redsync/v4"
)

var Locker *redsync.Redsync

var Snowflake *snowflake.Node
