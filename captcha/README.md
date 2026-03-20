# Captcha 组件

点击式验证码生成和验证组件，支持中文字符验证码。

## 功能特性

- 点击式验证码生成
- 可自定义字符集（支持中文）
- 可配置图片尺寸
- 可配置字符数量和内边距
- 支持自定义字体和背景图
- Base64 编码的图片输出
- 坐标验证

## 使用方法

### 生成验证码

```go
import "github.com/herhe-com/framework/captcha"

// 生成验证码
result, err := captcha.Click()
if err != nil {
    // 处理错误
}

// 返回给前端
response := map[string]interface{}{
    "master": result.Master,     // 主图（Base64）
    "thumb":  result.Thumb,      // 缩略图（Base64）
    "key":    result.Key,        // 验证码标识
}
```

### 验证验证码

```go
// 用户点击坐标
dots := []captcha.Dot{
    {X: 100, Y: 150},
    {X: 200, Y: 180},
}

// 验证
valid := captcha.ClickVerify(result.Key, dots)
if valid {
    // 验证通过
} else {
    // 验证失败
}
```

## 配置选项

在配置文件中自定义验证码参数：

```yaml
captcha:
  width: 300              # 主图宽度
  height: 200             # 主图高度
  thumb_width: 150        # 缩略图宽度
  thumb_height: 40        # 缩略图高度
  char_count: 4           # 字符数量
  padding: 20             # 内边距
  font_path: fonts/custom.ttf        # 自定义字体路径
  background: images/bg.jpg          # 背景图路径
  chars: "的一是在不了有和人这中大为上个国我以要他时来用们生到作地于出就分对成会可主发年动同工也能下过子说产种面而方后多定行学法所民得经十三之进着等部度家电力里如水化高自二理起小物现实加量都两体制机当使点从业本去把性好应开它合还因由其些然前外天政四日那社义事平形相全表间样与关各重新线内数正心反你明看原又么利比或但质气第向道命此变条只没结解问意建月公无系军很情者最立代想已通并提直题党程展五果料象员革位入常文总次品式活设及管特件长求老头基资边流路级少图山统接知较将组见计别她手角期根论运农指几九区强放决西被干做必战先回则任取据处队南给色光门即保治北造百规热领七海口东导器压志世金增争济阶油思术极交受联什认六共权收证改清己美再采转更单风切打白教速花带安场身车例真务具万每目至达走积示议声报斗完类八离华名确才科张信马节话米整空元况今集温传土许步群广石记需段研界拉林律叫且究观越织装影算低持音众书布复容儿须际商非验连断深难近矿千周委素技备半办青省列习响约支般史感劳便团往酸历市克何除消构府称太准精值号率族维划选标写存候毛亲快效斯院查江型眼王按格养易置派层片始却专状育厂京识适属圆包火住调满县局照参红细引听该铁价严"
```

## 核心类型

### ClickResponse

```go
type ClickResponse struct {
    Master string      // 主图 Base64 编码
    Thumb  string      // 缩略图 Base64 编码
    Key    string      // 验证码唯一标识
    Dots   []Dot       // 正确的点击坐标（内部使用）
}
```

### Dot

```go
type Dot struct {
    X int  // X 坐标
    Y int  // Y 坐标
}
```

## 工作原理

### 生成流程

1. 从字符集中随机选择指定数量的字符
2. 在主图上随机位置绘制这些字符
3. 记录每个字符的坐标
4. 生成包含相同字符的缩略图
5. 将坐标信息存储到 Redis（带过期时间）
6. 返回 Base64 编码的图片和验证码标识

### 验证流程

1. 从 Redis 获取存储的正确坐标
2. 比较用户点击坐标与正确坐标
3. 允许一定的误差范围（默认 ±10 像素）
4. 验证通过后删除 Redis 中的记录（防止重复使用）

## 高级用法

### 自定义字符集

```go
// 使用英文字符
captcha.SetCharset("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

// 使用数字
captcha.SetCharset("0123456789")
```

### 自定义验证容差

```go
// 设置更严格的验证（±5 像素）
captcha.SetTolerance(5)

// 设置更宽松的验证（±15 像素）
captcha.SetTolerance(15)
```

### 自定义过期时间

```go
// 设置验证码 5 分钟过期
captcha.SetExpiration(5 * time.Minute)
```

## 前端集成示例

### 获取验证码

```javascript
// 请求验证码
fetch('/api/captcha/generate')
  .then(res => res.json())
  .then(data => {
    // 显示主图
    document.getElementById('master-img').src = 'data:image/png;base64,' + data.master;
    
    // 显示缩略图（提示用户点击哪些字符）
    document.getElementById('thumb-img').src = 'data:image/png;base64,' + data.thumb;
    
    // 保存验证码 key
    captchaKey = data.key;
  });
```

### 收集点击坐标

```javascript
const dots = [];

document.getElementById('master-img').addEventListener('click', (e) => {
  const rect = e.target.getBoundingClientRect();
  const x = e.clientX - rect.left;
  const y = e.clientY - rect.top;
  
  dots.push({ x: Math.round(x), y: Math.round(y) });
  
  // 绘制点击标记
  drawDot(x, y);
});
```

### 提交验证

```javascript
fetch('/api/captcha/verify', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    key: captchaKey,
    dots: dots
  })
})
.then(res => res.json())
.then(data => {
  if (data.valid) {
    // 验证通过
  } else {
    // 验证失败，重新生成验证码
  }
});
```

## 安全建议

1. 验证码应设置合理的过期时间（建议 2-5 分钟）
2. 验证后立即删除验证码记录，防止重复使用
3. 限制同一 IP 的验证码生成频率
4. 记录验证失败次数，多次失败后增加难度或临时封禁
5. 使用 HTTPS 传输验证码数据

## 依赖项

- go-captcha（验证码生成库）
- Redis（存储验证码数据）
- Config facade（配置管理）

## 文件结构

```
captcha/
├── application.go    # 验证码生成和验证逻辑
└── provider.go       # 服务提供者
```

## 性能优化

1. 使用 Redis 缓存字体文件，避免重复加载
2. 预生成验证码池，减少实时生成压力
3. 使用 CDN 分发背景图片
4. 压缩 Base64 图片大小
