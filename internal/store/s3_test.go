package store

import (
	"testing"

	"butterfly.orx.me/core/mod"
	"github.com/stretchr/testify/require"
)

func TestNormalizeS3Provider(t *testing.T) {
	require.Equal(t, s3ProviderAWS, normalizeS3Provider(""))
	require.Equal(t, s3ProviderAWS, normalizeS3Provider("aws"))
	require.Equal(t, s3ProviderMinIO, normalizeS3Provider("MINIO"))
}

func TestMinIOEndpoint(t *testing.T) {
	t.Run("plain endpoint", func(t *testing.T) {
		endpoint, secure, err := minioEndpoint(mod.S3Config{Endpoint: "localhost:9000", UseSSL: true})
		require.NoError(t, err)
		require.Equal(t, "localhost:9000", endpoint)
		require.True(t, secure)
	})

	t.Run("url endpoint", func(t *testing.T) {
		endpoint, secure, err := minioEndpoint(mod.S3Config{Endpoint: "http://localhost:9000", UseSSL: true})
		require.NoError(t, err)
		require.Equal(t, "localhost:9000", endpoint)
		require.False(t, secure)
	})

	t.Run("missing endpoint", func(t *testing.T) {
		_, _, err := minioEndpoint(mod.S3Config{})
		require.Error(t, err)
	})
}
