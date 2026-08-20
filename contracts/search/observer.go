package search

import "context"

// Operation describes a search operation without exposing sensitive bodies.
type Operation struct {
	Driver     string
	Connection string
	Name       string
	Index      string
}

// Observer receives search operation lifecycle callbacks.
type Observer interface {
	Before(ctx context.Context, operation Operation) context.Context
	After(ctx context.Context, operation Operation, response *Response, err error)
}
