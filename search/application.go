package search

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	contractsearch "github.com/herhe-com/framework/contracts/search"
	"github.com/herhe-com/framework/facades"
)

var errManagerClosed = errors.New("search: manager is closed")

type connectionState struct {
	name   string
	ready  chan struct{}
	driver contractsearch.Driver
	err    error
}

// Search manages context-aware search driver connections.
type Search struct {
	mu            sync.RWMutex
	closeMu       sync.Mutex
	registry      contractsearch.Registry
	defaultName   string
	defaultDri    contractsearch.Driver
	connections   map[string]*connectionState
	closedDrivers map[driverIdentity]struct{}
	closedStates  map[*connectionState]struct{}
	closed        bool
}

// NewSearch creates a search manager using the process-wide driver registry.
func NewSearch() (*Search, error) {
	return NewSearchWithRegistry(DefaultRegistry())
}

// NewSearchWithRegistry creates a search manager with an isolated registry.
func NewSearchWithRegistry(registry contractsearch.Registry) (*Search, error) {
	if registry == nil {
		return nil, errors.New("search: registry is nil")
	}

	manager := &Search{
		registry:      registry,
		defaultName:   DefaultName(),
		connections:   make(map[string]*connectionState),
		closedDrivers: make(map[driverIdentity]struct{}),
		closedStates:  make(map[*connectionState]struct{}),
	}
	driver, err := manager.Connection(manager.defaultName)
	if err != nil {
		return nil, err
	}
	manager.defaultDri = driver

	return manager, nil
}

// DefaultName returns the configured default search connection name.
func DefaultName() string {
	return facades.Config().GetString("search.default", "default")
}

// Default returns the eagerly initialized default driver.
func (manager *Search) Default() contractsearch.Driver {
	if manager == nil {
		return nil
	}
	return manager.defaultDri
}

// DriverName returns the default driver's name.
func (manager *Search) DriverName() string {
	driver := manager.Default()
	if driver == nil {
		return ""
	}
	return driver.DriverName()
}

// Ping forwards to the default driver.
func (manager *Search) Ping(ctx context.Context) (*contractsearch.Response, error) {
	driver, err := manager.defaultDriver()
	if err != nil {
		return nil, err
	}
	return driver.Ping(ctx)
}

// Search forwards to the default driver.
func (manager *Search) Search(ctx context.Context, index string, request contractsearch.SearchRequest) (*contractsearch.Response, error) {
	driver, err := manager.defaultDriver()
	if err != nil {
		return nil, err
	}
	return driver.Search(ctx, index, request)
}

// UpsertDocument forwards to the default driver.
func (manager *Search) UpsertDocument(ctx context.Context, index, id string, request contractsearch.DocumentWriteRequest) (*contractsearch.Response, error) {
	driver, err := manager.defaultDriver()
	if err != nil {
		return nil, err
	}
	return driver.UpsertDocument(ctx, index, id, request)
}

// GetDocument forwards to the default driver.
func (manager *Search) GetDocument(ctx context.Context, index, id string, options contractsearch.RequestOptions) (*contractsearch.Response, error) {
	driver, err := manager.defaultDriver()
	if err != nil {
		return nil, err
	}
	return driver.GetDocument(ctx, index, id, options)
}

// DeleteDocument forwards to the default driver.
func (manager *Search) DeleteDocument(ctx context.Context, index, id string, options contractsearch.RequestOptions) (*contractsearch.Response, error) {
	driver, err := manager.defaultDriver()
	if err != nil {
		return nil, err
	}
	return driver.DeleteDocument(ctx, index, id, options)
}

// Connection returns a cached driver, constructing each connection at most once.
func (manager *Search) Connection(name string) (contractsearch.Driver, error) {
	if manager == nil {
		return nil, errors.New("search: manager is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("search: connection name is required")
	}

	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return nil, errManagerClosed
	}
	if state, exists := manager.connections[name]; exists {
		manager.mu.Unlock()
		<-state.ready
		return manager.connectionResult(state)
	}

	state := &connectionState{name: name, ready: make(chan struct{})}
	manager.connections[name] = state
	manager.mu.Unlock()

	config, err := connectionConfig(name)
	if err == nil {
		state.driver, err = manager.registry.Create(config)
	}
	state.err = err
	close(state.ready)

	return manager.connectionResult(state)
}

// Close closes every created driver once and aggregates close errors.
func (manager *Search) Close(ctx context.Context) error {
	if manager == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("search: context is nil")
	}

	manager.closeMu.Lock()
	defer manager.closeMu.Unlock()

	manager.mu.Lock()
	manager.closed = true
	states := make([]*connectionState, 0, len(manager.connections))
	for _, state := range manager.connections {
		states = append(states, state)
	}
	manager.mu.Unlock()

	attemptedDrivers := make(map[driverIdentity]struct{})
	attemptedStates := make(map[*connectionState]struct{})
	var closeErrors []error
	for _, state := range states {
		select {
		case <-ctx.Done():
			closeErrors = append(closeErrors, ctx.Err())
			return errors.Join(closeErrors...)
		case <-state.ready:
		}
		if state.driver == nil {
			continue
		}
		identity, identifiable := identifyDriver(state.driver)
		if identifiable {
			manager.mu.RLock()
			_, closed := manager.closedDrivers[identity]
			manager.mu.RUnlock()
			if closed {
				continue
			}
			if _, exists := attemptedDrivers[identity]; exists {
				continue
			}
			attemptedDrivers[identity] = struct{}{}
		} else {
			manager.mu.RLock()
			_, closed := manager.closedStates[state]
			manager.mu.RUnlock()
			if closed {
				continue
			}
			if _, exists := attemptedStates[state]; exists {
				continue
			}
			attemptedStates[state] = struct{}{}
		}
		if err := state.driver.Close(ctx); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close search connection %q: %w", state.name, err))
			continue
		}
		manager.mu.Lock()
		if identifiable {
			manager.closedDrivers[identity] = struct{}{}
		} else {
			manager.closedStates[state] = struct{}{}
		}
		manager.mu.Unlock()
	}

	return errors.Join(closeErrors...)
}

func (manager *Search) defaultDriver() (contractsearch.Driver, error) {
	if manager == nil {
		return nil, errors.New("search: manager is nil")
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	if manager.closed {
		return nil, errManagerClosed
	}
	if manager.defaultDri == nil {
		return nil, errors.New("search: default driver is not initialized")
	}
	return manager.defaultDri, nil
}

func (manager *Search) connectionResult(state *connectionState) (contractsearch.Driver, error) {
	manager.mu.RLock()
	closed := manager.closed
	manager.mu.RUnlock()
	if closed {
		return nil, errManagerClosed
	}
	return state.driver, state.err
}

func connectionConfig(name string) (contractsearch.ConnectionConfig, error) {
	configKey := fmt.Sprintf("search.connections.%s", name)
	values, ok := toStringMap(facades.Config().Get(configKey))
	if !ok {
		return contractsearch.ConnectionConfig{}, fmt.Errorf("search: connection %q is not configured", name)
	}
	driver := strings.TrimSpace(fmt.Sprint(values["driver"]))
	if driver == "" || driver == "<nil>" {
		return contractsearch.ConnectionConfig{}, fmt.Errorf("search: connection %q has no driver", name)
	}
	prefix := ""
	if value, exists := values["prefix"]; exists && value != nil {
		prefix = fmt.Sprint(value)
	}

	return contractsearch.ConnectionConfig{
		Name:   name,
		Driver: driver,
		Prefix: prefix,
		Values: values,
	}, nil
}

func toStringMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return cloneConnectionValues(typed), true
	case map[any]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[fmt.Sprint(key)] = child
		}
		return result, true
	default:
		return nil, false
	}
}

type driverIdentity struct {
	typeName string
	pointer  uintptr
}

func identifyDriver(driver contractsearch.Driver) (driverIdentity, bool) {
	value := reflect.ValueOf(driver)
	if !value.IsValid() {
		return driverIdentity{}, false
	}
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.Slice, reflect.UnsafePointer:
		if value.IsNil() {
			return driverIdentity{}, false
		}
		return driverIdentity{typeName: value.Type().String(), pointer: value.Pointer()}, true
	default:
		return driverIdentity{}, false
	}
}
