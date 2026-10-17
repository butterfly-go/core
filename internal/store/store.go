package store

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"butterfly.orx.me/core/internal/health"
	"butterfly.orx.me/core/internal/log"
)

func Init() error {
	err := InitRedis()
	if err != nil {
		return err
	}
	err = InitSQLDB()
	if err != nil {
		return err
	}
	err = InitGORM()
	if err != nil {
		return err
	}
	err = InitMongo()
	if err != nil {
		return err
	}
	return InitS3()
}

// HealthChecks returns one readiness check per open Redis, SQL and Mongo
// client, named "redis/<name>", "sql/<name>" and "mongo/<name>", in a stable
// order. It snapshots the clients, so call it after Init. S3 is not included:
// a meaningful probe needs bucket permissions and costs a request, so apps
// that want one can register their own.
func HealthChecks() []health.Check {
	var checks []health.Check
	for _, name := range sortedKeys(redisClients) {
		client := redisClients[name]
		checks = append(checks, health.Check{
			Name:  "redis/" + name,
			Check: func(ctx context.Context) error { return client.Ping(ctx).Err() },
		})
	}
	for _, name := range sortedKeys(sqldbClients) {
		db := sqldbClients[name]
		checks = append(checks, health.Check{
			Name:  "sql/" + name,
			Check: db.PingContext,
		})
	}
	for _, name := range sortedKeys(mongoClients) {
		client := mongoClients[name]
		checks = append(checks, health.Check{
			Name:  "mongo/" + name,
			Check: func(ctx context.Context) error { return client.Ping(ctx, nil) },
		})
	}
	return checks
}

// Close releases every client opened by Init (Redis, SQL, Mongo).
//
// It is best-effort: a failure to close one client does not prevent the
// others from being closed, and all errors are joined into the returned
// error. The registries are emptied afterwards, so calling Close twice is
// safe. The context bounds clients that support a deadline (Mongo).
func Close(ctx context.Context) error {
	logger := log.CoreLogger("store.close")
	var errs []error

	for _, name := range sortedKeys(redisClients) {
		if err := redisClients[name].Close(); err != nil {
			logger.Error("close redis client failed", "name", name, "error", err.Error())
			errs = append(errs, fmt.Errorf("close redis %q: %w", name, err))
		} else {
			logger.Info("closed redis client", "name", name)
		}
	}
	clear(redisClients)

	for _, name := range sortedKeys(sqldbClients) {
		if err := sqldbClients[name].Close(); err != nil {
			logger.Error("close sql db failed", "name", name, "error", err.Error())
			errs = append(errs, fmt.Errorf("close sql db %q: %w", name, err))
		} else {
			logger.Info("closed sql db", "name", name)
		}
	}
	clear(sqldbClients)

	for _, name := range sortedKeys(mongoClients) {
		if err := mongoClients[name].Disconnect(ctx); err != nil {
			logger.Error("disconnect mongo failed", "name", name, "error", err.Error())
			errs = append(errs, fmt.Errorf("disconnect mongo %q: %w", name, err))
		} else {
			logger.Info("closed mongo client", "name", name)
		}
	}
	clear(mongoClients)

	// The AWS S3 client is HTTP based and holds nothing that needs closing.
	clear(s3Clients)
	clear(s3Buckets)

	return errors.Join(errs...)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
