package store

import (
	"context"
	"fmt"
	"time"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/log"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/v2/mongo/otelmongo"
)

var (
	mongoClients = make(map[string]*mongo.Client)
	timeout      = 10 * time.Second
)

func InitMongo() error {
	cfg := config.CoreConfig().Store.Mongo

	for k, v := range cfg {
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

func setupMongo(k, uri string) error {
	logger := log.CoreLogger("store.mongo")
	opts := options.Client().ApplyURI(uri).SetMonitor(otelmongo.NewMonitor())
	client, err := mongo.Connect(opts)
	if err != nil {
		logger.Error("connect mongo failed", "name", k, "error", err.Error())
		return fmt.Errorf("connect mongo %q: %w", k, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		logger.Error("ping mongo failed", "name", k, "error", err.Error())
		return fmt.Errorf("ping mongo %q: %w", k, err)
	}

	logger.Info("initialize mongo client", "name", k)
	mongoClients[k] = client
	return nil
}
