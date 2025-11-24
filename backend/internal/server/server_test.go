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
err := os.WriteFile(filepath.Join(tempDir, "index.html"), []byte("<html></html>"), 0644)
assert.NoError(t, err)

router := NewRouter(tempDir)
assert.NotNil(t, router)

// Test static file serving
req, _ := http.NewRequest("GET", "/", nil)
w := httptest.NewRecorder()
router.ServeHTTP(w, req)
assert.Equal(t, http.StatusOK, w.Code)
assert.Contains(t, w.Body.String(), "<html></html>")
}
