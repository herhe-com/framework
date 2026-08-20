package meilisearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	contractsearch "github.com/herhe-com/framework/contracts/search"
)

type closeTrackingTransport struct {
	closes atomic.Int32
}

func (*closeTrackingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("unexpected request")
}

func (transport *closeTrackingTransport) CloseIdleConnections() {
	transport.closes.Add(1)
}

func TestSearchPreservesNativeBodyAndResponse(t *testing.T) {
	requestBody := []byte(`{"q":"golang","filter":"category = software","sort":["created_at:desc"],"facets":["category"]}`)
	responseBody := []byte(`{"hits":[{"id":"1","_formatted":{"title":"<em>Go</em>"}}],"facetDistribution":{"category":{"software":1}},"processingTimeMs":1}`)
	params := url.Values{"federation": {"true"}}
	headers := http.Header{"X-Request-Id": {"request-1"}}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.EscapedPath() != "/indexes/app_users%2Farchive/search" {
			t.Errorf("escaped path = %q", request.URL.EscapedPath())
		}
		if request.URL.Query().Get("federation") != "true" {
			t.Errorf("query = %q", request.URL.RawQuery)
		}
		if request.Header.Get("X-Request-Id") != "request-1" {
			t.Errorf("request id = %q", request.Header.Get("X-Request-Id"))
		}
		body, _ := io.ReadAll(request.Body)
		if !bytes.Equal(body, requestBody) {
			t.Errorf("body changed: %s", body)
		}
		writer.Header().Set("X-Meili-Request-Id", "response-1")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(responseBody)
	}))
	defer server.Close()

	driver, err := New(Config{Connection: "default", Prefix: "app_", Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	result, err := driver.Search(t.Context(), "users/archive", contractsearch.SearchRequest{
		Body: requestBody,
		RequestOptions: contractsearch.RequestOptions{
			Params: params,
			Header: headers,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result.Body, responseBody) || result.Header.Get("X-Meili-Request-Id") != "response-1" {
		t.Fatal("raw response was not preserved")
	}
	if len(params) != 1 || len(headers) != 1 {
		t.Fatalf("caller options were modified: params=%#v headers=%#v", params, headers)
	}
}

func TestUpsertInjectsStableIDAndReturnsTask(t *testing.T) {
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/indexes/users/documents" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if request.URL.Query().Get("primaryKey") != "id" {
			t.Errorf("primaryKey = %q", request.URL.Query().Get("primaryKey"))
		}
		body, _ = io.ReadAll(request.Body)
		writer.WriteHeader(http.StatusAccepted)
		_, _ = writer.Write([]byte(`{"taskUid":42,"indexUid":"users","status":"enqueued","type":"documentAdditionOrUpdate"}`))
	}))
	defer server.Close()

	driver, err := New(Config{Connection: "default", Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	result, err := driver.UpsertDocument(t.Context(), "users", "fixed-id", contractsearch.DocumentWriteRequest{
		Body:       []byte(`{"id":"caller-value","name":"Alice"}`),
		PrimaryKey: "id",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != http.StatusAccepted || !bytes.Contains(result.Body, []byte(`"taskUid":42`)) {
		t.Fatalf("task response = %#v", result)
	}
	var documents []map[string]any
	if err := json.Unmarshal(body, &documents); err != nil {
		t.Fatal(err)
	}
	if got := documents[0]["id"]; got != "fixed-id" {
		t.Fatalf("document id = %#v", got)
	}
	if got := documents[0]["name"]; got != "Alice" {
		t.Fatalf("document name = %#v", got)
	}
}

func TestDeleteIndexAndDeleteAllDocumentsUseDifferentEndpoints(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.URL.Path)
		writer.WriteHeader(http.StatusAccepted)
		_, _ = writer.Write([]byte(`{"taskUid":1,"status":"enqueued"}`))
	}))
	defer server.Close()

	driver, err := New(Config{Connection: "default", Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driver.DeleteIndex(t.Context(), "users", contractsearch.RequestOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.DeleteAllDocuments(t.Context(), "users", contractsearch.RequestOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "/indexes/users" || paths[1] != "/indexes/users/documents" {
		t.Fatalf("paths = %#v", paths)
	}
}

func TestWaitTaskPollsUntilTerminalStatus(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/tasks/9" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if calls.Add(1) == 1 {
			_, _ = writer.Write([]byte(`{"uid":9,"status":"processing"}`))
			return
		}
		_, _ = writer.Write([]byte(`{"uid":9,"status":"succeeded"}`))
	}))
	defer server.Close()

	driver, err := New(Config{Connection: "default", Host: server.URL, TaskPollInterval: time.Nanosecond})
	if err != nil {
		t.Fatal(err)
	}
	result, err := driver.WaitTask(t.Context(), 9, contractsearch.RequestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || !bytes.Contains(result.Body, []byte(`"succeeded"`)) {
		t.Fatalf("calls=%d response=%s", calls.Load(), result.Body)
	}
}

func TestWaitTaskReturnsStructuredFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"uid":9,"status":"failed","error":{"message":"invalid document","code":"invalid_document","type":"invalid_request"}}`))
	}))
	defer server.Close()

	driver, err := New(Config{Connection: "default", Host: server.URL, TaskPollInterval: time.Nanosecond})
	if err != nil {
		t.Fatal(err)
	}
	result, err := driver.WaitTask(t.Context(), 9, contractsearch.RequestOptions{})
	var searchErr *contractsearch.Error
	if !errors.As(err, &searchErr) || searchErr.Code != "invalid_document" || searchErr.Reason != "invalid document" {
		t.Fatalf("error = %#v", err)
	}
	if result == nil || !bytes.Contains(result.Body, []byte(`"failed"`)) {
		t.Fatalf("response = %#v", result)
	}
}

func TestWaitTaskHonorsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"uid":9,"status":"processing"}`))
	}))
	defer server.Close()

	driver, err := New(Config{Connection: "default", Host: server.URL, TaskPollInterval: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = driver.WaitTask(ctx, 9, contractsearch.RequestOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}

func TestNoAPIKeyIsAllowedAndConfiguredKeyIsProtected(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authorization = request.Header.Get("Authorization")
		_, _ = writer.Write([]byte(`{"status":"available"}`))
	}))
	defer server.Close()

	driver, err := New(Config{Connection: "default", Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driver.Ping(t.Context()); err != nil {
		t.Fatal(err)
	}
	if authorization != "" {
		t.Fatalf("unexpected auth = %q", authorization)
	}

	driver, err = New(Config{Connection: "default", Host: server.URL, APIKey: "master-key"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driver.Ping(t.Context()); err != nil {
		t.Fatal(err)
	}
	if authorization != "Bearer master-key" {
		t.Fatalf("auth = %q", authorization)
	}
	if _, err := driver.Ping(t.Context()); err != nil {
		t.Fatal(err)
	}
	_, err = driver.Search(t.Context(), "users", contractsearch.SearchRequest{
		RequestOptions: contractsearch.RequestOptions{Header: http.Header{"Authorization": {"Bearer attacker"}}},
	})
	if err == nil {
		t.Fatal("expected managed authentication header error")
	}
}

func TestClientLifecycleDoesNotOwnSharedTransport(t *testing.T) {
	defaultDriver, err := New(Config{Connection: "default", Host: "http://127.0.0.1:7700"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = defaultDriver.Close(context.Background()) })
	if defaultDriver.httpClient.Transport == nil || defaultDriver.httpClient.Transport == http.DefaultTransport {
		t.Fatal("default Meilisearch client must use an isolated transport")
	}

	transport := &closeTrackingTransport{}
	customDriver, err := New(Config{
		Connection: "custom",
		Host:       "http://127.0.0.1:7700",
		HTTPClient: &http.Client{Transport: transport},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := customDriver.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if transport.closes.Load() != 0 {
		t.Fatalf("driver closed caller-owned transport %d time(s)", transport.closes.Load())
	}
}

func TestStructuredMeilisearchErrorAndRawSafety(t *testing.T) {
	body := `{"message":"Index users not found","code":"index_not_found","type":"invalid_request","link":"https://docs.meilisearch.com/errors#index_not_found"}`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte(body))
	}))
	defer server.Close()

	driver, err := New(Config{Connection: "default", Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	result, err := driver.GetDocument(t.Context(), "users", "1", contractsearch.RequestOptions{})
	if !contractsearch.IsNotFound(err) {
		t.Fatalf("error = %v", err)
	}
	var searchErr *contractsearch.Error
	if !errors.As(err, &searchErr) || searchErr.Code != "index_not_found" || searchErr.Type != "invalid_request" || string(searchErr.Body) != body {
		t.Fatalf("structured error = %#v", searchErr)
	}
	if result == nil || string(result.Body) != body {
		t.Fatal("response was not preserved")
	}
	for _, path := range []string{"http://evil.test/tasks", "/../tasks", "/tasks?limit=1"} {
		if _, err := driver.Execute(t.Context(), contractsearch.RawRequest{Method: http.MethodGet, Path: path}); err == nil {
			t.Fatalf("raw path %q should fail", path)
		}
	}
}

func TestParseConfigSupportsSecretCompatibilityAndEmptyAuth(t *testing.T) {
	config, err := ParseConfig(contractsearch.ConnectionConfig{
		Name: "default",
		Values: map[string]any{
			"host":   "http://127.0.0.1:7700",
			"prefix": "app_",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.APIKey != "" || config.Prefix != "app_" {
		t.Fatalf("config = %#v", config)
	}

	config, err = ParseConfig(contractsearch.ConnectionConfig{
		Name: "default",
		Values: map[string]any{
			"host":   "http://127.0.0.1:7700",
			"secret": "legacy-key",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.APIKey != "legacy-key" {
		t.Fatalf("API key = %q", config.APIKey)
	}
}

func TestUpsertRejectsMissingPrimaryKeyAndInvalidDocument(t *testing.T) {
	driver, err := New(Config{Connection: "default", Host: "http://127.0.0.1:7700"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driver.UpsertDocument(t.Context(), "users", "1", contractsearch.DocumentWriteRequest{Body: []byte(`{}`)}); err == nil {
		t.Fatal("expected missing primary key error")
	}
	if _, err := driver.UpsertDocument(t.Context(), "users", "1", contractsearch.DocumentWriteRequest{Body: []byte(`[]`), PrimaryKey: "id"}); err == nil {
		t.Fatal("expected invalid object error")
	}
	if _, err := driver.UpsertDocument(t.Context(), "users", "1", contractsearch.DocumentWriteRequest{Body: []byte(`null`), PrimaryKey: "id"}); err == nil {
		t.Fatal("expected null object error")
	}
}

func TestNon2xxResponseIsStructuredError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusPermanentRedirect)
		_, _ = writer.Write([]byte("moved"))
	}))
	defer server.Close()

	driver, err := New(Config{Connection: "default", Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	result, err := driver.Ping(t.Context())
	var searchErr *contractsearch.Error
	if !errors.As(err, &searchErr) || searchErr.StatusCode != http.StatusPermanentRedirect {
		t.Fatalf("error = %#v", err)
	}
	if result == nil || result.StatusCode != http.StatusPermanentRedirect {
		t.Fatalf("response = %#v", result)
	}
}

func TestRawExecutorForwardsBodyAndOptions(t *testing.T) {
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ = io.ReadAll(request.Body)
		if request.URL.Path != "/experimental-features" || request.URL.Query().Get("verbose") != "true" {
			t.Errorf("request = %s?%s", request.URL.Path, request.URL.RawQuery)
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	driver, err := New(Config{Connection: "default", Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	result, err := driver.Execute(t.Context(), contractsearch.RawRequest{
		Method: http.MethodPatch,
		Path:   "/experimental-features",
		Body:   []byte(`{"metrics":true}`),
		RequestOptions: contractsearch.RequestOptions{
			Params: url.Values{"verbose": {"true"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, []byte(`{"metrics":true}`)) || !strings.Contains(string(result.Body), `"ok":true`) {
		t.Fatalf("body=%s response=%s", body, result.Body)
	}
}
