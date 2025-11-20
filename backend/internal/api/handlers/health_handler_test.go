package handlers

import (
"encoding/json"
"net/http"
"net/http/httptest"
"testing"

"github.com/gin-gonic/gin"
"github.com/stretchr/testify/assert"
)

func TestHealthHandler(t *testing.T) {
gin.SetMode(gin.TestMode)
r := gin.New()
r.GET("/health", HealthHandler)

req, _ := http.NewRequest("GET", "/health", nil)
w := httptest.NewRecorder()
r.ServeHTTP(w, req)

assert.Equal(t, http.StatusOK, w.Code)

var resp map[string]string
err := json.Unmarshal(w.Body.Bytes(), &resp)
assert.NoError(t, err)
assert.Equal(t, "ok", resp["status"])
assert.NotEmpty(t, resp["version"])
}
