package search

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	contractsearch "github.com/herhe-com/framework/contracts/search"
	"github.com/herhe-com/framework/search/elasticsearch"
	"github.com/herhe-com/framework/search/meilisearch"
)

// DriverRegistry is a concurrency-safe registry of search driver factories.
type DriverRegistry struct {
	mu        sync.RWMutex
	factories map[string]contractsearch.Factory
}

var (
	defaultRegistry     *DriverRegistry
	defaultRegistryOnce sync.Once
)

// NewRegistry creates an isolated registry with the built-in drivers registered.
func NewRegistry() *DriverRegistry {
	registry := &DriverRegistry{factories: make(map[string]contractsearch.Factory)}
	_ = registry.Register(DriverElasticSearch, func(connection contractsearch.ConnectionConfig) (contractsearch.Driver, error) {
		config, err := elasticsearch.ParseConfig(connection)
		if err != nil {
			return nil, err
		}
		return elasticsearch.New(config)
	})
	_ = registry.Register(DriverMeiliSearch, func(connection contractsearch.ConnectionConfig) (contractsearch.Driver, error) {
		config, err := meilisearch.ParseConfig(connection)
		if err != nil {
			return nil, err
		}
		return meilisearch.New(config)
	})
	return registry
}

// DefaultRegistry returns the process-wide registry used by the Search provider.
func DefaultRegistry() *DriverRegistry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = NewRegistry()
	})
	return defaultRegistry
}

// RegisterDriver registers a custom driver in the process-wide registry.
func RegisterDriver(name string, factory contractsearch.Factory) error {
	return DefaultRegistry().Register(name, factory)
}

// Register adds a driver factory without replacing an existing registration.
func (registry *DriverRegistry) Register(name string, factory contractsearch.Factory) error {
	if registry == nil {
		return errors.New("search: registry is nil")
	}
	name = normalizeDriverName(name)
	if name == "" {
		return errors.New("search: driver name is required")
	}
	if factory == nil {
		return fmt.Errorf("search: factory for driver %q is nil", name)
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.factories == nil {
		registry.factories = make(map[string]contractsearch.Factory)
	}
	if _, exists := registry.factories[name]; exists {
		return fmt.Errorf("search: driver %q is already registered", name)
	}
	registry.factories[name] = factory
	return nil
}

// Create constructs a registered driver from explicit connection configuration.
func (registry *DriverRegistry) Create(config contractsearch.ConnectionConfig) (contractsearch.Driver, error) {
	if registry == nil {
		return nil, errors.New("search: registry is nil")
	}
	driverName := normalizeDriverName(config.Driver)
	if driverName == "" {
		return nil, fmt.Errorf("search: connection %q has no driver", config.Name)
	}

	registry.mu.RLock()
	factory, exists := registry.factories[driverName]
	registry.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("search: driver %q is not registered", driverName)
	}

	config.Driver = driverName
	config.Values = cloneConnectionValues(config.Values)
	driver, err := factory(config)
	if err != nil {
		return nil, fmt.Errorf("search: create connection %q with driver %q: %w", config.Name, driverName, err)
	}
	if driver == nil {
		return nil, fmt.Errorf("search: factory for driver %q returned nil", driverName)
	}

	return driver, nil
}

func normalizeDriverName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func cloneConnectionValues(values map[string]any) map[string]any {
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
