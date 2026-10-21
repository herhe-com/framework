# Changelog

## v1.11.6 (2026-10-22)

### 🚀 Features

- **filesystem**: 新增 `TemporaryUpload` 直接上传凭证与 `InitiateMultipartUpload`/`UploadMultipartPart`/`CompleteMultipartUpload`/`AbortMultipartUpload` 分片上传生命周期，s3、oss、cos、minio、qiniu 驱动全部实现 ([458df5e](https://github.com/herhe-com/framework/commit/458df5e))
- **captcha**: 新增场景化配置与中间件 ([27fe8d8](https://github.com/herhe-com/framework/commit/27fe8d8))
- **console**: Swagger UI 展示扩展字段 `showExtensions` ([9ea41b4](https://github.com/herhe-com/framework/commit/9ea41b4))

### 🐛 Bug Fixes

- **captcha**: 拒绝非法资源与滑块输入 ([33b1f88](https://github.com/herhe-com/framework/commit/33b1f88))
- **util**: 验证码数字改用加密安全随机数生成 ([bd54a4c](https://github.com/herhe-com/framework/commit/bd54a4c))

### 🔨 Refactor

- **console**: Swagger UI 迁移到 swaggo/http-swagger 并通过 HTTP adaptor 挂载 ([d06ee16](https://github.com/herhe-com/framework/commit/d06ee16))

### ⚙️ Miscellaneous

- 升级全部依赖模块至最新版本 ([48c60f3](https://github.com/herhe-com/framework/commit/48c60f3))

**完整变更**: https://github.com/herhe-com/framework/compare/v1.11.5...v1.11.6

## v1.11.5 (2026-09-21)

### 🐛 Bug Fixes

- **queue/rabbitmq**: 复用缓存的 Publisher 并安全关闭，`publish` 不再为每条消息重复创建 Publisher，关闭状态改用 `atomic.Bool` 保证并发安全，关闭后的发布/消费返回 `ErrClosed` ([87b63bc](https://github.com/herhe-com/framework/commit/87b63bc))
- **queue/rabbitmq**: 拒绝同时启用 `delayed` 与 `ttl` 选项的队列配置，启动时校验并报错 ([80e69a8](https://github.com/herhe-com/framework/commit/80e69a8))

### 📝 补充说明

- RabbitMQ 驱动新增 `publisherKey` 缓存机制，Publisher 构造选项改为惰性构建（仅缓存未命中时创建），显著降低连接与资源开销
- 新增 `ErrClosed` 错误变量，用于驱动关闭后的操作防护

**完整变更**: https://github.com/herhe-com/framework/compare/v1.11.4...v1.11.5
