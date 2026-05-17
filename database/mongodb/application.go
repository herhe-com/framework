package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gookit/color"
	mongodbconfig "github.com/herhe-com/framework/database/mongodb/config"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoDB struct {
	client  *mongo.Client
	clients map[string]*mongo.Client
}

func NewApplication() (*MongoDB, error) {
	defaultName := DefaultName()
	client, name, err := NewClient(defaultName)

	if err != nil {
		color.Errorf("[mongodb] %s", err)
		return nil, err
	}

	clients := make(map[string]*mongo.Client)
	clients[name] = client

	return &MongoDB{
		client:  client,
		clients: clients,
	}, nil
}

// DefaultName returns the configured default MongoDB connection name.
func DefaultName() string {
	return mongodbconfig.DefaultName()
}

func NewClient(name string) (*mongo.Client, string, error) {
	var uri, username, password, host, port, db, authSource string

	if configDriver := mongodbconfig.Driver(name, "mongodb"); configDriver != "mongodb" {
		return nil, "", fmt.Errorf("invalid mongodb config: driver %s", configDriver)
	}

	uri = mongodbconfig.ConnectionString(name, "uri", "")

	if uri == "" {
		username = mongodbconfig.ConnectionString(name, "username", "")
		password = mongodbconfig.ConnectionString(name, "password", "")
		host = mongodbconfig.ConnectionString(name, "host", "")
		port = mongodbconfig.ConnectionString(name, "port", "27017")
		db = mongodbconfig.ConnectionString(name, "db", "")
		authSource = mongodbconfig.ConnectionString(name, "auth_source", "admin")

		if host == "" || db == "" {
			return nil, "", errors.New("invalid mongodb config: missing host or db")
		}

		if username != "" && password != "" {
			uri = fmt.Sprintf("mongodb://%s:%s@%s:%s/%s?authSource=%s", username, password, host, port, db, authSource)
		} else {
			uri = fmt.Sprintf("mongodb://%s:%s/%s", host, port, db)
		}
	}

	timeout := mongodbconfig.ConnectionInt(name, "timeout", 10)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(clientOptions)

	if err != nil {
		return nil, "", err
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, "", err
	}

	return client, name, nil
}

func (r *MongoDB) Default() *mongo.Client {
	return r.client
}

func (r *MongoDB) Driver(name string) (*mongo.Client, error) {
	if client, exist := r.clients[name]; exist {
		return client, nil
	}

	client, clientName, err := NewClient(name)

	if err != nil {
		return nil, err
	}

	r.clients[clientName] = client

	return client, nil
}
