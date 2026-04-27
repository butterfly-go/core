package s3

import (
	"butterfly.orx.me/core/internal/store"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/minio/minio-go/v7"
)

type Client = awss3.Client
type MinIOClient = minio.Client

func GetClient(name string) *awss3.Client {
	return store.GetS3Client(name)
}

func GetMinIOClient(name string) *minio.Client {
	return store.GetMinIOClient(name)
}

func GetBucket(name string) string {
	return store.GetS3Bucket(name)
}
