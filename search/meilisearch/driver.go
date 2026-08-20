package meilisearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	contractsearch "github.com/herhe-com/framework/contracts/search"
)

// Config configures a Meilisearch client without global container access.
type Config struct {
	Connection       string
	Prefix           string
	Host             string
	APIKey           string
	HTTPClient       *http.Client
	Observer         contractsearch.Observer
	TaskPollInterval time.Duration
}

// ParseConfig parses Meilisearch-specific connection values.
func ParseConfig(connection contractsearch.ConnectionConfig) (Config, error) {
	values := normalizeValues(connection.Values)
	host := strings.TrimSpace(fmt.Sprint(values["host"]))
	if host == "" || host == "<nil>" {
		return Config{}, fmt.Errorf("meilisearch connection %q: host is required", connection.Name)
	}
	parsed, err := url.Parse(host)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return Config{}, fmt.Errorf("meilisearch connection %q: invalid host %q", connection.Name, host)
	}

	apiKey := stringValue(values, "api_key")
	if apiKey == "" {
		apiKey = stringValue(values, "secret")
	}
	prefix := connection.Prefix
	if prefix == "" {
		prefix = stringValue(values, "prefix")
	}
	pollInterval := 100 * time.Millisecond
	if task := mapValue(values, "task"); task != nil {
		if raw := stringValue(task, "poll_interval"); raw != "" {
			pollInterval, err = time.ParseDuration(raw)
			if err != nil || pollInterval <= 0 {
				return Config{}, fmt.Errorf("meilisearch connection %q: invalid task.poll_interval", connection.Name)
			}
		}
	}

	return Config{
		Connection:       connection.Name,
		Prefix:           prefix,
		Host:             strings.TrimRight(host, "/"),
		APIKey:           apiKey,
		TaskPollInterval: pollInterval,
	}, nil
}

// Driver implements the context-aware search contracts for Meilisearch.
type Driver struct {
	config     Config
	httpClient *http.Client
	closeIdle  func()
	closeOnce  sync.Once
}

// New creates a Meilisearch client from explicit configuration.
func New(config Config) (*Driver, error) {
	parsed, err := url.Parse(config.Host)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("meilisearch connection %q: invalid host %q", config.Connection, config.Host)
	}
	httpClient := config.HTTPClient
	var closeIdle func()
	if httpClient == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		httpClient = &http.Client{Transport: transport}
		closeIdle = transport.CloseIdleConnections
	} else {
		clone := *httpClient
		httpClient = &clone
	}
	if config.TaskPollInterval <= 0 {
		config.TaskPollInterval = 100 * time.Millisecond
	}
	config.Host = strings.TrimRight(config.Host, "/")

	return &Driver{config: config, httpClient: httpClient, closeIdle: closeIdle}, nil
}

func (client *Driver) DriverName() string {
	return "meilisearch"
}

func (client *Driver) ResolveIndex(index string) string {
	return client.config.Prefix + index
}

func (client *Driver) Ping(ctx context.Context) (*contractsearch.Response, error) {
	return client.perform(ctx, client.operation("ping", ""), http.MethodGet, "/health", nil, contractsearch.RequestOptions{})
}

func (client *Driver) Search(ctx context.Context, index string, request contractsearch.SearchRequest) (*contractsearch.Response, error) {
	if err := requireName("index", index); err != nil {
		return nil, err
	}
	return client.perform(ctx, client.operation("search", index), http.MethodPost, client.path("indexes", client.ResolveIndex(index), "search"), request.Body, request.RequestOptions)
}

func (client *Driver) UpsertDocument(ctx context.Context, index, id string, request contractsearch.DocumentWriteRequest) (*contractsearch.Response, error) {
	if err := requireName("index", index); err != nil {
		return nil, err
	}
	if err := requireName("document id", id); err != nil {
		return nil, err
	}
	if err := requireName("primary key", request.PrimaryKey); err != nil {
		return nil, errors.New("search: Meilisearch document write requires PrimaryKey")
	}

	var document map[string]json.RawMessage
	if err := json.Unmarshal(request.Body, &document); err != nil {
		return nil, fmt.Errorf("search: invalid Meilisearch document: %w", err)
	}
	if document == nil {
		return nil, errors.New("search: Meilisearch document must be a JSON object")
	}
	idJSON, err := json.Marshal(id)
	if err != nil {
		return nil, err
	}
	document[request.PrimaryKey] = idJSON
	body, err := json.Marshal([]map[string]json.RawMessage{document})
	if err != nil {
		return nil, err
	}
	options := cloneOptions(request.RequestOptions)
	options.Params.Set("primaryKey", request.PrimaryKey)

	return client.perform(ctx, client.operation("upsert_document", index), http.MethodPost, client.path("indexes", client.ResolveIndex(index), "documents"), body, options)
}

func (client *Driver) GetDocument(ctx context.Context, index, id string, options contractsearch.RequestOptions) (*contractsearch.Response, error) {
	if err := requireName("index", index); err != nil {
		return nil, err
	}
	if err := requireName("document id", id); err != nil {
		return nil, err
	}
	return client.perform(ctx, client.operation("get_document", index), http.MethodGet, client.path("indexes", client.ResolveIndex(index), "documents", id), nil, options)
}

func (client *Driver) DeleteDocument(ctx context.Context, index, id string, options contractsearch.RequestOptions) (*contractsearch.Response, error) {
	if err := requireName("index", index); err != nil {
		return nil, err
	}
	if err := requireName("document id", id); err != nil {
		return nil, err
	}
	return client.perform(ctx, client.operation("delete_document", index), http.MethodDelete, client.path("indexes", client.ResolveIndex(index), "documents", id), nil, options)
}

func (client *Driver) IndexExists(ctx context.Context, index string, options contractsearch.RequestOptions) (bool, *contractsearch.Response, error) {
	if err := requireName("index", index); err != nil {
		return false, nil, err
	}
	response, err := client.perform(ctx, client.operation("index_exists", index), http.MethodGet, client.path("indexes", client.ResolveIndex(index)), nil, options)
	if contractsearch.IsNotFound(err) {
		return false, response, nil
	}
	return err == nil, response, err
}

func (client *Driver) CreateIndex(ctx context.Context, index string, request contractsearch.IndexRequest) (*contractsearch.Response, error) {
	if err := requireName("index", index); err != nil {
		return nil, err
	}
	var body map[string]json.RawMessage
	if len(request.Body) > 0 {
		if err := json.Unmarshal(request.Body, &body); err != nil {
			return nil, fmt.Errorf("search: invalid Meilisearch index request: %w", err)
		}
	}
	if body == nil {
		body = make(map[string]json.RawMessage)
	}
	uid, _ := json.Marshal(client.ResolveIndex(index))
	body["uid"] = uid
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return client.perform(ctx, client.operation("create_index", index), http.MethodPost, "/indexes", encoded, request.RequestOptions)
}

func (client *Driver) DeleteIndex(ctx context.Context, index string, options contractsearch.RequestOptions) (*contractsearch.Response, error) {
	if err := requireName("index", index); err != nil {
		return nil, err
	}
	return client.perform(ctx, client.operation("delete_index", index), http.MethodDelete, client.path("indexes", client.ResolveIndex(index)), nil, options)
}

// DeleteAllDocuments clears documents while preserving the index.
func (client *Driver) DeleteAllDocuments(ctx context.Context, index string, options contractsearch.RequestOptions) (*contractsearch.Response, error) {
	if err := requireName("index", index); err != nil {
		return nil, err
	}
	return client.perform(ctx, client.operation("delete_all_documents", index), http.MethodDelete, client.path("indexes", client.ResolveIndex(index), "documents"), nil, options)
}

func (client *Driver) GetTask(ctx context.Context, uid int64, options contractsearch.RequestOptions) (*contractsearch.Response, error) {
	if uid < 0 {
		return nil, errors.New("search: task UID cannot be negative")
	}
	return client.perform(ctx, client.operation("get_task", ""), http.MethodGet, client.path("tasks", strconv.FormatInt(uid, 10)), nil, options)
}

func (client *Driver) WaitTask(ctx context.Context, uid int64, options contractsearch.RequestOptions) (*contractsearch.Response, error) {
	for {
		response, err := client.GetTask(ctx, uid, options)
		if err != nil {
			return response, err
		}
		var task struct {
			Status string `json:"status"`
			Error  struct {
				Message string `json:"message"`
				Code    string `json:"code"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if err := response.DecodeJSON(&task); err != nil {
			return response, client.localError("wait_task", err, response)
		}
		switch task.Status {
		case "succeeded":
			return response, nil
		case "failed":
			reason := task.Error.Message
			if reason == "" {
				reason = "Meilisearch task failed"
			}
			return response, &contractsearch.Error{
				Driver: client.DriverName(), Connection: client.config.Connection, Operation: "wait_task",
				StatusCode: response.StatusCode, Code: task.Error.Code, Type: task.Error.Type, Reason: reason,
				Header: response.Header.Clone(), Body: append([]byte(nil), response.Body...),
			}
		case "canceled":
			return response, &contractsearch.Error{
				Driver: client.DriverName(), Connection: client.config.Connection, Operation: "wait_task",
				StatusCode: response.StatusCode, Code: "task_canceled", Type: "task_error", Reason: "Meilisearch task was canceled",
				Header: response.Header.Clone(), Body: append([]byte(nil), response.Body...),
			}
		}

		timer := time.NewTimer(client.config.TaskPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return response, client.transportError("wait_task", ctx.Err())
		case <-timer.C:
		}
	}
}

// Execute sends a validated raw Meilisearch endpoint request without prefix rewriting.
func (client *Driver) Execute(ctx context.Context, request contractsearch.RawRequest) (*contractsearch.Response, error) {
	if err := validateRawPath(request.Path); err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.Method) == "" {
		return nil, errors.New("search: raw method is required")
	}
	return client.perform(ctx, client.operation("raw", ""), strings.ToUpper(request.Method), request.Path, request.Body, request.RequestOptions)
}

func (client *Driver) Close(ctx context.Context) error {
	if client == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("search: context is nil")
	}
	client.closeOnce.Do(func() {
		if client.closeIdle != nil {
			client.closeIdle()
		}
	})
	return nil
}

func (client *Driver) perform(ctx context.Context, operation contractsearch.Operation, method, path string, body []byte, options contractsearch.RequestOptions) (response *contractsearch.Response, err error) {
	if ctx == nil {
		return nil, errors.New("search: context is nil")
	}
	if err = validateHeaders(options.Header); err != nil {
		return nil, err
	}
	if client.config.Observer != nil {
		if observed := client.config.Observer.Before(ctx, operation); observed != nil {
			ctx = observed
		}
		defer func() {
			client.config.Observer.After(ctx, operation, response, err)
		}()
	}

	target := client.config.Host + path
	params := cloneValues(options.Params)
	if encoded := params.Encode(); encoded != "" {
		target += "?" + encoded
	}
	request, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header = cloneHeader(options.Header)
	request.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	if client.config.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+client.config.APIKey)
	}

	httpResponse, err := client.httpClient.Do(request)
	if err != nil {
		return nil, client.transportError(operation.Name, err)
	}
	body, readErr := io.ReadAll(httpResponse.Body)
	closeErr := httpResponse.Body.Close()
	response = &contractsearch.Response{StatusCode: httpResponse.StatusCode, Header: httpResponse.Header.Clone(), Body: body}
	if readErr != nil {
		return response, client.localError(operation.Name, readErr, response)
	}
	if closeErr != nil {
		return response, client.localError(operation.Name, closeErr, response)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return response, client.responseError(operation.Name, response)
	}
	return response, nil
}

func (client *Driver) operation(name, index string) contractsearch.Operation {
	return contractsearch.Operation{Driver: client.DriverName(), Connection: client.config.Connection, Name: name, Index: index}
}

func (client *Driver) path(parts ...string) string {
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, url.PathEscape(part))
	}
	return "/" + strings.Join(escaped, "/")
}

func (client *Driver) responseError(operation string, response *contractsearch.Response) error {
	result := &contractsearch.Error{
		Driver: client.DriverName(), Connection: client.config.Connection, Operation: operation,
		StatusCode: response.StatusCode, Header: response.Header.Clone(), Body: append([]byte(nil), response.Body...),
		Retryable: response.StatusCode == http.StatusRequestTimeout || response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500,
	}
	var payload struct {
		Message string `json:"message"`
		Code    string `json:"code"`
		Type    string `json:"type"`
	}
	if json.Unmarshal(response.Body, &payload) == nil {
		result.Reason = payload.Message
		result.Code = payload.Code
		result.Type = payload.Type
	}
	if result.Reason == "" {
		result.Reason = strings.TrimSpace(string(response.Body))
	}
	if result.Reason == "" {
		result.Reason = http.StatusText(response.StatusCode)
	}
	return result
}

func (client *Driver) transportError(operation string, cause error) error {
	return &contractsearch.Error{Driver: client.DriverName(), Connection: client.config.Connection, Operation: operation, Reason: cause.Error(), Cause: cause}
}

func (client *Driver) localError(operation string, cause error, response *contractsearch.Response) error {
	result := &contractsearch.Error{Driver: client.DriverName(), Connection: client.config.Connection, Operation: operation, Reason: cause.Error(), Cause: cause}
	if response != nil {
		result.StatusCode = response.StatusCode
		result.Header = response.Header.Clone()
		result.Body = append([]byte(nil), response.Body...)
	}
	return result
}

func cloneOptions(options contractsearch.RequestOptions) contractsearch.RequestOptions {
	return contractsearch.RequestOptions{Params: cloneValues(options.Params), Header: cloneHeader(options.Header)}
}

func cloneValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, value := range values {
		cloned[key] = append([]string(nil), value...)
	}
	return cloned
}

func cloneHeader(header http.Header) http.Header {
	if header == nil {
		return make(http.Header)
	}
	return header.Clone()
}

func validateHeaders(header http.Header) error {
	for name := range header {
		if strings.EqualFold(name, "Authorization") || strings.EqualFold(name, "Host") {
			return fmt.Errorf("search: request header %s is managed by the driver", name)
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

func stringValue(values map[string]any, key string) string {
	if values == nil || values[key] == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(values[key]))
}

func mapValue(values map[string]any, key string) map[string]any {
	value, ok := values[key]
	if !ok || value == nil {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return normalizeValues(typed)
	}
	if typed, ok := value.(map[any]any); ok {
		result := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			result[strings.ToLower(fmt.Sprint(childKey))] = childValue
		}
		return result
	}
	return nil
}

func normalizeValues(values map[string]any) map[string]any {
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[strings.ToLower(key)] = value
	}
	return result
}
