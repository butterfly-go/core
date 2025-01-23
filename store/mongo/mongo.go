package mongo

import (
	"butterfly.orx.me/core/internal/store"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func GetClient(k string) *mongo.Client {
	return store.GetMongoClients(k)
}
