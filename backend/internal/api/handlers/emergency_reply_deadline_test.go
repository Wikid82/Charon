package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmergencyResetReplyBudget(t *testing.T) {
	assert.Equal(t, 75*time.Second, EmergencyResetReplyBudget)
}

// TestExtendEmergencyReplyDeadline_ReplyOutlivesServerWriteTimeout: a server with a 100 ms write timeout answers 200
// after 300 ms only when the handler extended its own deadline.
func TestExtendEmergencyReplyDeadline_ReplyOutlivesServerWriteTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/reset", func(c *gin.Context) {
		extendEmergencyReplyDeadline(c)
		time.Sleep(300 * time.Millisecond)
		c.Status(http.StatusOK)
	})

	srv := httptest.NewUnstartedServer(router)
	srv.Config.WriteTimeout = 100 * time.Millisecond
	srv.Start()
	t.Cleanup(srv.Close)

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(srv.URL + "/reset") //nolint:noctx // test-only request against httptest server
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestExtendEmergencyReplyDeadline_RecorderWithoutDeadlineSupport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.NotPanics(t, func() { extendEmergencyReplyDeadline(c) })
}
