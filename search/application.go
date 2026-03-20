package search

import (
	"fmt"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/contracts/search"
	"github.com/herhe-com/framework/facades"
	"github.com/herhe-com/framework/search/elasticsearch"
	"github.com/herhe-com/framework/search/meilisearch"
)

type Search struct {
	search.Driver
	drivers map[string]search.Driver
}

func NewSearch() *Search {

	defaultDriver := facades.Cfg.GetString("search.driver")

	if defaultDriver == "" {
		color.Errorln("[search] please set default driver")
		return nil
	}

	driver, err := NewDriver(defaultDriver, "default")

	if err != nil {
		color.Errorf("[search] %s", err)
		return nil
	}

	drivers := make(map[string]search.Driver)
	key := fmt.Sprintf("%s_%s", defaultDriver, "default")
	drivers[key] = driver

	return &Search{
		drivers: drivers,
		Driver:  driver,
	}
}

func NewDriver(driver string, name string) (search.Driver, error) {

	switch driver {
	case search.DriverMeiliSearch:
		return meilisearch.NewClient(name)
	case search.DriverElasticSearch:
		return elasticsearch.NewClient(name)
	}

	return nil, fmt.Errorf("invalid driver: %s, only support %s, %s", driver, search.DriverMeiliSearch, search.DriverElasticSearch)
}

func (r *Search) Channel(driver string, name string) (search.Driver, error) {

	key := fmt.Sprintf("%s_%s", driver, name)

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
