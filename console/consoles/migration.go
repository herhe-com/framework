package consoles

import (
	"database/sql"
	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/console"
	"github.com/herhe-com/framework/database/database"
	"github.com/herhe-com/framework/facades"
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"
	"os"
)

type MigrationProvider struct {
	db *sql.DB
}

func (that *MigrationProvider) Register() console.Console {

	migrate := console.Console{
		Cmd:  "migrate",
		Name: "数据迁移",
	}

	if facades.Database.Default() == nil {
		color.Errorln("\n\n请初始化数据库\n\n")
		return migrate
	}

	that.init()

	var err error

	if that.db, err = facades.Database.Default().DB(); err != nil {
		color.Errorln("\n\n数据库获取失败：%v\n\n", err)
		return migrate
	}

	migrate.Consoles = []console.Console{
		{
			Cmd:  "make",
			Name: "生成文件",
			Run:  that.make,
			Tags: func(cmd *cobra.Command) {
				cmd.Flags().StringP("name", "n", "", "表名")
				_ = cmd.MarkFlagRequired("name")
			},
		},
		{
			Cmd:  "commit",
			Name: "提交迁移",
			Run:  that.commit,
			Tags: func(cmd *cobra.Command) {
				cmd.Flags().BoolP("one", "o", false, "执行一次迁移")
				cmd.Flags().BoolP("all", "a", false, "执行所有迁移")
				cmd.Flags().Int64P("version", "v", 0, "迁移到指定版本")
			},
		},
		{
			Cmd:  "rollback",
			Name: "回滚迁移",
			Run:  that.rollback,
			Tags: func(cmd *cobra.Command) {
				cmd.Flags().BoolP("one", "o", false, "回滚最近的一次")
				cmd.Flags().BoolP("all", "a", false, "回滚所有迁移")
				cmd.Flags().Int64P("version", "v", 0, "回滚到指定版本")
			},
		},
		{
			Cmd:  "redo",
			Name: "重新运行",
			Run:  that.redo,
		},
		{
			Cmd:  "status",
			Name: "查看状态",
			Run:  that.status,
		},
		{
			Cmd:  "version",
			Name: "查看版本",
			Run:  that.version,
		},
	}

	return migrate
}

func (that *MigrationProvider) init() {

	defaultDriver := facades.Cfg.GetString("database.driver", database.DriverMySQL)

	table := facades.Cfg.GetString("database."+defaultDriver+".prefix") + facades.Cfg.GetString("database.migration.table")

	goose.SetTableName(table)

	_ = goose.SetDialect(defaultDriver)
}

func (that *MigrationProvider) make(cmd *cobra.Command, args []string) {

	name, _ := cmd.Flags().GetString("name")

	if err := os.MkdirAll(that.dir(), os.ModePerm); err != nil {
		return
	}

	if err := goose.Create(that.db, that.dir(), name, "sql"); err != nil {
		color.Errorln("迁移文件生成失败：%v", err)
		return
	}
}

func (that *MigrationProvider) commit(cmd *cobra.Command, args []string) {

	var err error

	one, _ := cmd.Flags().GetBool("one")
	all, _ := cmd.Flags().GetBool("all")
	version, _ := cmd.Flags().GetInt64("version")

	if one {
		err = goose.UpByOne(that.db, that.dir())
	} else if version > 0 {
		err = goose.UpTo(that.db, that.dir(), version)
	} else if all {
		err = goose.Up(that.db, that.dir())
	} else {
		_ = cmd.Help()
	}

	if err != nil {
		color.Errorln("\n\n文件迁移失败：%v\n\n", err)
		return
	}
}

func (that *MigrationProvider) rollback(cmd *cobra.Command, args []string) {

	var err error

	one, _ := cmd.Flags().GetBool("one")
	all, _ := cmd.Flags().GetBool("all")
	version, _ := cmd.Flags().GetInt64("version")

	if version > 0 {
		err = goose.DownTo(that.db, that.dir(), version)
	} else if all {
		err = goose.Reset(that.db, that.dir())
	} else if one {
		err = goose.Down(that.db, that.dir())
	} else {
		_ = cmd.Help()
	}

	if err != nil {
		color.Errorln("\n\n回滚迁移失败：%v\n\n", err)
		return
	}
}

func (that *MigrationProvider) redo(cmd *cobra.Command, args []string) {

	if err := goose.Redo(that.db, that.dir()); err != nil {
		color.Errorln("\n\n重新运行数据迁移失败：%v\n\n", err)
	}
}

func (that *MigrationProvider) status(cmd *cobra.Command, args []string) {

	if err := goose.Status(that.db, that.dir()); err != nil {
		color.Errorln("\n\n查看迁移文件失败：%v\n\n", err)
	}
}

func (that *MigrationProvider) version(cmd *cobra.Command, args []string) {

	if err := goose.Version(that.db, that.dir()); err != nil {
		color.Errorln("\n\n查看迁移版本失败：%v\n\n", err)
	}
}

func (that *MigrationProvider) dir() string {
	return facades.Root + facades.Cfg.GetString("database.migration.dir")
}
