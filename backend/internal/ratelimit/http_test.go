package ratelimit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReject_SetsRetryAfterAndGenericBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	nextCalled := false
	r := gin.New()
	r.POST("/login", func(c *gin.Context) {
		Reject(c, Decision{Allowed: false, RetryAfter: 59*time.Second + 999*time.Millisecond + 700*time.Microsecond})
	}, func(_ *gin.Context) { nextCalled = true })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login?user=probe-secret", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "60", w.Header().Get("Retry-After"))
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]string{"error": TooManyRequestsMessage}, body)
	assert.NotContains(t, w.Body.String(), "probe-secret")
	assert.False(t, nextCalled, "Reject must abort the chain")
}
