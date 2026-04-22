package mongo

import (
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var clients map[string]*mongo.Client

// Set sets the MongoDB clients map. Called by the app during initialization.
func Set(c map[string]*mongo.Client) {
	clients = c
}

// GetClient returns a MongoDB client by name.
func GetClient(k string) *mongo.Client {
	return clients[k]
}
