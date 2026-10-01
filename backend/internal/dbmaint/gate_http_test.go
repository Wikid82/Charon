package dbmaint

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { gin.SetMode(gin.TestMode) }

func testHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "charon"})
}

// gateInPhase returns a gate forced into phase.
func gateInPhase(phase Phase) *Gate {
	g := NewGate()
	g.phase = phase
	return g
}

// newGatedRouter mounts the gate middleware first, like main.go, in front of a
// normal route, a health route and a NoRoute handler that stands for the SPA.
func newGatedRouter(g *Gate) *gin.Engine {
	r := gin.New()
	r.RedirectTrailingSlash = false // the downstream health route must not turn "/health/" into a redirect
	r.NoRoute(func(c *gin.Context) { c.String(http.StatusOK, "spa-index") })
	r.Use(g.Middleware(testHealth))
	r.GET(HealthPath, func(c *gin.Context) { c.String(http.StatusOK, downstreamHealthBody) })
	r.HEAD(HealthPath, func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v1/auth/me", func(c *gin.Context) { c.String(http.StatusOK, "me") })
	r.GET("/api/v1/health/db", func(c *gin.Context) { c.String(http.StatusOK, "db") })
	return r
}

func do(r http.Handler, method, path string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, http.NoBody)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

var allPhases = []Phase{PhaseIdle, PhasePlanned, PhaseChecking, PhaseConverting, PhaseDone, PhaseSkipped, PhaseFailed}

func TestMiddleware_StatusIsAnsweredInEveryPhase(t *testing.T) {
	for _, phase := range allPhases {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(string(phase)+"_"+method, func(t *testing.T) {
				g := gateInPhase(phase)
				w := do(newGatedRouter(g), method, StatusPath)

				require.Equal(t, http.StatusOK, w.Code)
				assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
				assertCommonHeaders(t, w)
				if method == http.MethodHead {
					assert.Empty(t, w.Body.String())
					return
				}
				var body struct {
					Active         bool   `json:"active"`
					Phase          string `json:"phase"`
					ElapsedSeconds int64  `json:"elapsed_seconds"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
				assert.Equal(t, string(phase), body.Phase)
				assert.Equal(t, phase.active(), body.Active)
				assert.NotContains(t, w.Body.String(), "spa-index")
			})
		}
	}
}

func TestMiddleware_StatusKeysAreExactlyTheUnauthenticatedMinimum(t *testing.T) {
	w := do(newGatedRouter(gateInPhase(PhaseConverting)), http.MethodGet, StatusPath)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	assert.Len(t, raw, 3)
	for _, k := range []string{"active", "phase", "elapsed_seconds"} {
		assert.Contains(t, raw, k)
	}
}

// downstreamHealthBody marks a response that came from the normal chain
// (standing for the rate-limited router health route), not from the gate.
const downstreamHealthBody = "downstream-health"

func TestMiddleware_HealthPassesThroughWhenInactive(t *testing.T) {
	for _, phase := range []Phase{PhaseIdle, PhasePlanned, PhaseDone, PhaseSkipped, PhaseFailed} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(string(phase)+"_"+method, func(t *testing.T) {
				w := do(newGatedRouter(gateInPhase(phase)), method, HealthPath)
				require.Equal(t, http.StatusOK, w.Code)
				assert.Empty(t, w.Header().Get("Content-Security-Policy"), "the gate must not write the response")
				assert.Empty(t, w.Header().Get("Cache-Control"))
				if method == http.MethodGet {
					assert.Equal(t, downstreamHealthBody, w.Body.String())
				}
			})
		}
	}
}

func TestMiddleware_HealthIsAnsweredByTheGateWhileActive(t *testing.T) {
	for _, phase := range []Phase{PhaseChecking, PhaseConverting} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(string(phase)+"_"+method, func(t *testing.T) {
				w := do(newGatedRouter(gateInPhase(phase)), method, HealthPath)
				require.Equal(t, http.StatusOK, w.Code)
				assertCommonHeaders(t, w)
				if method == http.MethodGet {
					assert.JSONEq(t, `{"status":"ok","service":"charon"}`, w.Body.String())
				}
			})
		}
	}
}

func TestMiddleware_HeadAndGetHealthAgreeInIdleAndActivePhases(t *testing.T) {
	for _, phase := range []Phase{PhaseIdle, PhaseConverting} {
		r := newGatedRouter(gateInPhase(phase))
		assert.Equal(t, do(r, http.MethodGet, HealthPath).Code, do(r, http.MethodHead, HealthPath).Code, string(phase))
	}
}

func TestMiddleware_InactivePhasesPassEverythingElseThrough(t *testing.T) {
	for _, phase := range []Phase{PhaseIdle, PhasePlanned, PhaseDone, PhaseSkipped, PhaseFailed} {
		t.Run(string(phase), func(t *testing.T) {
			r := newGatedRouter(gateInPhase(phase))
			assert.Equal(t, "me", do(r, http.MethodGet, "/api/v1/auth/me").Body.String())
			assert.Equal(t, "db", do(r, http.MethodGet, "/api/v1/health/db").Body.String())
			assert.Equal(t, "spa-index", do(r, http.MethodGet, "/settings/system").Body.String())
		})
	}
}

func TestMiddleware_ActivePhasesBlockAPIWithJSON503(t *testing.T) {
	for _, phase := range []Phase{PhaseChecking, PhaseConverting} {
		for _, path := range []string{"/api/v1/auth/me", "/api/v1/health/db", "/api/v1/nope", "/api"} {
			t.Run(string(phase)+path, func(t *testing.T) {
				w := do(newGatedRouter(gateInPhase(phase)), http.MethodGet, path)

				require.Equal(t, http.StatusServiceUnavailable, w.Code)
				assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
				assert.Equal(t, "15", w.Header().Get("Retry-After"))
				assertCommonHeaders(t, w)
				assert.JSONEq(t,
					`{"error":"Database optimization in progress","maintenance":true,"retry_after_seconds":15}`,
					w.Body.String())
			})
		}
	}
}

func TestMiddleware_ActivePhaseServesHTMLPageToBrowsers(t *testing.T) {
	w := do(newGatedRouter(gateInPhase(PhaseConverting)), http.MethodGet, "/settings/system",
		"Accept", "text/html,application/xhtml+xml")

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Equal(t, "15", w.Header().Get("Retry-After"))
	assertCommonHeaders(t, w)
	assert.Contains(t, w.Body.String(), "Optimizing the database")
	assert.Contains(t, w.Body.String(), "Your proxies are still running")
}

func TestMiddleware_AcceptJSONSelectsJSONOutsideAPI(t *testing.T) {
	w := do(newGatedRouter(gateInPhase(PhaseChecking)), http.MethodGet, "/settings/system", "Accept", "application/json")
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestMiddleware_HeadGetsStatusAndHeadersWithoutBody(t *testing.T) {
	r := newGatedRouter(gateInPhase(PhaseConverting))
	for _, path := range []string{"/settings/system", "/api/v1/auth/me"} {
		w := do(r, http.MethodHead, path)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code, path)
		assert.Equal(t, "15", w.Header().Get("Retry-After"), path)
		assert.Empty(t, w.Body.String(), path)
	}
}

func TestMiddleware_OnlyExactStatusAndHealthPathsAreAnswered(t *testing.T) {
	r := newGatedRouter(gateInPhase(PhaseConverting))
	for _, path := range []string{
		"/api/v1/health/", "/api/v1/health/db", "/api/v1/healthz", "/api/v1/maintenance/status/x",
		"/api/v1/maintenance/statusx", "/api/v1/health?x=1/../db",
	} {
		w := do(r, http.MethodGet, path)
		if strings.HasPrefix(path, "/api/v1/health?") {
			assert.Equal(t, http.StatusOK, w.Code, "the query string is not part of the path")
			continue
		}
		assert.Equal(t, http.StatusServiceUnavailable, w.Code, path)
	}
}

func TestMiddleware_PostToHealthIsBlockedWhileActive(t *testing.T) {
	w := do(newGatedRouter(gateInPhase(PhaseConverting)), http.MethodPost, HealthPath)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestMiddleware_NilGateIsAPassThrough(t *testing.T) {
	var g *Gate
	r := gin.New()
	r.Use(g.Middleware(testHealth))
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "x") })
	assert.Equal(t, "x", do(r, http.MethodGet, "/x").Body.String())
	assert.Equal(t, http.StatusNotFound, do(r, http.MethodGet, HealthPath).Code, "a nil gate answers nothing")
}

func TestEmergencyMiddleware(t *testing.T) {
	build := func(g *Gate) *gin.Engine {
		r := gin.New()
		r.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "alive") })
		r.Use(g.EmergencyMiddleware())
		r.POST("/emergency/security-reset", func(c *gin.Context) { c.String(http.StatusOK, "reset") })
		return r
	}

	for _, phase := range []Phase{PhaseChecking, PhaseConverting} {
		r := build(gateInPhase(phase))
		w := do(r, http.MethodPost, "/emergency/security-reset")
		require.Equal(t, http.StatusServiceUnavailable, w.Code)
		assert.Equal(t, "15", w.Header().Get("Retry-After"))
		assert.JSONEq(t, `{"error":"Database optimization in progress","maintenance":true}`, w.Body.String())
		assert.Equal(t, "alive", do(r, http.MethodGet, "/health").Body.String(), "the DB-free health stays 200")
	}

	for _, phase := range []Phase{PhaseIdle, PhasePlanned, PhaseDone, PhaseSkipped, PhaseFailed} {
		assert.Equal(t, "reset", do(build(gateInPhase(phase)), http.MethodPost, "/emergency/security-reset").Body.String())
	}
	assert.Equal(t, "reset", do(build(nil), http.MethodPost, "/emergency/security-reset").Body.String())
}

func assertCommonHeaders(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	csp := w.Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "default-src 'none'")
	assert.Contains(t, csp, "frame-ancestors 'none'")
	assert.Contains(t, csp, "connect-src 'self'")
}

// The CSP must authorize exactly the page's inline script and style by hash.
func TestMaintenancePage_CSPHashesMatchTheEmbeddedInlineCode(t *testing.T) {
	page := string(maintenancePage)
	scripts := regexp.MustCompile(`(?s)<script>(.*?)</script>`).FindAllStringSubmatch(page, -1)
	styles := regexp.MustCompile(`(?s)<style>(.*?)</style>`).FindAllStringSubmatch(page, -1)
	require.Len(t, scripts, 1, "the page has exactly one inline script")
	require.Len(t, styles, 1, "the page has exactly one inline style")
	assert.NotRegexp(t, `<script[^>]*\ssrc=`, page, "no external script")
	assert.NotRegexp(t, `(?i)\son[a-z]+=`, page, "no inline event handlers")
	assert.NotRegexp(t, `(?i)\sstyle=`, page, "no inline style attributes")

	hash := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	}
	want := "default-src 'none'; script-src " + hash(scripts[0][1]) + "; style-src " + hash(styles[0][1]) +
		"; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
	assert.Equal(t, want, maintenanceCSP)
}

func TestMaintenancePage_PollsTheStatusEndpoint(t *testing.T) {
	assert.Contains(t, string(maintenancePage), StatusPath)
	assert.Contains(t, string(maintenancePage), "5000")
}
