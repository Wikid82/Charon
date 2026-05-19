// Package tests contains integration tests for the API.
package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/routes"
	"github.com/Wikid82/charon/backend/internal/config"
)

// TestIntegration_WAF_BlockAndMonitor exercises middleware behavior and metrics exposure.
// Note: Actual WAF blocking is handled by Coraza at the Caddy layer, not by the API middleware.
// The cerberus middleware only tracks metrics and handles ACL enforcement.
func TestIntegration_WAF_BlockAndMonitor(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Helper to spin server with given WAF mode
	newServer := func(mode string) *gin.Engine {
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
		if err := routes.Register(context.Background(), r, db, cfg); err != nil {
			t.Fatalf("register: %v", err)
		}
		return r
	}

	// Block mode: cerberus middleware doesn't block requests - that's Coraza's job at the Caddy layer
	// The API middleware only tracks metrics when WAF is enabled
	rBlock := newServer("block")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/remote-servers?test=<script>", http.NoBody)
	w := httptest.NewRecorder()
	rBlock.ServeHTTP(w, req)
	// Request passes through API layer - actual WAF blocking happens at Caddy/Coraza
	// We just verify the middleware doesn't crash and allows the request through to auth check
	// (returns 401 since no auth token is provided)
	if w.Code != http.StatusUnauthorized && w.Code != http.StatusOK {
		t.Fatalf("unexpected status in block mode: %d, expected 401 (auth required)", w.Code)
	}

	// Monitor mode should allow request but still evaluate (log-only)
	rMon := newServer("monitor")
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/remote-servers?test=<script>", http.NoBody)
	w2 := httptest.NewRecorder()
	rMon.ServeHTTP(w2, req2)
	// Same behavior - request passes through to auth check
	if w2.Code != http.StatusUnauthorized && w2.Code != http.StatusOK {
		t.Fatalf("unexpected status in monitor mode: %d, expected 401 (auth required)", w2.Code)
	}

	// Metrics should be exposed
	reqM := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
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
