# 配置示例目录

这个目录把框架的配置按模块拆开，方便直接对照各模块实际读取的 key。

## 文件说明

- `app.yaml`：应用基础信息。
- `env.example.yaml`：所有模块合并后的单文件示例，可直接复制到 `conf/env.yaml`。
- `cache.yaml`：缓存相关配置。
- `console.md`：HTTP / CLI 启动相关配置。
- `database.yaml`：ORM、Redis。
- `filesystem.yaml`：对象存储与磁盘映射。
- `auth.yaml`：JWT、Casbin、登录限制、权限树。
- `queue.yaml`：RabbitMQ 队列配置，使用 `default` 选择默认连接名，再用 `connections.<name>.driver`。
- `search.yaml`：Elasticsearch、Meilisearch，使用 `default` 选择默认连接名，再用 `connections.<name>.driver`。
- `ai.yaml`：OpenAI、Ollama。
- `captcha.yaml`：点击式验证码。
- `validation.yaml`：字段标签和多语言翻译。
- `microservice.yaml`：Snowflake 节点配置。
- `crontab.yaml`：定时任务说明。
- `kernel.md`：`kernel.providers` 和 `kernel.consoles` 的 Go 注入说明。

## 使用方式

1. 只复制你需要的段落到项目自己的 `conf/env.yaml`。
2. 如果想先从单文件起步，直接复制 `env.example.yaml`。
3. 需要 Go 类型的配置，按 `kernel.md` 里的方式在代码里写入 `facades.Cfg`。
4. `database.orm.default` 只保存默认连接名；真正的 ORM 配置要放在 `database.orm.connections.<name>`，其中 `driver` 和 `prefix` 都按连接名读取。Redis、文件系统、队列和搜索也分别使用 `database.redis.default` / `database.redis.connections.<name>.db`、`filesystem.default` / `filesystem.disks.<disk>.driver`、`queue.default` / `queue.connections.<name>.driver`、`search.default` / `search.connections.<name>.driver` 这类默认选择名 + 实例级字段。

## 注意

- 这些示例是按当前实现整理的，不是抽象设计稿。
- 函数、回调、`[]service.Provider`、`[]console.Provider` 这类值不能用纯 YAML 完整表达，必须通过 Go 代码注入。
