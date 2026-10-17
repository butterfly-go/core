package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestCloseReleasesAllClients(t *testing.T) {
	// None of these clients connect eagerly, so no server is required.
	rc := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	redisClients["r"] = rc

	db, err := sql.Open("pgx", "postgres://u:p@127.0.0.1:1/d")
	if err != nil {
		t.Fatal(err)
	}
	sqldbClients["d"] = db

	mc, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	mongoClients["m"] = mc

	s3Buckets["s"] = "bucket"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if len(redisClients) != 0 || len(sqldbClients) != 0 || len(mongoClients) != 0 || len(s3Clients) != 0 || len(s3Buckets) != 0 {
		t.Fatal("registries should be empty after Close")
	}
	if err := rc.Ping(ctx).Err(); err == nil {
		t.Fatal("redis client should be closed")
	}
	if err := db.PingContext(ctx); err == nil {
		t.Fatal("sql db should be closed")
	}
	if err := mc.Ping(ctx, nil); err == nil {
		t.Fatal("mongo client should be disconnected")
	}
}

func TestCloseIsIdempotentAndHandlesEmpty(t *testing.T) {
	ctx := context.Background()
	if err := Close(ctx); err != nil {
		t.Fatalf("Close on empty: %v", err)
	}
	if err := Close(ctx); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestCloseJoinsErrorsAndContinues(t *testing.T) {
	rc := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	if err := rc.Close(); err != nil {
		t.Fatal(err)
	}
	// Closing an already-closed redis client errors; the SQL db after it must
	// still be closed.
	redisClients["r"] = rc
	db, err := sql.Open("pgx", "postgres://u:p@127.0.0.1:1/d")
	if err != nil {
		t.Fatal(err)
	}
	sqldbClients["d"] = db

	err = Close(context.Background())
	if err == nil {
		t.Fatal("expected error from double-closed redis client")
	}
	if len(sqldbClients) != 0 {
		t.Fatal("sql db should still be released after an earlier failure")
	}
	if pingErr := db.PingContext(context.Background()); pingErr == nil {
		t.Fatal("sql db should be closed")
	}
}

func TestHealthChecksNamesAndFailures(t *testing.T) {
	t.Cleanup(func() { _ = Close(context.Background()) })
	redisClients["cache"] = redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	redisClients["a"] = redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	db, err := sql.Open("pgx", "postgres://u:p@127.0.0.1:1/d")
	if err != nil {
		t.Fatal(err)
	}
	sqldbClients["main"] = db
	mc, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:1").SetServerSelectionTimeout(100 * time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	mongoClients["m"] = mc
	s3Buckets["s"] = "bucket" // S3 is deliberately not health-checked

	checks := HealthChecks()
	names := make([]string, 0, len(checks))
	for _, c := range checks {
		names = append(names, c.Name)
	}
	want := []string{"redis/a", "redis/cache", "sql/main", "mongo/m"}
	if len(names) != len(want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v, want %v", names, want)
		}
	}

	// Nothing is listening, so every check must report an error rather than hang.
	for _, c := range checks {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := c.Check(ctx)
		cancel()
		if err == nil {
			t.Fatalf("%s: expected an error against an unreachable server", c.Name)
		}
	}
}

func TestHealthChecksEmpty(t *testing.T) {
	if got := HealthChecks(); len(got) != 0 {
		t.Fatalf("got %d checks with no clients", len(got))
	}
}
