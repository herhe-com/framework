# kernel 动态配置说明

`kernel.providers` 和 `kernel.consoles` 都是 Go 类型，不能只写成 YAML 字符串。

## providers

```go
facades.Cfg.Add("kernel", map[string]any{
	"providers": []service.Provider{
		&orm.ServiceProvider{},
		&redis.ServiceProvider{},
		&filesystem.ServiceProvider{},
		&validation.ServiceProvider{},
		&queue.ServiceProvider{},
		&search.ServiceProvider{},
		&ai.ServiceProvider{},
		&auth.ServiceProvider{},
		&console.ServiceProvider{},
	},
})
```

## consoles

```go
facades.Cfg.Add("kernel", map[string]any{
	"consoles": []console.Provider{
		&consoles.ServerProvider{},
		&consoles.ServiceProvider{},
		&consoles.MigrationProvider{},
		&consoles.ReloadProvider{},
		&consoles.RestartProvider{},
	},
})
```

## 说明

- provider 顺序要和依赖顺序一致。
- 例如 `auth.ServiceProvider` 依赖数据库和 Casbin，必须放在 `orm.ServiceProvider` 之后。
- `console.ServiceProvider` 需要先注册，`kernel.consoles` 才会被解析并执行。
