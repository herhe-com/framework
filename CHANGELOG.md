# Changelog

## v1.11.5 (2026-09-21)

### 🐛 Bug Fixes

- **queue/rabbitmq**: 复用缓存的 Publisher 并安全关闭，`publish` 不再为每条消息重复创建 Publisher，关闭状态改用 `atomic.Bool` 保证并发安全，关闭后的发布/消费返回 `ErrClosed` ([87b63bc](https://github.com/herhe-com/framework/commit/87b63bc))
- **queue/rabbitmq**: 拒绝同时启用 `delayed` 与 `ttl` 选项的队列配置，启动时校验并报错 ([80e69a8](https://github.com/herhe-com/framework/commit/80e69a8))

### 📝 补充说明

- RabbitMQ 驱动新增 `publisherKey` 缓存机制，Publisher 构造选项改为惰性构建（仅缓存未命中时创建），显著降低连接与资源开销
- 新增 `ErrClosed` 错误变量，用于驱动关闭后的操作防护

**完整变更**: https://github.com/herhe-com/framework/compare/v1.11.4...v1.11.5
