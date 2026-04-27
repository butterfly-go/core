package store

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"butterfly.orx.me/core/internal/config"
	"butterfly.orx.me/core/internal/log"
	"butterfly.orx.me/core/mod"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscred "github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/minio/minio-go/v7"
	miniocred "github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	s3ProviderAWS   = "aws"
	s3ProviderMinIO = "minio"
)

var (
	s3Clients    = make(map[string]*awss3.Client)
	minioClients = make(map[string]*minio.Client)
	s3Buckets    = make(map[string]string)
)

func InitS3() error {
	cfg := config.CoreConfig().Store.S3
	for k, v := range cfg {
		provider := normalizeS3Provider(v.Provider)
		switch provider {
		case s3ProviderMinIO:
			client, err := newMinIOClient(k, v)
			if err != nil {
				return err
			}
			minioClients[k] = client
		default:
			client, err := newS3Client(k, v)
			if err != nil {
				return err
			}
			s3Clients[k] = client
		}
		s3Buckets[k] = v.Bucket
	}
	return nil
}

func GetS3Client(k string) *awss3.Client {
	return s3Clients[k]
}

func GetMinIOClient(k string) *minio.Client {
	return minioClients[k]
}

func GetS3Bucket(k string) string {
	return s3Buckets[k]
}

func newS3Client(name string, v mod.S3Config) (*awss3.Client, error) {
	logger := log.CoreLogger("store.s3")
	region := s3Region(v)
	ak, sk := s3Credentials(v)
	provider := normalizeS3Provider(v.Provider)

	logger.Info("initialize s3 client",
		"name", name,
		"provider", provider,
		"endpoint", v.Endpoint,
		"region", region,
		"bucket", v.Bucket,
		"use_ssl", v.UseSSL,
		"use_path_style", v.UsePathStyle,
		"access_key_len", len(ak),
		"secret_key_len", len(sk),
		"session_token_len", len(v.SessionToken),
	)

	cfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(awscred.NewStaticCredentialsProvider(
			ak,
			sk,
			v.SessionToken,
		)),
	)
	if err != nil {
		return nil, err
	}

	return awss3.NewFromConfig(cfg, func(o *awss3.Options) {
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

func newMinIOClient(name string, v mod.S3Config) (*minio.Client, error) {
	logger := log.CoreLogger("store.s3")
	region := s3Region(v)
	ak, sk := s3Credentials(v)
	endpoint, secure, err := minioEndpoint(v)
	if err != nil {
		return nil, err
	}

	logger.Info("initialize s3 client",
		"name", name,
		"provider", s3ProviderMinIO,
		"endpoint", endpoint,
		"region", region,
		"bucket", v.Bucket,
		"use_ssl", secure,
		"use_path_style", true,
		"access_key_len", len(ak),
		"secret_key_len", len(sk),
		"session_token_len", len(v.SessionToken),
	)

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  miniocred.NewStaticV4(ak, sk, v.SessionToken),
		Secure: secure,
		Region: region,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

func normalizeS3Provider(provider string) string {
	if strings.EqualFold(provider, s3ProviderMinIO) {
		return s3ProviderMinIO
	}
	return s3ProviderAWS
}

func s3Region(v mod.S3Config) string {
	if v.Region == "" {
		return "us-east-1"
	}
	return v.Region
}

func s3Credentials(v mod.S3Config) (string, string) {
	ak := v.AccessKeyID
	if ak == "" {
		ak = v.AK
	}
	sk := v.SecretAccessKey
	if sk == "" {
		sk = v.SK
	}
	return ak, sk
}

func minioEndpoint(v mod.S3Config) (string, bool, error) {
	endpoint := strings.TrimSpace(v.Endpoint)
	if endpoint == "" {
		return "", v.UseSSL, fmt.Errorf("minio endpoint is required")
	}
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return "", false, err
		}
		if parsed.Host == "" {
			return "", false, fmt.Errorf("invalid minio endpoint: %s", endpoint)
		}
		return parsed.Host, parsed.Scheme == "https", nil
	}
	return endpoint, v.UseSSL, nil
}
