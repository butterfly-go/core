package s3

import (
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type Client = awss3.Client

var (
	clients map[string]*awss3.Client
	buckets map[string]string
)

// Set sets the S3 clients and bucket mappings. Called by the app during initialization.
func Set(c map[string]*awss3.Client, b map[string]string) {
	clients = c
	buckets = b
}

// GetClient returns an S3 client by name.
func GetClient(name string) *awss3.Client {
	return clients[name]
}

// GetBucket returns an S3 bucket name by key.
func GetBucket(name string) string {
	return buckets[name]
}
