package response

type Opening[T uint | string] struct {
	ID   T      `json:"id"`
	Name string `json:"name"`
}
