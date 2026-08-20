package search

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	contractconfig "github.com/herhe-com/framework/contracts/config"
	contractsearch "github.com/herhe-com/framework/contracts/search"
	"github.com/herhe-com/framework/facades"
)

type fakeConfig struct {
	values map[string]any
}

func (f fakeConfig) Env(key string, defaultValue ...any) any       { return f.Get(key, defaultValue...) }
func (f fakeConfig) Add(name string, configuration map[string]any) {}
func (f fakeConfig) Set(key string, configuration any)             {}

func (f fakeConfig) Get(key string, defaultValue ...any) any {
	if value, ok := f.values[key]; ok {
		return value
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return nil
}

func (f fakeConfig) GetString(key string, defaultValue ...string) string {
	if value, ok := f.values[key]; ok {
		return fmt.Sprint(value)
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return ""
}

func (f fakeConfig) GetStrings(key string, defaultValue ...[]string) []string {
	if value, ok := f.values[key].([]string); ok {
		return value
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return nil
}

func (f fakeConfig) GetMaps(key string, defaultValue ...map[string]any) map[string]any {
	if value, ok := f.values[key].(map[string]any); ok {
		return value
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return nil
}

func (f fakeConfig) GetInt(key string, defaultValue ...int) int {
	if value, ok := f.values[key].(int); ok {
		return value
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return 0
}

func (f fakeConfig) GetInt64(key string, defaultValue ...int64) int64 {
	if value, ok := f.values[key].(int64); ok {
		return value
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return 0
}

func (f fakeConfig) GetBool(key string, defaultValue ...bool) bool {
	if value, ok := f.values[key].(bool); ok {
		return value
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return false
}

func (f fakeConfig) IsSet(key string) bool {
	_, ok := f.values[key]
	return ok
}

type fakeSearchDriver struct {
	name        string
	closeCount  atomic.Int32
	searchCount atomic.Int32
}

func (driver *fakeSearchDriver) DriverName() string { return driver.name }
func (driver *fakeSearchDriver) Ping(context.Context) (*contractsearch.Response, error) {
	return &contractsearch.Response{}, nil
}
func (driver *fakeSearchDriver) Search(context.Context, string, contractsearch.SearchRequest) (*contractsearch.Response, error) {
	driver.searchCount.Add(1)
	return &contractsearch.Response{}, nil
}
func (driver *fakeSearchDriver) UpsertDocument(context.Context, string, string, contractsearch.DocumentWriteRequest) (*contractsearch.Response, error) {
	return &contractsearch.Response{}, nil
}
func (driver *fakeSearchDriver) GetDocument(context.Context, string, string, contractsearch.RequestOptions) (*contractsearch.Response, error) {
	return &contractsearch.Response{}, nil
}
func (driver *fakeSearchDriver) DeleteDocument(context.Context, string, string, contractsearch.RequestOptions) (*contractsearch.Response, error) {
	return &contractsearch.Response{}, nil
}
func (driver *fakeSearchDriver) Close(context.Context) error {
	driver.closeCount.Add(1)
	return nil
}

func withSearchConfig(t *testing.T, values map[string]any) {
	t.Helper()
	original := facades.Container()
	facades.SetContainer(&facades.Services{})
	facades.Register[contractconfig.Application](fakeConfig{values: values})
	t.Cleanup(func() {
		facades.SetContainer(original)
	})
}

func TestNewSearchReturnsConfigError(t *testing.T) {
	withSearchConfig(t, map[string]any{})

	manager, err := NewSearchWithRegistry(&DriverRegistry{})
	if err == nil {
		t.Fatal("expected missing default search connection to return an error")
	}
	if manager != nil {
		t.Fatal("expected manager to be nil when initialization fails")
	}
}

func TestSearchConnectionConstructedOnceConcurrently(t *testing.T) {
	withSearchConfig(t, map[string]any{
		"search.default":               "default",
		"search.connections.default":   map[string]any{"driver": "fake"},
		"search.connections.secondary": map[string]any{"driver": "fake"},
	})

	registry := &DriverRegistry{factories: make(map[string]contractsearch.Factory)}
	var constructions atomic.Int32
	if err := registry.Register("fake", func(config contractsearch.ConnectionConfig) (contractsearch.Driver, error) {
		constructions.Add(1)
		return &fakeSearchDriver{name: config.Name}, nil
	}); err != nil {
		t.Fatal(err)
	}

	manager, err := NewSearchWithRegistry(registry)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	if manager.Default() == nil || manager.Default().DriverName() != "default" {
		t.Fatal("default driver was not initialized")
	}
	if _, err := manager.Search(t.Context(), "users", contractsearch.SearchRequest{Body: []byte(`{}`)}); err != nil {
		t.Fatalf("default search forwarding: %v", err)
	}
	if got := manager.Default().(*fakeSearchDriver).searchCount.Load(); got != 1 {
		t.Fatalf("default driver search count = %d, want 1", got)
	}

	const goroutines = 32
	drivers := make(chan contractsearch.Driver, goroutines)
	var wait sync.WaitGroup
	for range goroutines {
		wait.Add(1)
		go func() {
			defer wait.Done()
			driver, connectionErr := manager.Connection("secondary")
			if connectionErr != nil {
				t.Errorf("load secondary connection: %v", connectionErr)
				return
			}
			drivers <- driver
		}()
	}
	wait.Wait()
	close(drivers)

	var first contractsearch.Driver
	for driver := range drivers {
		if first == nil {
			first = driver
			continue
		}
		if driver != first {
			t.Fatal("concurrent connection calls returned different driver instances")
		}
	}
	if got := constructions.Load(); got != 2 {
		t.Fatalf("factory constructions = %d, want 2 (default and secondary)", got)
	}
}

func TestSearchDifferentConnectionsConstructOutsideManagerLock(t *testing.T) {
	withSearchConfig(t, map[string]any{
		"search.default":             "default",
		"search.connections.default": map[string]any{"driver": "fake"},
		"search.connections.slow":    map[string]any{"driver": "fake"},
		"search.connections.fast":    map[string]any{"driver": "fake"},
	})

	registry := &DriverRegistry{factories: make(map[string]contractsearch.Factory)}
	slowStarted := make(chan struct{})
	releaseSlow := make(chan struct{})
	if err := registry.Register("fake", func(config contractsearch.ConnectionConfig) (contractsearch.Driver, error) {
		if config.Name == "slow" {
			close(slowStarted)
			<-releaseSlow
		}
		return &fakeSearchDriver{name: config.Name}, nil
	}); err != nil {
		t.Fatal(err)
	}
	manager, err := NewSearchWithRegistry(registry)
	if err != nil {
		t.Fatal(err)
	}

	slowDone := make(chan error, 1)
	go func() {
		_, connectionErr := manager.Connection("slow")
		slowDone <- connectionErr
	}()
	<-slowStarted

	if _, err := manager.Connection("fast"); err != nil {
		t.Fatalf("fast connection was blocked by slow construction: %v", err)
	}
	close(releaseSlow)
	if err := <-slowDone; err != nil {
		t.Fatalf("slow connection: %v", err)
	}
}

func TestSearchCloseIsIdempotentAndClosesEachInstanceOnce(t *testing.T) {
	withSearchConfig(t, map[string]any{
		"search.default":             "default",
		"search.connections.default": map[string]any{"driver": "fake"},
		"search.connections.alias":   map[string]any{"driver": "fake"},
	})

	shared := &fakeSearchDriver{name: "fake"}
	registry := &DriverRegistry{factories: make(map[string]contractsearch.Factory)}
	if err := registry.Register("fake", func(contractsearch.ConnectionConfig) (contractsearch.Driver, error) {
		return shared, nil
	}); err != nil {
		t.Fatal(err)
	}
	manager, err := NewSearchWithRegistry(registry)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Connection("alias"); err != nil {
		t.Fatal(err)
	}

	if err := manager.Close(t.Context()); err != nil {
		t.Fatalf("close manager: %v", err)
	}
	if err := manager.Close(t.Context()); err != nil {
		t.Fatalf("close manager twice: %v", err)
	}
	if got := shared.closeCount.Load(); got != 1 {
		t.Fatalf("driver close count = %d, want 1", got)
	}
	if _, err := manager.Connection("default"); !errors.Is(err, errManagerClosed) {
		t.Fatalf("connection after close error = %v, want manager closed", err)
	}
}

func TestSearchCloseCanResumeAfterContextCancellation(t *testing.T) {
	withSearchConfig(t, map[string]any{
		"search.default":             "default",
		"search.connections.default": map[string]any{"driver": "fake"},
		"search.connections.slow":    map[string]any{"driver": "fake"},
	})

	defaultDriver := &fakeSearchDriver{name: "default"}
	slowDriver := &fakeSearchDriver{name: "slow"}
	slowStarted := make(chan struct{})
	releaseSlow := make(chan struct{})
	registry := &DriverRegistry{factories: make(map[string]contractsearch.Factory)}
	if err := registry.Register("fake", func(config contractsearch.ConnectionConfig) (contractsearch.Driver, error) {
		if config.Name == "slow" {
			close(slowStarted)
			<-releaseSlow
			return slowDriver, nil
		}
		return defaultDriver, nil
	}); err != nil {
		t.Fatal(err)
	}
	manager, err := NewSearchWithRegistry(registry)
	if err != nil {
		t.Fatal(err)
	}

	connectionDone := make(chan error, 1)
	go func() {
		_, connectionErr := manager.Connection("slow")
		connectionDone <- connectionErr
	}()
	<-slowStarted

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := manager.Close(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("first close error = %v, want context canceled", err)
	}
	close(releaseSlow)
	if err := <-connectionDone; !errors.Is(err, errManagerClosed) {
		t.Fatalf("in-flight connection error = %v, want manager closed", err)
	}
	if err := manager.Close(t.Context()); err != nil {
		t.Fatalf("resume close: %v", err)
	}
	if defaultDriver.closeCount.Load() != 1 || slowDriver.closeCount.Load() != 1 {
		t.Fatalf("close counts default=%d slow=%d", defaultDriver.closeCount.Load(), slowDriver.closeCount.Load())
	}
}
