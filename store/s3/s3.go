package s3

import (
	"butterfly.orx.me/core/internal/store"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type Client = awss3.Client

// GetClient returns an S3 client by name.
func GetClient(name string) *awss3.Client {
	return store.GetS3Client(name)
}

// GetBucket returns an S3 bucket name by key.
func GetBucket(name string) string {
	return store.GetS3Bucket(name)
}
