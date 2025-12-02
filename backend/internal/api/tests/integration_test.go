package tests

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "strings"

    "github.com/gin-gonic/gin"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"

    "github.com/Wikid82/charon/backend/internal/api/routes"
    "github.com/Wikid82/charon/backend/internal/config"
)

// TestIntegration_WAF_BlockAndMonitor exercises middleware behavior and metrics exposure.
func TestIntegration_WAF_BlockAndMonitor(t *testing.T) {
    gin.SetMode(gin.TestMode)

    // Helper to spin server with given WAF mode
    newServer := func(mode string) (*gin.Engine, *gorm.DB) {
        db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
        if err != nil {
            t.Fatalf("db open: %v", err)
        }
        cfg, err := config.Load()
        if err != nil {
            t.Fatalf("load cfg: %v", err)
        }
        cfg.Security.WAFMode = mode
        r := gin.New()
        if err := routes.Register(r, db, cfg); err != nil {
            t.Fatalf("register: %v", err)
        }
        return r, db
    }

    // Block mode should reject suspicious payload on an API route covered by middleware
    rBlock, _ := newServer("block")
    req := httptest.NewRequest(http.MethodGet, "/api/v1/remote-servers?test=<script>", nil)
    w := httptest.NewRecorder()
    rBlock.ServeHTTP(w, req)
    if w.Code == http.StatusOK {
        t.Fatalf("expected block in block mode, got 200: body=%s", w.Body.String())
    }

    // Monitor mode should allow request but still evaluate (log-only)
    rMon, _ := newServer("monitor")
    req2 := httptest.NewRequest(http.MethodGet, "/api/v1/remote-servers?test=<script>", nil)
    w2 := httptest.NewRecorder()
    rMon.ServeHTTP(w2, req2)
        if w2.Code != http.StatusOK {
            t.Fatalf("unexpected status in monitor mode: %d", w2.Code)
        }

    // Metrics should be exposed
    reqM := httptest.NewRequest(http.MethodGet, "/metrics", nil)
    wM := httptest.NewRecorder()
    rMon.ServeHTTP(wM, reqM)
    if wM.Code != http.StatusOK {
        t.Fatalf("metrics not served: %d", wM.Code)
    }
    body := wM.Body.String()
    required := []string{"charon_waf_requests_total", "charon_waf_blocked_total", "charon_waf_monitored_total"}
    for _, k := range required {
        if !strings.Contains(body, k) {
            t.Fatalf("missing metric %s in /metrics output", k)
        }
    }
}
