package db

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Client is the global MongoDB client (use a singleton in production).
var Client *mongo.Client

// Connect initializes the MongoDB connection using the provided URI.
func Connect(uri string) error {
	var err error
	Client, err = mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = Client.Ping(ctx, readpref.Primary())
	if err != nil {
		return err
	}

	log.Println("Connected to MongoDB!")
	return nil
}

// Disconnect closes the connection (call in main defer).
func Disconnect(ctx context.Context) error {
	return Client.Disconnect(ctx)
}

// GetCollection returns a collection from the specified database and collection name.
func GetCollection(dbName, collectionName string) *mongo.Collection {
	return Client.Database(dbName).Collection(collectionName)
}
