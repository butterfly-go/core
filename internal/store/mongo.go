package store

import (
	"context"
	"time"

	"butterfly.orx.me/core/internal/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	mongoClients = make(map[string]*mongo.Client)
	timeout      = 10 * time.Second
)

func InitMongo() error {
	config := config.CoreConfig().Store.Mongo

	for k, v := range config {
		err := setupMongo(k, v.URI)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetMongoClients(k string) *mongo.Client {
	return mongoClients[k]
}

func setupMongo(k, v string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(v))
	if err != nil {
		return err
	}
	mongoClients[k] = client
	return nil
}
