package elasticsearch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"

	elastic6 "github.com/elastic/go-elasticsearch/v6"
	elastic7 "github.com/elastic/go-elasticsearch/v7"
	elastic8 "github.com/elastic/go-elasticsearch/v8"
	elastic9 "github.com/elastic/go-elasticsearch/v9"
)

const defaultMajorVersion = 7

type transport interface {
	Perform(*http.Request) (*http.Response, error)
}

type transportCloser interface {
	Close(context.Context) error
}

type directTransport struct {
	client   *http.Client
	hosts    []*url.URL
	nextHost atomic.Uint64
}

type escapedPathRoundTripper struct {
	next http.RoundTripper
}

func (transport escapedPathRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.RawPath == "" {
		return transport.next.RoundTrip(request)
	}

	decoded, err := url.PathUnescape(request.URL.RawPath)
	if err != nil || !strings.HasSuffix(request.URL.Path, decoded) {
		return transport.next.RoundTrip(request)
	}
	prefix := strings.TrimSuffix(request.URL.Path, decoded)
	rawPath := (&url.URL{Path: prefix}).EscapedPath() + request.URL.RawPath
	if decodedRawPath, decodeErr := url.PathUnescape(rawPath); decodeErr != nil || decodedRawPath != request.URL.Path {
		return transport.next.RoundTrip(request)
	}

	clone := request.Clone(request.Context())
	clone.URL.RawPath = rawPath
	return transport.next.RoundTrip(clone)
}

func newDirectTransport(client *http.Client, addresses []string) (*directTransport, error) {
	if client == nil {
		return nil, errors.New("elasticsearch: HTTP client is nil")
	}
	hosts := make([]*url.URL, 0, len(addresses))
	for _, address := range addresses {
		host, err := url.Parse(address)
		if err != nil || (host.Scheme != "http" && host.Scheme != "https") || host.Host == "" {
			return nil, fmt.Errorf("elasticsearch: invalid host %q", address)
		}
		hosts = append(hosts, host)
	}
	if len(hosts) == 0 {
		return nil, errors.New("elasticsearch: at least one host is required")
	}
	return &directTransport{client: client, hosts: hosts}, nil
}

func (transport *directTransport) Perform(request *http.Request) (*http.Response, error) {
	host := transport.hosts[(transport.nextHost.Add(1)-1)%uint64(len(transport.hosts))]
	target := *host
	rawPath := strings.TrimRight(host.EscapedPath(), "/") + request.URL.EscapedPath()
	decodedPath, err := url.PathUnescape(rawPath)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: invalid request path: %w", err)
	}
	target.Path = decodedPath
	target.RawPath = rawPath
	target.RawQuery = request.URL.RawQuery
	target.Fragment = ""

	clone := request.Clone(request.Context())
	clone.URL = &target
	clone.Host = ""
	clone.RequestURI = ""
	return transport.client.Do(clone)
}

func (transport *directTransport) Close(context.Context) error {
	transport.client.CloseIdleConnections()
	return nil
}

func newTransport(version int, addresses []string, roundTripper http.RoundTripper) (transport, error) {
	if roundTripper == nil {
		roundTripper = http.DefaultTransport
	}
	roundTripper = escapedPathRoundTripper{next: roundTripper}

	switch version {
	case 6:
		client, err := elastic6.NewClient(elastic6.Config{
			Addresses:    addresses,
			Transport:    roundTripper,
			DisableRetry: true,
		})
		if err != nil {
			return nil, err
		}
		return client.Transport, nil
	case 7:
		client, err := elastic7.NewClient(elastic7.Config{
			Addresses:         addresses,
			Transport:         roundTripper,
			DisableRetry:      true,
			DisableMetaHeader: true,
		})
		if err != nil {
			return nil, err
		}
		return client.Transport, nil
	case 8:
		client, err := elastic8.NewBaseClient(elastic8.Config{
			Addresses:         addresses,
			Transport:         roundTripper,
			DisableRetry:      true,
			DisableMetaHeader: true,
		})
		if err != nil {
			return nil, err
		}
		return client.Transport, nil
	case 9:
		client, err := elastic9.NewBaseClient(elastic9.Config{
			Addresses:         addresses,
			Transport:         roundTripper,
			DisableRetry:      true,
			DisableMetaHeader: true,
		})
		if err != nil {
			return nil, err
		}
		return client.Transport, nil
	default:
		return nil, fmt.Errorf("unsupported major version %d; supported versions: 6, 7, 8, 9", version)
	}
}

func majorVersion(version string) (int, error) {
	version = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(version), "v"))
	if version == "" {
		return defaultMajorVersion, nil
	}

	major, _, _ := strings.Cut(version, ".")
	value, err := strconv.Atoi(major)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%q is not a valid version", version)
	}

	switch value {
	case 6, 7, 8, 9:
		return value, nil
	default:
		return 0, fmt.Errorf("unsupported major version %d; supported versions: 6, 7, 8, 9", value)
	}
}
