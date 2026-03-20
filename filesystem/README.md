# Filesystem 组件

多云存储抽象层，提供统一的文件存储接口，支持多种云存储服务。

## 功能特性

- 统一的存储接口
- 多驱动支持（AWS S3、阿里云 OSS、腾讯云 COS、MinIO、七牛云）
- 多磁盘配置
- 文件上传处理
- 临时 URL 生成
- 预签名上传 URL
- 文件操作（上传、下载、删除、复制、移动）

## 支持的驱动

- **AWS S3**: Amazon Simple Storage Service
- **阿里云 OSS**: Alibaba Cloud Object Storage Service
- **腾讯云 COS**: Tencent Cloud Object Storage
- **MinIO**: 开源对象存储
- **七牛云**: Qiniu Cloud Storage

## 配置

```yaml
filesystem:
  driver: s3
  
  s3:
    default:
      access: your-access-key
      secret: your-secret-key
      region: us-east-1
      bucket: my-bucket
      domain: https://cdn.example.com
  
  oss:
    default:
      access: your-access-key
      secret: your-secret-key
      endpoint: oss-cn-hangzhou.aliyuncs.com
      bucket: my-bucket
      domain: https://cdn.example.com
  
  cos:
    default:
      access: your-secret-id
      secret: your-secret-key
      region: ap-guangzhou
      bucket: my-bucket-1234567890
      domain: https://cdn.example.com
  
  minio:
    default:
      access: minioadmin
      secret: minioadmin
      endpoint: localhost:9000
      bucket: my-bucket
      ssl: false
  
  qiniu:
    default:
      access: your-access-key
      secret: your-secret-key
      bucket: my-bucket
      domain: https://cdn.example.com
```

## 使用方法

### 基础操作

```go
import "github.com/herhe-com/framework/facades"

// 上传文件
file, _ := os.Open("image.jpg")
defer file.Close()

fileInfo, _ := file.Stat()
err := facades.Storage.Put("images/photo.jpg", file, fileInfo.Size())

// 下载文件
data, err := facades.Storage.Get("images/photo.jpg")

// 删除文件
err := facades.Storage.Delete("images/photo.jpg")

// 检查文件是否存在
exists := facades.Storage.Exists("images/photo.jpg")

// 获取文件大小
size, err := facades.Storage.Size("images/photo.jpg")
```

### 文件操作

```go
// 复制文件
err := facades.Storage.Copy("images/photo.jpg", "images/photo-copy.jpg")

// 移动文件
err := facades.Storage.Move("images/photo.jpg", "archive/photo.jpg")

// 列出文件
files, err := facades.Storage.List("images/")
for _, file := range files {
    fmt.Println(file)
}
```

### 临时 URL

```go
// 生成临时访问 URL（1 小时有效）
url, err := facades.Storage.TemporaryUrl("images/photo.jpg", 1*time.Hour)

// 使用临时 URL
fmt.Printf("临时访问链接: %s\n", url)
```

### 预签名上传 URL

```go
// 生成预签名上传 URL（允许客户端直接上传）
uploadUrl, err := facades.Storage.PresignedUploadUrl("images/new-photo.jpg", 10*time.Minute)

// 返回给前端
response := map[string]string{
    "upload_url": uploadUrl,
    "key": "images/new-photo.jpg",
}
```

### 切换磁盘

```go
// 使用默认磁盘
facades.Storage.Put("file.txt", reader, size)

// 使用指定磁盘
s3 := facades.Storage.Disk("s3")
s3.Put("file.txt", reader, size)

oss := facades.Storage.Disk("oss")
oss.Put("file.txt", reader, size)

minio := facades.Storage.Disk("minio")
minio.Put("file.txt", reader, size)
```

## HTTP 文件上传

### 处理表单上传

```go
import (
    "github.com/cloudwego/hertz/pkg/app"
    "github.com/herhe-com/framework/facades"
)

func UploadHandler(ctx context.Context, c *app.RequestContext) {
    // 获取上传的文件
    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(400, map[string]string{"error": "文件上传失败"})
        return
    }
    
    // 打开文件
    src, err := file.Open()
    if err != nil {
        c.JSON(500, map[string]string{"error": "无法读取文件"})
        return
    }
    defer src.Close()
    
    // 生成文件路径
    filename := fmt.Sprintf("uploads/%d-%s", time.Now().Unix(), file.Filename)
    
    // 上传到存储
    err = facades.Storage.Put(filename, src, file.Size)
    if err != nil {
        c.JSON(500, map[string]string{"error": "存储失败"})
        return
    }
    
    c.JSON(200, map[string]string{
        "message": "上传成功",
        "path": filename,
    })
}
```

### 多文件上传

```go
func MultiUploadHandler(ctx context.Context, c *app.RequestContext) {
    form, err := c.MultipartForm()
    if err != nil {
        c.JSON(400, map[string]string{"error": "表单解析失败"})
        return
    }
    
    files := form.File["files"]
    var uploadedFiles []string
    
    for _, file := range files {
        src, _ := file.Open()
        defer src.Close()
        
        filename := fmt.Sprintf("uploads/%d-%s", time.Now().Unix(), file.Filename)
        
        if err := facades.Storage.Put(filename, src, file.Size); err != nil {
            continue
        }
        
        uploadedFiles = append(uploadedFiles, filename)
    }
    
    c.JSON(200, map[string]interface{}{
        "message": "上传完成",
        "files": uploadedFiles,
    })
}
```

### 图片上传并生成缩略图

```go
import (
    "image"
    "image/jpeg"
    "github.com/nfnt/resize"
)

func UploadImageHandler(ctx context.Context, c *app.RequestContext) {
    file, _ := c.FormFile("image")
    src, _ := file.Open()
    defer src.Close()
    
    // 解码图片
    img, _, err := image.Decode(src)
    if err != nil {
        c.JSON(400, map[string]string{"error": "无效的图片"})
        return
    }
    
    // 生成缩略图
    thumbnail := resize.Thumbnail(200, 200, img, resize.Lanczos3)
    
    // 保存原图
    src.Seek(0, 0)
    originalPath := fmt.Sprintf("images/%d-original.jpg", time.Now().Unix())
    facades.Storage.Put(originalPath, src, file.Size)
    
    // 保存缩略图
    var buf bytes.Buffer
    jpeg.Encode(&buf, thumbnail, nil)
    thumbnailPath := fmt.Sprintf("images/%d-thumb.jpg", time.Now().Unix())
    facades.Storage.Put(thumbnailPath, &buf, int64(buf.Len()))
    
    c.JSON(200, map[string]string{
        "original": originalPath,
        "thumbnail": thumbnailPath,
    })
}
```

## 驱动特定功能

### AWS S3

```go
// S3 特定配置
s3Config := map[string]any{
    "access": "AKIAIOSFODNN7EXAMPLE",
    "secret": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
    "region": "us-west-2",
    "bucket": "my-bucket",
    "domain": "https://cdn.example.com",
}

// 使用 S3 Transfer Manager 加速大文件上传
// 自动处理分片上传
```

### 阿里云 OSS

```go
// OSS 特定配置
ossConfig := map[string]any{
    "access": "your-access-key",
    "secret": "your-secret-key",
    "endpoint": "oss-cn-hangzhou.aliyuncs.com",
    "bucket": "my-bucket",
    "domain": "https://cdn.example.com",
}
```

### MinIO

```go
// MinIO 本地开发配置
minioConfig := map[string]any{
    "access": "minioadmin",
    "secret": "minioadmin",
    "endpoint": "localhost:9000",
    "bucket": "my-bucket",
    "ssl": false,
}
```

## 接口定义

```go
type Driver interface {
    // Put 上传文件
    Put(key string, file io.Reader, size int64) error
    
    // Get 下载文件
    Get(key string) ([]byte, error)
    
    // Delete 删除文件
    Delete(key string) error
    
    // Copy 复制文件
    Copy(src, dst string) error
    
    // Move 移动文件
    Move(src, dst string) error
    
    // Exists 检查文件是否存在
    Exists(key string) bool
    
    // Size 获取文件大小
    Size(key string) (int64, error)
    
    // List 列出文件
    List(prefix string) ([]string, error)
    
    // TemporaryUrl 生成临时访问 URL
    TemporaryUrl(key string, ttl time.Duration) (string, error)
    
    // PresignedUploadUrl 生成预签名上传 URL
    PresignedUploadUrl(key string, ttl time.Duration) (string, error)
}
```

## 最佳实践

### 文件路径组织

```go
// 按日期组织
date := time.Now().Format("2006/01/02")
path := fmt.Sprintf("uploads/%s/%s", date, filename)

// 按用户组织
path := fmt.Sprintf("users/%d/avatar.jpg", userID)

// 按类型组织
path := fmt.Sprintf("images/products/%s", filename)
```

### 文件名处理

```go
import (
    "path/filepath"
    "github.com/google/uuid"
)

// 生成唯一文件名
ext := filepath.Ext(originalFilename)
newFilename := fmt.Sprintf("%s%s", uuid.New().String(), ext)

// 清理文件名
filename = strings.ReplaceAll(filename, " ", "-")
filename = strings.ToLower(filename)
```

### 错误处理

```go
if err := facades.Storage.Put(key, file, size); err != nil {
    log.Printf("文件上传失败: %v", err)
    // 回滚或重试逻辑
    return err
}
```

### 大文件处理

```go
// 使用流式上传，避免内存溢出
file, _ := os.Open("large-file.zip")
defer file.Close()

fileInfo, _ := file.Stat()
err := facades.Storage.Put("files/large-file.zip", file, fileInfo.Size())
```

## 依赖项

- AWS SDK v2（S3）
- 阿里云 OSS SDK
- 腾讯云 COS SDK
- MinIO Go SDK
- 七牛云 Go SDK
- Config facade

## 文件结构

```
filesystem/
├── application.go    # 存储应用实现
├── provider.go       # 服务提供者
├── s3/              # AWS S3 驱动
├── oss/             # 阿里云 OSS 驱动
├── cos/             # 腾讯云 COS 驱动
├── minio/           # MinIO 驱动
└── qiniu/           # 七牛云驱动
```

## 安全建议

1. 使用 IAM 角色而不是硬编码凭证
2. 限制存储桶的公共访问
3. 使用临时 URL 而不是公开文件
4. 验证上传文件的类型和大小
5. 对敏感文件进行加密存储
