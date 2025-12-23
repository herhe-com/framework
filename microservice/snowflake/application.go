package snowflake

import (
	"github.com/bwmarrin/snowflake"
	"github.com/herhe-com/framework/facades"
)

func NewApplication() (err error) {

	facades.Snowflake, err = snowflake.NewNode(facades.Cfg.GetInt64("app.node", 1))

	if err != nil {
		return err
	}

	return nil
}
