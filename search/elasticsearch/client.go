package elasticsearch

import (
	elastic "github.com/elastic/go-elasticsearch/v7"
	"github.com/herhe-com/framework/facades"
)

type Client struct {
	client *elastic.Client

	prefix   string
	hosts    []string
	host     string `valid:"required_without=hosts"`
	username string `valid:"required"`
	password string `valid:"required"`
}

func NewClient() (*Client, error) {

	client := &Client{
		prefix:   facades.Cfg.GetString("search.engine.elasticsearch.prefix"),
		host:     facades.Cfg.GetString("search.engine.elasticsearch.host"),
		hosts:    facades.Cfg.GetStrings("search.engine.elasticsearch.hosts"),
		username: facades.Cfg.GetString("search.engine.elasticsearch.username"),
		password: facades.Cfg.GetString("search.engine.elasticsearch.password"),
	}

	if err := facades.Validator.Struct(client); err != nil {
		return nil, err
	}

	cfg := elastic.Config{
		Addresses: []string{
			client.host,
		},
	}

	if len(client.hosts) > 0 {
		cfg.Addresses = client.hosts
	}

	if client.username != "" && client.password != "" {
		cfg.Username = client.username
		cfg.Password = client.password
	}

	es, err := elastic.NewClient(cfg)

	if err != nil {
		return nil, err
	}

	client.client = es

	return client, nil
}
