package crontab

import (
	"testing"

	frameworkconfig "github.com/herhe-com/framework/config"
	contractcrontab "github.com/herhe-com/framework/contracts/crontab"
	"github.com/herhe-com/framework/facades"
)

type testTask struct {
	key  string
	name string
}

func (task testTask) Key() string  { return task.key }
func (task testTask) Name() string { return task.name }
func (testTask) Func()             {}

func TestInitUsesConfiguredTaskSchedules(t *testing.T) {
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[facades.RootPath](facades.RootPath(t.TempDir()))
	t.Cleanup(func() {
		facades.SetContainer(original)
	})

	if err := frameworkconfig.NewApplication(); err != nil {
		t.Fatalf("initialize config: %v", err)
	}

	facades.Config().Set("crontab.functions", []contractcrontab.Crontab{
		testTask{key: "enabled", name: "启用任务"},
		testTask{key: "default_enabled", name: "默认启用任务"},
		testTask{key: "disabled", name: "停用任务"},
		testTask{key: "missing_rule", name: "缺少规则任务"},
		testTask{key: "invalid_rule", name: "非法规则任务"},
	})
	facades.Config().Set("crontab.tasks.enabled.enable", true)
	facades.Config().Set("crontab.tasks.enabled.rule", "* * * * *")
	facades.Config().Set("crontab.tasks.default_enabled.rule", "0 * * * *")
	facades.Config().Set("crontab.tasks.disabled.enable", false)
	facades.Config().Set("crontab.tasks.disabled.rule", "* * * * *")
	facades.Config().Set("crontab.tasks.invalid_rule.rule", "invalid")

	application := &Application{}
	application.Init()

	if got := len(application.client.Entries()); got != 2 {
		t.Fatalf("registered tasks = %d, want 2", got)
	}
}
