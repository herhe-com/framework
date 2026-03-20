package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gookit/color"
	"github.com/herhe-com/framework/facades"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoDB struct {
	client  *mongo.Client
	clients map[string]*mongo.Client
}

func NewApplication() (*MongoDB, error) {
	client, name, err := NewClient("default")

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

func NewClient(name string) (*mongo.Client, string, error) {
	var uri, username, password, host, port, db, authSource string

	uri = facades.Cfg.GetString("mongodb." + name + ".uri")

	if uri == "" {
		username = facades.Cfg.GetString("mongodb." + name + ".username")
		password = facades.Cfg.GetString("mongodb." + name + ".password")
		host = facades.Cfg.GetString("mongodb." + name + ".host")
		port = facades.Cfg.GetString("mongodb."+name+".port", "27017")
		db = facades.Cfg.GetString("mongodb." + name + ".db")
		authSource = facades.Cfg.GetString("mongodb."+name+".auth_source", "admin")

		if host == "" || db == "" {
			return nil, "", errors.New("invalid mongodb config: missing host or db")
		}

		if username != "" && password != "" {
			uri = fmt.Sprintf("mongodb://%s:%s@%s:%s/%s?authSource=%s", username, password, host, port, db, authSource)
		} else {
			uri = fmt.Sprintf("mongodb://%s:%s/%s", host, port, db)
		}
	}

	timeout := facades.Cfg.GetInt("mongodb."+name+".timeout", 10)
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
