package meilisearch

import (
	"github.com/herhe-com/framework/facades"
	"github.com/meilisearch/meilisearch-go"
)

type Client struct {
	client meilisearch.ServiceManager

	prefix string
	host   string `valid:"required"`
	secret string `valid:"required"`
}

func NewClient() (*Client, error) {

	c := &Client{
		prefix: facades.Cfg.GetString("search.engine.meilisearch.prefix"),
		host:   facades.Cfg.GetString("search.engine.meilisearch.host"),
		secret: facades.Cfg.GetString("search.engine.meilisearch.secret"),
	}

	if err := facades.Validator.Struct(c); err != nil {
		return nil, err
	}

	options := make([]meilisearch.Option, 0)

	if c.secret != "" {
		options = append(options, meilisearch.WithAPIKey(c.secret))
	}

	c.client = meilisearch.New(c.host, options...)

	return c, nil
}
