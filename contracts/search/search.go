package search

import "context"

// Search manages configured search connections and their lifecycle.
type Search interface {
	Driver
	Default() Driver
	Connection(name string) (Driver, error)
}

// Manager is an alias for Search for code that prefers the architectural name.
type Manager = Search

// Driver defines the operations whose semantics are shared by search engines.
type Driver interface {
	DriverName() string
	Ping(ctx context.Context) (*Response, error)
	Search(ctx context.Context, index string, request SearchRequest) (*Response, error)
	UpsertDocument(ctx context.Context, index, id string, request DocumentWriteRequest) (*Response, error)
	GetDocument(ctx context.Context, index, id string, options RequestOptions) (*Response, error)
	DeleteDocument(ctx context.Context, index, id string, options RequestOptions) (*Response, error)
	Close(ctx context.Context) error
}

// ConnectionConfig is the engine-neutral configuration passed to a driver factory.
type ConnectionConfig struct {
	Name   string
	Driver string
	Prefix string
	Values map[string]any
}

// Factory constructs a search driver from explicit connection configuration.
type Factory func(config ConnectionConfig) (Driver, error)

// Registry creates drivers registered under a driver name.
type Registry interface {
	Register(name string, factory Factory) error
	Create(config ConnectionConfig) (Driver, error)
}
