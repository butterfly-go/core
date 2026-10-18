package redis

import (
	"butterfly.orx.me/core/internal/store"
	"github.com/redis/go-redis/v9"
)

type Client = redis.Client

// ClusterClient and UniversalClient are the go-redis types returned for
// cluster entries and for any entry, respectively.
type (
	ClusterClient   = redis.ClusterClient
	UniversalClient = redis.UniversalClient
)

// GetClient returns the client of a standalone or sentinel entry. It returns
// nil for an unknown name and for a cluster entry: use GetUniversalClient to
// get a client whatever the entry's mode.
func GetClient(k string) *redis.Client {
	return store.GetRedisClient(k)
}

// GetClusterClient returns the client of a cluster entry, or nil when the name
// is unknown or the entry is not a cluster.
func GetClusterClient(k string) *redis.ClusterClient {
	return store.GetRedisClusterClient(k)
}

// GetUniversalClient returns the client of an entry in any mode, or nil when
// the name is unknown. Use it when the same code must work against standalone,
// sentinel and cluster deployments.
func GetUniversalClient(k string) redis.UniversalClient {
	return store.GetRedisUniversalClient(k)
}
