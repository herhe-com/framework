package search

import (
	"net/http"
	"net/url"
)

// RequestOptions contains request query parameters and headers.
type RequestOptions struct {
	Params url.Values
	Header http.Header
}

// SearchRequest contains a complete engine-native search request body.
type SearchRequest struct {
	Body []byte
	RequestOptions
}

// DocumentWriteRequest contains an engine-native document body.
type DocumentWriteRequest struct {
	Body       []byte
	PrimaryKey string
	RequestOptions
}

// IndexRequest contains an engine-native index creation body.
type IndexRequest struct {
	Body []byte
	RequestOptions
}

// BulkRequest contains an engine-native bulk request body.
type BulkRequest struct {
	Body []byte
	RequestOptions
}

// RawRequest describes an engine-native endpoint request.
type RawRequest struct {
	Method string
	Path   string
	Body   []byte
	RequestOptions
}
