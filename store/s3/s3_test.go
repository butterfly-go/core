package s3

import (
	"testing"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestSetAndGetClient(t *testing.T) {
	Set(
		map[string]*awss3.Client{"assets": {}},
		map[string]string{"assets": "my-bucket"},
	)

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

func TestGetClient_BeforeSet(t *testing.T) {
	Set(nil, nil)
	if got := GetClient("any"); got != nil {
		t.Fatalf("expected nil before Set, got %v", got)
	}
	if got := GetBucket("any"); got != "" {
		t.Fatalf("expected empty string before Set, got %s", got)
	}
}
