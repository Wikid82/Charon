package handlers

import (
	"net/http"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	service *services.NotificationService
}

func NewNotificationHandler(service *services.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: service}
}

func (h *NotificationHandler) List(c *gin.Context) {
	unreadOnly := c.Query("unread") == "true"
	notifications, err := h.service.List(unreadOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list notifications"})
		return
	}
	c.JSON(http.StatusOK, notifications)
}

func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.MarkAsRead(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as read"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Notification marked as read"})
}

func (h *NotificationHandler) MarkAllAsRead(c *gin.Context) {
	if err := h.service.MarkAllAsRead(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark all notifications as read"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "All notifications marked as read"})
}
