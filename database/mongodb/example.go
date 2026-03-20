package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/herhe-com/framework/facades"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Example demonstrates basic MongoDB operations using the framework

// ExampleBasicOperations shows CRUD operations
func ExampleBasicOperations() error {
	// Get default MongoDB client
	client := facades.Mongo.Default()
	if client == nil {
		return fmt.Errorf("MongoDB not initialized")
	}

	// Get database and collection
	database := client.Database("mydb")
	collection := database.Collection("users")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Insert a document
	user := bson.M{
		"name":  "John Doe",
		"email": "john@example.com",
		"age":   30,
	}
	result, err := collection.InsertOne(ctx, user)
	if err != nil {
		return err
	}
	fmt.Printf("Inserted document with ID: %v\n", result.InsertedID)

	// Find a document
	var foundUser bson.M
	err = collection.FindOne(ctx, bson.M{"email": "john@example.com"}).Decode(&foundUser)
	if err != nil {
		return err
	}
	fmt.Printf("Found user: %v\n", foundUser)

	// Update a document
	update := bson.M{"$set": bson.M{"age": 31}}
	_, err = collection.UpdateOne(ctx, bson.M{"email": "john@example.com"}, update)
	if err != nil {
		return err
	}

	// Delete a document
	_, err = collection.DeleteOne(ctx, bson.M{"email": "john@example.com"})
	return err
}

// ExampleMultipleConnections shows how to use multiple MongoDB connections
func ExampleMultipleConnections() error {
	// Get a specific MongoDB connection
	analyticsClient, err := facades.Mongo.Driver("analytics")
	if err != nil {
		return err
	}

	database := analyticsClient.Database("analytics")
	collection := database.Collection("events")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Insert analytics event
	event := bson.M{
		"event_type": "page_view",
		"user_id":    "12345",
		"timestamp":  time.Now(),
	}
	_, err = collection.InsertOne(ctx, event)
	return err
}

// ExampleTransaction shows how to use MongoDB transactions
func ExampleTransaction() error {
	client := facades.Mongo.Default()
	if client == nil {
		return fmt.Errorf("MongoDB not initialized")
	}

	session, err := client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(context.Background())

	callback := func(ctx context.Context) (interface{}, error) {
		database := client.Database("mydb")
		usersCollection := database.Collection("users")
		ordersCollection := database.Collection("orders")

		// Insert user
		user := bson.M{"name": "Jane Doe", "email": "jane@example.com"}
		userResult, err := usersCollection.InsertOne(ctx, user)
		if err != nil {
			return nil, err
		}

		// Insert order
		order := bson.M{
			"user_id": userResult.InsertedID,
			"amount":  100.50,
		}
		_, err = ordersCollection.InsertOne(ctx, order)
		return nil, err
	}

	_, err = session.WithTransaction(context.Background(), callback)
	return err
}

// ExampleAggregation shows how to use aggregation pipeline
func ExampleAggregation() error {
	client := facades.Mongo.Default()
	database := client.Database("mydb")
	collection := database.Collection("orders")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Aggregation pipeline
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: "status", Value: "completed"}}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$user_id"},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: "$amount"}}},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "total", Value: -1}}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return err
	}

	fmt.Printf("Aggregation results: %v\n", results)
	return nil
}

// ExampleIndexes shows how to create indexes
func ExampleIndexes() error {
	client := facades.Mongo.Default()
	database := client.Database("mydb")
	collection := database.Collection("users")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a single field index
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}

	_, err := collection.Indexes().CreateOne(ctx, indexModel)
	return err
}
