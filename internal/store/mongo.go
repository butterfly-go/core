package store

import (
	"context"
	"fmt"
	"time"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/log"
	"butterfly.orx.me/core/internal/redact"
	"butterfly.orx.me/core/mod"
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

	for _, k := range sortedKeys(cfg) {
		if err := setupMongo(k, cfg[k]); err != nil {
			return err
		}
	}
	return nil
}

func GetMongoClients(k string) *mongo.Client {
	return mongoClients[k]
}

func setupMongo(k string, v mod.MongoConfig) error {
	logger := log.CoreLogger("store.mongo")
	opts := mongoOptions(v)
	client, err := mongo.Connect(opts)
	if err != nil {
		// Driver errors can quote the URI, credentials included.
		err = redact.Error(err)
		logger.Error("connect mongo failed", "name", k, "uri", redact.URI(v.URI), "error", err.Error())
		return fmt.Errorf("connect mongo %q: %w", k, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// Ping is best-effort: mongo.Connect does not connect eagerly, so an
	// unreachable server must not prevent the service from starting.
	if err := client.Ping(ctx, nil); err != nil {
		logger.Warn("ping mongo failed", "name", k, "uri", redact.URI(v.URI), "error", redact.Error(err).Error())
	}

	logger.Info("initialize mongo client", "name", k, "uri", redact.URI(v.URI),
		"max_pool_size", v.MaxPoolSize, "min_pool_size", v.MinPoolSize)
	mongoClients[k] = client
	return nil
}

// mongoOptions applies the URI first and then the explicit pool and timeout
// fields, so a non-zero field in the config wins over the same URI option.
func mongoOptions(v mod.MongoConfig) *options.ClientOptions {
	opts := options.Client().ApplyURI(v.URI).SetMonitor(otelmongo.NewMonitor())
	if v.MaxPoolSize > 0 {
		opts.SetMaxPoolSize(v.MaxPoolSize)
	}
	if v.MinPoolSize > 0 {
		opts.SetMinPoolSize(v.MinPoolSize)
	}
	if v.MaxConnecting > 0 {
		opts.SetMaxConnecting(v.MaxConnecting)
	}
	if v.MaxConnIdleTime > 0 {
		opts.SetMaxConnIdleTime(v.MaxConnIdleTime)
	}
	if v.ConnectTimeout > 0 {
		opts.SetConnectTimeout(v.ConnectTimeout)
	}
	if v.ServerSelectionTimeout > 0 {
		opts.SetServerSelectionTimeout(v.ServerSelectionTimeout)
	}
	if v.Timeout > 0 {
		opts.SetTimeout(v.Timeout)
	}
	return opts
}
