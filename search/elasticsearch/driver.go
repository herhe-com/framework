package elasticsearch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	contractsearch "github.com/herhe-com/framework/contracts/search"
)

// Option customizes an Elasticsearch client.
type Option func(*clientOptions)

type clientOptions struct {
	httpClient *http.Client
	transport  http.RoundTripper
	observer   contractsearch.Observer
}

// WithHTTPClient uses the provided HTTP client for Elasticsearch requests.
func WithHTTPClient(client *http.Client) Option {
	return func(options *clientOptions) {
		options.httpClient = client
	}
}

// WithRoundTripper uses the provided transport for Elasticsearch requests.
func WithRoundTripper(transport http.RoundTripper) Option {
	return func(options *clientOptions) {
		options.transport = transport
	}
}

// WithObserver registers an operation observer.
func WithObserver(observer contractsearch.Observer) Option {
	return func(options *clientOptions) {
		options.observer = observer
	}
}

// Driver implements the context-aware Elasticsearch search contracts.
type Driver struct {
	config    Config
	client    transport
	observer  contractsearch.Observer
	closeFn   func(context.Context) error
	closeOnce sync.Once
	closeErr  error
}

type httpRequest struct {
	method      string
	path        string
	body        []byte
	options     contractsearch.RequestOptions
	contentType string
	retryable   bool
}

var (
	errNoPeerCertificate              = errors.New("Elasticsearch TLS peer returned no certificate")
	errCertificateFingerprintMismatch = errors.New("Elasticsearch TLS certificate fingerprint mismatch")
)

// New creates an Elasticsearch client from explicit configuration.
func New(config Config, options ...Option) (*Driver, error) {
	if config.Version == 0 {
		config.Version = defaultMajorVersion
	}
	if len(config.Retry.StatusCodes) == 0 {
		config.Retry.StatusCodes = []int{408, 429, 502, 503, 504}
	}
	if config.Retry.MinBackoff == 0 {
		config.Retry.MinBackoff = 100 * time.Millisecond
	}
	if config.Retry.MaxBackoff == 0 {
		config.Retry.MaxBackoff = 2 * time.Second
	}
	if err := config.validate(); err != nil {
		return nil, configError(config.Connection, "", err)
	}
	if len(config.Hosts) == 0 {
		return nil, configError(config.Connection, "hosts", errors.New("host or hosts is required"))
	}
	if _, err := majorVersion(fmt.Sprint(config.Version)); err != nil {
		return nil, configError(config.Connection, "version", err)
	}

	for _, address := range config.Hosts {
		host, err := url.Parse(address)
		if err != nil || (host.Scheme != "http" && host.Scheme != "https") || host.Host == "" {
			return nil, configError(config.Connection, "hosts", fmt.Errorf("invalid host %q", address))
		}
	}

	clientOptions := clientOptions{}
	for _, option := range options {
		if option != nil {
			option(&clientOptions)
		}
	}

	var (
		client  transport
		closeFn func(context.Context) error
	)
	if clientOptions.httpClient != nil {
		httpClient := *clientOptions.httpClient
		if clientOptions.transport != nil {
			httpClient.Transport = clientOptions.transport
		}
		direct, err := newDirectTransport(&httpClient, config.Hosts)
		if err != nil {
			return nil, err
		}
		client = direct
	} else {
		roundTripper := clientOptions.transport
		ownsRoundTripper := roundTripper == nil
		if roundTripper == nil {
			created, err := newHTTPTransport(config)
			if err != nil {
				return nil, err
			}
			roundTripper = created
		}

		created, err := newTransport(config.Version, config.Hosts, roundTripper)
		if err != nil {
			return nil, err
		}
		client = created
		if ownsRoundTripper {
			if closer, ok := created.(transportCloser); ok {
				closeFn = closer.Close
			} else if closer, ok := roundTripper.(interface{ CloseIdleConnections() }); ok {
				closeFn = func(context.Context) error {
					closer.CloseIdleConnections()
					return nil
				}
			}
		}
	}

	return &Driver{
		config:   config,
		client:   client,
		observer: clientOptions.observer,
		closeFn:  closeFn,
	}, nil
}

// DriverName returns the registered driver name.
func (client *Driver) DriverName() string {
	return "elasticsearch"
}

// ResolveIndex applies the configured prefix to a logical index name.
func (client *Driver) ResolveIndex(index string) string {
	return client.config.Prefix + index
}

// Ping checks the configured Elasticsearch endpoint.
func (client *Driver) Ping(ctx context.Context) (*contractsearch.Response, error) {
	return client.perform(ctx, contractsearch.Operation{
		Driver:     client.DriverName(),
		Connection: client.config.Connection,
		Name:       "ping",
	}, httpRequest{method: http.MethodHead, path: "/", retryable: true})
}

// Search sends an engine-native Elasticsearch query body unchanged.
func (client *Driver) Search(ctx context.Context, index string, request contractsearch.SearchRequest) (*contractsearch.Response, error) {
	if err := requireName("index", index); err != nil {
		return nil, err
	}

	return client.perform(ctx, client.operation("search", index), httpRequest{
		method:      http.MethodPost,
		path:        client.path(client.ResolveIndex(index), "_search"),
		body:        request.Body,
		options:     request.RequestOptions,
		contentType: "application/json",
		retryable:   true,
	})
}

// UpsertDocument writes a document using the caller-provided stable ID.
func (client *Driver) UpsertDocument(ctx context.Context, index, id string, request contractsearch.DocumentWriteRequest) (*contractsearch.Response, error) {
	if err := requireName("index", index); err != nil {
		return nil, err
	}
	if err := requireName("document id", id); err != nil {
		return nil, err
	}

	return client.perform(ctx, client.operation("upsert_document", index), httpRequest{
		method:      http.MethodPut,
		path:        client.path(client.ResolveIndex(index), "_doc", id),
		body:        request.Body,
		options:     request.RequestOptions,
		contentType: "application/json",
		retryable:   true,
	})
}

// GetDocument fetches a document and preserves its complete response.
func (client *Driver) GetDocument(ctx context.Context, index, id string, options contractsearch.RequestOptions) (*contractsearch.Response, error) {
	if err := requireName("index", index); err != nil {
		return nil, err
	}
	if err := requireName("document id", id); err != nil {
		return nil, err
	}

	return client.perform(ctx, client.operation("get_document", index), httpRequest{
		method:    http.MethodGet,
		path:      client.path(client.ResolveIndex(index), "_doc", id),
		options:   options,
		retryable: true,
	})
}

// DeleteDocument deletes a document without hiding a not-found response.
func (client *Driver) DeleteDocument(ctx context.Context, index, id string, options contractsearch.RequestOptions) (*contractsearch.Response, error) {
	if err := requireName("index", index); err != nil {
		return nil, err
	}
	if err := requireName("document id", id); err != nil {
		return nil, err
	}

	return client.perform(ctx, client.operation("delete_document", index), httpRequest{
		method:    http.MethodDelete,
		path:      client.path(client.ResolveIndex(index), "_doc", id),
		options:   options,
		retryable: true,
	})
}

// IndexExists checks whether an index exists.
func (client *Driver) IndexExists(ctx context.Context, index string, options contractsearch.RequestOptions) (bool, *contractsearch.Response, error) {
	if err := requireName("index", index); err != nil {
		return false, nil, err
	}

	response, err := client.perform(ctx, client.operation("index_exists", index), httpRequest{
		method:    http.MethodHead,
		path:      client.path(client.ResolveIndex(index)),
		options:   options,
		retryable: true,
	})
	if contractsearch.IsNotFound(err) {
		return false, response, nil
	}
	if err != nil {
		return false, response, err
	}

	return true, response, nil
}

// CreateIndex creates an index using the complete caller-provided body.
func (client *Driver) CreateIndex(ctx context.Context, index string, request contractsearch.IndexRequest) (*contractsearch.Response, error) {
	if err := requireName("index", index); err != nil {
		return nil, err
	}

	return client.perform(ctx, client.operation("create_index", index), httpRequest{
		method:      http.MethodPut,
		path:        client.path(client.ResolveIndex(index)),
		body:        request.Body,
		options:     request.RequestOptions,
		contentType: "application/json",
	})
}

// DeleteIndex deletes a complete index.
func (client *Driver) DeleteIndex(ctx context.Context, index string, options contractsearch.RequestOptions) (*contractsearch.Response, error) {
	if err := requireName("index", index); err != nil {
		return nil, err
	}

	return client.perform(ctx, client.operation("delete_index", index), httpRequest{
		method:  http.MethodDelete,
		path:    client.path(client.ResolveIndex(index)),
		options: options,
	})
}

// Bulk submits single-index NDJSON and reports every failed item.
func (client *Driver) Bulk(ctx context.Context, index string, request contractsearch.BulkRequest) (*contractsearch.BulkResponse, error) {
	if err := requireName("index", index); err != nil {
		return nil, err
	}

	body := append([]byte(nil), request.Body...)
	if len(body) > 0 && body[len(body)-1] != '\n' {
		body = append(body, '\n')
	}

	response, err := client.perform(ctx, client.operation("bulk", index), httpRequest{
		method:      http.MethodPost,
		path:        client.path(client.ResolveIndex(index), "_bulk"),
		body:        body,
		options:     request.RequestOptions,
		contentType: "application/x-ndjson",
	})
	bulkResponse := &contractsearch.BulkResponse{}
	if response != nil {
		bulkResponse.Response = *response
	}
	if err != nil {
		return bulkResponse, err
	}

	var payload struct {
		Took   int64 `json:"took"`
		Errors bool  `json:"errors"`
		Items  []map[string]struct {
			Index  string          `json:"_index"`
			ID     string          `json:"_id"`
			Status int             `json:"status"`
			Error  json.RawMessage `json:"error"`
		} `json:"items"`
	}
	if decodeErr := json.Unmarshal(response.Body, &payload); decodeErr != nil {
		return bulkResponse, client.localError("bulk", decodeErr, response)
	}

	bulkResponse.Took = payload.Took
	bulkResponse.Errors = payload.Errors
	failed := make([]contractsearch.BulkItemResult, 0)
	for _, item := range payload.Items {
		for operation, result := range item {
			parsed := contractsearch.BulkItemResult{
				Operation:  operation,
				Index:      result.Index,
				ID:         result.ID,
				StatusCode: result.Status,
				Error:      decodeErrorCause(result.Error),
			}
			bulkResponse.Items = append(bulkResponse.Items, parsed)
			if result.Status >= http.StatusBadRequest || parsed.Error != nil {
				failed = append(failed, parsed)
			}
		}
	}
	if len(failed) > 0 {
		return bulkResponse, &contractsearch.BulkError{Failed: failed}
	}

	return bulkResponse, nil
}

// UpdateAliases applies every alias action in one atomic Elasticsearch request.
func (client *Driver) UpdateAliases(ctx context.Context, request contractsearch.AliasRequest) (*contractsearch.Response, error) {
	actions := make([]map[string]map[string]any, 0, len(request.Actions))
	for _, action := range request.Actions {
		if err := requireName("alias action index", action.Index); err != nil {
			return nil, err
		}

		data := map[string]any{"index": client.ResolveIndex(action.Index)}
		switch action.Action {
		case contractsearch.AliasActionAdd, contractsearch.AliasActionRemove:
			if err := requireName("alias", action.Alias); err != nil {
				return nil, err
			}
			data["alias"] = client.ResolveIndex(action.Alias)
			if action.IsWriteIndex != nil {
				data["is_write_index"] = *action.IsWriteIndex
			}
		case contractsearch.AliasActionRemoveIndex:
			if action.Alias != "" || action.IsWriteIndex != nil {
				return nil, errors.New("search: remove_index does not accept alias or is_write_index")
			}
		default:
			return nil, fmt.Errorf("search: unsupported alias action %q", action.Action)
		}
		actions = append(actions, map[string]map[string]any{string(action.Action): data})
	}
	if len(actions) == 0 {
		return nil, errors.New("search: at least one alias action is required")
	}

	body, err := json.Marshal(map[string]any{"actions": actions})
	if err != nil {
		return nil, err
	}

	return client.perform(ctx, client.operation("update_aliases", ""), httpRequest{
		method:      http.MethodPost,
		path:        "/_aliases",
		body:        body,
		options:     request.RequestOptions,
		contentType: "application/json",
	})
}

// Execute sends a validated raw Elasticsearch endpoint request without prefix rewriting.
func (client *Driver) Execute(ctx context.Context, request contractsearch.RawRequest) (*contractsearch.Response, error) {
	if err := validateRawPath(request.Path); err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.Method) == "" {
		return nil, errors.New("search: raw method is required")
	}

	return client.perform(ctx, client.operation("raw", ""), httpRequest{
		method:  strings.ToUpper(request.Method),
		path:    request.Path,
		body:    request.Body,
		options: request.RequestOptions,
	})
}

// Close releases idle HTTP connections. It is safe to call more than once.
func (client *Driver) Close(ctx context.Context) error {
	if client == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("search: context is nil")
	}
	client.closeOnce.Do(func() {
		if client.closeFn != nil {
			client.closeErr = client.closeFn(ctx)
		}
	})
	return client.closeErr
}

func (client *Driver) operation(name, index string) contractsearch.Operation {
	return contractsearch.Operation{
		Driver:     client.DriverName(),
		Connection: client.config.Connection,
		Name:       name,
		Index:      index,
	}
}

func (client *Driver) perform(ctx context.Context, operation contractsearch.Operation, spec httpRequest) (response *contractsearch.Response, err error) {
	if ctx == nil {
		return nil, errors.New("search: context is nil")
	}
	if err = validateHeaders(spec.options.Header); err != nil {
		return nil, err
	}

	if client.observer != nil {
		if observed := client.observer.Before(ctx, operation); observed != nil {
			ctx = observed
		}
		defer func() {
			client.observer.After(ctx, operation, response, err)
		}()
	}

	maxAttempts := 1
	if spec.retryable {
		maxAttempts += client.config.Retry.MaxRetries
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, client.transportError(operation.Name, contextErr, false)
		}

		request, requestErr := client.newRequest(ctx, spec, attempt)
		if requestErr != nil {
			return nil, requestErr
		}
		httpResponse, requestErr := client.client.Perform(request)
		if requestErr != nil {
			retryable := spec.retryable && client.retryableTransportError(requestErr)
			if retryable && attempt+1 < maxAttempts {
				if waitErr := client.waitRetry(ctx, attempt); waitErr != nil {
					return nil, client.transportError(operation.Name, waitErr, false)
				}
				continue
			}
			return nil, client.transportError(operation.Name, requestErr, retryable)
		}

		body, readErr := io.ReadAll(httpResponse.Body)
		closeErr := httpResponse.Body.Close()
		response = &contractsearch.Response{
			StatusCode: httpResponse.StatusCode,
			Header:     httpResponse.Header.Clone(),
			Body:       body,
		}
		if readErr != nil {
			return response, client.localError(operation.Name, readErr, response)
		}
		if closeErr != nil {
			return response, client.localError(operation.Name, closeErr, response)
		}

		if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
			requestErr = client.responseError(operation.Name, response)
			if spec.retryable && client.retryableStatus(httpResponse.StatusCode) && attempt+1 < maxAttempts {
				if waitErr := client.waitRetry(ctx, attempt); waitErr != nil {
					return response, client.transportError(operation.Name, waitErr, false)
				}
				continue
			}
			return response, requestErr
		}

		return response, nil
	}

	return response, err
}

func (client *Driver) newRequest(ctx context.Context, spec httpRequest, attempt int) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, spec.method, spec.path, bytes.NewReader(spec.body))
	if err != nil {
		return nil, err
	}
	request.URL.RawPath = spec.path
	request.URL.RawQuery = cloneValues(spec.options.Params).Encode()
	request.Header = cloneHeader(spec.options.Header)
	if spec.contentType != "" && len(spec.body) > 0 {
		request.Header.Set("Content-Type", spec.contentType)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "herhe-framework-search")
	switch {
	case client.config.Auth.Username != "":
		request.SetBasicAuth(client.config.Auth.Username, client.config.Auth.Password)
	case client.config.Auth.APIKey != "":
		request.Header.Set("Authorization", "ApiKey "+client.config.Auth.APIKey)
	case client.config.Auth.BearerToken != "":
		request.Header.Set("Authorization", "Bearer "+client.config.Auth.BearerToken)
	}

	return request, nil
}

func newHTTPTransport(config Config) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if config.Transport.ProxyURL != "" {
		proxy, err := url.Parse(config.Transport.ProxyURL)
		if err != nil {
			return nil, configError(config.Connection, "transport.proxy_url", err)
		}
		transport.Proxy = http.ProxyURL(proxy)
	}
	if config.Transport.DialTimeout > 0 {
		dialer := &net.Dialer{Timeout: config.Transport.DialTimeout, KeepAlive: 30 * time.Second}
		transport.DialContext = dialer.DialContext
	}
	if config.Transport.ResponseHeaderTimeout > 0 {
		transport.ResponseHeaderTimeout = config.Transport.ResponseHeaderTimeout
	}
	if config.Transport.IdleConnectionTimeout > 0 {
		transport.IdleConnTimeout = config.Transport.IdleConnectionTimeout
	}
	if config.Transport.MaxIdleConnections > 0 {
		transport.MaxIdleConns = config.Transport.MaxIdleConnections
	}
	if config.Transport.MaxIdleConnectionsPerHost > 0 {
		transport.MaxIdleConnsPerHost = config.Transport.MaxIdleConnectionsPerHost
	}

	tlsConfig := transport.TLSClientConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		tlsConfig = tlsConfig.Clone()
		if tlsConfig.MinVersion == 0 {
			tlsConfig.MinVersion = tls.VersionTLS12
		}
	}
	if config.TLS.CAFile != "" {
		certificate, err := os.ReadFile(config.TLS.CAFile)
		if err != nil {
			return nil, configError(config.Connection, "tls.ca_file", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(certificate) {
			return nil, configError(config.Connection, "tls.ca_file", errors.New("file contains no valid certificates"))
		}
		tlsConfig.RootCAs = pool
	}
	if config.TLS.CertificateFile != "" {
		certificate, err := tls.LoadX509KeyPair(config.TLS.CertificateFile, config.TLS.KeyFile)
		if err != nil {
			return nil, configError(config.Connection, "tls", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	//nolint:gosec // Explicit opt-in for development clusters with self-signed certificates.
	tlsConfig.InsecureSkipVerify = config.TLS.InsecureSkipVerify
	if config.TLS.CertificateFingerprint != "" {
		fingerprint, err := parseCertificateFingerprint(config.TLS.CertificateFingerprint)
		if err != nil {
			return nil, configError(config.Connection, "tls.certificate_fingerprint", err)
		}
		previousVerifyConnection := tlsConfig.VerifyConnection
		if tlsConfig.RootCAs == nil && !tlsConfig.InsecureSkipVerify {
			// A fingerprint is a complete certificate trust decision for self-signed clusters.
			tlsConfig.InsecureSkipVerify = true
		}
		tlsConfig.VerifyConnection = func(state tls.ConnectionState) error {
			if previousVerifyConnection != nil {
				if err := previousVerifyConnection(state); err != nil {
					return err
				}
			}
			if len(state.PeerCertificates) == 0 {
				return errNoPeerCertificate
			}
			for _, certificate := range state.PeerCertificates {
				actual := sha256.Sum256(certificate.Raw)
				if bytes.Equal(actual[:], fingerprint) {
					return nil
				}
			}
			return errCertificateFingerprintMismatch
		}
	}
	transport.TLSClientConfig = tlsConfig

	return transport, nil
}

func (client *Driver) path(parts ...string) string {
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, url.PathEscape(part))
	}
	return "/" + strings.Join(escaped, "/")
}

func parseCertificateFingerprint(value string) ([]byte, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), ":", "")
	if len(normalized) != sha256.Size*2 {
		return nil, errors.New("certificate fingerprint must be a SHA-256 hex value")
	}
	fingerprint, err := hex.DecodeString(normalized)
	if err != nil {
		return nil, errors.New("certificate fingerprint must be a SHA-256 hex value")
	}
	return fingerprint, nil
}

func (client *Driver) responseError(operation string, response *contractsearch.Response) error {
	errorResult := &contractsearch.Error{
		Driver:     client.DriverName(),
		Connection: client.config.Connection,
		Operation:  operation,
		StatusCode: response.StatusCode,
		Retryable:  client.retryableStatus(response.StatusCode),
		Header:     response.Header.Clone(),
		Body:       append([]byte(nil), response.Body...),
	}

	var payload map[string]any
	if json.Unmarshal(response.Body, &payload) == nil {
		switch detail := payload["error"].(type) {
		case map[string]any:
			errorResult.Type = stringFromAny(detail["type"])
			errorResult.Code = errorResult.Type
			errorResult.Reason = stringFromAny(detail["reason"])
			if causes, ok := detail["root_cause"].([]any); ok {
				for _, cause := range causes {
					if causeMap, ok := cause.(map[string]any); ok {
						errorResult.RootCause = append(errorResult.RootCause, errorCauseFromMap(causeMap))
					}
				}
			}
		case string:
			errorResult.Reason = detail
		}
	}
	if errorResult.Reason == "" {
		if len(errorResult.RootCause) > 0 {
			errorResult.Reason = errorResult.RootCause[0].Reason
		}
	}
	if errorResult.Reason == "" {
		errorResult.Reason = strings.TrimSpace(string(response.Body))
	}
	if errorResult.Reason == "" {
		errorResult.Reason = http.StatusText(response.StatusCode)
	}

	return errorResult
}

func (client *Driver) transportError(operation string, cause error, retryable bool) error {
	return &contractsearch.Error{
		Driver:     client.DriverName(),
		Connection: client.config.Connection,
		Operation:  operation,
		Retryable:  retryable,
		Reason:     cause.Error(),
		Cause:      cause,
	}
}

func (client *Driver) localError(operation string, cause error, response *contractsearch.Response) error {
	errorResult := &contractsearch.Error{
		Driver:     client.DriverName(),
		Connection: client.config.Connection,
		Operation:  operation,
		Reason:     cause.Error(),
		Cause:      cause,
	}
	if response != nil {
		errorResult.StatusCode = response.StatusCode
		errorResult.Header = response.Header.Clone()
		errorResult.Body = append([]byte(nil), response.Body...)
	}
	return errorResult
}

func (client *Driver) retryableStatus(status int) bool {
	for _, retryStatus := range client.config.Retry.StatusCodes {
		if status == retryStatus {
			return true
		}
	}
	return false
}

func (client *Driver) retryableTransportError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, errNoPeerCertificate) || errors.Is(err, errCertificateFingerprintMismatch) {
		return false
	}
	var unknownAuthority x509.UnknownAuthorityError
	var certificateInvalid x509.CertificateInvalidError
	var hostnameError x509.HostnameError
	var recordHeaderError tls.RecordHeaderError
	if errors.As(err, &unknownAuthority) || errors.As(err, &certificateInvalid) || errors.As(err, &hostnameError) || errors.As(err, &recordHeaderError) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return client.config.Retry.RetryOnTimeout
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.IsTemporary || dnsErr.IsTimeout
	}
	var operationErr *net.OpError
	if errors.As(err, &operationErr) {
		return true
	}
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

func (client *Driver) waitRetry(ctx context.Context, attempt int) error {
	delay := client.config.Retry.MinBackoff
	for range attempt {
		if client.config.Retry.MaxBackoff > 0 && delay >= client.config.Retry.MaxBackoff/2 {
			delay = client.config.Retry.MaxBackoff
			break
		}
		delay *= 2
	}
	if client.config.Retry.MaxBackoff > 0 && delay > client.config.Retry.MaxBackoff {
		delay = client.config.Retry.MaxBackoff
	}
	if delay <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func validateHeaders(header http.Header) error {
	for name := range header {
		if strings.EqualFold(name, "Authorization") || strings.EqualFold(name, "Host") {
			return fmt.Errorf("search: request header %s is managed by the driver", name)
		}
	}
	return nil
}

func cloneHeader(header http.Header) http.Header {
	if header == nil {
		return make(http.Header)
	}
	return header.Clone()
}

func cloneValues(values url.Values) url.Values {
	if values == nil {
		return make(url.Values)
	}
	cloned := make(url.Values, len(values))
	for key, value := range values {
		cloned[key] = append([]string(nil), value...)
	}
	return cloned
}

func validateRawPath(path string) error {
	if !strings.HasPrefix(path, "/") {
		return errors.New("search: raw path must start with /")
	}
	parsed, err := url.Parse(path)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("search: raw path must be a relative endpoint without query or fragment")
	}
	for _, segment := range strings.Split(parsed.EscapedPath(), "/") {
		decoded, decodeErr := url.PathUnescape(segment)
		if decodeErr != nil {
			return errors.New("search: raw path contains invalid escaping")
		}
		if decoded == ".." {
			return errors.New("search: raw path cannot contain .. segments")
		}
	}
	return nil
}

func requireName(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("search: %s is required", name)
	}
	return nil
}

func decodeErrorCause(raw json.RawMessage) *contractsearch.ErrorCause {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	var causeMap map[string]any
	if json.Unmarshal(raw, &causeMap) == nil {
		cause := errorCauseFromMap(causeMap)
		return &cause
	}
	var reason string
	if json.Unmarshal(raw, &reason) == nil {
		return &contractsearch.ErrorCause{Reason: reason}
	}
	return &contractsearch.ErrorCause{Reason: strings.TrimSpace(string(raw))}
}

func errorCauseFromMap(values map[string]any) contractsearch.ErrorCause {
	cause := contractsearch.ErrorCause{
		Type:      stringFromAny(values["type"]),
		Reason:    stringFromAny(values["reason"]),
		Index:     stringFromAny(values["index"]),
		IndexUUID: stringFromAny(values["index_uuid"]),
		Shard:     stringFromAny(values["shard"]),
	}
	if causedBy, ok := values["caused_by"].(map[string]any); ok {
		nested := errorCauseFromMap(causedBy)
		cause.CausedBy = &nested
	}
	return cause
}

func stringFromAny(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
