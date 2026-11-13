package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAllowedWebSocketOrigin_PublicOrigin(t *testing.T) {
	const public = "https://charon.example.com"
	tests := []struct {
		name   string
		origin string
		host   string
		public string
		want   bool
	}{
		{"public origin allowed when Host is the inner address", public, "charon.internal:8080", public, true},
		{"public origin match ignores host case", "https://CHARON.example.com", "charon.internal:8080", public, true},
		{"public origin scheme must match", "http://charon.example.com", "charon.internal:8080", public, false},
		{"other origin refused even with public origin set", "https://evil.example", "charon.internal:8080", public, false},
		{"public origin refused when none is configured", public, "charon.internal:8080", "", false},
		{"host match still allowed with public origin set", "http://charon.internal:8080", "charon.internal:8080", public, true},
		{"no Origin header allowed", "", "charon.internal:8080", public, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			assert.Equal(t, tc.want, isAllowedWebSocketOrigin(req, tc.public))
		})
	}
}
