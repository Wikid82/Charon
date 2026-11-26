// Package redact holds the credential rules shared by the packages that show or store configuration
// text: which key names hold a credential, how to blank a URL password and how to redact free text.
// It imports only the standard library.
package redact

import (
	"strings"
)

// Placeholder replaces a credential value in free text.
const Placeholder = "[redacted]"

// credentialKeyNames are the key names whose scalar value is a credential wherever they appear (compared
// case-insensitively).
var credentialKeyNames = map[string]struct{}{
	"password": {}, "passwd": {}, "pwd": {}, "passphrase": {}, "secret": {}, "token": {}, "api_key": {}, "apikey": {}, "private_key": {},
	"authorization": {}, "bearer": {}, "psk": {}, "dsn": {}, "connection_string": {},
}

// credentialKeySuffixes and credentialKeyParts widen the exact names to compound ones such as db_password,
// webhook_token or client_secret_key: a lower-cased key that ends in a suffix or contains a part is a credential.
var (
	credentialKeySuffixes = []string{
		"password", "passwd", "secret", "token", "secret_key", "apikey", "api_key", "private_key",
		"passphrase", "access_key", "signing_key", "encryption_key", "_pwd", "_psk", "_dsn", "_connection_string",
	}
	credentialKeyParts = []string{"_password", "_token"}
)

// benignKeySuffixes and benignKeyNames exclude keys that look like credentials but hold something else: where a
// credential lives (a path, file or folder), or a limit. Without them a rule such as "contains _token" would
// blank settings like token_path or tokens_per_minute.
var (
	benignKeySuffixes = []string{"_path", "_file", "_dir", "_folder", "_url", "_uri", "_ttl", "_len", "_length", "_count", "_per_minute", "_per_second", "_enabled", "_expiry", "_seconds", "_interval", "_timeout"}
	benignKeyNames    = map[string]struct{}{"tokens_per_minute": {}, "tokens_per_second": {}, "max_tokens": {}}
)

// IsCredentialKeyName reports whether a key name names a credential: one of the exact names, or a compound
// name ending in a credential suffix or containing a credential part, minus the benign names and suffixes.
// The comparison ignores case.
func IsCredentialKeyName(name string) bool {
	name = strings.ToLower(name)
	if _, ok := credentialKeyNames[name]; ok {
		return true
	}
	return isCompoundCredentialName(name)
}

// isCompoundCredentialName reports whether a lower-cased key name is a compound credential name that the exact
// list does not cover and that is not a known benign setting.
func isCompoundCredentialName(name string) bool {
	if _, ok := benignKeyNames[name]; ok {
		return false
	}
	for _, suffix := range benignKeySuffixes {
		if strings.HasSuffix(name, suffix) {
			return false
		}
	}
	for _, suffix := range credentialKeySuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	for _, part := range credentialKeyParts {
		if strings.Contains(name, part) {
			return true
		}
	}
	return false
}
