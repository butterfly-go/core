// Package tlsutil builds crypto/tls client configurations from the TLS
// section shared by Redis, Consul and tracing settings.
package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	"butterfly.orx.me/core/mod"
)

// Build returns the client tls.Config for c, or nil when TLS is not enabled.
// It reads the CA bundle and the client key pair from disk. The minimum
// protocol version is TLS 1.2.
func Build(c mod.TLSConfig) (*tls.Config, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if !c.Enabled {
		return nil, nil
	}
	cfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         c.ServerName,
		InsecureSkipVerify: c.InsecureSkipVerify, //nolint:gosec // explicit opt-in, documented as testing only
	}
	if c.CAFile != "" {
		pem, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read tls.ca_file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("tls.ca_file contains no valid PEM certificates")
		}
		cfg.RootCAs = pool
	}
	if c.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load tls.cert_file/tls.key_file: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
}
