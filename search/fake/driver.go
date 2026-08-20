package fake

import (
	"context"
	"sync"

	contractsearch "github.com/herhe-com/framework/contracts/search"
)

// Operation records one fake driver call.
type Operation struct {
	Name    string
	Index   string
	ID      string
	Request any
}

// Driver is a configurable, recording search driver for application tests.
type Driver struct {
	Name string

	PingResponse           *contractsearch.Response
	PingError              error
	SearchResponse         *contractsearch.Response
	SearchError            error
	UpsertDocumentResponse *contractsearch.Response
	UpsertDocumentError    error
	GetDocumentResponse    *contractsearch.Response
	GetDocumentError       error
	DeleteDocumentResponse *contractsearch.Response
	DeleteDocumentError    error
	CloseError             error

	mu         sync.Mutex
	operations []Operation
	closed     bool
}

func (driver *Driver) DriverName() string {
	if driver.Name == "" {
		return "fake"
	}
	return driver.Name
}

func (driver *Driver) Ping(ctx context.Context) (*contractsearch.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	driver.record(Operation{Name: "ping"})
	return cloneResponse(driver.PingResponse), driver.PingError
}

func (driver *Driver) Search(ctx context.Context, index string, request contractsearch.SearchRequest) (*contractsearch.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	driver.record(Operation{Name: "search", Index: index, Request: cloneSearchRequest(request)})
	return cloneResponse(driver.SearchResponse), driver.SearchError
}

func (driver *Driver) UpsertDocument(ctx context.Context, index, id string, request contractsearch.DocumentWriteRequest) (*contractsearch.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	driver.record(Operation{Name: "upsert_document", Index: index, ID: id, Request: cloneDocumentWriteRequest(request)})
	return cloneResponse(driver.UpsertDocumentResponse), driver.UpsertDocumentError
}

func (driver *Driver) GetDocument(ctx context.Context, index, id string, options contractsearch.RequestOptions) (*contractsearch.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	driver.record(Operation{Name: "get_document", Index: index, ID: id, Request: cloneOptions(options)})
	return cloneResponse(driver.GetDocumentResponse), driver.GetDocumentError
}

func (driver *Driver) DeleteDocument(ctx context.Context, index, id string, options contractsearch.RequestOptions) (*contractsearch.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	driver.record(Operation{Name: "delete_document", Index: index, ID: id, Request: cloneOptions(options)})
	return cloneResponse(driver.DeleteDocumentResponse), driver.DeleteDocumentError
}

func (driver *Driver) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.closed {
		return nil
	}
	driver.closed = true
	driver.operations = append(driver.operations, Operation{Name: "close"})
	return driver.CloseError
}

// Operations returns a snapshot of recorded calls.
func (driver *Driver) Operations() []Operation {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	return append([]Operation(nil), driver.operations...)
}

func (driver *Driver) record(operation Operation) {
	driver.mu.Lock()
	driver.operations = append(driver.operations, operation)
	driver.mu.Unlock()
}

func cloneResponse(response *contractsearch.Response) *contractsearch.Response {
	if response == nil {
		return nil
	}
	return &contractsearch.Response{
		StatusCode: response.StatusCode,
		Header:     response.Header.Clone(),
		Body:       append([]byte(nil), response.Body...),
	}
}

func cloneOptions(options contractsearch.RequestOptions) contractsearch.RequestOptions {
	params := make(map[string][]string, len(options.Params))
	for key, values := range options.Params {
		params[key] = append([]string(nil), values...)
	}
	return contractsearch.RequestOptions{Params: params, Header: options.Header.Clone()}
}

func cloneSearchRequest(request contractsearch.SearchRequest) contractsearch.SearchRequest {
	return contractsearch.SearchRequest{Body: append([]byte(nil), request.Body...), RequestOptions: cloneOptions(request.RequestOptions)}
}

func cloneDocumentWriteRequest(request contractsearch.DocumentWriteRequest) contractsearch.DocumentWriteRequest {
	return contractsearch.DocumentWriteRequest{
		Body:           append([]byte(nil), request.Body...),
		PrimaryKey:     request.PrimaryKey,
		RequestOptions: cloneOptions(request.RequestOptions),
	}
}
