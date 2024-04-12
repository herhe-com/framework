package scope

import (
	"fmt"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/herhe-com/framework/auth"
	"gorm.io/gorm"
)

func Platform(ctx *app.RequestContext, tables ...string) func(db *gorm.DB) *gorm.DB {

	table := ""

	if len(tables) > 0 {
		table = tables[0]
	}

	return func(db *gorm.DB) *gorm.DB {

		query := "`platform`=? and `organization_id`=?"

		if len(tables) > 0 {
			query = fmt.Sprintf("`%s`.`platform`=? and `%s`.`organization_id`=?", table, table)
		}

		db.Where(query, auth.Platform(ctx), auth.Organization(ctx))

		return db
	}
}
