package store

import (
	"context"
	"errors"
	"fmt"
	"sort"

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
	err = InitMongo()
	if err != nil {
		return err
	}
	return InitS3()
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
