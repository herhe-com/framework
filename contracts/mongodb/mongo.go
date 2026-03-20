package mongodb

import "go.mongodb.org/mongo-driver/v2/mongo"

type Mongo interface {
	Default() *mongo.Client

	Driver(name string) (*mongo.Client, error)
}
