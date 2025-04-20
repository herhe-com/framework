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

	driver, err := NewDriver(defaultDriver)

	if err != nil {
		color.Errorln("[search] %s\n", err)
		return nil
	}

	drivers := make(map[string]search.Driver)
	drivers[defaultDriver] = driver

	return &Search{
		drivers: drivers,
		Driver:  driver,
	}
}

func NewDriver(driver string) (search.Driver, error) {

	switch driver {
	case search.DriverMeiliSearch:
		return meilisearch.NewClient()
	case search.DriverElasticSearch:
		return elasticsearch.NewClient()
	}

	return nil, fmt.Errorf("invalid driver: %s, only support rabbitmq", driver)
}

func (r *Search) Channel(driver string) (search.Driver, error) {

	if dri, exist := r.drivers[driver]; exist {
		return dri, nil
	}

	dri, err := NewDriver(driver)
	if err != nil {
		return nil, err
	}

	r.drivers[driver] = dri

	return dri, nil
}
