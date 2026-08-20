package search

import "context"

// IndexManager manages complete search indexes.
type IndexManager interface {
	IndexExists(ctx context.Context, index string, options RequestOptions) (bool, *Response, error)
	CreateIndex(ctx context.Context, index string, request IndexRequest) (*Response, error)
	DeleteIndex(ctx context.Context, index string, options RequestOptions) (*Response, error)
}

// BulkWriter submits an engine-native bulk request.
type BulkWriter interface {
	Bulk(ctx context.Context, index string, request BulkRequest) (*BulkResponse, error)
}

// AliasManager updates index aliases atomically.
type AliasManager interface {
	UpdateAliases(ctx context.Context, request AliasRequest) (*Response, error)
}

// RawExecutor executes endpoints not yet covered by a typed capability.
type RawExecutor interface {
	Execute(ctx context.Context, request RawRequest) (*Response, error)
}

// IndexResolver resolves a logical index to its physical engine name.
type IndexResolver interface {
	ResolveIndex(index string) string
}

// DocumentCleaner removes every document while preserving the index.
type DocumentCleaner interface {
	DeleteAllDocuments(ctx context.Context, index string, options RequestOptions) (*Response, error)
}

// TaskManager exposes asynchronous engine task state.
type TaskManager interface {
	GetTask(ctx context.Context, uid int64, options RequestOptions) (*Response, error)
	WaitTask(ctx context.Context, uid int64, options RequestOptions) (*Response, error)
}

// AliasActionType identifies an alias update action.
type AliasActionType string

const (
	AliasActionAdd         AliasActionType = "add"
	AliasActionRemove      AliasActionType = "remove"
	AliasActionRemoveIndex AliasActionType = "remove_index"
)

// AliasAction describes one atomic alias update action.
type AliasAction struct {
	Action       AliasActionType
	Index        string
	Alias        string
	IsWriteIndex *bool
}

// AliasRequest contains alias actions and request options.
type AliasRequest struct {
	Actions []AliasAction
	RequestOptions
}

// BulkResponse preserves a bulk response and its parsed item results.
type BulkResponse struct {
	Response
	Took   int64
	Errors bool
	Items  []BulkItemResult
}

// BulkItemResult describes one item returned by a bulk operation.
type BulkItemResult struct {
	Operation  string
	Index      string
	ID         string
	StatusCode int
	Error      *ErrorCause
}
