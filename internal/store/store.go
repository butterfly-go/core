package store

// SetLegacyClients populates all legacy global variables for backward compatibility.
func SetLegacyClients(redis RedisClients, mongo MongoClients, sqldb SQLDBClients, s3 *S3Store) {
	SetLegacyRedisClients(redis)
	SetLegacyMongoClients(mongo)
	SetLegacySQLDBClients(sqldb)
	SetLegacyS3(s3)
}
