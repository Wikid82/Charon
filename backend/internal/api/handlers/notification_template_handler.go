package handlers

import (
	"net/http"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type NotificationTemplateHandler struct {
	service         *services.NotificationService
	securityService *services.SecurityService
	dataRoot        string
}

func NewNotificationTemplateHandler(s *services.NotificationService) *NotificationTemplateHandler {
	return NewNotificationTemplateHandlerWithDeps(s, nil, "")
}

func NewNotificationTemplateHandlerWithDeps(s *services.NotificationService, securityService *services.SecurityService, dataRoot string) *NotificationTemplateHandler {
	return &NotificationTemplateHandler{service: s, securityService: securityService, dataRoot: dataRoot}
}

func (h *NotificationTemplateHandler) List(c *gin.Context) {
	list, err := h.service.ListTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list templates"})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *NotificationTemplateHandler) Create(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var t models.NotificationTemplate
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.CreateTemplate(&t); err != nil {
		if respondPermissionError(c, h.securityService, "notification_template_save_failed", err, h.dataRoot) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create template"})
		return
	}
	c.JSON(http.StatusCreated, t)
}

func (h *NotificationTemplateHandler) Update(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	id := c.Param("id")
	var t models.NotificationTemplate
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	t.ID = id
	if err := h.service.UpdateTemplate(&t); err != nil {
		if respondPermissionError(c, h.securityService, "notification_template_save_failed", err, h.dataRoot) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update template"})
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h *NotificationTemplateHandler) Delete(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	id := c.Param("id")
	if err := h.service.DeleteTemplate(id); err != nil {
		if respondPermissionError(c, h.securityService, "notification_template_delete_failed", err, h.dataRoot) {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete template"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// Preview allows rendering an arbitrary template (provided in request) or a stored template by id.
func (h *NotificationTemplateHandler) Preview(c *gin.Context) {
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var tmplStr string
	if id, ok := raw["template_id"].(string); ok && id != "" {
		t, err := h.service.GetTemplate(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "template not found"})
			return
		}
		tmplStr = t.Config
	} else if s, ok := raw["template"].(string); ok {
		tmplStr = s
	}

	data := map[string]any{}
	if d, ok := raw["data"].(map[string]any); ok {
		data = d
	}

	// Build a fake provider to leverage existing RenderTemplate logic
	provider := models.NotificationProvider{Template: "custom", Config: tmplStr}
	rendered, parsed, err := h.service.RenderTemplate(provider, data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "rendered": rendered})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rendered": rendered, "parsed": parsed})
}
