package store

import (
	"database/sql"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// RedisClients holds named Redis client instances.
type RedisClients map[string]*redis.Client

// MongoClients holds named MongoDB client instances.
type MongoClients map[string]*mongo.Client

// SQLDBClients holds named SQL database instances.
type SQLDBClients map[string]*sql.DB

// S3Store holds named S3 clients and their associated bucket names.
type S3Store struct {
	Clients map[string]*s3.Client
	Buckets map[string]string
}
