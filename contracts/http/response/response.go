package response

type Response[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type Paginate[T any] struct {
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
	Data  []T   `json:"data"`
}
