# Search 组件

Search 提供面向 Elasticsearch、Meilisearch 和自定义搜索引擎的连接管理与能力合同。核心层只传递引擎原生请求和原始响应，不定义跨引擎查询 DSL。

## 核心能力

- 所有运行时操作接收 `context.Context`；
- 搜索、文档写入和响应均不丢失引擎原生字段；
- 索引、Bulk、Alias、Raw、Task 等能力通过小型接口按需暴露；
- 驱动由并发安全的 Registry 创建，外部项目可以注册自定义驱动；
- Manager 缓存连接、并发去重构造并统一关闭已创建驱动；
- 错误可通过 `errors.As` 获取状态码、类型、原因、响应头和原始 Body；
- Elasticsearch 支持 v6、v7、v8、v9 transport、认证、TLS、重试和自定义 HTTP Transport；
- Meilisearch 写操作返回真实 task 响应，并提供 task 查询与等待能力。

## 配置

```yaml
search:
  default: elasticsearch

  connections:
    elasticsearch:
      driver: elasticsearch
      version: 8
      hosts:
        - http://127.0.0.1:9200
      prefix: app_

      auth:
        username: ""
        password: ""
        api_key: ""
        bearer_token: ""

      tls:
        ca_file: ""
        certificate_fingerprint: ""
        certificate_file: ""
        key_file: ""
        insecure_skip_verify: false

      transport:
        proxy_url: ""
        dial_timeout: 5s
        response_header_timeout: 10s
        idle_connection_timeout: 90s
        max_idle_connections: 100
        max_idle_connections_per_host: 10

      retry:
        max_retries: 3
        retry_on_timeout: true
        status_codes: [408, 429, 502, 503, 504]
        min_backoff: 100ms
        max_backoff: 2s

    meilisearch:
      driver: meilisearch
      host: http://127.0.0.1:7700
      api_key: ""
      prefix: app_
      task:
        poll_interval: 100ms
```

Elasticsearch 地址配置 `host` 与 `hosts` 二选一：只配其中一个即可，同时配置会报错。认证方式互斥：Basic、API Key、Bearer Token 只能配置一种。无认证连接合法。`host`、顶层 `username/password` 和 Meilisearch 的 `secret` 仍作为旧配置兼容项保留。

Elasticsearch `version` 支持 `6`、`7`、`8`、`9`，也可以写成 `v8` 或 `8.19.7`；未配置时默认使用 v7。

## 获取连接

```go
package example

import (
	"context"

	contractsearch "github.com/herhe-com/framework/contracts/search"
	"github.com/herhe-com/framework/facades"
)

func useSearch(ctx context.Context) error {
	manager := facades.Search()
	driver := manager.Default()

	secondary, err := manager.Connection("meilisearch")
	if err != nil {
		return err
	}

	var _ contractsearch.Driver = driver
	var _ contractsearch.Driver = secondary
	return nil
}
```

默认连接在 Provider 注册时创建；其他连接首次调用 `Connection` 时创建并缓存。同一连接的并发请求只会执行一次构造。

## Elasticsearch 原生搜索

```go
package example

import (
	"context"
	"net/http"
	"net/url"

	contractsearch "github.com/herhe-com/framework/contracts/search"
	"github.com/herhe-com/framework/facades"
	frameworkelastic "github.com/herhe-com/framework/search/elasticsearch"
)

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func searchUsers(ctx context.Context) ([]frameworkelastic.SearchHit[User], error) {
	driver := facades.Search().Default()
	response, err := driver.Search(ctx, "users", contractsearch.SearchRequest{
		Body: []byte(`{
			"query": {"multi_match": {"query": "alice", "fields": ["name^2", "bio"]}},
			"sort": [{"_score": "desc"}],
			"highlight": {"fields": {"name": {}}},
			"aggs": {"roles": {"terms": {"field": "role.keyword"}}}
		}`),
		RequestOptions: contractsearch.RequestOptions{
			Params: url.Values{"routing": {"tenant-1"}},
			Header: http.Header{"X-Opaque-Id": {"request-123"}},
		},
	})
	if err != nil {
		return nil, err
	}

	result, err := frameworkelastic.DecodeSearch[User](response)
	if err != nil {
		return nil, err
	}
	return result.Hits.Hits, nil
}
```

`SearchRequest.Body` 是完整 Elasticsearch DSL。框架不会补全、替换或解析 `query`，`Response.Body` 始终保留完整原始响应。

## 文档操作

```go
func upsertUser(ctx context.Context) error {
	driver := facades.Search().Default()
	_, err := driver.UpsertDocument(ctx, "users", "user-1", contractsearch.DocumentWriteRequest{
		Body: []byte(`{"id":"user-1","name":"Alice"}`),
	})
	return err
}

func getUser(ctx context.Context) (*contractsearch.Response, error) {
	return facades.Search().Default().GetDocument(
		ctx,
		"users",
		"user-1",
		contractsearch.RequestOptions{},
	)
}

func deleteUser(ctx context.Context) error {
	_, err := facades.Search().Default().DeleteDocument(
		ctx,
		"users",
		"user-1",
		contractsearch.RequestOptions{},
	)
	return err
}
```

`UpsertDocument` 始终使用调用方给出的稳定 ID。Get/Delete 遇到不存在文档时返回结构化 404，不会静默忽略。

## 索引管理

```go
func createIndex(ctx context.Context) error {
	driver := facades.Search().Default()
	indexes, ok := driver.(contractsearch.IndexManager)
	if !ok {
		return errors.New("search driver does not support index management")
	}

	_, err := indexes.CreateIndex(ctx, "users", contractsearch.IndexRequest{
		Body: []byte(`{
			"settings": {
				"analysis": {
					"analyzer": {
						"custom_text": {"type": "custom", "tokenizer": "standard"}
					}
				}
			},
			"mappings": {
				"properties": {"name": {"type": "text", "analyzer": "custom_text"}}
			}
		}`),
	})
	return err
}
```

`IndexManager` 明确区分 `CreateIndex`、`DeleteIndex` 和 `IndexExists`，不再提供语义含糊的 `Del`。

## Elasticsearch Bulk

```go
func bulkUsers(ctx context.Context) error {
	driver := facades.Search().Default()
	bulkWriter, ok := driver.(contractsearch.BulkWriter)
	if !ok {
		return errors.New("search driver does not support bulk")
	}

	result, err := bulkWriter.Bulk(ctx, "users", contractsearch.BulkRequest{
		Body: []byte("{\"index\":{\"_id\":\"1\"}}\n{\"name\":\"Alice\"}\n" +
			"{\"index\":{\"_id\":\"2\"}}\n{\"name\":\"Bob\"}\n"),
	})
	if err == nil {
		_ = result
		return nil
	}

	var bulkErr *contractsearch.BulkError
	if errors.As(err, &bulkErr) {
		for _, failed := range bulkErr.Failed {
			log.Printf("bulk failed: operation=%s id=%s status=%d", failed.Operation, failed.ID, failed.StatusCode)
		}
	}
	return err
}
```

框架会确保 NDJSON 以换行结尾。HTTP 200 中的全部失败 item 会同时出现在 `BulkResponse` 和 `BulkError` 中；收到响应后不会自动重试整个 Bulk。

## Elasticsearch Alias 原子切换

```go
func switchAlias(ctx context.Context) error {
	driver := facades.Search().Default()
	aliases, ok := driver.(contractsearch.AliasManager)
	if !ok {
		return errors.New("search driver does not support aliases")
	}

	write := true
	_, err := aliases.UpdateAliases(ctx, contractsearch.AliasRequest{
		Actions: []contractsearch.AliasAction{
			{Action: contractsearch.AliasActionRemove, Index: "users_v1", Alias: "users"},
			{Action: contractsearch.AliasActionAdd, Index: "users_v2", Alias: "users", IsWriteIndex: &write},
		},
	})
	return err
}
```

所有 action 在单个 `/_aliases` 请求中执行；连接 prefix 会同时应用到 index 和 alias。

## RawExecutor

```go
func analyze(ctx context.Context) (*contractsearch.Response, error) {
	driver := facades.Search().Default()
	raw, ok := driver.(contractsearch.RawExecutor)
	if !ok {
		return nil, errors.New("search driver does not support raw execution")
	}

	return raw.Execute(ctx, contractsearch.RawRequest{
		Method: http.MethodPost,
		Path:   "/_analyze",
		Body:   []byte(`{"analyzer":"standard","text":"hello world"}`),
	})
}
```

Raw Path 必须以 `/` 开头，不能包含 scheme、host、query、fragment 或 `..`；Raw 默认不添加 index prefix。Query 参数应放在 `RequestOptions.Params` 中。

## Meilisearch 原生请求与 Task

```go
func saveMeilisearchDocument(ctx context.Context) error {
	driver, err := facades.Search().Connection("meilisearch")
	if err != nil {
		return err
	}

	response, err := driver.UpsertDocument(ctx, "users", "user-1", contractsearch.DocumentWriteRequest{
		Body:       []byte(`{"name":"Alice"}`),
		PrimaryKey: "id",
	})
	if err != nil {
		return err
	}

	var task struct {
		TaskUID int64 `json:"taskUid"`
	}
	if err := response.DecodeJSON(&task); err != nil {
		return err
	}

	tasks, ok := driver.(contractsearch.TaskManager)
	if !ok {
		return errors.New("search driver does not support tasks")
	}
	_, err = tasks.WaitTask(ctx, task.TaskUID, contractsearch.RequestOptions{})
	return err
}
```

Meilisearch 写入响应仅表示 task 已提交。只有 `WaitTask` 返回终态后，调用方才可以把操作视为完成。

Meilisearch 搜索同样直接传递原生 Body：

```go
response, err := driver.Search(ctx, "users", contractsearch.SearchRequest{
	Body: []byte(`{"q":"alice","filter":"active = true","sort":["created_at:desc"]}`),
})
```

如需清空文档但保留索引，断言 `contractsearch.DocumentCleaner` 并调用 `DeleteAllDocuments`；删除索引则使用 `IndexManager.DeleteIndex`。

## 结构化错误

```go
response, err := facades.Search().Default().GetDocument(
	ctx,
	"users",
	"missing",
	contractsearch.RequestOptions{},
)
if err != nil {
	var searchErr *contractsearch.Error
	if errors.As(err, &searchErr) {
		log.Printf(
			"driver=%s operation=%s status=%d type=%s reason=%s",
			searchErr.Driver,
			searchErr.Operation,
			searchErr.StatusCode,
			searchErr.Type,
			searchErr.Reason,
		)
	}
	if contractsearch.IsNotFound(err) {
		_ = response // 原始 404 Response 仍可用
	}
}
```

可用 helper：`IsNotFound`、`IsConflict`、`IsRateLimited`、`IsRetryable`。Transport、TLS、DNS 和 context 错误保存在 `Error.Cause`，支持 `errors.Is`。

## 注册自定义驱动

自定义驱动应在 Search Provider 注册前加入全局 Registry：

```go
func registerOpenSearch() error {
	return frameworksearch.RegisterDriver("opensearch", func(config contractsearch.ConnectionConfig) (contractsearch.Driver, error) {
		return newOpenSearchDriver(config)
	})
}
```

测试可以使用 `frameworksearch.NewRegistry()` 创建隔离 Registry，再通过 `NewSearchWithRegistry` 构造 Manager。

## 程序化构造与自定义 Transport

```go
driver, err := frameworkelastic.New(
	frameworkelastic.Config{
		Connection: "custom",
		Version:    8,
		Hosts:      []string{"https://127.0.0.1:9200"},
	},
	frameworkelastic.WithRoundTripper(tracingTransport),
	frameworkelastic.WithObserver(observer),
)
```

具体驱动构造函数不读取 `facades.Config()` 或 `facades.Validator()`，因此可以独立测试、复用并注入 OpenTelemetry、AWS SigV4 或测试 Transport。

驱动会关闭自己创建的默认 Transport；通过 `WithHTTPClient`、`WithRoundTripper` 或 Meilisearch `Config.HTTPClient` 注入的客户端和 Transport 仍由调用方管理。

## 测试 Fake

`search/fake.Driver` 可以预设每个核心操作的 Response/Error，并记录请求：

```go
driver := &fake.Driver{
	SearchResponse: &contractsearch.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"hits":[]}`),
	},
}

_, _ = driver.Search(ctx, "users", contractsearch.SearchRequest{Body: []byte(`{"query":{"match_all":{}}}`)})
operations := driver.Operations()
```

Fake 只实现核心 Driver，不伪装不支持的 Bulk、Alias 或 Task capability。

## 生命周期

应用关闭时应关闭 Search Manager：

```go
if err := facades.Search().Close(shutdownCtx); err != nil {
	log.Printf("close search: %v", err)
}
```

`Close` 幂等，聚合多个连接的关闭错误，并保证同一驱动实例只关闭一次。

## 破坏性变更

本次升级直接替换原 Search API，Facade 和 Provider 保持单一 `Search` 入口：

| 旧 API | 新 API |
|---|---|
| `Channel` | `Connection` |
| `Dri` | `DriverName` |
| `Ping()` | `Ping(ctx)` |
| `Index` | `IndexManager.CreateIndex` |
| `Del` | `IndexManager.DeleteIndex` 或 `DocumentCleaner.DeleteAllDocuments` |
| `Save` | `UpsertDocument` |
| `Document` | `GetDocument` |
| `Delete` | `DeleteDocument` |
| `Search(query, Request)` | `Search(ctx, native SearchRequest)` |
| `Paginate` | 原始 `Response` + 驱动专属 decoder |

框架不再尝试将 Elasticsearch DSL 翻译为 Meilisearch 请求，也不再为业务层定义分页、字段权重、分词器或索引模板。
