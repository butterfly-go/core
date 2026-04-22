package mongo

import (
	"butterfly.orx.me/core/internal/store"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// GetClient returns a MongoDB client by name.
func GetClient(k string) *mongo.Client {
	return store.GetMongoClient(k)
}
