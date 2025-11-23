package handlers

import (
	"net/http"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type NotificationProviderHandler struct {
	service *services.NotificationService
}

func NewNotificationProviderHandler(service *services.NotificationService) *NotificationProviderHandler {
	return &NotificationProviderHandler{service: service}
}

func (h *NotificationProviderHandler) List(c *gin.Context) {
	providers, err := h.service.ListProviders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list providers"})
		return
	}
	c.JSON(http.StatusOK, providers)
}

func (h *NotificationProviderHandler) Create(c *gin.Context) {
	var provider models.NotificationProvider
	if err := c.ShouldBindJSON(&provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.CreateProvider(&provider); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create provider"})
		return
	}
	c.JSON(http.StatusCreated, provider)
}

func (h *NotificationProviderHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var provider models.NotificationProvider
	if err := c.ShouldBindJSON(&provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	provider.ID = id

	if err := h.service.UpdateProvider(&provider); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update provider"})
		return
	}
	c.JSON(http.StatusOK, provider)
}

func (h *NotificationProviderHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteProvider(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete provider"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Provider deleted"})
}

func (h *NotificationProviderHandler) Test(c *gin.Context) {
	var provider models.NotificationProvider
	if err := c.ShouldBindJSON(&provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.TestProvider(provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Test notification sent"})
}
