package response

// Opening is a lightweight ID/name pair used by selector endpoints such as
// dropdown options.
type Opening[T uint | string] struct {
	// ID is the value submitted back when this option is selected: an unsigned
	// integer or a string ID depending on T.
	ID T `json:"id" example:"1"`
	// Name is the label shown for this option.
	Name string `json:"name" example:"管理员"`
}
