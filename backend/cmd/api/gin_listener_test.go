package main

import (
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGin_RunListenerServesOnABoundListener pins the gin API main relies on
// to bind the listener explicitly before handing it to the engine.
func TestGin_RunListenerServesOnABoundListener(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	served := make(chan error, 1)
	go func() { served <- engine.RunListener(ln) }()

	resp, err := http.Get("http://" + ln.Addr().String() + "/ping") //nolint:gosec,noctx // loopback test server
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, "pong", string(body))

	require.NoError(t, ln.Close())
	select {
	case <-served:
	case <-time.After(5 * time.Second):
		t.Fatal("RunListener did not return after the listener was closed")
	}
}
