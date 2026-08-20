package fake

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	contractsearch "github.com/herhe-com/framework/contracts/search"
)

func TestDriverRecordsClonedOperationsAndResponses(t *testing.T) {
	driver := &Driver{
		SearchResponse: &contractsearch.Response{StatusCode: http.StatusOK, Header: http.Header{"X-Test": {"yes"}}, Body: []byte(`{"ok":true}`)},
	}
	request := contractsearch.SearchRequest{
		Body: []byte(`{"q":"go"}`),
		RequestOptions: contractsearch.RequestOptions{
			Params: url.Values{"limit": {"10"}},
			Header: http.Header{"X-Request": {"one"}},
		},
	}
	response, err := driver.Search(t.Context(), "users", request)
	if err != nil {
		t.Fatal(err)
	}
	request.Body[0] = 'x'
	request.Params.Set("limit", "20")
	response.Body[0] = 'x'

	operations := driver.Operations()
	recorded := operations[0].Request.(contractsearch.SearchRequest)
	if !bytes.Equal(recorded.Body, []byte(`{"q":"go"}`)) || recorded.Params.Get("limit") != "10" {
		t.Fatalf("recorded request = %#v", recorded)
	}
	if !bytes.Equal(driver.SearchResponse.Body, []byte(`{"ok":true}`)) {
		t.Fatal("configured response was modified")
	}
}

func TestDriverHonorsCancellationAndIdempotentClose(t *testing.T) {
	driver := &Driver{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := driver.Ping(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("ping error = %v", err)
	}
	if err := driver.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := driver.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	operations := driver.Operations()
	if len(operations) != 1 || operations[0].Name != "close" {
		t.Fatalf("operations = %#v", operations)
	}
}
