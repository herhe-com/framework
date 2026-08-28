# Captcha 组件

验证码生成与验证组件，支持点击、滑块、旋转三种验证码，并可随机选择验证码类型。

## 功能特性

- 支持 `click`、`slide`、`rotate` 和 `auto` 驱动
- 点击、滑块、旋转分别由独立驱动包实现
- 统一返回验证码类型、标识、主图和提示图
- 验证数据存储到 Redis，并在验证成功后删除
- 支持配置有效期、图片尺寸和验证容差
- 点击验证码支持自定义字符集与字体
- 支持自定义背景图和滑块素材
- 未配置资源目录时使用内置资源

## 配置

```yaml
captcha:
  enable: true           # 是否启用验证码，关闭后生成返回 nil，校验直接通过
  driver: click          # click、slide、rotate 或 auto，默认 click
  expire: 300            # Redis 中的有效期，单位秒，必须大于 0

  click:
    min: 4               # 最少点击字符数
    max: 4               # 最多点击字符数
    width: 300            # 主图宽度
    height: 220           # 主图高度
    char: ""              # 自定义字符集，空值使用内置中文字符
    padding: 5            # 点击坐标验证容差

  slide:
    width: 300            # 主图宽度
    height: 220           # 主图高度
    padding: 5            # 滑块横坐标验证容差

  rotate:
    size: 220             # 方形图片边长
    padding: 5            # 旋转角度验证容差

  resources:
    bg: ""                # 背景图目录，支持 png、jpg、jpeg
    font: ""              # 点击验证码字体目录，支持 ttf
    tile: ""              # 滑块素材目录
```

资源目录相对于 `facades.Root()`。留空时使用组件内置资源。

自定义滑块素材目录中的每套素材需要放在独立子目录内：

```text
resources/tile/
└── default/
    ├── tile.png
    ├── tile-shadow.png
    └── tile-mask.png
```

## 服务注册

验证码应用与 `filesystem` 一样通过服务提供者注册，并由 facade 统一访问：

```go
facades.Cfg.Add("kernel", map[string]any{
    "providers": []service.Provider{
        &redis.ServiceProvider{},
        &captcha.ServiceProvider{},
    },
})
```

应用层负责驱动选择、缓存和 Redis 生命周期，具体实现分别位于 `captcha/click`、`captcha/slide`、`captcha/rotate`。未注册服务提供者时，原有包级 `Generate` 和 `Verify` 仍可独立使用。

## 生成验证码

推荐使用 `facades.Captcha().Generate`。它会按 `captcha.driver` 选择服务商，并将验证数据写入已初始化的默认 Redis 连接。

```go
package handler

import (
    "context"

    "github.com/herhe-com/framework/facades"
)

func GenerateCaptcha(ctx context.Context) error {
    result, err := facades.Captcha().Generate(ctx)
    if err != nil {
        return err
    }

    // result.Key：验证码标识，验证时原样提交
    // result.Driver：click、slide 或 rotate
    // result.Master：主图 Base64
    // result.Thumb：提示图、滑块图或旋转缩略图 Base64
    // result.Y：仅 slide 模式返回，表示缺口在主图上的纵向坐标，用于前端对齐滑块；其他模式为 0
    _ = result

    return nil
}
```

返回结构：

```json
{
  "key": "df62452fc5574fa59c465a6b80ec80e2",
  "driver": "click",
  "master": "...",
  "thumb": "..."
}
```

slide 模式还会额外返回缺口所在位置的纵向坐标 `y`，前端需要用它把滑块拼图垂直对齐到主图缺口上（横向 `x` 是验证答案，不会返回）：

```json
{
  "key": "df62452fc5574fa59c465a6b80ec80e2",
  "driver": "slide",
  "master": "...",
  "thumb": "...",
  "y": 96
}
```

当驱动为 `auto` 时，每次会从 `click`、`slide`、`rotate` 中随机选择一种，前端应根据返回的 `driver` 展示对应交互。

## 验证验证码

`facades.Captcha().Verify` 会根据 Redis 中保存的验证码类型选择对应服务商。验证成功后会删除 Redis 记录，验证码不能重复使用；验证失败时记录会保留到过期。

```go
package handler

import (
    "context"

    captchacontract "github.com/herhe-com/framework/contracts/captcha"
    "github.com/herhe-com/framework/facades"
)

func VerifyClick(ctx context.Context, key string) error {
    return facades.Captcha().Verify(ctx, captchacontract.VerifyData{
        Key: key,
        Dots: []captchacontract.Dot{
            {Index: 0, X: 100, Y: 150},
            {Index: 1, X: 180, Y: 120},
        },
    })
}

func VerifySlide(ctx context.Context, key string, x int) error {
    return facades.Captcha().Verify(ctx, captchacontract.VerifyData{Key: key, X: x})
}

func VerifyRotate(ctx context.Context, key string, angle int) error {
    return facades.Captcha().Verify(ctx, captchacontract.VerifyData{Key: key, Angle: angle})
}
```

对应的请求数据可以统一为：

```json
{
  "key": "df62452fc5574fa59c465a6b80ec80e2",
  "dots": [{"index": 0, "x": 100, "y": 150}],
  "x": 0,
  "angle": 0
}
```

只需提交当前 `driver` 使用的字段：

| 驱动 | 验证字段 | 说明 |
| --- | --- | --- |
| `click` | `dots` | 点击点数组，每个点包含 `index`、`x`、`y` |
| `slide` | `dots[0].x/y` | 滑块最终坐标；顶层 `x` 已废弃，仅保留一个版本周期且无法校验 `x=0` |
| `rotate` | `angle` | 用户旋转后的角度 |

## 安全说明：暴露与隐藏

返回给客户端的响应只包含渲染必需的信息，**绝不包含校验答案**：

| 驱动 | 返回给客户端 | 隐藏（仅存 Redis `target` / 驱动内存） |
| --- | --- | --- |
| `click` | `master`、`thumb` | 点击点坐标 `dots`、索引、文字 |
| `slide` | `master`、`thumb`、`y`（缺口纵向坐标） | 横向答案 `x` |
| `rotate` | `master`、`thumb` | 旋转角度 `angle` |

- 校验答案通过 `key` 关联存入 Redis，客户端只提交 `key` 与用户操作结果，服务端比对后删除记录。
- legacy 的 `captcha.Click` / `captcha.Slide` / `captcha.Rotate` 结构内携带完整答案字段（`Dots` / `Block`），已通过 `json:"-"` 隐藏，不应直接序列化返回给客户端，仅供服务端复用与校验。
- 前端需要的展示坐标（如滑块缺口纵向位置 `y`）会显式返回，但横向/点击答案永远不会包含在响应中。

## 直接调用单一服务商

应用可以像选择文件系统磁盘一样选择具体验证码驱动：

```go
driver, err := facades.Captcha().Driver(frameworkcaptcha.DriverSlide)
if err != nil {
    return err
}

challenge, err := driver.Generate()
```

驱动层只负责生成和校验，不会生成 `key`，也不会读写 Redis。`challenge.Target` 是内部正确答案，不能返回给前端；slide 驱动会在 `challenge.Y` 上暴露缺口纵向展示坐标，供前端对齐滑块使用。原有 `Click`、`Slide`、`Rotate` 及对应校验函数作为兼容入口继续保留。

## Redis 要求

调用 `Generate` 和 `Verify` 前必须初始化默认 Redis 连接。验证码记录使用以下键格式：

```text
{app.name}:captcha:{key}
```

`captcha.expire` 是 Redis 有效期，单位秒，未设置时默认 `300`，必须大于 `0`。

## 错误处理

以下情况会返回错误：

- 配置了不支持的驱动
- `captcha.expire` 小于或等于 `0`
- 默认 Redis 未初始化
- 验证码不存在或已过期
- 用户提交的数据与目标不匹配
- 已配置的自定义背景资源无法读取或解析
