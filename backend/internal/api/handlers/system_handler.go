package handlers

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
)

type SystemHandler struct{}

func NewSystemHandler() *SystemHandler {
	return &SystemHandler{}
}

type MyIPResponse struct {
	IP     string `json:"ip"`
	Source string `json:"source"`
}

// GetMyIP returns the client's IP address as resolved by the server.
// Forwarded-address headers are honored only through the engine's trusted-proxy
// configuration (c.ClientIP), never read directly from the request.
func (h *SystemHandler) GetMyIP(c *gin.Context) {
	ip := c.ClientIP()

	source := "direct"
	if _, viaProxy := middleware.RequestOriginFrom(c); viaProxy {
		source = "forwarded"
	} else if peer, _, err := net.SplitHostPort(c.Request.RemoteAddr); err != nil || peer != ip {
		source = "forwarded"
	}

	c.JSON(http.StatusOK, MyIPResponse{
		IP:     ip,
		Source: source,
	})
}
