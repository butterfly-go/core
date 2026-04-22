package s3

import (
	"testing"

	"butterfly.orx.me/core/internal/store"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestGetClient(t *testing.T) {
	store.SetS3Store(&store.S3Store{
		Clients: map[string]*awss3.Client{"assets": {}},
		Buckets: map[string]string{"assets": "my-bucket"},
	})

	if got := GetClient("assets"); got == nil {
		t.Fatal("expected non-nil client for 'assets'")
	}
	if got := GetClient("nonexistent"); got != nil {
		t.Fatalf("expected nil for missing key, got %v", got)
	}
	if got := GetBucket("assets"); got != "my-bucket" {
		t.Fatalf("expected my-bucket, got %s", got)
	}
	if got := GetBucket("nonexistent"); got != "" {
		t.Fatalf("expected empty string, got %s", got)
	}
}
