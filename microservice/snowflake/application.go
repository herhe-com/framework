package snowflake

import (
	"github.com/bwmarrin/snowflake"
	"github.com/herhe-com/framework/facades"
)

func NewApplication() (err error) {

	node, err := snowflake.NewNode(facades.Config().GetInt64("app.node", 1))

	if err != nil {
		return err
	}

	facades.Register[*snowflake.Node](node)

	return nil
}
