package consoles

import (
	"database/sql"
	"os"
	"strconv"
	"strings"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/console"
	"github.com/herhe-com/framework/database/database"
	"github.com/herhe-com/framework/facades"
	"github.com/pressly/goose/v3"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type MigrationProvider struct {
	db *sql.DB
}

func (that *MigrationProvider) Register() console.Console {

	migrate := console.Console{
		Cmd:  "migrate",
		Name: "数据迁移",
	}

	if facades.DB.Default() == nil {
		color.Errorln("\n\n请初始化数据库\n\n")
		return migrate
	}

	that.init()

	var err error

	if that.db, err = facades.DB.Default().DB(); err != nil {
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
			},
		},
		{
			Cmd:  "commit",
			Name: "提交迁移",
			Run:  that.commit,
			Tags: func(cmd *cobra.Command) {
				cmd.Flags().BoolP("one", "o", false, "执行一次迁移")
				cmd.Flags().BoolP("all", "a", false, "执行所有迁移")
				cmd.Flags().IntP("step", "s", 0, "执行指定步骤数")
			},
		},
		{
			Cmd:  "rollback",
			Name: "回滚迁移",
			Run:  that.rollback,
			Tags: func(cmd *cobra.Command) {
				cmd.Flags().BoolP("one", "o", false, "回滚最近的一次")
				cmd.Flags().BoolP("all", "a", false, "回滚所有迁移")
				cmd.Flags().IntP("step", "s", 0, "回滚指定步骤数")
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

	// 如果参数为空，使用 pterm 进行交互式输入
	if name == "" {
		var err error
		name, err = pterm.DefaultInteractiveTextInput.
			WithDefaultText("表名").
			WithOnInterruptFunc(func() {
				color.Errorln("输入取消")
			}).
			Show()

		if err != nil {
			color.Errorln("输入取消")
			return
		}

		s := strings.TrimSpace(name)
		if s == "" {
			color.Errorln("表名不能为空")
			return
		} else if s != name {
			color.Errorln("表名不能包含空字符")
			return
		}
	}

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
	step, _ := cmd.Flags().GetInt("step")

	// 如果没有任何参数，使用 pterm 进行交互式选择
	if !one && !all && step == 0 {
		options := []string{"执行一次迁移", "执行所有迁移", "执行指定步骤数"}
		selectedOption, err := pterm.DefaultInteractiveSelect.
			WithOptions(options).
			WithDefaultText("请选择迁移方式").
			Show()

		if err != nil {
			color.Errorln("选择取消")
			return
		}

		switch selectedOption {
		case "执行一次迁移":
			one = true
		case "执行所有迁移":
			all = true
		case "执行指定步骤数":
			var stepStr string
			stepStr, err = pterm.DefaultInteractiveTextInput.
				WithDefaultText("请输入执行步骤数").
				WithOnInterruptFunc(func() {
					color.Errorln("输入取消")
				}).
				Show()

			if err != nil {
				color.Errorln("输入取消")
				return
			}

			s, err := strconv.Atoi(strings.TrimSpace(stepStr))
			if err != nil || s <= 0 {
				color.Errorln("步骤数必须是大于0的整数")
				return
			}

			step = s
		}
	}

	if step > 0 {
		// 获取当前已应用的迁移版本
		currentVersion, err := goose.GetDBVersion(that.db)
		if err != nil {
			color.Errorln("\n\n获取当前版本失败：%v\n\n", err)
			return
		}

		// 获取所有迁移文件
		migrations, err := goose.CollectMigrations(that.dir(), 0, goose.MaxVersion)
		if err != nil {
			color.Errorln("\n\n获取迁移列表失败：%v\n\n", err)
			return
		}

		// 筛选出未应用的迁移（版本号大于当前版本）
		var pendingMigrations []int64
		for _, m := range migrations {
			if m.Version > currentVersion {
				pendingMigrations = append(pendingMigrations, m.Version)
			}
		}

		if len(pendingMigrations) == 0 {
			color.Infoln("\n\n没有待执行的迁移\n\n")
			return
		}

		// 计算目标版本：执行指定步骤数的迁移
		targetIndex := step - 1
		if targetIndex >= len(pendingMigrations) {
			// 如果步骤数超过待执行的迁移数量，执行所有待执行的迁移
			targetIndex = len(pendingMigrations) - 1
		}

		targetVersion := pendingMigrations[targetIndex]

		color.Infoln("当前版本：%d，执行 %d 步后的目标版本：%d", currentVersion, step, targetVersion)

		err = goose.UpTo(that.db, that.dir(), targetVersion)
	} else if one {
		err = goose.UpByOne(that.db, that.dir())
	} else if all {
		err = goose.Up(that.db, that.dir())
	} else {
		_ = cmd.Help()
		return
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
	step, _ := cmd.Flags().GetInt("step")

	// 如果没有任何参数，使用 pterm 进行交互式选择
	if !one && !all && step == 0 {
		options := []string{"回滚最近的一次", "回滚所有迁移", "回滚指定步骤数"}
		selectedOption, err := pterm.DefaultInteractiveSelect.
			WithOptions(options).
			WithDefaultText("请选择回滚方式").
			Show()

		if err != nil {
			color.Errorln("选择取消")
			return
		}

		switch selectedOption {
		case "回滚最近的一次":
			one = true
		case "回滚所有迁移":
			all = true
		case "回滚指定步骤数":
			var stepStr string
			stepStr, err = pterm.DefaultInteractiveTextInput.
				WithDefaultText("请输入回滚步骤数").
				WithOnInterruptFunc(func() {
					color.Errorln("输入取消")
				}).
				Show()

			if err != nil {
				color.Errorln("输入取消")
				return
			}

			s, err := strconv.Atoi(strings.TrimSpace(stepStr))
			if err != nil || s <= 0 {
				color.Errorln("步骤数必须是大于0的整数")
				return
			}

			step = s
		}
	}

	if step > 0 {
		// 获取当前已应用的迁移版本
		currentVersion, err := goose.GetDBVersion(that.db)
		if err != nil {
			color.Errorln("\n\n获取当前版本失败：%v\n\n", err)
			return
		}

		if currentVersion == 0 {
			color.Errorln("\n\n当前没有已应用的迁移，无需回滚\n\n")
			return
		}

		// 获取所有已应用的迁移记录
		migrations, err := goose.CollectMigrations(that.dir(), 0, goose.MaxVersion)
		if err != nil {
			color.Errorln("\n\n获取迁移列表失败：%v\n\n", err)
			return
		}

		// 筛选出已应用且版本号小于等于当前版本的迁移
		var appliedMigrations []int64
		for _, m := range migrations {
			if m.Version <= currentVersion {
				appliedMigrations = append(appliedMigrations, m.Version)
			}
		}

		if len(appliedMigrations) == 0 {
			color.Errorln("\n\n没有找到已应用的迁移记录\n\n")
			return
		}

		// 计算目标版本：从当前位置往前回滚指定步骤数
		targetIndex := len(appliedMigrations) - step - 1
		var targetVersion int64 = 0

		if targetIndex >= 0 {
			targetVersion = appliedMigrations[targetIndex]
		}

		color.Infoln("当前版本：%d，回滚 %d 步后的目标版本：%d", currentVersion, step, targetVersion)

		err = goose.DownTo(that.db, that.dir(), targetVersion)
	} else if all {
		err = goose.Reset(that.db, that.dir())
	} else if one {
		err = goose.Down(that.db, that.dir())
	} else {
		_ = cmd.Help()
		return
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
