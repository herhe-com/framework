package search

import (
	"fmt"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/search"
	"github.com/herhe-com/framework/facades"
	searchconfig "github.com/herhe-com/framework/search/config"
	"github.com/herhe-com/framework/search/elasticsearch"
	"github.com/herhe-com/framework/search/meilisearch"
)

type Search struct {
	search.Driver
	drivers map[string]search.Driver
}

func NewSearch() *Search {
	defaultName := DefaultName()
	driver, err := NewDriver("", defaultName)

	if err != nil {
		color.Errorf("[search] %s", err)
		return nil
	}

	drivers := make(map[string]search.Driver)
	drivers[defaultName] = driver

	return &Search{
		drivers: drivers,
		Driver:  driver,
	}
}

// DefaultName returns the configured default search connection name.
func DefaultName() string {
	return facades.Cfg.GetString("search.default", "default")
}

func NewDriver(driver string, name string) (search.Driver, error) {
	if driver == "" {
		driver = searchconfig.Driver(name, "")
	}

	switch driver {
	case search.DriverMeiliSearch:
		return meilisearch.NewClient(name)
	case search.DriverElasticSearch:
		return elasticsearch.NewClient(name)
	}

	return nil, fmt.Errorf("invalid driver: %s, only support %s, %s", driver, search.DriverMeiliSearch, search.DriverElasticSearch)
}

// ConnectionString returns the configured string value for a search connection field.
func ConnectionString(name, field, defaultValue string) string {
	return searchconfig.ConnectionString(name, field, defaultValue)
}

// ConnectionStrings returns the configured string slice value for a search connection field.
func ConnectionStrings(name, field string, defaultValue []string) []string {
	return searchconfig.ConnectionStrings(name, field, defaultValue)
}

func (r *Search) Channel(driver string, name string) (search.Driver, error) {

	key := name

	if dri, exist := r.drivers[key]; exist {
		return dri, nil
	}

	dri, err := NewDriver(driver, name)
	if err != nil {
		return nil, err
	}

	r.drivers[key] = dri

	return dri, nil
}
