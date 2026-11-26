package redact

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// fixtureValue is an obviously fake credential used by the known-value tests.
const fixtureValue = "FAKE-value-0001"

func TestTextPatternPass(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"api_key=abcdef123":                         "api_key=" + Placeholder,
		`"password":"abcdef123"`:                    `"password":"` + Placeholder + `"`,
		"password: abcdef123":                       "password: " + Placeholder,
		"Bearer abcdef123456":                       "Bearer " + Placeholder,
		"authorization: Basic dXNlcjpwYXNzd29yZA==": "authorization: Basic " + Placeholder,
		"dsn=postgres-secret-x":                     "dsn=" + Placeholder,
		"https://u:abcdef123@host/x":                "https://u:" + Placeholder + "@host/x",
	}
	for in, want := range cases {
		require.Equal(t, want, Text(in, nil), in)
	}
}

func TestTextKeepsBenignKeysAndShortValues(t *testing.T) {
	t.Parallel()
	cases := []string{
		"token_path=/etc/creds/token.txt",
		"tokens_per_minute: 500000",
		"key_id: abcdef123456",
		"password_policy: strictmode",
		"token_expiry_seconds: 3600",
		`"password": "on"`,
		"api_key=1",
		"Bearer abc",
	}
	for _, in := range cases {
		require.Equal(t, in, Text(in, nil), in)
	}
}

func TestTextKnownValueForms(t *testing.T) {
	t.Parallel()
	// The value appears raw, JSON-escaped (quote and backslash) and Go-quoted in the same message.
	value := `FAKE"quote\slash-01`
	msg := `raw: ` + value + ` json: "FAKE\"quote\\slash-01" quoted: "FAKE\"quote\\slash-01"`
	out := Text(msg, []string{value})

	require.NotContains(t, out, "slash-01")
	require.Equal(t, 3, strings.Count(out, Placeholder))
}

func TestTextKnownValueHTMLEscaped(t *testing.T) {
	t.Parallel()
	value := "FAKE<tag>&value-02"
	msg := `{"error":"token FAKE\u003ctag\u003e\u0026value-02 rejected"}`
	out := Text(msg, []string{value})
	require.NotContains(t, out, "value-02")
	require.Contains(t, out, Placeholder)
}

func TestTextLongerKnownValueWins(t *testing.T) {
	t.Parallel()
	out := Text("token FAKE-value-0001-extra rejected", []string{"FAKE-value-0001", "FAKE-value-0001-extra"})
	require.Equal(t, "token "+Placeholder+" rejected", out)
}

func TestTextIgnoresShortKnownValues(t *testing.T) {
	t.Parallel()
	out := Text("the value true is on", []string{"true", "on"})
	require.Equal(t, "the value true is on", out)
}

func TestTextReplacesKnownValueWithoutKeyword(t *testing.T) {
	t.Parallel()
	out := Text("bad: "+fixtureValue, []string{fixtureValue})
	require.Equal(t, "bad: "+Placeholder, out)
}

func TestTextIsIdempotent(t *testing.T) {
	t.Parallel()
	once := Text("api_key=abcdef123 and Bearer abcdef123456", nil)
	require.Equal(t, once, Text(once, nil))
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	require.Equal(t, "short", Truncate("short", 10))
	require.Equal(t, "abcd [truncated]", Truncate("abcdefgh", 4))
	require.Equal(t, "", Truncate("", 0))
}

func TestTruncateKeepsRuneBoundary(t *testing.T) {
	t.Parallel()
	// "é" is two bytes; a cut at byte 1 must back off to the start of the rune.
	out := Truncate("éé", 1)
	require.True(t, utf8.ValidString(out))
	require.Equal(t, " [truncated]", out[len(out)-len(" [truncated]"):])
	require.Equal(t, "", strings.TrimSuffix(out, truncateMarker))
}
