package elasticsearch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	contractsearch "github.com/herhe-com/framework/contracts/search"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type closeTrackingTransport struct {
	calls  atomic.Int32
	closes atomic.Int32
}

func (transport *closeTrackingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.calls.Add(1)
	return response(http.StatusOK, `{}`), nil
}

func (transport *closeTrackingTransport) CloseIdleConnections() {
	transport.closes.Add(1)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func testConfig() Config {
	return Config{
		Connection: "default",
		Version:    8,
		Hosts:      []string{"http://search.test"},
		Retry: RetryConfig{
			StatusCodes: []int{429, 503},
			MinBackoff:  time.Nanosecond,
			MaxBackoff:  time.Nanosecond,
		},
	}
}

func TestMajorVersion(t *testing.T) {
	tests := []struct {
		version string
		want    int
		wantErr bool
	}{
		{want: 7},
		{version: "6", want: 6},
		{version: "v7", want: 7},
		{version: "8.19.7", want: 8},
		{version: "V9.5.0", want: 9},
		{version: "5", wantErr: true},
		{version: "latest", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			got, err := majorVersion(test.version)
			if (err != nil) != test.wantErr {
				t.Fatalf("majorVersion(%q) error = %v, wantErr %t", test.version, err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("majorVersion(%q) = %d, want %d", test.version, got, test.want)
			}
		})
	}
}

func TestConfiguredMajorTransportsPerformRequests(t *testing.T) {
	for _, version := range []int{6, 7, 8, 9} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			var calls atomic.Int32
			driver, err := New(Config{
				Connection: "default",
				Version:    version,
				Hosts:      []string{"http://search.test"},
			}, WithRoundTripper(roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls.Add(1)
				if request.URL.Path != "/" {
					t.Fatalf("request path = %q", request.URL.Path)
				}
				return response(http.StatusOK, `{}`), nil
			})))
			if err != nil {
				t.Fatalf("create version %d driver: %v", version, err)
			}
			if _, err := driver.Ping(t.Context()); err != nil {
				t.Fatalf("ping version %d: %v", version, err)
			}
			if got := calls.Load(); got != 1 {
				t.Fatalf("request count = %d, want 1", got)
			}
		})
	}
}

func TestSearchPreservesNativeRequestAndResponse(t *testing.T) {
	requestBody := []byte(`{"query":{"bool":{"filter":[{"term":{"active":true}}]}},"sort":["_score"],"aggs":{"roles":{"terms":{"field":"role"}}}}`)
	responseBody := []byte(`{"took":3,"hits":{"hits":[{"_id":"1","_score":1.2,"sort":[1],"highlight":{"name":["<em>A</em>"]}}]},"aggregations":{"roles":{"buckets":[]}}}`)
	params := url.Values{"routing": {"tenant-1"}}
	headers := http.Header{"X-Opaque-Id": {"trace-1"}}

	var captured *http.Request
	var capturedBody []byte
	driver, err := New(Config{
		Connection: "default",
		Prefix:     "tenant_",
		Version:    8,
		Hosts:      []string{"http://search.test/base"},
		Auth:       AuthConfig{APIKey: "encoded-key"},
	}, WithRoundTripper(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		captured = request.Clone(request.Context())
		capturedBody, _ = io.ReadAll(request.Body)
		result := response(http.StatusOK, string(responseBody))
		result.Header.Set("X-Elastic-Product", "Elasticsearch")
		return result, nil
	})))
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
		t.Fatalf("search: %v", err)
	}
	if captured.Method != http.MethodPost {
		t.Fatalf("method = %s", captured.Method)
	}
	if got, want := captured.URL.EscapedPath(), "/base/tenant_users%2Farchive/_search"; got != want {
		t.Fatalf("escaped path = %q, want %q", got, want)
	}
	if captured.URL.Query().Get("routing") != "tenant-1" {
		t.Fatalf("routing = %q", captured.URL.Query().Get("routing"))
	}
	if !bytes.Equal(capturedBody, requestBody) {
		t.Fatalf("request body changed:\n%s", capturedBody)
	}
	if captured.Header.Get("X-Opaque-Id") != "trace-1" {
		t.Fatalf("opaque id = %q", captured.Header.Get("X-Opaque-Id"))
	}
	if captured.Header.Get("Authorization") != "ApiKey encoded-key" {
		t.Fatalf("authorization = %q", captured.Header.Get("Authorization"))
	}
	if got := captured.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q", got)
	}
	if !bytes.Equal(result.Body, responseBody) {
		t.Fatalf("response body changed:\n%s", result.Body)
	}
	if result.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", result.StatusCode)
	}
	if params.Get("routing") != "tenant-1" || len(params) != 1 {
		t.Fatalf("caller params were modified: %#v", params)
	}
	if headers.Get("Authorization") != "" || headers.Get("Content-Type") != "" || len(headers) != 1 {
		t.Fatalf("caller headers were modified: %#v", headers)
	}
}

func TestStructuredErrorPreservesResponse(t *testing.T) {
	body := `{"error":{"type":"document_missing_exception","reason":"missing","root_cause":[{"type":"document_missing_exception","reason":"missing","index":"users"}]},"status":404}`
	driver, err := New(testConfig(), WithRoundTripper(roundTripFunc(func(*http.Request) (*http.Response, error) {
		result := response(http.StatusNotFound, body)
		result.Header.Set("X-Request-Id", "request-1")
		return result, nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	result, err := driver.GetDocument(t.Context(), "users", "404", contractsearch.RequestOptions{})
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !contractsearch.IsNotFound(err) {
		t.Fatalf("IsNotFound(%v) = false", err)
	}
	var searchErr *contractsearch.Error
	if !errors.As(err, &searchErr) {
		t.Fatalf("error type = %T", err)
	}
	if searchErr.Driver != "elasticsearch" || searchErr.Connection != "default" || searchErr.Operation != "get_document" {
		t.Fatalf("error metadata = %#v", searchErr)
	}
	if searchErr.Type != "document_missing_exception" || searchErr.Reason != "missing" || len(searchErr.RootCause) != 1 {
		t.Fatalf("parsed error = %#v", searchErr)
	}
	if string(searchErr.Body) != body || searchErr.Header.Get("X-Request-Id") != "request-1" {
		t.Fatal("raw error response was not preserved")
	}
	if result == nil || string(result.Body) != body {
		t.Fatal("response was not returned alongside the error")
	}
}

func TestRetrySemanticsRespectOperationSafety(t *testing.T) {
	config := testConfig()
	config.Retry.MaxRetries = 1
	var searchCalls atomic.Int32
	driver, err := New(config, WithRoundTripper(roundTripFunc(func(*http.Request) (*http.Response, error) {
		if searchCalls.Add(1) == 1 {
			return response(http.StatusServiceUnavailable, `{"error":{"reason":"busy"}}`), nil
		}
		return response(http.StatusOK, `{"hits":{"hits":[]}}`), nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driver.Search(t.Context(), "users", contractsearch.SearchRequest{Body: []byte(`{}`)}); err != nil {
		t.Fatalf("retryable search: %v", err)
	}
	if got := searchCalls.Load(); got != 2 {
		t.Fatalf("search calls = %d, want 2", got)
	}

	var bulkCalls atomic.Int32
	driver, err = New(config, WithRoundTripper(roundTripFunc(func(*http.Request) (*http.Response, error) {
		bulkCalls.Add(1)
		return response(http.StatusServiceUnavailable, `{"error":{"reason":"busy"}}`), nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driver.Bulk(t.Context(), "users", contractsearch.BulkRequest{Body: []byte("{}\n{}\n")}); err == nil {
		t.Fatal("expected bulk error")
	}
	if got := bulkCalls.Load(); got != 1 {
		t.Fatalf("bulk calls = %d, want 1", got)
	}
}

func TestPermanentTLSErrorIsNotRetried(t *testing.T) {
	config := testConfig()
	config.Retry.MaxRetries = 3
	var calls atomic.Int32
	driver, err := New(config, WithRoundTripper(roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, x509.UnknownAuthorityError{}
	})))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driver.Search(t.Context(), "users", contractsearch.SearchRequest{Body: []byte(`{}`)}); err == nil {
		t.Fatal("expected TLS error")
	}
	if calls.Load() != 1 {
		t.Fatalf("TLS calls = %d, want 1", calls.Load())
	}
}

func TestNon2xxResponseIsStructuredError(t *testing.T) {
	driver, err := New(testConfig(), WithRoundTripper(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusPermanentRedirect, "moved"), nil
	})))
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

func TestBulkReportsEveryFailedItemAndAddsTrailingNewline(t *testing.T) {
	var requestBody []byte
	var contentType string
	driver, err := New(testConfig(), WithRoundTripper(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestBody, _ = io.ReadAll(request.Body)
		contentType = request.Header.Get("Content-Type")
		return response(http.StatusOK, `{"took":4,"errors":true,"items":[{"index":{"_index":"users","_id":"1","status":400,"error":{"type":"mapper_parsing_exception","reason":"bad one"}}},{"delete":{"_index":"users","_id":"2","status":404,"error":{"type":"document_missing_exception","reason":"bad two"}}}]}`), nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	result, err := driver.Bulk(t.Context(), "users", contractsearch.BulkRequest{Body: []byte("{}\n{}")})
	var bulkErr *contractsearch.BulkError
	if !errors.As(err, &bulkErr) {
		t.Fatalf("bulk error = %T %v", err, err)
	}
	if len(bulkErr.Failed) != 2 || len(result.Items) != 2 || result.Took != 4 || !result.Errors {
		t.Fatalf("bulk result = %#v, error = %#v", result, bulkErr)
	}
	if requestBody[len(requestBody)-1] != '\n' {
		t.Fatalf("bulk body has no trailing newline: %q", requestBody)
	}
	if contentType != "application/x-ndjson" {
		t.Fatalf("content type = %q", contentType)
	}
}

func TestAliasesApplyPrefixInSingleRequest(t *testing.T) {
	var requestBody []byte
	driver, err := New(Config{Connection: "default", Prefix: "app_", Version: 8, Hosts: []string{"http://search.test"}}, WithRoundTripper(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/_aliases" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		requestBody, _ = io.ReadAll(request.Body)
		return response(http.StatusOK, `{"acknowledged":true}`), nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	write := true
	_, err = driver.UpdateAliases(t.Context(), contractsearch.AliasRequest{Actions: []contractsearch.AliasAction{
		{Action: contractsearch.AliasActionRemove, Index: "users_v1", Alias: "users"},
		{Action: contractsearch.AliasActionAdd, Index: "users_v2", Alias: "users", IsWriteIndex: &write},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"index":"app_users_v1"`, `"index":"app_users_v2"`, `"alias":"app_users"`, `"is_write_index":true`} {
		if !bytes.Contains(requestBody, []byte(expected)) {
			t.Fatalf("alias body %s missing %s", requestBody, expected)
		}
	}
}

func TestRawSafetyAndContextCancellation(t *testing.T) {
	var calls atomic.Int32
	driver, err := New(testConfig(), WithRoundTripper(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		<-request.Context().Done()
		return nil, request.Context().Err()
	})))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"http://evil.test/_search", "/../_search", "/_search?q=x"} {
		if _, err := driver.Execute(t.Context(), contractsearch.RawRequest{Method: http.MethodGet, Path: path}); err == nil {
			t.Fatalf("raw path %q should fail", path)
		}
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = driver.Search(ctx, "users", contractsearch.SearchRequest{Body: []byte(`{}`)})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("search error = %v, want context canceled", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("transport calls = %d, want 0 for pre-canceled context", got)
	}
}

func TestRequestHeadersCannotOverrideAuthentication(t *testing.T) {
	driver, err := New(testConfig(), WithRoundTripper(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{}`), nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	_, err = driver.Search(t.Context(), "users", contractsearch.SearchRequest{
		RequestOptions: contractsearch.RequestOptions{Header: http.Header{"Authorization": {"Bearer attacker"}}},
	})
	if err == nil {
		t.Fatal("expected managed header error")
	}
}

func TestAuthenticationModes(t *testing.T) {
	tests := []struct {
		name string
		auth AuthConfig
		want string
	}{
		{name: "none"},
		{name: "basic", auth: AuthConfig{Username: "elastic", Password: "secret"}, want: "Basic ZWxhc3RpYzpzZWNyZXQ="},
		{name: "api key", auth: AuthConfig{APIKey: "encoded"}, want: "ApiKey encoded"},
		{name: "bearer", auth: AuthConfig{BearerToken: "token"}, want: "Bearer token"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testConfig()
			config.Auth = test.auth
			driver, err := New(config, WithRoundTripper(roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if got := request.Header.Get("Authorization"); got != test.want {
					t.Fatalf("authorization = %q, want %q", got, test.want)
				}
				return response(http.StatusOK, `{}`), nil
			})))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := driver.Ping(t.Context()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCreateIndexPreservesSettingsAndMappings(t *testing.T) {
	body := []byte(`{"settings":{"analysis":{"analyzer":{"custom":{"type":"custom","tokenizer":"standard"}}}},"mappings":{"properties":{"name":{"type":"text","analyzer":"custom"}}}}`)
	var captured []byte
	driver, err := New(testConfig(), WithRoundTripper(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		captured, _ = io.ReadAll(request.Body)
		return response(http.StatusOK, `{"acknowledged":true}`), nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driver.CreateIndex(t.Context(), "users", contractsearch.IndexRequest{Body: body}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(captured, body) {
		t.Fatalf("index body changed: %s", captured)
	}
}

func TestWithHTTPClientUsesCustomClient(t *testing.T) {
	transport := &closeTrackingTransport{}
	httpClient := &http.Client{Transport: transport}
	driver, err := New(testConfig(), WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driver.Ping(t.Context()); err != nil {
		t.Fatal(err)
	}
	if transport.calls.Load() != 1 {
		t.Fatalf("calls = %d", transport.calls.Load())
	}
	if err := driver.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if transport.closes.Load() != 0 {
		t.Fatalf("driver closed caller-owned transport %d time(s)", transport.closes.Load())
	}
}

func TestTLSCertificateFingerprint(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{}`))
	}))
	defer server.Close()

	fingerprint := sha256.Sum256(server.Certificate().Raw)
	config := testConfig()
	config.Hosts = []string{server.URL}
	config.TLS.CertificateFingerprint = hex.EncodeToString(fingerprint[:])
	driver, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driver.Ping(t.Context()); err != nil {
		t.Fatalf("matching fingerprint: %v", err)
	}

	config.TLS.CertificateFingerprint = strings.Repeat("0", sha256.Size*2)
	driver, err = New(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driver.Ping(t.Context()); err == nil {
		t.Fatal("expected fingerprint mismatch")
	}
}

func TestParseConfigValidation(t *testing.T) {
	base := contractsearch.ConnectionConfig{
		Name:   "default",
		Driver: "elasticsearch",
		Values: map[string]any{"hosts": []any{"http://127.0.0.1:9200"}},
	}
	config, err := ParseConfig(base)
	if err != nil {
		t.Fatalf("parse no-auth config: %v", err)
	}
	if config.Version != 7 || config.Auth != (AuthConfig{}) {
		t.Fatalf("config = %#v", config)
	}
	if len(config.Hosts) != 1 || config.Hosts[0] != "http://127.0.0.1:9200" {
		t.Fatalf("hosts-only config = %#v", config.Hosts)
	}

	// host alone is enough; hosts is not required.
	hostOnly := base
	hostOnly.Values = map[string]any{"host": "http://127.0.0.1:9201"}
	config, err = ParseConfig(hostOnly)
	if err != nil {
		t.Fatalf("parse host-only config: %v", err)
	}
	if len(config.Hosts) != 1 || config.Hosts[0] != "http://127.0.0.1:9201" {
		t.Fatalf("host-only config = %#v", config.Hosts)
	}

	// hosts may also be a single string.
	hostsString := base
	hostsString.Values = map[string]any{"hosts": "http://127.0.0.1:9202"}
	config, err = ParseConfig(hostsString)
	if err != nil {
		t.Fatalf("parse hosts string config: %v", err)
	}
	if len(config.Hosts) != 1 || config.Hosts[0] != "http://127.0.0.1:9202" {
		t.Fatalf("hosts string config = %#v", config.Hosts)
	}

	// empty hosts entries are ignored so host can stand alone.
	hostWithEmptyHosts := base
	hostWithEmptyHosts.Values = map[string]any{
		"host":  "http://127.0.0.1:9203",
		"hosts": []any{"", "  "},
	}
	config, err = ParseConfig(hostWithEmptyHosts)
	if err != nil {
		t.Fatalf("parse host with empty hosts: %v", err)
	}
	if len(config.Hosts) != 1 || config.Hosts[0] != "http://127.0.0.1:9203" {
		t.Fatalf("host with empty hosts = %#v", config.Hosts)
	}

	invalid := base
	invalid.Values = map[string]any{
		"host":  "http://127.0.0.1:9200",
		"hosts": []string{"http://127.0.0.1:9201"},
	}
	if _, err := ParseConfig(invalid); err == nil {
		t.Fatal("expected host/hosts conflict")
	}

	invalid.Values = map[string]any{}
	if _, err := ParseConfig(invalid); err == nil {
		t.Fatal("expected missing host/hosts error")
	}

	invalid.Values = map[string]any{
		"hosts": []string{"http://127.0.0.1:9200"},
		"auth":  map[string]any{"username": "elastic"},
	}
	if _, err := ParseConfig(invalid); err == nil {
		t.Fatal("expected incomplete basic auth error")
	}

	invalid.Values = map[string]any{
		"hosts": []string{"http://127.0.0.1:9200"},
		"tls":   map[string]any{"certificate_fingerprint": "invalid"},
	}
	if _, err := ParseConfig(invalid); err == nil {
		t.Fatal("expected invalid fingerprint error")
	}
}

func TestObserverReceivesOperationLifecycle(t *testing.T) {
	observer := &recordingObserver{}
	driver, err := New(testConfig(), WithObserver(observer), WithRoundTripper(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{}`), nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driver.Ping(t.Context()); err != nil {
		t.Fatal(err)
	}
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if observer.before.Name != "ping" || observer.after.Name != "ping" || observer.response == nil || observer.err != nil {
		t.Fatalf("observer = %#v", observer)
	}
}

type recordingObserver struct {
	mu       sync.Mutex
	before   contractsearch.Operation
	after    contractsearch.Operation
	response *contractsearch.Response
	err      error
}

func (observer *recordingObserver) Before(ctx context.Context, operation contractsearch.Operation) context.Context {
	observer.mu.Lock()
	observer.before = operation
	observer.mu.Unlock()
	return ctx
}

func (observer *recordingObserver) After(_ context.Context, operation contractsearch.Operation, response *contractsearch.Response, err error) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.after = operation
	observer.response = response
	observer.err = err
}
