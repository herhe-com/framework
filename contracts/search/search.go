package search

type Search interface {
	Driver
	Channel(channel string) (Driver, error)
}

type Driver interface {
	Index(index, data string) error                                 // 创建索引
	Del(index string) error                                         // 删除索引
	Save(index, key string, doc map[string]any) error               // 保存文档
	Document(index, id string) (document map[string]any, err error) // 查询文档
	Delete(index, id string) error                                  // 删除文档
	Search(index, query string, request Request) (*Paginate, error) // 搜索
	Dri() string                                                    // 获取驱动
	Ping() (bool, error)                                            // 测试连接
}
