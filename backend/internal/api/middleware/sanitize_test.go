package middleware

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeHeaders(t *testing.T) {
	t.Run("nil headers", func(t *testing.T) {
		require.Nil(t, SanitizeHeaders(nil))
	})

	t.Run("redacts sensitive headers", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("Authorization", "secret")
		headers.Set("X-Api-Key", "token")
		headers.Set("Cookie", "sessionid=abc")

		sanitized := SanitizeHeaders(headers)

		require.Equal(t, []string{"<redacted>"}, sanitized["Authorization"])
		require.Equal(t, []string{"<redacted>"}, sanitized["X-Api-Key"])
		require.Equal(t, []string{"<redacted>"}, sanitized["Cookie"])
	})

	t.Run("sanitizes and truncates values", func(t *testing.T) {
		headers := http.Header{}
		headers.Add("X-Trace", "line1\nline2\r\t")
		headers.Add("X-Custom", strings.Repeat("a", 210))

		sanitized := SanitizeHeaders(headers)

		traceValue := sanitized["X-Trace"][0]
		require.NotContains(t, traceValue, "\n")
		require.NotContains(t, traceValue, "\r")
		require.NotContains(t, traceValue, "\t")

		customValue := sanitized["X-Custom"][0]
		require.Equal(t, 200, len(customValue))
		require.True(t, strings.HasPrefix(customValue, strings.Repeat("a", 200)))
	})
}

func TestSanitizePath(t *testing.T) {
	paddedPath := "/api/v1/resource/" + strings.Repeat("x", 210) + "?token=secret"

	sanitized := SanitizePath(paddedPath)

	require.NotContains(t, sanitized, "?")
	require.False(t, strings.ContainsAny(sanitized, "\n\r\t"))
	require.Equal(t, 200, len(sanitized))
}
