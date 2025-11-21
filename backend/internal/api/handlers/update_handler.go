package handlers

import (
	"net/http"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type UpdateHandler struct {
	service *services.UpdateService
}

func NewUpdateHandler(service *services.UpdateService) *UpdateHandler {
	return &UpdateHandler{service: service}
}

func (h *UpdateHandler) Check(c *gin.Context) {
	info, err := h.service.CheckForUpdates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check for updates"})
		return
	}
	c.JSON(http.StatusOK, info)
}
