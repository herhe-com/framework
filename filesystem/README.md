# Filesystem 组件

`filesystem` 提供统一对象存储接口，支持 S3、OSS、COS、MinIO、Qiniu。服务启动后默认驱动会写入 `facades.Storage`。

## 支持驱动

- `s3`
- `oss`
- `cos`
- `minio`
- `qiniu`

## 配置

当前实现读取统一的磁盘配置：`filesystem.disks.<disk>`，并从每个磁盘实例内读取 `driver`。默认磁盘名由 `filesystem.default` 决定，未配置时回退到 `default`。

```yaml
filesystem:
  default: default
  disks:
    default:
      driver: s3
      access: your-access-key
      secret: your-secret-key
      region: us-east-1
      bucket: my-bucket
      domain: https://cdn.example.com
      endpoint: https://s3.example.com
    public:
      driver: s3
      access: your-access-key
      secret: your-secret-key
      region: us-east-1
      bucket: public-bucket
      domain: https://static.example.com
      endpoint: https://s3.example.com
```

`example` 基础项目推荐采用这种结构：

```go
cfg.Add("filesystem", map[string]any{
	"default": "default",
	"disks": map[string]any{
		"default": map[string]any{
			"driver":   filesystem.DriverS3,
			"access":   cfg.Env("filesystem.disks.default.access"),
			"secret":   cfg.Env("filesystem.disks.default.secret"),
			"bucket":   cfg.Env("filesystem.disks.default.bucket"),
			"domain":   cfg.Env("filesystem.disks.default.domain"),
			"endpoint": cfg.Env("filesystem.disks.default.endpoint"),
		},
	},
})
```

## 使用

默认存储：

```go
file, err := os.Open("image.jpg")
if err != nil {
	return err
}
defer file.Close()

info, err := file.Stat()
if err != nil {
	return err
}

err = facades.Storage.Put("images/photo.jpg", file, info.Size())
```

切换驱动和磁盘：

```go
s3Default, err := facades.Storage.Disk("s3", "default")
if err != nil {
	return err
}

minioPublic, err := facades.Storage.Disk("minio", "public")
if err != nil {
	return err
}
```

注意：`Disk` 的签名是 `Disk(driver string, disk string)`，不是 `Disk("s3")`。

## 接口

核心接口位于 `contracts/filesystem/storage.go`：

```go
type Storage interface {
	Driver
	Disk(driver string, disk string) (Driver, error)
}

type Driver interface {
	Dirs(path string) ([]Pathname, error)
	Files(path string) ([]Pathname, error)
	List(path string) ([]Pathname, error)
	Copy(oldFile, newFile string) error
	Delete(file ...string) error
	DeleteDirectory(directory string) error
	Exists(file string) bool
	MakeDirectory(directory string) error
	Missing(file string) bool
	Move(oldFile, newFile string) error
	Path(file string) string
	Put(file string, content io.Reader, size int64) error
	PutFile(path string, source File) (string, error)
	PutFileAs(path string, source File, name string) (string, error)
	Size(file string) (int64, error)
	TemporaryUrl(file string, time time.Duration) (string, error)
	PresignedUploadUrl(file string, time time.Duration) (string, error)
	Url(file string) string
}
```

## 注意事项

- `filesystem.default` 只保存默认磁盘名，实际配置位于 `filesystem.disks.<disk>`。
- `filesystem.disks.default.driver` 不能为空，否则 `NewStorage()` 返回 `nil`。
- `NewDriver()` 不校验所有必填字段，部分错误会在实际请求对象存储时暴露。
- S3 默认 `region` 为空时使用 `us-east-1`，并启用 path-style。
- 对象 key 应避免以 `/` 开头；驱动内部会处理部分场景，但业务层保持统一更清晰。
