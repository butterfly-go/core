package arg

import (
	"testing"

	"butterfly.orx.me/core/mod"
)

func TestTLS(t *testing.T) {
	t.Setenv("BUTTERFLY_X_TLS_ENABLED", "true")
	t.Setenv("BUTTERFLY_X_TLS_CA_FILE", "/ca.pem")
	t.Setenv("BUTTERFLY_X_TLS_CERT_FILE", "/cert.pem")
	t.Setenv("BUTTERFLY_X_TLS_KEY_FILE", "/key.pem")
	t.Setenv("BUTTERFLY_X_TLS_SERVER_NAME", "consul.internal")
	t.Setenv("BUTTERFLY_X_TLS_INSECURE_SKIP_VERIFY", "1")

	want := mod.TLSConfig{
		Enabled: true, CAFile: "/ca.pem", CertFile: "/cert.pem", KeyFile: "/key.pem",
		ServerName: "consul.internal", InsecureSkipVerify: true,
	}
	if got := TLS("x.tls"); got != want {
		t.Fatalf("TLS = %+v, want %+v", got, want)
	}
	if got := TLS("unset.tls"); got != (mod.TLSConfig{}) {
		t.Fatalf("unset TLS = %+v, want zero", got)
	}
}
