# Support 组件

工具函数和辅助功能组件，提供常用的实用工具。

## 功能特性

- Redis 键生成工具
- 文件类型检测
- 文件扩展名获取
- 通用工具函数

## 子组件

### Util - 工具函数

提供常用的工具函数，特别是 Redis 键生成。

#### Redis 键生成

```go
import "github.com/herhe-com/framework/support/util"

// 生成命名空间化的 Redis 键
key := util.Keys("user", "profile", "123")
// 返回: "user:profile:123"

// 多个参数
cacheKey := util.Keys("cache", "product", "1", "details")
// 返回: "cache:product:1:details"

// 单个参数
lockKey := util.Keys("lock", "order-processing")
// 返回: "lock:order-processing"
```

#### 使用场景

```go
// 用户缓存键
userCacheKey := util.Keys("cache", "user", userID)
redis.Set(ctx, userCacheKey, userData, 1*time.Hour)

// 分布式锁键
lockKey := util.Keys("lock", "inventory", productID)
mutex := facades.Locker.NewMutex(lockKey)

// 会话键
sessionKey := util.Keys("session", sessionID)
redis.Get(ctx, sessionKey)

// 限流键
rateLimitKey := util.Keys("ratelimit", userID, "api")
redis.Incr(ctx, rateLimitKey)

// 队列键
queueKey := util.Keys("queue", "email", "pending")
redis.LPush(ctx, queueKey, emailData)
```

### File - 文件工具

提供文件类型检测和扩展名处理功能。

#### 文件类型检测

```go
import "github.com/herhe-com/framework/support/file"

// 从文件内容检测文件类型
fileData, _ := os.ReadFile("image.jpg")
ext := file.Extension(fileData)
fmt.Println(ext) // 输出: jpg

// 支持的文件类型
// 图片: jpg, png, gif, bmp, webp, tiff, ico
// 视频: mp4, avi, mov, wmv, flv, mkv
// 音频: mp3, wav, flac, aac, ogg
// 文档: pdf, doc, docx, xls, xlsx, ppt, pptx
// 压缩: zip, rar, 7z, tar, gz
// 其他: txt, json, xml, html, css, js
```

#### 从文件名获取扩展名

```go
// 从文件名获取扩展名
ext := file.ClientOriginalExtension("document.pdf")
fmt.Println(ext) // 输出: pdf

ext = file.ClientOriginalExtension("image.jpeg")
fmt.Println(ext) // 输出: jpeg

// 处理没有扩展名的文件
ext = file.ClientOriginalExtension("README")
fmt.Println(ext) // 输出: ""
```

#### 应用场景

##### 文件上传验证

```go
func UploadFile(c *app.RequestContext) {
    file, _ := c.FormFile("file")
    
    // 读取文件内容
    src, _ := file.Open()
    defer src.Close()
    
    buffer := make([]byte, 512) // 只需要前 512 字节检测类型
    src.Read(buffer)
    
    // 检测实际文件类型
    actualExt := file.Extension(buffer)
    
    // 获取声明的扩展名
    declaredExt := file.ClientOriginalExtension(file.Filename)
    
    // 验证文件类型
    if actualExt != declaredExt {
        http.BadRequest(c, "文件类型不匹配")
        return
    }
    
    // 验证允许的类型
    allowedTypes := []string{"jpg", "png", "gif", "pdf"}
    if !contains(allowedTypes, actualExt) {
        http.BadRequest(c, "不支持的文件类型")
        return
    }
    
    // 继续处理文件
}
```

##### 文件存储路径生成

```go
func GenerateStoragePath(filename string, content []byte) string {
    // 检测实际文件类型
    ext := file.Extension(content)
    
    // 生成唯一文件名
    uniqueName := fmt.Sprintf("%d-%s", time.Now().Unix(), uuid.New().String())
    
    // 根据文件类型组织路径
    var basePath string
    switch ext {
    case "jpg", "png", "gif", "webp":
        basePath = "images"
    case "pdf", "doc", "docx":
        basePath = "documents"
    case "mp4", "avi", "mov":
        basePath = "videos"
    default:
        basePath = "files"
    }
    
    // 按日期组织
    date := time.Now().Format("2006/01/02")
    
    return fmt.Sprintf("%s/%s/%s.%s", basePath, date, uniqueName, ext)
}
```

##### 文件类型过滤

```go
func FilterImageFiles(files []string) []string {
    imageExts := []string{"jpg", "jpeg", "png", "gif", "webp", "bmp"}
    var images []string
    
    for _, filename := range files {
        ext := file.ClientOriginalExtension(filename)
        if contains(imageExts, ext) {
            images = append(images, filename)
        }
    }
    
    return images
}
```

##### 内容类型设置

```go
func ServeFile(c *app.RequestContext, filename string) {
    // 读取文件
    data, err := facades.Storage.Get(filename)
    if err != nil {
        http.NotFound(c, "文件不存在")
        return
    }
    
    // 检测文件类型
    ext := file.Extension(data)
    
    // 设置 Content-Type
    contentType := getContentType(ext)
    c.Header("Content-Type", contentType)
    
    // 返回文件
    c.Data(200, contentType, data)
}

func getContentType(ext string) string {
    contentTypes := map[string]string{
        "jpg":  "image/jpeg",
        "png":  "image/png",
        "gif":  "image/gif",
        "pdf":  "application/pdf",
        "json": "application/json",
        "xml":  "application/xml",
        "txt":  "text/plain",
        "html": "text/html",
        "css":  "text/css",
        "js":   "application/javascript",
    }
    
    if ct, ok := contentTypes[ext]; ok {
        return ct
    }
    
    return "application/octet-stream"
}
```

## 工具函数列表

### util 包

| 函数 | 说明 | 示例 |
|------|------|------|
| `Keys(parts ...string) string` | 生成 Redis 键 | `Keys("user", "123")` → `"user:123"` |

### file 包

| 函数 | 说明 | 示例 |
|------|------|------|
| `Extension(data []byte) string` | 从内容检测文件类型 | `Extension(imageData)` → `"jpg"` |
| `ClientOriginalExtension(filename string) string` | 从文件名获取扩展名 | `ClientOriginalExtension("file.pdf")` → `"pdf"` |

## 最佳实践

### Redis 键命名

1. **使用命名空间**：避免键冲突
   ```go
   // 好的做法
   util.Keys("app", "user", "cache", userID)
   
   // 避免
   fmt.Sprintf("user_%s", userID)
   ```

2. **保持一致性**：使用统一的键命名规范
   ```go
   // 缓存键
   util.Keys("cache", resource, id)
   
   // 锁键
   util.Keys("lock", resource, id)
   
   // 会话键
   util.Keys("session", sessionID)
   ```

3. **使用有意义的名称**：键名应该清晰表达用途
   ```go
   // 好的做法
   util.Keys("cache", "product", "details", productID)
   
   // 避免
   util.Keys("c", "p", "d", productID)
   ```

### 文件类型检测

1. **始终验证实际类型**：不要仅依赖文件扩展名
   ```go
   // 好的做法
   actualType := file.Extension(fileData)
   if actualType != expectedType {
       return errors.New("invalid file type")
   }
   
   // 避免
   ext := filepath.Ext(filename) // 仅检查扩展名
   ```

2. **白名单验证**：只允许特定的文件类型
   ```go
   allowedTypes := []string{"jpg", "png", "pdf"}
   if !contains(allowedTypes, fileType) {
       return errors.New("file type not allowed")
   }
   ```

3. **大小限制**：检测文件类型前先验证大小
   ```go
   if fileSize > maxSize {
       return errors.New("file too large")
   }
   
   fileType := file.Extension(data)
   ```

## 扩展工具函数

可以在 support 包中添加更多工具函数：

```go
// support/util/string.go
package util

// RandomString 生成随机字符串
func RandomString(length int) string {
    // 实现
}

// Slug 生成 URL 友好的字符串
func Slug(text string) string {
    // 实现
}

// support/util/array.go
package util

// Contains 检查数组是否包含元素
func Contains[T comparable](slice []T, item T) bool {
    for _, v := range slice {
        if v == item {
            return true
        }
    }
    return false
}

// Unique 数组去重
func Unique[T comparable](slice []T) []T {
    seen := make(map[T]bool)
    result := []T{}
    
    for _, v := range slice {
        if !seen[v] {
            seen[v] = true
            result = append(result, v)
        }
    }
    
    return result
}
```

## 依赖项

- h2non/filetype（文件类型检测）

## 文件结构

```
support/
├── util/
│   └── util.go      # 工具函数
└── file/
    └── file.go      # 文件工具
```

## 使用示例

### 完整的文件上传处理

```go
import (
    "github.com/herhe-com/framework/facades"
    "github.com/herhe-com/framework/support/file"
    "github.com/herhe-com/framework/support/util"
    "github.com/herhe-com/framework/http"
)

func UploadHandler(ctx context.Context, c *app.RequestContext) {
    // 获取上传文件
    uploadedFile, err := c.FormFile("file")
    if err != nil {
        http.BadRequest(c, "文件上传失败")
        return
    }
    
    // 打开文件
    src, _ := uploadedFile.Open()
    defer src.Close()
    
    // 读取文件内容
    data, _ := io.ReadAll(src)
    
    // 检测文件类型
    fileType := file.Extension(data)
    
    // 验证文件类型
    allowedTypes := []string{"jpg", "png", "gif", "pdf"}
    if !contains(allowedTypes, fileType) {
        http.BadRequest(c, "不支持的文件类型")
        return
    }
    
    // 生成存储路径
    userID := auth.ID(c)
    filename := fmt.Sprintf("%d-%s.%s", time.Now().Unix(), uuid.New(), fileType)
    storagePath := fmt.Sprintf("uploads/%s/%s", userID, filename)
    
    // 上传到存储
    err = facades.Storage.Put(storagePath, bytes.NewReader(data), int64(len(data)))
    if err != nil {
        http.ServerError(c, "文件保存失败")
        return
    }
    
    // 缓存文件信息
    cacheKey := util.Keys("cache", "file", userID, filename)
    fileInfo := map[string]any{
        "path": storagePath,
        "type": fileType,
        "size": len(data),
        "uploaded_at": time.Now(),
    }
    
    redis := facades.Redis.Default()
    redis.Set(ctx, cacheKey, fileInfo, 24*time.Hour)
    
    // 返回成功响应
    http.Success(c, map[string]string{
        "path": storagePath,
        "type": fileType,
    })
}
```
