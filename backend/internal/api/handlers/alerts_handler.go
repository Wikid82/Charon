package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/alerts"
)

// AlertsHandler serves the active alerts shown by the notification bell.
type AlertsHandler struct {
	svc *alerts.Service
}

// NewAlertsHandler returns a handler that lists the active alerts of svc.
func NewAlertsHandler(svc *alerts.Service) *AlertsHandler {
	return &AlertsHandler{svc: svc}
}

// List returns the active alerts computed at request time. It always answers 200.
func (h *AlertsHandler) List(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"alerts": h.svc.Active(c.Request.Context())})
}
