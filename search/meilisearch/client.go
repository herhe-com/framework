package meilisearch

import (
	"github.com/herhe-com/framework/facades"
	searchconfig "github.com/herhe-com/framework/search/config"
	"github.com/meilisearch/meilisearch-go"
)

type Client struct {
	client meilisearch.ServiceManager

	prefix string
	host   string `valid:"required"`
	secret string `valid:"required"`
}

func NewClient(name string) (*Client, error) {
	c := &Client{
		prefix: searchconfig.ConnectionString(name, "prefix", ""),
		host:   searchconfig.ConnectionString(name, "host", ""),
		secret: searchconfig.ConnectionString(name, "secret", ""),
	}

	if err := facades.Validator.Struct(c); err != nil {
		return nil, err
	}

	c.client = meilisearch.New(c.host, meilisearch.WithAPIKey(c.secret))

	return c, nil
}
