package arg

import "butterfly.orx.me/core/mod"

// TLS reads a TLS section from environment variables named after prefix:
// for "config.consul.tls", BUTTERFLY_CONFIG_CONSUL_TLS_ENABLED, _CA_FILE,
// _CERT_FILE, _KEY_FILE, _SERVER_NAME and _INSECURE_SKIP_VERIFY.
func TLS(prefix string) mod.TLSConfig {
	return mod.TLSConfig{
		Enabled:            Bool(prefix + ".enabled"),
		CAFile:             String(prefix + ".ca_file"),
		CertFile:           String(prefix + ".cert_file"),
		KeyFile:            String(prefix + ".key_file"),
		ServerName:         String(prefix + ".server_name"),
		InsecureSkipVerify: Bool(prefix + ".insecure_skip_verify"),
	}
}
