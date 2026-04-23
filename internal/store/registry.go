package store

import (
	"database/sql"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	redisClients RedisClients
	mongoClients MongoClients
	sqldbClients SQLDBClients
	s3Store      *S3Store
)

func SetRedisClients(c RedisClients)   { redisClients = c }
func SetMongoClients(c MongoClients)   { mongoClients = c }
func SetSQLDBClients(c SQLDBClients)   { sqldbClients = c }
func SetS3Store(s *S3Store)            { s3Store = s }

func GetRedisClient(k string) *redis.Client {
	if redisClients == nil {
		return nil
	}
	return redisClients[k]
}

func GetMongoClient(k string) *mongo.Client {
	if mongoClients == nil {
		return nil
	}
	return mongoClients[k]
}

func GetSQLDB(k string) *sql.DB {
	if sqldbClients == nil {
		return nil
	}
	return sqldbClients[k]
}

func GetS3Client(k string) *s3.Client {
	if s3Store == nil {
		return nil
	}
	return s3Store.Clients[k]
}

func GetS3Bucket(k string) string {
	if s3Store == nil {
		return ""
	}
	return s3Store.Buckets[k]
}
