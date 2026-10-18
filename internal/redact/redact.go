// Package redact keeps credentials out of logs and error messages.
//
// Connection strings and driver errors routinely embed passwords:
// "postgres://user:secret@host/db", "user:secret@tcp(host:3306)/db", or an
// error that quotes the DSN it failed to parse. The helpers here scrub those
// from strings, errors and structured log attributes.
package redact

import (
	"regexp"
	"strings"
)

// Mask replaces a redacted value.
const Mask = "***"

var (
	// scheme://user:password@host, as used by postgres://, mongodb://,
	// mongodb+srv://, redis:// and so on. The password runs to the last '@'
	// before the first '/' or whitespace, so an unescaped '@' inside it is
	// still covered.
	uriUserinfo = regexp.MustCompile(`(://[^/\s:@]*):([^/\s]*)@`)
	// user:password@tcp(host:3306)/db, the go-sql-driver/mysql DSN form.
	mysqlDSN = regexp.MustCompile(`([^\s:/@]+):([^\s]*)@(tcp|unix)\(`)
	// password=... in key/value DSNs and URI query strings.
	kvPassword = regexp.MustCompile(`(?i)((?:password|passwd|pwd)=)([^\s&;]*)`)
)

// String masks credentials in s: the password of any URI or MySQL DSN,
// password=... pairs, and every occurrence of the given secrets. Empty
// secrets are ignored.
func String(s string, secrets ...string) string {
	if s == "" {
		return s
	}
	for _, secret := range secrets {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, Mask)
		}
	}
	if strings.Contains(s, "@") {
		s = uriUserinfo.ReplaceAllString(s, "${1}:"+Mask+"@")
		s = mysqlDSN.ReplaceAllString(s, "${1}:"+Mask+"@${3}(")
	}
	if lower := strings.ToLower(s); strings.Contains(lower, "pass") || strings.Contains(lower, "pwd") {
		s = kvPassword.ReplaceAllString(s, "${1}"+Mask)
	}
	return s
}

// URI is String without extra secrets. It is meant for connection strings you
// want to log with the host and database still visible.
func URI(s string) string { return String(s) }

// Secret returns Mask for a non-empty secret and "" for an empty one, so a log
// line shows whether a password is configured without revealing it.
func Secret(s string) string {
	if s == "" {
		return ""
	}
	return Mask
}

type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string { return e.msg }
func (e *redactedError) Unwrap() error { return e.err }

// Error returns an error whose message is err's message passed through
// String. The original error stays reachable with errors.Is and errors.As. A
// nil error stays nil, and an error that needs no redaction is returned as is.
func Error(err error, secrets ...string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	clean := String(msg, secrets...)
	if clean == msg {
		return err
	}
	return &redactedError{msg: clean, err: err}
}

// sensitiveKeys are attribute names whose values are always masked.
var sensitiveKeys = map[string]struct{}{
	"password":          {},
	"passwd":            {},
	"pwd":               {},
	"secret":            {},
	"token":             {},
	"dsn":               {},
	"authorization":     {},
	"cookie":            {},
	"credentials":       {},
	"access_key":        {},
	"secret_key":        {},
	"secret_access_key": {},
	"session_token":     {},
	"api_key":           {},
	"apikey":            {},
	"private_key":       {},
	"sk":                {},
}

var sensitiveSuffixes = []string{"_password", "_passwd", "_secret", "_token", "_dsn", "_api_key", "_private_key"}

// IsSensitiveKey reports whether a log attribute or config key with this name
// holds a credential, such as "password", "db_password" or "api-key".
func IsSensitiveKey(key string) bool {
	k := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	if _, ok := sensitiveKeys[k]; ok {
		return true
	}
	for _, suffix := range sensitiveSuffixes {
		if strings.HasSuffix(k, suffix) {
			return true
		}
	}
	return false
}
