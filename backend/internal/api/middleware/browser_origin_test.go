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

func TestRequireSameOriginForBrowserAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const host = "charon.test"

	cases := []struct {
		name          string
		method        string
		authorization string
		secFetchSite  string
		origin        string
		wantStatus    int
	}{
		{name: "same-origin browser request", method: http.MethodPost, secFetchSite: "same-origin", wantStatus: http.StatusOK},
		{name: "user-initiated request", method: http.MethodPost, secFetchSite: "none", wantStatus: http.StatusOK},
		{
			name: "same-site sibling subdomain", method: http.MethodPost, secFetchSite: "same-site",
			origin: "https://app.charon.test", wantStatus: http.StatusForbidden,
		},
		{
			name: "cross-site request", method: http.MethodPost, secFetchSite: "cross-site",
			origin: "https://evil.example", wantStatus: http.StatusForbidden,
		},
		{name: "origin matches host without fetch metadata", method: http.MethodPost, origin: "https://charon.test", wantStatus: http.StatusOK},
		{name: "origin of a sibling host without fetch metadata", method: http.MethodPost, origin: "https://app.charon.test", wantStatus: http.StatusForbidden},
		{name: "non-browser client without either header", method: http.MethodPost, wantStatus: http.StatusOK},
		{
			name: "header-authenticated cross-site request", method: http.MethodPost, authorization: "Bearer x",
			secFetchSite: "cross-site", origin: "https://evil.example", wantStatus: http.StatusOK,
		},
		{name: "cross-site safe method", method: http.MethodGet, secFetchSite: "cross-site", wantStatus: http.StatusOK},
		{
			name: "cross-site delete", method: http.MethodDelete, secFetchSite: "cross-site",
			origin: "https://evil.example", wantStatus: http.StatusForbidden,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(RequireSameOriginForBrowserAuth())
			r.Handle(tc.method, "/security/geoip/database", func(c *gin.Context) { c.Status(http.StatusOK) })

			req := httptest.NewRequest(tc.method, "/security/geoip/database", http.NoBody)
			req.Host = host
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			if tc.secFetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tc.secFetchSite)
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
			if tc.wantStatus == http.StatusForbidden {
				var body map[string]string
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.Equal(t, "cross_origin_refused", body["error_code"])
				assert.Equal(t, "Cross-origin request refused", body["error"])
			}
		})
	}
}
