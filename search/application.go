package search

import (
	"fmt"
	"sync"

	"github.com/gookit/color"
	contractsearch "github.com/herhe-com/framework/contracts/search"
	"github.com/herhe-com/framework/facades"
	searchconfig "github.com/herhe-com/framework/search/config"
	"github.com/herhe-com/framework/search/elasticsearch"
	"github.com/herhe-com/framework/search/meilisearch"
)

type Search struct {
	contractsearch.Driver
	mu      sync.RWMutex
	drivers map[string]contractsearch.Driver
}

func NewSearch() *Search {
	search, err := NewSearchWithError()
	if err != nil {
		color.Errorf("[search] %s", err)
		return nil
	}

	return search
}

// NewSearchWithError creates the search application and returns initialization errors.
func NewSearchWithError() (*Search, error) {
	defaultName := DefaultName()
	driver, err := NewDriver(defaultName)

	if err != nil {
		return nil, err
	}

	drivers := make(map[string]contractsearch.Driver)
	drivers[defaultName] = driver

	return &Search{
		drivers: drivers,
		Driver:  driver,
	}, nil
}

// DefaultName returns the configured default search connection name.
func DefaultName() string {
	return facades.Config().GetString("search.default", "default")
}

// NewDriver creates a search driver from the given connection's configuration.
func NewDriver(name string) (contractsearch.Driver, error) {
	configKey := fmt.Sprintf("search.connections.%s", name)
	cfg, _ := facades.Config().Get(configKey).(map[string]any)

	driver, ok := cfg["driver"].(string)
	if !ok || driver == "" {
		return nil, fmt.Errorf("please set driver for connection: %s", name)
	}

	switch driver {
	case DriverMeiliSearch:
		return meilisearch.NewClient(name)
	case DriverElasticSearch:
		return elasticsearch.NewClient(name)
	}

	return nil, fmt.Errorf("invalid driver: %s, only support %s, %s", driver, DriverMeiliSearch, DriverElasticSearch)
}

// ConnectionString returns the configured string value for a search connection field.
func ConnectionString(name, field, defaultValue string) string {
	return searchconfig.ConnectionString(name, field, defaultValue)
}

// ConnectionStrings returns the configured string slice value for a search connection field.
func ConnectionStrings(name, field string, defaultValue []string) []string {
	return searchconfig.ConnectionStrings(name, field, defaultValue)
}

func (r *Search) Channel(name string) (contractsearch.Driver, error) {
	r.mu.RLock()
	if dri, exist := r.drivers[name]; exist {
		r.mu.RUnlock()
		return dri, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if dri, exist := r.drivers[name]; exist {
		return dri, nil
	}

	dri, err := NewDriver(name)
	if err != nil {
		return nil, err
	}

	r.drivers[name] = dri

	return dri, nil
}
