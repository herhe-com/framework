# Crontab 组件

定时任务管理组件，基于 robfig/cron 实现，提供 Cron 表达式调度功能。

## 功能特性

- Cron 表达式支持
- 定时任务调度
- 任务启动/停止/重启
- 时区感知
- 可配置的任务列表

## 使用方法

### 定义定时任务

```go
import (
    "fmt"
    "github.com/herhe-com/framework/contracts/crontab"
)

type MyTask struct{}

func (t *MyTask) Name() string {
    return "my-task"
}

func (t *MyTask) Rule() string {
    // 每天凌晨 2 点执行
    return "0 2 * * *"
}

func (t *MyTask) Func() func() {
    return func() {
        fmt.Println("执行定时任务")
        // 任务逻辑
    }
}
```

### 注册任务

在配置文件中注册任务：

```yaml
crontab:
  functions:
    - name: my-task
      rule: "0 2 * * *"
    - name: cleanup-task
      rule: "0 0 * * 0"  # 每周日午夜执行
```

### 启动定时任务

```go
import "github.com/herhe-com/framework/facades"

func main() {
    // 初始化定时任务
    facades.Crontab.Init()
    
    // 启动定时任务
    facades.Crontab.Start()
    
    // 应用运行...
    
    // 停止定时任务
    defer facades.Crontab.Stop()
}
```

## Cron 表达式

### 表达式格式

```
┌───────────── 分钟 (0 - 59)
│ ┌───────────── 小时 (0 - 23)
│ │ ┌───────────── 日期 (1 - 31)
│ │ │ ┌───────────── 月份 (1 - 12)
│ │ │ │ ┌───────────── 星期 (0 - 6) (0 = 周日)
│ │ │ │ │
* * * * *
```

### 常用表达式示例

```go
// 每分钟执行
"* * * * *"

// 每小时执行
"0 * * * *"

// 每天凌晨 2 点执行
"0 2 * * *"

// 每周一上午 9 点执行
"0 9 * * 1"

// 每月 1 号凌晨执行
"0 0 1 * *"

// 每 5 分钟执行
"*/5 * * * *"

// 每天上午 9 点到下午 5 点，每小时执行
"0 9-17 * * *"

// 工作日上午 9 点执行
"0 9 * * 1-5"

// 每季度第一天执行
"0 0 1 1,4,7,10 *"
```

### 特殊字符

- `*`: 匹配任意值
- `,`: 列举多个值（如 `1,3,5`）
- `-`: 范围（如 `1-5`）
- `/`: 步长（如 `*/5` 表示每 5 个单位）

## 任务示例

### 数据库清理任务

```go
type CleanupTask struct{}

func (t *CleanupTask) Name() string {
    return "database-cleanup"
}

func (t *CleanupTask) Rule() string {
    // 每天凌晨 3 点执行
    return "0 3 * * *"
}

func (t *CleanupTask) Func() func() {
    return func() {
        db := facades.DB.Default()
        
        // 删除 30 天前的日志
        db.Where("created_at < ?", time.Now().AddDate(0, 0, -30)).
            Delete(&Log{})
        
        fmt.Println("数据库清理完成")
    }
}
```

### 数据备份任务

```go
type BackupTask struct{}

func (t *BackupTask) Name() string {
    return "database-backup"
}

func (t *BackupTask) Rule() string {
    // 每天凌晨 1 点执行
    return "0 1 * * *"
}

func (t *BackupTask) Func() func() {
    return func() {
        // 执行备份逻辑
        timestamp := time.Now().Format("20060102_150405")
        filename := fmt.Sprintf("backup_%s.sql", timestamp)
        
        // 备份数据库
        cmd := exec.Command("mysqldump", "-u", "root", "mydb")
        output, err := cmd.Output()
        if err != nil {
            fmt.Printf("备份失败: %v\n", err)
            return
        }
        
        // 上传到存储
        facades.Storage.Put(filename, bytes.NewReader(output), int64(len(output)))
        
        fmt.Printf("备份完成: %s\n", filename)
    }
}
```

### 报表生成任务

```go
type ReportTask struct{}

func (t *ReportTask) Name() string {
    return "daily-report"
}

func (t *ReportTask) Rule() string {
    // 每天上午 8 点执行
    return "0 8 * * *"
}

func (t *ReportTask) Func() func() {
    return func() {
        db := facades.DB.Default()
        
        // 统计昨天的数据
        yesterday := time.Now().AddDate(0, 0, -1)
        
        var count int64
        db.Model(&Order{}).
            Where("DATE(created_at) = ?", yesterday.Format("2006-01-02")).
            Count(&count)
        
        // 生成报表
        report := fmt.Sprintf("日期: %s\n订单数: %d\n", 
            yesterday.Format("2006-01-02"), count)
        
        // 发送邮件或保存报表
        fmt.Println(report)
    }
}
```

### 缓存预热任务

```go
type CacheWarmupTask struct{}

func (t *CacheWarmupTask) Name() string {
    return "cache-warmup"
}

func (t *CacheWarmupTask) Rule() string {
    // 每小时执行
    return "0 * * * *"
}

func (t *CacheWarmupTask) Func() func() {
    return func() {
        db := facades.DB.Default()
        redis := facades.Redis.Default()
        
        // 预热热门商品缓存
        var products []Product
        db.Where("is_hot = ?", true).Find(&products)
        
        for _, product := range products {
            key := fmt.Sprintf("product:%d", product.ID)
            data, _ := json.Marshal(product)
            redis.Set(context.Background(), key, data, 1*time.Hour)
        }
        
        fmt.Printf("缓存预热完成，共 %d 个商品\n", len(products))
    }
}
```

## 接口定义

```go
package crontab

type Crontab interface {
    // Name 任务名称
    Name() string
    
    // Rule Cron 表达式
    Rule() string
    
    // Func 任务执行函数
    Func() func()
}
```

## 管理方法

```go
import "github.com/herhe-com/framework/facades"

// 初始化定时任务
facades.Crontab.Init()

// 启动所有任务
facades.Crontab.Start()

// 停止所有任务
facades.Crontab.Stop()

// 重启所有任务
facades.Crontab.Restart()
```

## 高级用法

### 动态添加任务

```go
import "github.com/robfig/cron/v3"

func AddDynamicTask(name, rule string, fn func()) error {
    c := cron.New()
    
    _, err := c.AddFunc(rule, fn)
    if err != nil {
        return err
    }
    
    c.Start()
    return nil
}
```

### 任务错误处理

```go
type SafeTask struct{}

func (t *SafeTask) Name() string {
    return "safe-task"
}

func (t *SafeTask) Rule() string {
    return "*/5 * * * *"
}

func (t *SafeTask) Func() func() {
    return func() {
        defer func() {
            if r := recover(); r != nil {
                fmt.Printf("任务执行失败: %v\n", r)
                // 记录错误日志
            }
        }()
        
        // 任务逻辑
        // 可能会 panic 的代码
    }
}
```

### 任务执行日志

```go
type LoggedTask struct{}

func (t *LoggedTask) Name() string {
    return "logged-task"
}

func (t *LoggedTask) Rule() string {
    return "0 * * * *"
}

func (t *LoggedTask) Func() func() {
    return func() {
        start := time.Now()
        fmt.Printf("[%s] 任务开始执行\n", t.Name())
        
        // 任务逻辑
        time.Sleep(2 * time.Second)
        
        duration := time.Since(start)
        fmt.Printf("[%s] 任务执行完成，耗时: %v\n", t.Name(), duration)
    }
}
```

## 时区配置

定时任务使用系统本地时区（`time.Local`）。如需使用其他时区：

```go
import "time"

func init() {
    // 设置为 UTC
    time.Local = time.UTC
    
    // 或设置为特定时区
    loc, _ := time.LoadLocation("Asia/Shanghai")
    time.Local = loc
}
```

## 注意事项

1. 任务执行时间不应过长，避免阻塞其他任务
2. 长时间运行的任务应使用 goroutine
3. 任务中应包含错误处理和恢复机制
4. 避免在任务中使用阻塞操作
5. 合理设置任务执行频率，避免资源浪费

## 依赖项

- robfig/cron（Cron 调度库）
- Config facade（配置管理）

## 文件结构

```
crontab/
├── application.go    # 定时任务应用实现
└── provider.go       # 服务提供者
```

## 最佳实践

1. 为每个任务提供清晰的名称
2. 使用标准的 Cron 表达式
3. 在任务中添加日志记录
4. 实现错误处理和恢复机制
5. 避免任务执行时间过长
6. 定期检查任务执行状态
7. 对于耗时任务，考虑使用消息队列
