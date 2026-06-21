package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNewRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a dummy frontend dir
	tempDir := t.TempDir()
	// #nosec G306 -- Test fixture HTML file needs to be world-readable for HTTP serving test
	err := os.WriteFile(filepath.Join(tempDir, "index.html"), []byte("<html></html>"), 0o644)
	assert.NoError(t, err)

	router := NewRouter(tempDir, "")
	assert.NotNil(t, router)

	// Test static file serving
	req, _ := http.NewRequest("GET", "/", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "<html></html>")

	// Test /api NoRoute special-case: do not serve SPA HTML
	apiReq, _ := http.NewRequest("GET", "/api/this-route-does-not-exist", http.NoBody)
	apiW := httptest.NewRecorder()
	router.ServeHTTP(apiW, apiReq)
	assert.Equal(t, http.StatusNotFound, apiW.Code)
	assert.NotContains(t, apiW.Body.String(), "<html></html>")
	assert.Contains(t, apiW.Body.String(), "not found")

	// Test WebP/SVG static routes return 200 when the file exists
	for _, asset := range []struct{ route, file string }{
		{"/banner.webp", "banner.webp"},
		{"/banner.svg", "banner.svg"},
		{"/logo.webp", "logo.webp"},
		{"/logo.svg", "logo.svg"},
	} {
		// #nosec G306 -- Test fixture needs to be world-readable for HTTP serving test
		_ = os.WriteFile(filepath.Join(tempDir, asset.file), []byte("data"), 0o644)
		r, _ := http.NewRequest("GET", asset.route, http.NoBody)
		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, r)
		assert.Equal(t, http.StatusOK, rw.Code, "route %s should return 200", asset.route)
	}
}
