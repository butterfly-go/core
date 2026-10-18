// Package tlstest generates throwaway certificates for tests.
package tlstest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Bundle is a CA with a server certificate (valid for localhost and 127.0.0.1)
// and a client certificate, all written to a temp directory.
type Bundle struct {
	CAFile         string
	ServerCertFile string
	ServerKeyFile  string
	ClientCertFile string
	ClientKeyFile  string

	CAPool     *x509.CertPool
	ServerCert tls.Certificate
}

// New creates a Bundle. Files live in t.TempDir.
func New(t testing.TB) Bundle {
	t.Helper()
	dir := t.TempDir()

	caKey := newKey(t)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "butterfly test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	b := Bundle{CAPool: x509.NewCertPool()}
	b.CAPool.AddCert(caCert)
	b.CAFile = write(t, dir, "ca.pem", "CERTIFICATE", caDER)

	serverTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.IPv6loopback},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	b.ServerCertFile, b.ServerKeyFile, b.ServerCert = leaf(t, dir, "server", serverTmpl, caTmpl, caKey)

	clientTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "butterfly test client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	b.ClientCertFile, b.ClientKeyFile, _ = leaf(t, dir, "client", clientTmpl, caTmpl, caKey)
	return b
}

func newKey(t testing.TB) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func leaf(t testing.TB, dir, name string, tmpl, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (certFile, keyFile string, cert tls.Certificate) {
	t.Helper()
	key := newKey(t)
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certFile = write(t, dir, name+".pem", "CERTIFICATE", der)
	keyFile = write(t, dir, name+"-key.pem", "EC PRIVATE KEY", keyDER)
	cert, err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile, cert
}

func write(t testing.TB, dir, name, typ string, der []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	data := pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
