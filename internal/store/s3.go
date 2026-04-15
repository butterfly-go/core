package store

import (
	"context"
	"strings"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/mod"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	s3Clients = make(map[string]*s3.Client)
	s3Buckets = make(map[string]string)
)

func InitS3() error {
	cfg := config.CoreConfig().Store.S3
	for k, v := range cfg {
		client, err := newS3Client(v)
		if err != nil {
			return err
		}
		s3Clients[k] = client
		s3Buckets[k] = v.Bucket
	}
	return nil
}

func GetS3Client(k string) *s3.Client {
	return s3Clients[k]
}

func GetS3Bucket(k string) string {
	return s3Buckets[k]
}

func newS3Client(v mod.S3Config) (*s3.Client, error) {
	region := v.Region
	if region == "" {
		region = "us-east-1"
	}
	ak := v.AccessKeyID
	if ak == "" {
		ak = v.AK
	}
	sk := v.SecretAccessKey
	if sk == "" {
		sk = v.SK
	}

	cfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			ak,
			sk,
			v.SessionToken,
		)),
	)
	if err != nil {
		return nil, err
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = v.UsePathStyle
		if v.Endpoint != "" {
			endpoint := v.Endpoint
			if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
				scheme := "http://"
				if v.UseSSL {
					scheme = "https://"
				}
				endpoint = scheme + endpoint
			}
			o.BaseEndpoint = &endpoint
		}
	}), nil
}
