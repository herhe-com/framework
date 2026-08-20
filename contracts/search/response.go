package search

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Response preserves the complete response returned by a search engine.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// DecodeJSON decodes the response body without changing it.
func (response *Response) DecodeJSON(target any) error {
	if response == nil {
		return errors.New("search: response is nil")
	}
	return json.Unmarshal(response.Body, target)
}

// Decode decodes a response body into the requested type.
func Decode[T any](response *Response) (T, error) {
	var result T
	if err := response.DecodeJSON(&result); err != nil {
		return result, err
	}
	return result, nil
}
