# JWT 中间件校验与接入说明

本文档说明 `http/middleware.Jwt()` 的实际校验规则、自动刷新机制、错误响应和业务接入边界，可作为前端、客户端及外部合作方的接口说明。

> `Jwt()` 负责解析身份、校验 Access Token 和自动刷新令牌，本身不强制要求用户登录。受保护接口必须在 `Jwt()` 后继续使用 `Auth()`。

```go
middleware.Jwt(),
middleware.Auth(),
```

如需权限控制，再追加：

```go
middleware.Permission("permission-name"),
```

## 1. 请求头约定

### 1.1 普通业务请求

```http
Authorization: <access_token>
```

`Authorization` 中直接传递 JWT 字符串，不支持 `Bearer` 前缀。

错误示例：

```http
Authorization: Bearer eyJ...
```

正确示例：

```http
Authorization: eyJ...
```

### 1.2 Access Token 失效后的刷新请求

```http
Authorization: <已失效或无效的 access_token>
Refresh-Token: <refresh_token>
```

代码也允许只携带 `Refresh-Token` 触发刷新，但推荐客户端保留原 Access Token，以便统一请求流程。

如果 Access Token 仍然有效，同时携带了 `Refresh-Token`，请求会直接返回未授权，并且不会消费 Refresh Token。因此客户端不能在 Access Token 仍有效时提前刷新。

## 2. JWT 基础校验规则

Access Token 和 Refresh Token 都会执行以下基础校验：

| 校验项 | 规则 |
| --- | --- |
| JWT 格式 | 必须是合法的 JWT 紧凑格式 |
| 签名算法 | 只允许 `HS256` |
| 签名密钥 | 使用 `jwt.secret`，不能为空 |
| `iss` | 必须存在，并严格等于 `app.name + ":" + jwt.sub` |
| `exp` | 必须存在，服务器当前时间必须早于 `exp` |
| `nbf` | 必须存在，服务器当前时间必须达到或超过 `nbf` |
| `iat` | 必须存在，并且不能是未来时间 |
| `sub` | 必须为非空字符串，作为当前登录用户 ID |
| `jti` | 必须为非空字符串，用于撤销和防重放 |
| `aud` | 当前不校验，也不要求存在 |
| 时间容差 | 当前 JWT 时间校验没有额外时钟容差 |

例如：

```yaml
app:
  name: example

jwt:
  sub: api
```

令牌中的发行方必须为：

```text
example:api
```

修改 `app.name`、`jwt.sub` 或 `jwt.secret` 后，已签发令牌将无法继续通过校验。

`jwt.refresh.leeway` 是 Refresh Token 并发重用的容错时间，不是 `exp`、`nbf`、`iat` 的时钟偏差容忍时间。

## 3. Claims 字段

框架签发的 Claims 类似：

```json
{
  "iss": "example:api",
  "sub": "10000",
  "exp": 1780000000,
  "nbf": 1779990000,
  "iat": 1779990000,
  "jti": "...",
  "typ": "access",
  "sid": "...",
  "atl": 43200,
  "ext": {
    "user_type": "reviewer"
  }
}
```

| 字段 | 含义及规则 |
| --- | --- |
| `typ` | 令牌类型，框架签发值为 `access` 或 `refresh` |
| `sid` | 登录会话 ID；同一令牌对以及后续轮换保持一致 |
| `atl` | Refresh Token 专用，表示下一枚 Access Token 的有效时长，单位为秒 |
| `ext` | 自定义业务数据，刷新时原样继承 |
| `ref` | 已废弃字段，不应继续使用 |

令牌类型校验规则：

- Access Token 的 `typ` 可以是 `access`；为兼容旧令牌，缺少 `typ` 也可以通过。
- Refresh Token 的 `typ` 必须严格等于 `refresh`。
- Refresh Token 必须包含非空 `sid`，且 `atl > 0`。
- Refresh Token 的总有效期必须大于零且不能超过 30 天。
- Access Token 校验目前不强制要求 `sid`，但框架正常签发的令牌都会包含该字段。

## 4. Access Token 完整校验流程

Access Token 会依次执行：

1. 校验 JWT 格式、HS256 签名和 `jwt.secret`。
2. 校验 `iss`、`exp`、`nbf`、`iat`。
3. 检查 `sub`、`jti` 等必要字段。
4. 检查令牌类型是否为 Access Token。
5. 使用 `jti` 查询 RedisBloom 黑名单。
6. 未被撤销时建立登录上下文。

Access Token 黑名单按令牌到期日期分桶，例如：

```text
<app.name>:blacklist:jwt:20260810
```

Redis 查询使用：

```text
BF.EXISTS <bucket> <jti>
```

因此：

- Access Token 校验依赖 Redis，并要求 Redis 支持 RedisBloom。
- Redis 不可用或 `BF.EXISTS` 执行失败时不会降级放行，而是返回认证服务不可用。
- Bloom Filter 可能以极低概率把未撤销令牌判断为已撤销，但不会把已经撤销的令牌判断为未撤销。

## 5. 中间件分支行为

| Access Token | Refresh Token | 处理结果 |
| --- | --- | --- |
| 未携带 | 未携带 | `Jwt()` 继续请求，但不建立登录身份 |
| 有效 | 未携带 | 建立登录身份并继续业务请求 |
| 有效 | 已携带 | 直接未授权，不消费 Refresh Token |
| 无效、过期或被撤销 | 有效 | 消费旧 Refresh Token，轮换令牌对并继续原业务请求 |
| 未携带 | 有效 | 允许轮换令牌对并继续请求 |
| 无效 | 未携带 | `Jwt()` 继续请求，但不建立登录身份 |
| 无效或缺失 | 无效、过期、已撤销或已使用 | 返回未授权 |
| Access 黑名单服务异常 | 任意 | HTTP 500，不再尝试刷新 |
| Refresh 状态服务异常 | 已携带 | HTTP 500 |

最重要的接入约束是：

> `Jwt()` 遇到无令牌，或者无效 Access Token 且没有 Refresh Token 时，不会直接拦截，而是以匿名身份继续执行。

受保护接口必须组合：

```go
middleware.Jwt(),
middleware.Auth(),
```

`Auth()` 会检查 JWT 中间件是否已经写入用户 ID，没有身份时返回未授权。

## 6. Refresh Token 轮换

Refresh Token 校验成功后，框架会：

1. 原子消费旧 Refresh Token。
2. 生成新的 Access Token 和 Refresh Token。
3. 保留原来的 `sub`、`iss`、`sid` 和 `ext`。
4. 为两枚新令牌生成新的 `jti`。
5. 将新令牌对写入响应头 `Token-Pair`。
6. 使用新 Access Token 建立身份并继续执行当前业务接口。

成功响应头示例：

```http
Token-Pair: {"session_id":"...","access_token":"...","refresh_token":"...","issued_at":1780000000,"access_lifetime":43200,"refresh_lifetime":2592000,"grace_lifetime":3}
Cache-Control: no-store
Pragma: no-cache
```

时间字段规则：

- `issued_at`：Unix 秒。
- `access_lifetime`：秒。
- `refresh_lifetime`：秒。
- `grace_lifetime`：秒。

客户端收到 `Token-Pair` 后必须同时替换本地 Access Token 和 Refresh Token，不能只更新其中一枚。

浏览器跨域调用时，应确保 CORS 配置允许前端读取 `Token-Pair` 响应头。

## 7. 并发刷新容错

配置示例：

```yaml
jwt:
  refresh:
    leeway: 3
```

默认容错窗口为 3 秒。负数会按 `0` 处理。

同一批并发请求可能同时使用相同的旧 Refresh Token。框架通过 Redis Lua 保证：

- 第一个请求原子消费旧 Refresh Token 并生成新令牌对。
- 容错窗口内的后续请求返回完全相同的新 `Token-Pair`。
- 不会为每个并发请求分别生成不同令牌。
- 容错窗口结束后再次使用旧 Refresh Token，将返回未授权。

## 8. Refresh Token 存储模式

### 8.1 blacklist，默认模式

```yaml
jwt:
  refresh:
    mode: blacklist
```

- 登录签发时不登记 Refresh Token。
- 首次使用或撤销时，将旧 `jti` 写入 RedisBloom。
- 已使用、已撤销或被 Bloom Filter 判断存在的令牌不能再次使用。
- 依赖 Redis 和 RedisBloom。

### 8.2 whitelist

```yaml
jwt:
  refresh:
    mode: whitelist
```

- 签发时在 Redis 精确登记每个 Refresh Token 的 `jti`。
- 刷新时原子删除旧记录并登记新记录。
- 未登记、已经删除或已撤销的 Refresh Token 不能使用。

两种模式都依赖 Redis 完成跨服务器防重放和并发刷新。除明确配置为 `whitelist` 外，其他值都会按 `blacklist` 处理。

## 9. 身份、用户状态与权限边界

JWT 校验成功后，中间件写入：

- `auth.ID(ctx)`：来自 JWT 的 `sub`。
- `auth.Claims(ctx)`：完整 Claims。
- `auth.Platform(ctx)`：配置的默认平台。
- `claims.Ext`：签发时写入的自定义业务数据。

JWT 中间件默认不会：

- 查询用户是否仍然存在。
- 检查用户是否禁用、冻结或离职。
- 检查密码是否已经修改。
- 检查租户、门店或组织状态。
- 检查 Casbin 权限。
- 重新从数据库加载 `Ext` 中的数据。

需要实时状态校验时，可配置 `auth.callback.jwt`。回调会在 JWT 上下文建立后执行，可以读取 `auth.ID(ctx)` 和 `auth.Claims(ctx)`；返回错误时，请求会被终止并返回未授权。

强制登录后的额外业务校验可以放在 `auth.callback.auth`，权限校验应使用单独的 `Permission()` 中间件。

## 10. 错误响应

普通认证失败不会向客户端暴露“过期、签名错误、类型错误、撤销”等具体原因，统一返回：

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "code": 40100,
  "message": "Unauthorized",
  "data": null
}
```

客户端应判断业务码 `40100`，不能只判断 HTTP 401，因为当前实现使用 HTTP 200 表示普通认证失败。

认证基础设施不可用时返回：

```http
HTTP/1.1 500 Internal Server Error
```

```text
authentication service unavailable
```

可能触发 HTTP 500 的情况包括：

- Access Token 黑名单 Redis/RedisBloom 不可用。
- Refresh Token 轮换 Redis 不可用。
- 刷新后新令牌无法重新校验。
- `Token-Pair` 无法序列化。

## 11. 客户端推荐流程

1. 登录成功后保存完整 TokenPair。
2. 普通请求只发送 `Authorization`。
3. 收到业务码 `40100` 后，如果仍持有 Refresh Token，可使用原请求加上 `Refresh-Token` 重试。
4. Access Token 已失效时，中间件会自动刷新，并继续执行该业务请求。
5. 从响应头读取 `Token-Pair`，原子替换本地两枚令牌。
6. 刷新请求仍返回 `40100` 时，清除登录信息并重新登录。
7. 返回 HTTP 500 时提示服务暂时不可用，不应直接清除用户登录状态。

退出登录必须同时撤销两枚令牌：

- 将当前 Access Token 加入黑名单。
- 撤销当前 Refresh Token。

只撤销 Access Token 时，如果 Refresh Token 仍有效，客户端仍可换取新的 Access Token。

## 12. 相关实现

- [JWT HTTP 中间件](jwt.go)
- [强制登录中间件](auth.go)
- [JWT 签发和 Access Token 校验](../../auth/jwt.go)
- [Refresh Token 轮换](../../auth/jwt_refresh.go)
- [Claims 定义](../../contracts/auth/jwt.go)
- [认证组件说明](../../auth/README.md)
