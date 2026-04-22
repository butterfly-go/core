package store

import (
	"testing"

	"butterfly.orx.me/core/mod"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestProvideRedisClients_EmptyConfig(t *testing.T) {
	cc := &mod.CoreConfig{}
	clients, cleanup, err := ProvideRedisClients(cc)
	if err != nil {
		t.Fatalf("ProvideRedisClients() error: %v", err)
	}
	defer cleanup()
	if len(clients) != 0 {
		t.Fatalf("expected 0 clients, got %d", len(clients))
	}
}

func TestProvideMongoClients_EmptyConfig(t *testing.T) {
	cc := &mod.CoreConfig{}
	clients, cleanup, err := ProvideMongoClients(cc)
	if err != nil {
		t.Fatalf("ProvideMongoClients() error: %v", err)
	}
	defer cleanup()
	if len(clients) != 0 {
		t.Fatalf("expected 0 clients, got %d", len(clients))
	}
}

func TestProvideSQLDBClients_EmptyConfig(t *testing.T) {
	cc := &mod.CoreConfig{}
	clients, cleanup, err := ProvideSQLDBClients(cc)
	if err != nil {
		t.Fatalf("ProvideSQLDBClients() error: %v", err)
	}
	defer cleanup()
	if len(clients) != 0 {
		t.Fatalf("expected 0 clients, got %d", len(clients))
	}
}

func TestProvideS3Store_EmptyConfig(t *testing.T) {
	cc := &mod.CoreConfig{}
	st, err := ProvideS3Store(cc)
	if err != nil {
		t.Fatalf("ProvideS3Store() error: %v", err)
	}
	if len(st.Clients) != 0 {
		t.Fatalf("expected 0 s3 clients, got %d", len(st.Clients))
	}
	if len(st.Buckets) != 0 {
		t.Fatalf("expected 0 s3 buckets, got %d", len(st.Buckets))
	}
}

func TestProvideS3Store_WithConfig(t *testing.T) {
	cc := &mod.CoreConfig{
		Store: mod.StoreConfig{
			S3: map[string]mod.S3Config{
				"test": {
					Region:          "us-west-2",
					AccessKeyID:     "AKID",
					SecretAccessKey: "SECRET",
					Bucket:          "test-bucket",
					Endpoint:        "localhost:9000",
					UsePathStyle:    true,
				},
			},
		},
	}
	st, err := ProvideS3Store(cc)
	if err != nil {
		t.Fatalf("ProvideS3Store() error: %v", err)
	}
	if len(st.Clients) != 1 {
		t.Fatalf("expected 1 s3 client, got %d", len(st.Clients))
	}
	if st.Clients["test"] == nil {
		t.Fatal("expected non-nil s3 client for key 'test'")
	}
	if st.Buckets["test"] != "test-bucket" {
		t.Fatalf("expected bucket test-bucket, got %s", st.Buckets["test"])
	}
}

func TestDBConfigToDSN(t *testing.T) {
	cfg := mod.DBConfig{
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "secret",
		DBName:   "testdb",
	}
	want := "root:secret@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local"
	got := DBConfigToDSN(cfg)
	if got != want {
		t.Fatalf("DBConfigToDSN() = %q, want %q", got, want)
	}
}

func TestRegistry_SetAndGet(t *testing.T) {
	// Redis
	SetRedisClients(nil)
	if got := GetRedisClient("any"); got != nil {
		t.Fatalf("expected nil redis client, got %v", got)
	}

	// Mongo
	SetMongoClients(nil)
	if got := GetMongoClient("any"); got != nil {
		t.Fatalf("expected nil mongo client, got %v", got)
	}

	// SQLDB
	SetSQLDBClients(nil)
	if got := GetSQLDB("any"); got != nil {
		t.Fatalf("expected nil sqldb, got %v", got)
	}

	// S3
	SetS3Store(nil)
	if got := GetS3Client("any"); got != nil {
		t.Fatalf("expected nil s3 client, got %v", got)
	}
	if got := GetS3Bucket("any"); got != "" {
		t.Fatalf("expected empty bucket, got %s", got)
	}

	// S3 with data
	SetS3Store(&S3Store{
		Clients: map[string]*s3.Client{"assets": {}},
		Buckets: map[string]string{"assets": "my-bucket"},
	})
	if got := GetS3Client("assets"); got == nil {
		t.Fatal("expected non-nil s3 client")
	}
	if got := GetS3Bucket("assets"); got != "my-bucket" {
		t.Fatalf("expected my-bucket, got %s", got)
	}
}
