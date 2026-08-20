package elasticsearch

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	contractsearch "github.com/herhe-com/framework/contracts/search"
)

// Config configures an Elasticsearch client without global container access.
type Config struct {
	Connection string
	Prefix     string
	Version    int
	Hosts      []string
	Auth       AuthConfig
	TLS        TLSConfig
	Transport  TransportConfig
	Retry      RetryConfig
}

type AuthConfig struct {
	Username    string
	Password    string
	APIKey      string
	BearerToken string
}

type TLSConfig struct {
	CAFile                 string
	CertificateFingerprint string
	CertificateFile        string
	KeyFile                string
	InsecureSkipVerify     bool
}

type TransportConfig struct {
	ProxyURL                  string
	DialTimeout               time.Duration
	ResponseHeaderTimeout     time.Duration
	IdleConnectionTimeout     time.Duration
	MaxIdleConnections        int
	MaxIdleConnectionsPerHost int
}

type RetryConfig struct {
	MaxRetries     int
	RetryOnTimeout bool
	StatusCodes    []int
	MinBackoff     time.Duration
	MaxBackoff     time.Duration
}

// ParseConfig parses engine-specific values from a registry connection.
func ParseConfig(connection contractsearch.ConnectionConfig) (Config, error) {
	values := cloneStringMap(connection.Values)
	config := Config{
		Connection: connection.Name,
		Prefix:     connection.Prefix,
		Retry: RetryConfig{
			MaxRetries:     3,
			RetryOnTimeout: true,
			StatusCodes:    []int{408, 429, 502, 503, 504},
			MinBackoff:     100 * time.Millisecond,
			MaxBackoff:     2 * time.Second,
		},
	}

	if config.Prefix == "" {
		config.Prefix = stringValue(values, "prefix")
	}

	version, err := majorVersion(stringValue(values, "version"))
	if err != nil {
		return Config{}, fmt.Errorf("elasticsearch connection %q: %w", connection.Name, err)
	}
	config.Version = version

	// host and hosts are mutually exclusive; either form is enough.
	host := stringValue(values, "host")
	hosts, err := stringSliceValue(values, "hosts")
	if err != nil {
		return Config{}, configError(connection.Name, "hosts", err)
	}
	hosts = compactStrings(hosts)
	if host != "" && len(hosts) > 0 {
		return Config{}, configError(connection.Name, "host", errors.New("host and hosts cannot both be configured"))
	}
	if len(hosts) == 0 && host != "" {
		hosts = []string{host}
	}
	if len(hosts) == 0 {
		return Config{}, configError(connection.Name, "hosts", errors.New("host or hosts is required"))
	}
	for _, address := range hosts {
		parsed, parseErr := url.Parse(address)
		if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return Config{}, configError(connection.Name, "hosts", fmt.Errorf("invalid host %q", address))
		}
	}
	config.Hosts = append([]string(nil), hosts...)

	auth := mapValue(values, "auth")
	config.Auth = AuthConfig{
		Username:    firstString(stringValue(auth, "username"), stringValue(values, "username")),
		Password:    firstString(stringValue(auth, "password"), stringValue(values, "password")),
		APIKey:      firstString(stringValue(auth, "api_key"), stringValue(values, "api_key")),
		BearerToken: firstString(stringValue(auth, "bearer_token"), stringValue(values, "bearer_token")),
	}
	if (config.Auth.Username == "") != (config.Auth.Password == "") {
		return Config{}, configError(connection.Name, "auth", errors.New("username and password must be configured together"))
	}
	authMethods := 0
	if config.Auth.Username != "" {
		authMethods++
	}
	if config.Auth.APIKey != "" {
		authMethods++
	}
	if config.Auth.BearerToken != "" {
		authMethods++
	}
	if authMethods > 1 {
		return Config{}, configError(connection.Name, "auth", errors.New("basic auth, API key, and bearer token are mutually exclusive"))
	}

	tlsValues := mapValue(values, "tls")
	config.TLS = TLSConfig{
		CAFile:                 stringValue(tlsValues, "ca_file"),
		CertificateFingerprint: firstString(stringValue(tlsValues, "certificate_fingerprint"), stringValue(values, "certificate_fingerprint")),
		CertificateFile:        stringValue(tlsValues, "certificate_file"),
		KeyFile:                stringValue(tlsValues, "key_file"),
	}
	if value, ok, valueErr := optionalBoolValue(tlsValues, "insecure_skip_verify"); valueErr != nil {
		return Config{}, configError(connection.Name, "tls.insecure_skip_verify", valueErr)
	} else if ok {
		config.TLS.InsecureSkipVerify = value
	}
	if (config.TLS.CertificateFile == "") != (config.TLS.KeyFile == "") {
		return Config{}, configError(connection.Name, "tls", errors.New("certificate_file and key_file must be configured together"))
	}

	transport := mapValue(values, "transport")
	config.Transport.ProxyURL = stringValue(transport, "proxy_url")
	if config.Transport.DialTimeout, err = durationValue(transport, "dial_timeout"); err != nil {
		return Config{}, configError(connection.Name, "transport.dial_timeout", err)
	}
	if config.Transport.ResponseHeaderTimeout, err = durationValue(transport, "response_header_timeout"); err != nil {
		return Config{}, configError(connection.Name, "transport.response_header_timeout", err)
	}
	if config.Transport.IdleConnectionTimeout, err = durationValue(transport, "idle_connection_timeout"); err != nil {
		return Config{}, configError(connection.Name, "transport.idle_connection_timeout", err)
	}
	if config.Transport.MaxIdleConnections, err = intValue(transport, "max_idle_connections"); err != nil {
		return Config{}, configError(connection.Name, "transport.max_idle_connections", err)
	}
	if config.Transport.MaxIdleConnectionsPerHost, err = intValue(transport, "max_idle_connections_per_host"); err != nil {
		return Config{}, configError(connection.Name, "transport.max_idle_connections_per_host", err)
	}
	if config.Transport.ProxyURL != "" {
		proxy, parseErr := url.Parse(config.Transport.ProxyURL)
		if parseErr != nil || proxy.Scheme == "" || proxy.Host == "" {
			return Config{}, configError(connection.Name, "transport.proxy_url", errors.New("invalid proxy URL"))
		}
	}

	retry := mapValue(values, "retry")
	if value, ok, valueErr := optionalIntValue(retry, "max_retries"); valueErr != nil {
		return Config{}, configError(connection.Name, "retry.max_retries", valueErr)
	} else if ok {
		config.Retry.MaxRetries = value
	}
	if value, ok, valueErr := optionalBoolValue(retry, "retry_on_timeout"); valueErr != nil {
		return Config{}, configError(connection.Name, "retry.retry_on_timeout", valueErr)
	} else if ok {
		config.Retry.RetryOnTimeout = value
	}
	if statusCodes, ok, statusErr := optionalIntSliceValue(retry, "status_codes"); statusErr != nil {
		return Config{}, configError(connection.Name, "retry.status_codes", statusErr)
	} else if ok {
		config.Retry.StatusCodes = statusCodes
	}
	if value, ok, valueErr := optionalDurationValue(retry, "min_backoff"); valueErr != nil {
		return Config{}, configError(connection.Name, "retry.min_backoff", valueErr)
	} else if ok {
		config.Retry.MinBackoff = value
	}
	if value, ok, valueErr := optionalDurationValue(retry, "max_backoff"); valueErr != nil {
		return Config{}, configError(connection.Name, "retry.max_backoff", valueErr)
	} else if ok {
		config.Retry.MaxBackoff = value
	}

	if err := config.validate(); err != nil {
		return Config{}, configError(connection.Name, "", err)
	}

	return config, nil
}

func (config Config) validate() error {
	if (config.Auth.Username == "") != (config.Auth.Password == "") {
		return errors.New("username and password must be configured together")
	}
	authMethods := 0
	if config.Auth.Username != "" {
		authMethods++
	}
	if config.Auth.APIKey != "" {
		authMethods++
	}
	if config.Auth.BearerToken != "" {
		authMethods++
	}
	if authMethods > 1 {
		return errors.New("basic auth, API key, and bearer token are mutually exclusive")
	}
	if (config.TLS.CertificateFile == "") != (config.TLS.KeyFile == "") {
		return errors.New("certificate_file and key_file must be configured together")
	}
	if config.TLS.CertificateFingerprint != "" {
		if _, err := parseCertificateFingerprint(config.TLS.CertificateFingerprint); err != nil {
			return err
		}
	}
	if config.Transport.DialTimeout < 0 || config.Transport.ResponseHeaderTimeout < 0 || config.Transport.IdleConnectionTimeout < 0 {
		return errors.New("transport timeouts cannot be negative")
	}
	if config.Transport.MaxIdleConnections < 0 || config.Transport.MaxIdleConnectionsPerHost < 0 {
		return errors.New("transport connection limits cannot be negative")
	}
	if config.Retry.MaxRetries < 0 || config.Retry.MinBackoff < 0 || config.Retry.MaxBackoff < 0 {
		return errors.New("retry values cannot be negative")
	}
	if config.Retry.MaxBackoff > 0 && config.Retry.MinBackoff > config.Retry.MaxBackoff {
		return errors.New("retry min_backoff cannot exceed max_backoff")
	}
	for _, status := range config.Retry.StatusCodes {
		if status < 100 || status > 599 {
			return fmt.Errorf("invalid retry status code %d", status)
		}
	}

	return nil
}

func configError(connection, field string, err error) error {
	if field == "" {
		return fmt.Errorf("elasticsearch connection %q: %w", connection, err)
	}

	return fmt.Errorf("elasticsearch connection %q field %s: %w", connection, field, err)
}

func cloneStringMap(values map[string]any) map[string]any {
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[strings.ToLower(key)] = value
	}
	return cloned
}

func mapValue(values map[string]any, key string) map[string]any {
	value, ok := values[key]
	if !ok || value == nil {
		return nil
	}

	switch typed := value.(type) {
	case map[string]any:
		return cloneStringMap(typed)
	case map[any]any:
		result := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			result[strings.ToLower(fmt.Sprint(childKey))] = childValue
		}
		return result
	default:
		return nil
	}
}

func stringValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func firstString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func optionalBoolValue(values map[string]any, key string) (bool, bool, error) {
	if values == nil {
		return false, false, nil
	}
	value, ok := values[key]
	if !ok || value == nil {
		return false, false, nil
	}
	switch typed := value.(type) {
	case bool:
		return typed, true, nil
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err != nil {
			return false, true, err
		}
		return parsed, true, nil
	default:
		return false, true, fmt.Errorf("expected boolean, got %T", value)
	}
}

func intValue(values map[string]any, key string) (int, error) {
	value, _, err := optionalIntValue(values, key)
	return value, err
}

func optionalIntValue(values map[string]any, key string) (int, bool, error) {
	if values == nil {
		return 0, false, nil
	}
	value, ok := values[key]
	if !ok || value == nil || value == "" {
		return 0, false, nil
	}
	switch typed := value.(type) {
	case int:
		return typed, true, nil
	case int8:
		return int(typed), true, nil
	case int16:
		return int(typed), true, nil
	case int32:
		return int(typed), true, nil
	case int64:
		return int(typed), true, nil
	case uint:
		return int(typed), true, nil
	case uint8:
		return int(typed), true, nil
	case uint16:
		return int(typed), true, nil
	case uint32:
		return int(typed), true, nil
	case uint64:
		if uint64(int(typed)) != typed {
			return 0, true, errors.New("value overflows int")
		}
		return int(typed), true, nil
	case float64:
		if typed != float64(int(typed)) {
			return 0, true, errors.New("value must be an integer")
		}
		return int(typed), true, nil
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		return parsed, true, err
	default:
		return 0, true, fmt.Errorf("unsupported integer type %T", value)
	}
}

func durationValue(values map[string]any, key string) (time.Duration, error) {
	value, _, err := optionalDurationValue(values, key)
	return value, err
}

func optionalDurationValue(values map[string]any, key string) (time.Duration, bool, error) {
	if values == nil {
		return 0, false, nil
	}
	value, ok := values[key]
	if !ok || value == nil || value == "" {
		return 0, false, nil
	}
	switch typed := value.(type) {
	case time.Duration:
		return typed, true, nil
	case string:
		parsed, err := time.ParseDuration(strings.TrimSpace(typed))
		return parsed, true, err
	default:
		return 0, true, fmt.Errorf("duration must be a string, got %T", value)
	}
}

func stringSliceValue(values map[string]any, key string) ([]string, error) {
	if values == nil {
		return nil, nil
	}
	value, ok := values[key]
	if !ok || value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...), nil
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, strings.TrimSpace(fmt.Sprint(item)))
		}
		return result, nil
	case string:
		// Allow a single address string for hosts, same shape as host.
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil, nil
		}
		return []string{trimmed}, nil
	default:
		return nil, fmt.Errorf("expected string slice, got %T", value)
	}
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func optionalIntSliceValue(values map[string]any, key string) ([]int, bool, error) {
	if values == nil {
		return nil, false, nil
	}
	value, ok := values[key]
	if !ok || value == nil {
		return nil, false, nil
	}
	switch typed := value.(type) {
	case []int:
		return append([]int(nil), typed...), true, nil
	case []any:
		result := make([]int, 0, len(typed))
		for _, item := range typed {
			parsed, _, err := optionalIntValue(map[string]any{"value": item}, "value")
			if err != nil {
				return nil, true, err
			}
			result = append(result, parsed)
		}
		return result, true, nil
	default:
		return nil, true, fmt.Errorf("expected integer slice, got %T", value)
	}
}
