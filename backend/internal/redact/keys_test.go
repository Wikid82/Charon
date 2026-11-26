package redact

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsCredentialKeyName(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"password", "API_KEY", "db_password", "x_passwd", "client_secret", "bot_token", "my_secret_key", "an_apikey", "a_api_key", "ssh_private_key", "x_token_y", "x_password_y", "dsn", "bearer"} {
		require.True(t, IsCredentialKeyName(name), name)
	}
	for _, name := range []string{"tokens_per_minute", "token_path", "x_token_file", "x_password_dir", "max_tokens", "password_policy", "retries", "token_expiry_seconds", "x_token_expiry", "password_check_interval", "api_token_timeout", "key_id", ""} {
		require.False(t, IsCredentialKeyName(name), name)
	}
}

func TestURLPassword(t *testing.T) {
	t.Parallel()
	keepEnv := func(p string) bool { return p == "${DB_PASS}" || p == Placeholder }
	cases := []struct {
		name     string
		in       string
		keep     func(string) bool
		out      string
		password string
		ok       bool
	}{
		{"plain", "https://u:FAKE-pass-0001@h.invalid/x", nil, "https://u:" + Placeholder + "@h.invalid/x", "FAKE-pass-0001", true},
		{"port and query", "mysql://u:p@tcp(host:3306)/db?x=1", nil, "mysql://u:" + Placeholder + "@tcp(host:3306)/db?x=1", "p", true},
		{"colon in password", "https://u:p:q@h", nil, "https://u:" + Placeholder + "@h", "p:q", true},
		{"env reference kept", "https://u:${DB_PASS}@h", keepEnv, "", "", false},
		{"placeholder kept", "https://u:" + Placeholder + "@h", keepEnv, "", "", false},
		{"no password", "https://user@host/", nil, "", "", false},
		{"empty password", "https://user:@host/", nil, "", "", false},
		{"at sign in path", "https://host/path@x:y", nil, "", "", false},
		{"no scheme", "no scheme", nil, "", "", false},
		{"bad scheme", "1https://u:p@h", nil, "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, password, ok := URLPassword(tc.in, Placeholder, tc.keep)

			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.password, password)
			if ok {
				require.Equal(t, tc.out, out)
			} else {
				require.Equal(t, tc.in, out)
			}
		})
	}
}
