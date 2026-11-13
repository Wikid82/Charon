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
			r.Use(RequireSameOriginForBrowserAuth(NewBrowserOriginGuard(nil)))
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

// originSetting is a test TrustedOriginSource whose configured value can change between refreshes.
type originSetting struct {
	origin string
	ok     bool
}

func (s *originSetting) source() (string, bool) { return s.origin, s.ok }

// serveOriginGuard runs one request through the guard and returns the status code.
func serveOriginGuard(t *testing.T, guard *BrowserOriginGuard, method, host string, headers map[string]string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequireSameOriginForBrowserAuth(guard))
	r.Handle(method, "/api/v1/security/geoip/database", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(method, "/api/v1/security/geoip/database", http.NoBody)
	req.Host = host
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestBrowserOriginGuard_TrustsConfiguredPublicOrigin(t *testing.T) {
	const publicOrigin = "https://charon.example.com"
	guard := NewBrowserOriginGuard((&originSetting{origin: publicOrigin, ok: true}).source)

	cases := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{
			name:    "public origin without fetch metadata passes when Host differs",
			headers: map[string]string{"Origin": publicOrigin},
			want:    http.StatusOK,
		},
		{
			name:    "public origin cross-site passes",
			headers: map[string]string{"Origin": publicOrigin, "Sec-Fetch-Site": "cross-site"},
			want:    http.StatusOK,
		},
		{
			name:    "other origin is still refused",
			headers: map[string]string{"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"},
			want:    http.StatusForbidden,
		},
		{
			name:    "same-site sibling of the public origin is still refused",
			headers: map[string]string{"Origin": "https://app.charon.example.com", "Sec-Fetch-Site": "same-site"},
			want:    http.StatusForbidden,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, serveOriginGuard(t, guard, http.MethodPost, "charon.internal:8080", tc.headers))
		})
	}
}

func TestBrowserOriginGuard_NoConfiguredOriginTrustsOnlyHostMatch(t *testing.T) {
	guard := NewBrowserOriginGuard(nil)

	assert.Equal(t, http.StatusForbidden,
		serveOriginGuard(t, guard, http.MethodPost, "charon.internal:8080", map[string]string{"Origin": "https://charon.example.com"}))
	assert.Equal(t, http.StatusOK,
		serveOriginGuard(t, guard, http.MethodPost, "charon.internal:8080", map[string]string{"Origin": "http://charon.internal:8080"}))
}

func TestBrowserOriginGuard_RefreshFollowsSettingChanges(t *testing.T) {
	setting := &originSetting{origin: "https://old.example.com", ok: true}
	guard := NewBrowserOriginGuard(setting.source)
	oldHeaders := map[string]string{"Origin": "https://old.example.com"}
	newHeaders := map[string]string{"Origin": "https://new.example.com"}

	require.Equal(t, http.StatusOK, serveOriginGuard(t, guard, http.MethodPost, "inner.test", oldHeaders))

	setting.origin = "https://new.example.com"
	require.NoError(t, guard.Refresh())
	assert.Equal(t, http.StatusForbidden, serveOriginGuard(t, guard, http.MethodPost, "inner.test", oldHeaders))
	assert.Equal(t, http.StatusOK, serveOriginGuard(t, guard, http.MethodPost, "inner.test", newHeaders))

	setting.ok = false
	require.NoError(t, guard.Refresh())
	assert.Equal(t, http.StatusForbidden, serveOriginGuard(t, guard, http.MethodPost, "inner.test", newHeaders))
}

func TestBrowserOriginGuard_RefreshFailsClosed(t *testing.T) {
	setting := &originSetting{origin: "https://old.example.com", ok: true}
	guard := NewBrowserOriginGuard(setting.source)
	oldHeaders := map[string]string{"Origin": "https://old.example.com"}
	require.Equal(t, http.StatusOK, serveOriginGuard(t, guard, http.MethodPost, "inner.test", oldHeaders))

	// A value the standard library rejects (no scheme) must drop the previous trust rather than keep it.
	setting.origin = "charon.example.com"
	err := guard.Refresh()
	require.Error(t, err)
	assert.ErrorContains(t, err, "trust configured public origin")
	assert.Equal(t, http.StatusForbidden, serveOriginGuard(t, guard, http.MethodPost, "inner.test", oldHeaders))
}

func TestBrowserOriginGuard_ConcurrentRefreshAndRequests(t *testing.T) {
	setting := &originSetting{origin: "https://a.example.com", ok: true}
	guard := NewBrowserOriginGuard(setting.source)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			setting.origin = "https://a.example.com"
			if i%2 == 1 {
				setting.origin = "https://b.example.com"
			}
			_ = guard.Refresh()
		}
	}()
	for i := 0; i < 50; i++ {
		_ = serveOriginGuard(t, guard, http.MethodPost, "inner.test", map[string]string{"Origin": "https://a.example.com"})
	}
	<-done
}

// A WebSocket handshake is a GET, which the standard library never checks. A cookie session must still not open a
// socket from another origin, so the upgrade is checked as an unsafe method.
func TestBrowserOriginGuard_WebSocketUpgradeIsChecked(t *testing.T) {
	guard := NewBrowserOriginGuard(nil)
	upgrade := map[string]string{"Connection": "Upgrade", "Upgrade": "websocket"}
	with := func(extra map[string]string) map[string]string {
		m := map[string]string{}
		for k, v := range upgrade {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}

	cases := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{
			name:    "cross-site upgrade is refused",
			headers: with(map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}),
			want:    http.StatusForbidden,
		},
		{
			name:    "same-site sibling upgrade is refused",
			headers: with(map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "https://app.charon.test"}),
			want:    http.StatusForbidden,
		},
		{
			name:    "same-origin upgrade passes",
			headers: with(map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://charon.internal:8080"}),
			want:    http.StatusOK,
		},
		{
			name:    "header-authenticated cross-site upgrade passes",
			headers: with(map[string]string{"Authorization": "Bearer x", "Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}),
			want:    http.StatusOK,
		},
		{
			name:    "non-browser upgrade without fetch metadata passes",
			headers: with(nil),
			want:    http.StatusOK,
		},
		{
			name:    "cross-site request that is not an upgrade stays a safe GET",
			headers: map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example", "Connection": "keep-alive"},
			want:    http.StatusOK,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, serveOriginGuard(t, guard, http.MethodGet, "charon.internal:8080", tc.headers))
		})
	}
}
