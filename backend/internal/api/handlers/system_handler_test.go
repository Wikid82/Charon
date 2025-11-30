package handlers

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
)

func TestGetClientIPHeadersAndRemoteAddr(t *testing.T) {
    // Cloudflare header should win
    req := httptest.NewRequest(http.MethodGet, "/", nil)
    req.Header.Set("CF-Connecting-IP", "5.6.7.8")
    ip := getClientIP(req)
    if ip != "5.6.7.8" {
        t.Fatalf("expected 5.6.7.8 got %s", ip)
    }

    // X-Real-IP should be preferred over RemoteAddr
    req2 := httptest.NewRequest(http.MethodGet, "/", nil)
    req2.Header.Set("X-Real-IP", "10.0.0.4")
    req2.RemoteAddr = "1.2.3.4:5678"
    ip2 := getClientIP(req2)
    if ip2 != "10.0.0.4" {
        t.Fatalf("expected 10.0.0.4 got %s", ip2)
    }

    // X-Forwarded-For returns first in list
    req3 := httptest.NewRequest(http.MethodGet, "/", nil)
    req3.Header.Set("X-Forwarded-For", "192.168.0.1, 192.168.0.2")
    ip3 := getClientIP(req3)
    if ip3 != "192.168.0.1" {
        t.Fatalf("expected 192.168.0.1 got %s", ip3)
    }

    // Fallback to remote addr port trimmed
    req4 := httptest.NewRequest(http.MethodGet, "/", nil)
    req4.RemoteAddr = "7.7.7.7:8888"
    ip4 := getClientIP(req4)
    if ip4 != "7.7.7.7" {
        t.Fatalf("expected 7.7.7.7 got %s", ip4)
    }
}

func TestGetMyIPHandler(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    handler := NewSystemHandler()
    r.GET("/myip", handler.GetMyIP)

    // With CF header
    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/myip", nil)
    req.Header.Set("CF-Connecting-IP", "5.6.7.8")
    r.ServeHTTP(w, req)
    if w.Code != http.StatusOK {
        t.Fatalf("expected 200 got %d", w.Code)
    }
}
