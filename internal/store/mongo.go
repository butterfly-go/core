package store

import (
	"context"
	"time"

	"butterfly.orx.me/core/mod"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var timeout = 10 * time.Second

// ProvideMongoClients creates MongoDB clients from config.
func ProvideMongoClients(cc *mod.CoreConfig) (MongoClients, func(), error) {
	clients := make(MongoClients)
	for k, v := range cc.Store.Mongo {
		client, err := mongo.Connect(options.Client().ApplyURI(v.URI))
		if err != nil {
			for _, c := range clients {
				_ = c.Disconnect(context.Background())
			}
			return nil, nil, err
		}
		clients[k] = client
	}
	cleanup := func() {
		for _, c := range clients {
			_ = c.Disconnect(context.Background())
		}
	}
	return clients, cleanup, nil
}
