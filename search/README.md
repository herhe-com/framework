# Search 组件

全文搜索引擎抽象层，提供统一的搜索接口，支持多种搜索引擎。

## 功能特性

- 统一的搜索接口
- 多驱动支持（Elasticsearch、Meilisearch）
- 索引管理
- 文档 CRUD 操作
- 全文搜索
- 分页支持
- 多通道支持

## 支持的驱动

- **Elasticsearch**: 强大的分布式搜索引擎
- **Meilisearch**: 快速、易用的搜索引擎

## 配置

```yaml
search:
  driver: elasticsearch
  
  elasticsearch:
    default:
      addresses:
        - http://localhost:9200
      username: elastic
      password: changeme
  
  meilisearch:
    default:
      host: http://localhost:7700
      api_key: masterKey
```

## 使用方法

### 创建索引

```go
import "github.com/herhe-com/framework/facades"

// 创建索引
err := facades.Search.Index("users")
if err != nil {
    log.Printf("Failed to create index: %v", err)
}
```

### 保存文档

```go
// 定义文档结构
type User struct {
    ID       string `json:"id"`
    Username string `json:"username"`
    Email    string `json:"email"`
    Bio      string `json:"bio"`
}

// 保存单个文档
user := User{
    ID:       "1",
    Username: "john",
    Email:    "john@example.com",
    Bio:      "Software developer",
}

err := facades.Search.Save("users", user.ID, user)
```

### 批量保存

```go
users := []User{
    {ID: "1", Username: "john", Email: "john@example.com"},
    {ID: "2", Username: "jane", Email: "jane@example.com"},
}

for _, user := range users {
    facades.Search.Save("users", user.ID, user)
}
```

### 搜索文档

```go
// 简单搜索
query := map[string]any{
    "query": map[string]any{
        "match": map[string]any{
            "username": "john",
        },
    },
}

results, err := facades.Search.Search("users", query, 1, 10)
if err != nil {
    log.Printf("Search failed: %v", err)
}

// 处理结果
for _, hit := range results.Hits {
    var user User
    json.Unmarshal(hit.Source, &user)
    fmt.Printf("Found user: %s\n", user.Username)
}
```

### 分页搜索

```go
page := 1
pageSize := 20

results, err := facades.Search.Search("users", query, page, pageSize)

fmt.Printf("Total: %d\n", results.Total)
fmt.Printf("Page: %d/%d\n", page, (results.Total+pageSize-1)/pageSize)
```

### 获取文档

```go
// 获取单个文档
doc, err := facades.Search.Document("users", "1")
if err != nil {
    log.Printf("Document not found: %v", err)
}

var user User
json.Unmarshal(doc.([]byte), &user)
```

### 删除文档

```go
// 删除单个文档
err := facades.Search.Delete("users", "1")
```

### 检查连接

```go
// 测试搜索引擎连接
err := facades.Search.Ping()
if err != nil {
    log.Printf("Search engine is not available: %v", err)
}
```

## 应用场景

### 用户搜索

```go
func SearchUsers(keyword string, page, size int) ([]User, int64, error) {
    query := map[string]any{
        "query": map[string]any{
            "multi_match": map[string]any{
                "query": keyword,
                "fields": []string{"username", "email", "bio"},
            },
        },
    }
    
    results, err := facades.Search.Search("users", query, page, size)
    if err != nil {
        return nil, 0, err
    }
    
    var users []User
    for _, hit := range results.Hits {
        var user User
        json.Unmarshal(hit.Source, &user)
        users = append(users, user)
    }
    
    return users, results.Total, nil
}
```

### 商品搜索

```go
type Product struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Category    string   `json:"category"`
    Price       float64  `json:"price"`
    Tags        []string `json:"tags"`
}

func SearchProducts(keyword, category string, minPrice, maxPrice float64, page, size int) ([]Product, int64, error) {
    must := []map[string]any{
        {
            "multi_match": map[string]any{
                "query":  keyword,
                "fields": []string{"name^2", "description", "tags"},
            },
        },
    }
    
    if category != "" {
        must = append(must, map[string]any{
            "term": map[string]any{
                "category": category,
            },
        })
    }
    
    if minPrice > 0 || maxPrice > 0 {
        rangeQuery := map[string]any{}
        if minPrice > 0 {
            rangeQuery["gte"] = minPrice
        }
        if maxPrice > 0 {
            rangeQuery["lte"] = maxPrice
        }
        must = append(must, map[string]any{
            "range": map[string]any{
                "price": rangeQuery,
            },
        })
    }
    
    query := map[string]any{
        "query": map[string]any{
            "bool": map[string]any{
                "must": must,
            },
        },
        "sort": []map[string]any{
            {"_score": map[string]string{"order": "desc"}},
            {"price": map[string]string{"order": "asc"}},
        },
    }
    
    results, err := facades.Search.Search("products", query, page, size)
    if err != nil {
        return nil, 0, err
    }
    
    var products []Product
    for _, hit := range results.Hits {
        var product Product
        json.Unmarshal(hit.Source, &product)
        products = append(products, product)
    }
    
    return products, results.Total, nil
}
```

### 文章搜索

```go
type Article struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    Content   string    `json:"content"`
    Author    string    `json:"author"`
    Tags      []string  `json:"tags"`
    CreatedAt time.Time `json:"created_at"`
}

func SearchArticles(keyword string, tags []string, page, size int) ([]Article, int64, error) {
    must := []map[string]any{
        {
            "multi_match": map[string]any{
                "query":  keyword,
                "fields": []string{"title^3", "content"},
            },
        },
    }
    
    if len(tags) > 0 {
        must = append(must, map[string]any{
            "terms": map[string]any{
                "tags": tags,
            },
        })
    }
    
    query := map[string]any{
        "query": map[string]any{
            "bool": map[string]any{
                "must": must,
            },
        },
        "sort": []map[string]any{
            {"created_at": map[string]string{"order": "desc"}},
        },
        "highlight": map[string]any{
            "fields": map[string]any{
                "title":   map[string]any{},
                "content": map[string]any{},
            },
        },
    }
    
    results, err := facades.Search.Search("articles", query, page, size)
    if err != nil {
        return nil, 0, err
    }
    
    var articles []Article
    for _, hit := range results.Hits {
        var article Article
        json.Unmarshal(hit.Source, &article)
        articles = append(articles, article)
    }
    
    return articles, results.Total, nil
}
```

### 自动完成

```go
func Autocomplete(prefix string, limit int) ([]string, error) {
    query := map[string]any{
        "query": map[string]any{
            "prefix": map[string]any{
                "username": prefix,
            },
        },
        "size": limit,
        "_source": []string{"username"},
    }
    
    results, err := facades.Search.Search("users", query, 1, limit)
    if err != nil {
        return nil, err
    }
    
    var suggestions []string
    for _, hit := range results.Hits {
        var user User
        json.Unmarshal(hit.Source, &user)
        suggestions = append(suggestions, user.Username)
    }
    
    return suggestions, nil
}
```

## 数据同步

### 从数据库同步到搜索引擎

```go
func SyncUsersToSearch() error {
    var users []User
    if err := facades.DB.Default().Find(&users).Error; err != nil {
        return err
    }
    
    // 创建索引
    facades.Search.Index("users")
    
    // 批量索引
    for _, user := range users {
        if err := facades.Search.Save("users", fmt.Sprintf("%d", user.ID), user); err != nil {
            log.Printf("Failed to index user %d: %v", user.ID, err)
        }
    }
    
    return nil
}
```

### 实时同步

```go
// 在创建用户时同步到搜索引擎
func CreateUser(user *User) error {
    // 保存到数据库
    if err := facades.DB.Default().Create(user).Error; err != nil {
        return err
    }
    
    // 同步到搜索引擎
    go func() {
        if err := facades.Search.Save("users", fmt.Sprintf("%d", user.ID), user); err != nil {
            log.Printf("Failed to sync user to search: %v", err)
        }
    }()
    
    return nil
}

// 在更新用户时同步
func UpdateUser(user *User) error {
    if err := facades.DB.Default().Save(user).Error; err != nil {
        return err
    }
    
    go facades.Search.Save("users", fmt.Sprintf("%d", user.ID), user)
    
    return nil
}

// 在删除用户时同步
func DeleteUser(id string) error {
    if err := facades.DB.Default().Delete(&User{}, id).Error; err != nil {
        return err
    }
    
    go facades.Search.Delete("users", id)
    
    return nil
}
```

## Elasticsearch 特定功能

### 复杂查询

```go
// 布尔查询
query := map[string]any{
    "query": map[string]any{
        "bool": map[string]any{
            "must": []map[string]any{
                {"match": map[string]any{"title": "golang"}},
            },
            "should": []map[string]any{
                {"match": map[string]any{"tags": "tutorial"}},
            },
            "must_not": []map[string]any{
                {"term": map[string]any{"status": "draft"}},
            },
            "filter": []map[string]any{
                {"range": map[string]any{
                    "created_at": map[string]any{
                        "gte": "2024-01-01",
                    },
                }},
            },
        },
    },
}
```

### 聚合查询

```go
// 统计每个分类的商品数量
query := map[string]any{
    "size": 0,
    "aggs": map[string]any{
        "categories": map[string]any{
            "terms": map[string]any{
                "field": "category.keyword",
                "size":  10,
            },
        },
    },
}
```

### 高亮显示

```go
query := map[string]any{
    "query": map[string]any{
        "match": map[string]any{
            "content": keyword,
        },
    },
    "highlight": map[string]any{
        "fields": map[string]any{
            "content": map[string]any{
                "pre_tags":  []string{"<em>"},
                "post_tags": []string{"</em>"},
            },
        },
    },
}
```

## Meilisearch 特定功能

### 配置可搜索属性

```go
// Meilisearch 自动配置可搜索属性
// 但也可以手动配置
```

### 过滤

```go
// Meilisearch 使用简单的过滤语法
query := map[string]any{
    "q":      "golang",
    "filter": "category = 'programming' AND price < 100",
}
```

## 接口定义

```go
type Driver interface {
    // Index 创建索引
    Index(index string) error
    
    // Save 保存文档
    Save(index, id string, document any) error
    
    // Delete 删除文档
    Delete(index, id string) error
    
    // Search 搜索文档
    Search(index string, query any, page, size int) (*SearchResult, error)
    
    // Document 获取文档
    Document(index, id string) (any, error)
    
    // Ping 检查连接
    Ping() error
}

type SearchResult struct {
    Total int64
    Hits  []Hit
}

type Hit struct {
    ID     string
    Score  float64
    Source []byte
}
```

## 最佳实践

1. **索引设计**：合理设计索引结构，避免过度嵌套
2. **字段映射**：为不同类型的字段设置合适的映射
3. **分词器**：根据语言选择合适的分词器
4. **查询优化**：使用过滤器而不是查询来提高性能
5. **批量操作**：使用批量 API 提高索引效率
6. **监控**：监控搜索性能和索引大小

## 依赖项

- Elasticsearch Go 客户端
- Meilisearch Go SDK
- Config facade

## 文件结构

```
search/
├── application.go      # 搜索应用实现
├── provider.go         # 服务提供者
├── elasticsearch/      # Elasticsearch 驱动
└── meilisearch/       # Meilisearch 驱动
```

## 访问方式

```go
import "github.com/herhe-com/framework/facades"

// 使用默认驱动
facades.Search.Save("index", "id", doc)

// 使用指定驱动
es := facades.Search.Driver("elasticsearch")
meilisearch := facades.Search.Driver("meilisearch")
```
