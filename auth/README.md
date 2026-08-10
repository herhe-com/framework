# Auth 组件

`auth` 提供 JWT、Casbin 权限、密码哈希、临时角色和 token 黑名单能力。它依赖 `facades.Cfg`，部分能力依赖 `facades.DB` 和 `facades.Redis`。

## 配置

```yaml
app:
  name: example

jwt:
  secret: your-secret
  sub: default
  lifetime: 720 # minutes
  refresh:
    mode: blacklist
    lifetime: 30 # days
    leeway: 3 # seconds

auth:
  casbin:
    table: sys_casbin
    database: default
  platforms:
    - 400
```

`auth.ServiceProvider` 会初始化 Casbin，因此需要先初始化数据库，并在项目根目录提供 `conf/casbin.conf`。

## JWT

登录时签发独立的 Access Token 和 Refresh Token：

```go
pair, err := auth.NewLoginJWToken(
	"10000",
	true,
	map[string]any{
		"user_type": "reviewer",
		"reviewer_id": 10000,
	},
)
```

`NewLoginJWToken` 自动读取 `jwt.sub`、`jwt.lifetime` 和 `jwt.refresh.lifetime`：Access 有效期单位为分钟，默认 720；Refresh 有效期单位为天，默认 30。传入 `refresh=false` 时仍返回 `TokenPair`，但只包含 Access Token 及其有效期；`refresh=true` 时返回完整令牌对。`TokenPair.issued_at` 是不带时区语义的 Unix 秒，`access_lifetime`、`refresh_lifetime` 和 `grace_lifetime` 均为秒，客户端通过签发时间加有效期计算失效时间。Access Token 只能访问业务接口，Refresh Token 只能用于轮换令牌对；刷新时不要求 Access Token 仍然有效。

底层兼容 API `NewJWToken` 和 `NewJWTokens` 仍保留显式 lifetime 参数，其中 `NewJWTokens` 的两个 lifetime 参数单位仍是分钟；新登录流程应优先使用 `NewLoginJWToken`。

校验 Access Token：

```go
var claims authContract.Claims

if err := auth.ValidateAccessToken(ctx, &claims, pair.AccessToken); err != nil {
	return err
}
```

刷新令牌对：

```go
pair, err = auth.RefreshJWTokens(ctx, request.RefreshToken)
if err != nil {
	return err
}
```

需要中间件无感刷新时，请求同时携带：

```http
Authorization: <access_token>
Refresh-Token: <refresh_token>
```

JWT 中间件会先校验 Access Token。Access Token 有效且没有携带 Refresh Token 时才会继续业务请求；如果 Access Token 仍然有效却提前上传 Refresh Token，请求会直接返回未授权，且不会消费 Refresh Token。只有 Access Token 无效或缺失时才会继续校验 Refresh Token，因此客户端必须等 Access Token 失效后再上传 Refresh Token。Refresh Token 的 `nbf` 尚未到、已过期、类型错误、已撤销或已在容错窗口外使用时会被拒绝。

刷新不依赖客户端的 `singleflight`、mutex 或请求暂停。第一次使用某个旧 Refresh Token 时，Redis Lua 会原子地消费它，并把新令牌对写入一个短期 Hash；`jwt.refresh.leeway` 秒内再次使用同一个旧 Refresh Token，会返回完全相同的 `TokenPair`。Hash 包含：

- `access_token`
- `refresh_token`
- `issued_at`
- `access_lifetime`
- `refresh_lifetime`
- `grace_lifetime`
- `session_id`

容错 Hash 使用 Redis `TIME` 计算自身的过期时间，并在当前 Redis 时间加 `grace_lifetime` 后删除。窗口结束后，旧 Refresh Token 会被拒绝。HTTP JWT 中间件先调用 `ValidateAccessToken` 完成 Access Token 类型、签名、有效期和 Bloom 黑名单校验；失败且存在 `Refresh-Token` 时，再调用 `RefreshJWTokens`。同批并发请求因此会收到完全相同的 `Token-Pair` Header。`Auth` 中间件只负责要求当前路由必须已经登录，不会重复查询 Redis。

可通过 `auth.callback.auth` 为 `Auth` 中间件追加业务校验，通过 `auth.callback.jwt` 在 JWT 上下文建立后追加校验。两者都支持 `func(context.Context, *app.RequestContext) error`，返回错误时请求会被终止并返回未授权；`auth.callback.jwt` 仍兼容原有的无返回值函数签名。

`Claims` 中的 `typ` 区分令牌类型，`sid` 关联同一轮登录会话，Refresh Token 的 `atl` 保存下一枚 Access Token 的有效时长（秒）：

```go
type Claims struct {
	jwt.RegisteredClaims
	Type           string         `json:"typ,omitempty"`
	SessionID      string         `json:"sid,omitempty"`
	AccessLifetime int64          `json:"atl,omitempty"`
	Ext            map[string]any `json:"ext,omitempty"`
}
```

退出登录时分别撤销客户端持有的两枚令牌：

```go
_, err := auth.BlacklistOfJwtValue(ctx, requestCtx) // 当前 Access Token
err = auth.RevokeRefreshToken(ctx, request.RefreshToken)
```

Access Token 和黑名单模式下已使用/已撤销的 Refresh Token 都使用 RedisBloom，按 UTC 到期日期 `YYYYMMDD` 分桶，桶在对应日期结束时自动过期。Bloom Filter 可能误判一个未加入的 ID 为已加入（多拒绝），但不会把仍在过滤器中的 ID 判断为不存在。

刷新策略由 `jwt.refresh.mode` 控制：

- `blacklist`（默认）：登录签发不写 Redis；Refresh Token 首次使用或撤销时，旧 `jti` 才写入日期 Bloom 桶。
- `whitelist`：登录签发时精确登记 Refresh Token 的 `jti`，轮换时原子删除旧 key 并登记新 key。

两种刷新模式都要求 Redis，因为重复使用检测和并发容错 Hash 必须由所有服务器共享。黑名单模式还要求 Redis 服务加载 RedisBloom 模块（例如 Redis Stack）。不在黑名单中的有效令牌直接放行，因此切换应用服务器不会让 JWT 本身失效。

兼容入口 `NewJWToken` 现在只签发 Access Token，旧 `refresh` 参数被忽略；`CheckJWToken` 只做严格 Access Token 校验且不再返回自动刷新信号。旧 `RefreshJWToken(ctx, claims)` 无法证明调用方持有签名后的 Refresh Token，因此会明确返回错误，请迁移到 `RefreshJWTokens`。

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
- 黑名单和临时角色依赖 Redis，使用前需要注册 `redis.ServiceProvider`；JWT 黑名单还要求 RedisBloom 模块。
- 当前没有 `token.Create()`、`token.Check()` 这类对象式 API。
