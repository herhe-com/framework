package search

type Paginate struct {
	Total int64
	Size  int
	Page  int
	Data  []map[string]any
}
