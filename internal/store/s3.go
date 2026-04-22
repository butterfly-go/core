package store

import (
	"context"
	"strings"

	"butterfly.orx.me/core/mod"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Legacy globals for backward compatibility.
var (
	s3Clients = make(map[string]*s3.Client)
	s3Buckets = make(map[string]string)
)

// ProvideS3Store creates S3 clients and bucket mappings from config.
func ProvideS3Store(cc *mod.CoreConfig) (*S3Store, error) {
	st := &S3Store{
		Clients: make(map[string]*s3.Client),
		Buckets: make(map[string]string),
	}
	for k, v := range cc.Store.S3 {
		client, err := newS3Client(v)
		if err != nil {
			return nil, err
		}
		st.Clients[k] = client
		st.Buckets[k] = v.Bucket
	}
	return st, nil
}

// SetLegacyS3 populates the legacy globals.
func SetLegacyS3(st *S3Store) {
	s3Clients = st.Clients
	s3Buckets = st.Buckets
}

// GetS3Client returns an S3 client by name from the legacy global.
func GetS3Client(k string) *s3.Client {
	return s3Clients[k]
}

// GetS3Bucket returns an S3 bucket name by key from the legacy global.
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
