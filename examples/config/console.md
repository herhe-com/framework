# console / server 示例

`console` 负责命令行入口，`consoles.ServerProvider` 和 `consoles.ServiceProvider` 会读取下面这些配置。

```yaml
server:
  address: 0.0.0.0
  port: "9600"

service:
  address: 0.0.0.0
  port: "8600"
```

## 说明

- `server.options`、`server.middlewares`、`server.route`、`server.handle`、`service.options`、`service.handle` 都是 Go 类型，不能只靠 YAML 配完。
- `kernel.consoles` 必须通过 Go 代码写入 `[]console.Provider`。
- 如果只启动 HTTP 服务，至少需要注册 `console.ServiceProvider` 和 `consoles.ServerProvider`。
