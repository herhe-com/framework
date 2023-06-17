package scope

import (
	"fmt"
	"github.com/herhe-com/framework/contracts/auth"
	"gorm.io/gorm"
)

func Platform(platform auth.RoleOfCache, tables ...string) func(db *gorm.DB) *gorm.DB {

	table := ""

	if len(tables) > 0 {
		table = tables[0]
	}

	return func(db *gorm.DB) *gorm.DB {

		query := "`platform`=?"

		if platform.CheckId() {
			query += " and `platform_id`=?"
		}

		if len(tables) > 0 {
			query = fmt.Sprintf("`%s`.`platform`=?", table)

			if platform.CheckId() {
				query += fmt.Sprintf(" and `%s`.`platform_id`=?", table)
			}
		}

		if platform.CheckId() {
			db.Where(query, platform.Platform, platform.Id)
		} else {
			db.Where(query, platform.Platform)
		}

		return db
	}
}
