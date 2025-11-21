package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCertificateHandler_List(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()
	caddyDir := filepath.Join(tmpDir, "caddy", "certificates", "acme-v02.api.letsencrypt.org-directory")
	err := os.MkdirAll(caddyDir, 0755)
	require.NoError(t, err)

	service := services.NewCertificateService(tmpDir)
	handler := NewCertificateHandler(service)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/certificates", handler.List)

	req, _ := http.NewRequest("GET", "/certificates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var certs []services.CertificateInfo
	err = json.Unmarshal(w.Body.Bytes(), &certs)
	assert.NoError(t, err)
	assert.Empty(t, certs)
}
