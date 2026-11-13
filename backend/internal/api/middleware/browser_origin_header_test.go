package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRequireSameOriginForBrowserAuth_AuthorizationHeaderSkipsOnlyWhenItCarriesAToken proves the origin check skips a
// request only when the Authorization header yields a token by the auth middleware's rule. An empty or bearer-less
// header falls back to the session cookie there, so the cross-site request must be refused here.
func TestRequireSameOriginForBrowserAuth_AuthorizationHeaderSkipsOnlyWhenItCarriesAToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name          string
		authorization *string // nil means the header is absent
		withCookie    bool
		wantStatus    int
	}{
		{name: "absent header with cookie is refused", wantStatus: http.StatusForbidden, withCookie: true},
		{name: "empty header with cookie is refused", authorization: ptr(""), withCookie: true, wantStatus: http.StatusForbidden},
		{name: "bearer prefix without token with cookie is refused", authorization: ptr("Bearer "), withCookie: true, wantStatus: http.StatusForbidden},
		{name: "real bearer token passes", authorization: ptr("Bearer x"), wantStatus: http.StatusOK},
		{name: "real bearer token with cookie passes", authorization: ptr("Bearer x"), withCookie: true, wantStatus: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(RequireSameOriginForBrowserAuth(NewBrowserOriginGuard(nil)))
			r.POST("/security/geoip/database", func(c *gin.Context) { c.Status(http.StatusOK) })

			req := httptest.NewRequest(http.MethodPost, "/security/geoip/database", http.NoBody)
			req.Host = "charon.test"
			req.Header.Set("Sec-Fetch-Site", "cross-site")
			req.Header.Set("Origin", "https://evil.example")
			if tc.authorization != nil {
				req.Header["Authorization"] = []string{*tc.authorization}
			}
			if tc.withCookie {
				req.Header.Set("Cookie", "auth_token=session-token")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, tc.wantStatus, w.Code)
			if tc.wantStatus == http.StatusForbidden {
				var body map[string]string
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.Equal(t, "cross_origin_refused", body["error_code"])
			}
		})
	}
}

// TestBearerToken proves the shared helper follows the auth middleware's rule for the Authorization header value.
func TestBearerToken(t *testing.T) {
	cases := map[string]string{
		"":                "",
		"Bearer ":         "",
		"Bearer abc":      "abc",
		"abc":             "abc",
		"Basic abc":       "Basic abc",
		"bearer abc":      "bearer abc",
		"Bearer Bearer x": "Bearer x",
	}
	for header, want := range cases {
		assert.Equal(t, want, bearerToken(header), "header %q", header)
	}
}

func ptr(s string) *string { return &s }
