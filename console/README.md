# Console 组件

CLI 命令框架，基于 Cobra 实现，提供命令行工具开发能力。

## 功能特性

- 基于 Cobra 的命令行框架
- 层级化命令结构
- 子命令支持
- 内置命令（密码生成等）
- 可配置的命令注册

## 使用方法

### 创建命令

```go
import (
    "github.com/spf13/cobra"
    "github.com/herhe-com/framework/contracts/console"
)

type MyCommand struct{}

func (c *MyCommand) Command() *cobra.Command {
    return &cobra.Command{
        Use:   "mycommand",
        Short: "My custom command",
        Long:  "This is a detailed description of my command",
        Run: func(cmd *cobra.Command, args []string) {
            // 命令逻辑
            fmt.Println("Hello from my command!")
        },
    }
}

func (c *MyCommand) Subcommands() []console.Console {
    return []console.Console{
        // 子命令列表
    }
}
```

### 注册命令

在应用配置中注册命令：

```go
import (
    "github.com/herhe-com/framework/facades"
    "github.com/herhe-com/framework/contracts/console"
)

func RegisterCommands() {
    commands := []console.Console{
        &MyCommand{},
        &AnotherCommand{},
    }
    
    for _, cmd := range commands {
        facades.Console.Register(cmd)
    }
}
```

### 执行命令

```bash
# 执行命令
./app mycommand

# 查看帮助
./app mycommand --help

# 执行子命令
./app mycommand subcommand
```

## 内置命令

### 密码生成命令

生成随机密码：

```bash
# 生成默认长度密码
./app password:generate

# 生成指定长度密码
./app password:generate --length 16

# 生成多个密码
./app password:generate --count 5
```

## 命令示例

### 简单命令

```go
type HelloCommand struct{}

func (c *HelloCommand) Command() *cobra.Command {
    return &cobra.Command{
        Use:   "hello",
        Short: "Say hello",
        Run: func(cmd *cobra.Command, args []string) {
            fmt.Println("Hello, World!")
        },
    }
}

func (c *HelloCommand) Subcommands() []console.Console {
    return nil
}
```

### 带参数的命令

```go
type GreetCommand struct{}

func (c *GreetCommand) Command() *cobra.Command {
    var name string
    
    cmd := &cobra.Command{
        Use:   "greet",
        Short: "Greet someone",
        Run: func(cmd *cobra.Command, args []string) {
            fmt.Printf("Hello, %s!\n", name)
        },
    }
    
    cmd.Flags().StringVarP(&name, "name", "n", "World", "Name to greet")
    
    return cmd
}

func (c *GreetCommand) Subcommands() []console.Console {
    return nil
}
```

### 带子命令的命令

```go
type UserCommand struct{}

func (c *UserCommand) Command() *cobra.Command {
    return &cobra.Command{
        Use:   "user",
        Short: "User management commands",
    }
}

func (c *UserCommand) Subcommands() []console.Console {
    return []console.Console{
        &UserListCommand{},
        &UserCreateCommand{},
        &UserDeleteCommand{},
    }
}

type UserListCommand struct{}

func (c *UserListCommand) Command() *cobra.Command {
    return &cobra.Command{
        Use:   "list",
        Short: "List all users",
        Run: func(cmd *cobra.Command, args []string) {
            // 列出用户
        },
    }
}

func (c *UserListCommand) Subcommands() []console.Console {
    return nil
}
```

### 数据库迁移命令

```go
type MigrateCommand struct{}

func (c *MigrateCommand) Command() *cobra.Command {
    return &cobra.Command{
        Use:   "migrate",
        Short: "Run database migrations",
        Run: func(cmd *cobra.Command, args []string) {
            db := facades.DB.Default()
            
            // 执行迁移
            if err := db.AutoMigrate(&User{}, &Post{}); err != nil {
                fmt.Printf("Migration failed: %v\n", err)
                return
            }
            
            fmt.Println("Migration completed successfully")
        },
    }
}

func (c *MigrateCommand) Subcommands() []console.Console {
    return nil
}
```

### 数据填充命令

```go
type SeedCommand struct{}

func (c *SeedCommand) Command() *cobra.Command {
    return &cobra.Command{
        Use:   "seed",
        Short: "Seed the database",
        Run: func(cmd *cobra.Command, args []string) {
            db := facades.DB.Default()
            
            // 创建测试数据
            users := []User{
                {Username: "admin", Email: "admin@example.com"},
                {Username: "user1", Email: "user1@example.com"},
            }
            
            if err := db.Create(&users).Error; err != nil {
                fmt.Printf("Seeding failed: %v\n", err)
                return
            }
            
            fmt.Println("Database seeded successfully")
        },
    }
}

func (c *SeedCommand) Subcommands() []console.Console {
    return nil
}
```

## 高级用法

### 命令验证

```go
type CreateUserCommand struct{}

func (c *CreateUserCommand) Command() *cobra.Command {
    var username, email string
    
    cmd := &cobra.Command{
        Use:   "create",
        Short: "Create a new user",
        PreRunE: func(cmd *cobra.Command, args []string) error {
            // 验证参数
            if username == "" {
                return fmt.Errorf("username is required")
            }
            if email == "" {
                return fmt.Errorf("email is required")
            }
            return nil
        },
        Run: func(cmd *cobra.Command, args []string) {
            // 创建用户
            user := User{Username: username, Email: email}
            facades.DB.Default().Create(&user)
            fmt.Printf("User created: %s\n", username)
        },
    }
    
    cmd.Flags().StringVar(&username, "username", "", "Username")
    cmd.Flags().StringVar(&email, "email", "", "Email address")
    cmd.MarkFlagRequired("username")
    cmd.MarkFlagRequired("email")
    
    return cmd
}

func (c *CreateUserCommand) Subcommands() []console.Console {
    return nil
}
```

### 交互式命令

```go
import "bufio"

type InteractiveCommand struct{}

func (c *InteractiveCommand) Command() *cobra.Command {
    return &cobra.Command{
        Use:   "interactive",
        Short: "Interactive command",
        Run: func(cmd *cobra.Command, args []string) {
            reader := bufio.NewReader(os.Stdin)
            
            fmt.Print("Enter your name: ")
            name, _ := reader.ReadString('\n')
            name = strings.TrimSpace(name)
            
            fmt.Printf("Hello, %s!\n", name)
        },
    }
}

func (c *InteractiveCommand) Subcommands() []console.Console {
    return nil
}
```

### 进度显示

```go
import "time"

type ProcessCommand struct{}

func (c *ProcessCommand) Command() *cobra.Command {
    return &cobra.Command{
        Use:   "process",
        Short: "Process with progress",
        Run: func(cmd *cobra.Command, args []string) {
            total := 100
            
            for i := 0; i <= total; i++ {
                fmt.Printf("\rProgress: %d%%", i)
                time.Sleep(50 * time.Millisecond)
            }
            
            fmt.Println("\nCompleted!")
        },
    }
}

func (c *ProcessCommand) Subcommands() []console.Console {
    return nil
}
```

## 接口定义

```go
type Console interface {
    // Command 返回 Cobra 命令
    Command() *cobra.Command
    
    // Subcommands 返回子命令列表
    Subcommands() []Console
}
```

## 配置

在配置文件中定义命令列表：

```yaml
console:
  commands:
    - MyCommand
    - AnotherCommand
```

## 最佳实践

1. 命令命名使用小写和连字符（如 `user:create`）
2. 提供清晰的 Short 和 Long 描述
3. 使用 Flags 而不是位置参数
4. 为必需参数使用 `MarkFlagRequired`
5. 在 PreRunE 中进行参数验证
6. 使用子命令组织相关功能
7. 提供 `--help` 信息

## 依赖项

- Cobra（命令行框架）
- Config facade（配置管理）

## 文件结构

```
console/
├── application.go    # 命令应用实现
├── provider.go       # 服务提供者
└── consoles/        # 内置命令
    └── password.go  # 密码生成命令
```

## 访问方式

```go
import "github.com/herhe-com/framework/facades"

// 注册命令
facades.Console.Register(command)

// 执行命令（通常在 main 函数中）
facades.Console.Execute()
```
