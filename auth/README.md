# Auth 组件

`auth` 提供 JWT、Casbin 权限、密码哈希、临时角色和 token 黑名单能力。它依赖 `facades.Cfg`，部分能力依赖 `facades.DB` 和 `facades.Redis`。

## 配置

```yaml
app:
  name: example

jwt:
  secret: your-secret
  sub: default
  lifetime: 720
  leeway: 3

auth:
  casbin:
    table: sys_casbin
    database: default
  platforms:
    - 400
```

`auth.ServiceProvider` 会初始化 Casbin，因此需要先初始化数据库，并在项目根目录提供 `conf/casbin.conf`。

## JWT

生成 token：

```go
token, err := auth.NewJWToken("10000", 720, true, map[string]any{
	"user_type": "reviewer",
	"reviewer_id": 10000,
})
```

校验 token：

```go
var claims authContract.Claims

refresh, err := auth.CheckJWToken(&claims, token)
if err != nil {
	return err
}

if refresh {
	newToken, err := auth.RefreshJWToken(ctx, &claims)
	if err != nil {
		return err
	}
	_ = newToken
}
```

`NewJWToken` 的 `lifetime` 单位是分钟。`Claims` 使用 `jwt.RegisteredClaims`，业务扩展字段放在 `Ext`：

```go
type Claims struct {
	jwt.RegisteredClaims
	Refresh bool           `json:"ref,omitempty"`
	Ext     map[string]any `json:"ext,omitempty"`
}
```

example 基础项目的 web 登录可以通过 `Ext["user_type"]` 区分 `company` 和 `reviewer` 等用户类型。

## 请求上下文

JWT 中间件会把解析结果写入 Hertz `RequestContext`，业务代码可读取：

```go
id := auth.ID(ctx)
claims := auth.Claims(ctx)
platform := auth.Platform(ctx)
```

如果需要业务身份类型，建议从 `claims.Ext` 中读取：

```go
claims := auth.Claims(ctx)
if claims != nil && claims.Ext["user_type"] == "company" {
	// company user
}
```

## 密码

```go
hash := auth.Password("secret")
ok := auth.CheckPassword("secret", hash)
```

## Casbin

初始化要求：

- `facades.DB` 已初始化。
- `auth.casbin.table` 已配置。
- `auth.casbin.database` 已配置，默认读取 `database.orm.default` 指向的 ORM 连接名。
- `facades.Root + "/conf/casbin.conf"` 文件存在。

常用方法：

```go
allowed, err := facades.Casbin.Enforce(auth.NameOfUser(userID), resource, action)
```

框架提供命名辅助：

```go
user := auth.NameOfUser("10000")
role := auth.NameOfRole("admin")
developer := auth.NameOfDeveloper()
```

## 临时角色

临时角色依赖 Redis：

```go
err := auth.SetTemporaryRole(ctx, requestCtx, platform, org, organization, clique)
role, err := auth.Temporary(ctx, requestCtx)
err = auth.DeleteTemporaryRole(ctx, requestCtx)
```

## 注意事项

- `jwt.secret` 不能为空，否则 token 生成和校验会失败。
- `auth.ServiceProvider` 依赖数据库，注册顺序应晚于 `orm.ServiceProvider`。
- 黑名单和临时角色依赖 Redis，使用前需要注册 `redis.ServiceProvider`。
- 当前没有 `token.Create()`、`token.Check()` 这类对象式 API。
