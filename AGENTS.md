# AGENTS.md - Developer Guide for herhe-com/framework

This document provides essential information for AI coding agents working on the `herhe-com/framework` codebase.

## Project Overview

This is a Go framework built on top of Cloudwego Hertz (HTTP) and Kitex (RPC), providing modular components for web applications including authentication, filesystem storage, database access, validation, search, queuing, and more.

**Go Version:** 1.24.9  
**Primary Dependencies:** Cloudwego Hertz, Kitex, GORM, Casbin, AWS SDK v2, Viper

## Build, Test & Lint Commands

### Building
```bash
# Build the entire module
go build ./...

# Build a specific package
go build ./auth
go build ./filesystem/s3
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./auth
go test ./filesystem/s3

# Run a single test function
go test ./auth -run TestJWT
go test -v ./filesystem/s3 -run TestS3Put

# Run tests with coverage
go test -cover ./...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

# Run tests with race detection
go test -race ./...
```

### Linting
```bash
# Run go vet
go vet ./...

# Run gofmt check
gofmt -l .

# Format code
gofmt -w .
go fmt ./...

# If golangci-lint is available
golangci-lint run
golangci-lint run ./auth/...
```

### Module Management
```bash
# Tidy dependencies
go mod tidy

# Verify dependencies
go mod verify

# Download dependencies
go mod download
```

## Code Style Guidelines

### Package Organization

- **Contracts:** Interface definitions in `contracts/` subdirectories (e.g., `contracts/filesystem/storage.go`)
- **Implementations:** Concrete implementations in top-level packages (e.g., `filesystem/`, `auth/`)
- **Facades:** Global singleton accessors in `facades/` (e.g., `facades.DB`, `facades.Cfg`)
- **Providers:** Service initialization logic in `*_provider.go` files
- **Applications:** Main application logic in `*_application.go` files

### Import Ordering

Organize imports in three groups separated by blank lines:

1. Standard library imports
2. Third-party imports
3. Local project imports

```go
import (
    "context"
    "fmt"
    "strings"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/spf13/viper"
    "gorm.io/gorm"

    "github.com/herhe-com/framework/contracts/filesystem"
    "github.com/herhe-com/framework/facades"
)
```

### Naming Conventions

- **Packages:** Lowercase, single word (e.g., `auth`, `filesystem`, `validation`)
- **Files:** Lowercase with underscores if needed (e.g., `application.go`, `s3.go`)
- **Types:** PascalCase (e.g., `Storage`, `Database`, `S3`)
- **Functions/Methods:** PascalCase for exported, camelCase for unexported
- **Constants:** PascalCase or SCREAMING_SNAKE_CASE for driver names (e.g., `DriverMySQL`, `DriverS3`)
- **Variables:** camelCase (e.g., `defaultDriver`, `username`)

### Type Definitions

Use explicit struct types with clear field names:

```go
type S3 struct {
    ctx      context.Context
    instance *s3.Client
    bucket   string
    domain   string
}
```

### Error Handling

- **Always check errors immediately** after function calls that return them
- **Return errors** rather than panicking (except in initialization code where appropriate)
- **Wrap errors** with context using `fmt.Errorf("context: %w", err)` or `fmt.Errorf("context: %v", err)`
- **Use early returns** for error cases to reduce nesting

```go
func (r *S3) Put(key string, file io.Reader, size int64) error {
    buffer, err := io.ReadAll(file)
    if err != nil {
        return err
    }

    _, err = r.instance.PutObject(r.ctx, &s3.PutObjectInput{
        Bucket: aws.String(r.bucket),
        Key:    aws.String(strings.TrimLeft(key, "/")),
        Body:   bytes.NewReader(buffer),
    })

    return err
}
```

### Configuration Access

Use the `facades.Cfg` singleton for configuration:

```go
defaultDriver := facades.Cfg.GetString("database.driver", DriverMySQL)
debug := facades.Cfg.GetBool("app.debug")
configs := facades.Cfg.Get("filesystem.s3").(map[string]any)
```

### Database Access

Use the `facades.DB` singleton for database operations:

```go
if facades.DB.Default() == nil {
    return errors.New("please initialize database first")
}

db := facades.DB.Default()
db.Where("id = ?", id).First(&user)
```

### String Manipulation

- **Trim leading slashes** from S3/filesystem keys: `strings.TrimLeft(key, "/")` or `strings.TrimPrefix(key, "/")`
- **Use `strings.TrimSuffix`** for trailing characters
- **Prefer `strings.Builder`** for concatenating multiple strings

### Context Usage

- Pass `context.Context` as the first parameter to functions that need it
- Store context in struct fields for service-level operations (e.g., `S3.ctx`)
- Use `context.Background()` for initialization

### Validation

- Use `github.com/go-playground/validator/v10` for struct validation
- Custom validation rules are registered in `validation/rules.go`
- Translation support for validation messages (zh, ja, en)

### Comments

- **Document exported types, functions, and methods** with comments starting with the name
- Use `//` for single-line comments
- Use `/* */` for block comments or documentation headers

```go
// NewS3 creates a new S3 storage driver with the provided configuration.
// It returns an error if the configuration is invalid or connection fails.
func NewS3(ctx context.Context, configs map[string]any) (*S3, error) {
    // implementation
}
```

## Common Patterns

### Driver Pattern

Many components use a driver pattern with a default driver and the ability to switch:

```go
type Storage struct {
    filesystem.Driver
    drivers map[string]filesystem.Driver
}

func (r *Storage) Disk(disk string) filesystem.Driver {
    if driver, exist := r.drivers[disk]; exist {
        return driver
    }
    // create and cache new driver
}
```

### Configuration-Based Initialization

Services are initialized from Viper configuration:

```go
cfg := viper.New()
cfg.Set("s3", configs)
access := cfg.GetString("s3.access")
secret := cfg.GetString("s3.secret")
```

### Error Logging

Use `github.com/gookit/color` for colored console output:

```go
color.Redln("[filesystem] please set default disk")
color.Redf("[filesystem] %s\n", err)
color.Errorf("[database] %s", err)
```

## Testing Considerations

- Tests should be placed in `*_test.go` files in the same package
- Use table-driven tests for multiple test cases
- Mock external dependencies (S3, databases, etc.)
- Clean up resources in test teardown

## Common Gotcases

1. **S3 Keys:** Always strip leading slashes from S3 keys to avoid API errors
2. **Database Initialization:** Check `facades.DB.Default() == nil` before use
3. **Configuration:** Provide sensible defaults for optional config values
4. **Context:** Don't use `context.TODO()` in production code
5. **Prepared Statements:** Disabled in debug mode (`config.PrepareStmt = false`)

## Module Structure

```
framework/
├── auth/           # JWT, permissions, Casbin integration
├── cache/          # Caching utilities
├── captcha/        # CAPTCHA generation
├── config/         # Configuration management
├── console/        # CLI commands (Cobra)
├── contracts/      # Interface definitions
├── crontab/        # Scheduled tasks
├── database/       # GORM database drivers (MySQL, SQLite)
├── facades/        # Global singletons
├── filesystem/     # Storage drivers (S3, Minio, Qiniu)
├── foundation/     # Core application foundation
├── http/           # HTTP response helpers (Hertz)
├── microservice/   # RPC services, distributed locks, Snowflake IDs
├── queue/          # Message queuing (RabbitMQ)
├── search/         # Search engines (Elasticsearch, Meilisearch)
├── support/        # Utility functions
└── validation/     # Request validation rules
```

## Additional Notes

- This framework follows a Laravel-inspired architecture with facades and service providers
- The codebase supports multiple languages for validation messages (Chinese, Japanese, English)
- All filesystem operations should go through the `filesystem.Driver` interface
- Database operations use GORM with support for MySQL and SQLite
