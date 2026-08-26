# Crontab 组件

定时任务组件基于 `robfig/cron`。业务代码只定义任务标识和执行逻辑，启停状态与 Cron 表达式统一由配置提供。

## 定义任务

```go
package task

import "fmt"

type CleanupTask struct{}

func (*CleanupTask) Key() string {
    return "database_cleanup"
}

func (*CleanupTask) Name() string {
    return "数据库清理"
}

func (*CleanupTask) Func() {
    fmt.Println("执行数据库清理")
}
```

`Key()` 必须返回稳定且唯一的配置键；`Name()` 仅用于日志展示。

## 注册与配置

任务实例通过 Go 配置注入，因为 `functions` 包含函数指针：

```go
facades.Config().Set("crontab.functions", []contractcrontab.Crontab{
    &task.CleanupTask{},
})
```

执行频率与启停状态写在配置文件中：

```yaml
crontab:
  tasks:
    database_cleanup:
      enable: true
      rule: "0 3 * * *"
```

配置路径为 `crontab.tasks.<任务 Key>.enable` 和 `crontab.tasks.<任务 Key>.rule`。`enable` 未配置时默认启用；`rule` 必须配置，缺失或表达式非法时任务不会注册。

## 启动

在 `kernel.consoles` 中注册 `consoles.CrontabProvider` 后，通过应用的 `crontab` 子命令启动：

```shell
./application crontab
```

调度器使用 `time.Local`，应用时区由 `app.location` 配置。

## Cron 格式

```text
┌───────────── 分钟 (0 - 59)
│ ┌───────────── 小时 (0 - 23)
│ │ ┌───────────── 日期 (1 - 31)
│ │ │ ┌───────────── 月份 (1 - 12)
│ │ │ │ ┌───────────── 星期 (0 - 6，0 为周日)
│ │ │ │ │
* * * * *
```

常用表达式：

- `* * * * *`：每分钟
- `*/5 * * * *`：每 5 分钟
- `0 * * * *`：每小时
- `0 2 * * *`：每天凌晨 2 点
- `0 9 * * 1-5`：工作日上午 9 点

## 接口

```go
type Crontab interface {
    Key() string
    Name() string
    Func()
}
```

任务应自行处理运行错误，并在多实例部署时按业务需要使用分布式锁避免重复执行。
