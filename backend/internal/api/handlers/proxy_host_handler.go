package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

// ProxyHostHandler manages CRUD operations for proxy hosts against the database.
type ProxyHostHandler struct {
	db *gorm.DB
}

// NewProxyHostHandler wires the handler with shared dependencies.
func NewProxyHostHandler(db *gorm.DB) *ProxyHostHandler {
	return &ProxyHostHandler{db: db}
}

// RegisterRoutes attaches the handler to the provided router group.
func (h *ProxyHostHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/proxy-hosts", h.List)
	rg.POST("/proxy-hosts", h.Create)
	rg.GET("/proxy-hosts/:uuid", h.Get)
	rg.PUT("/proxy-hosts/:uuid", h.Update)
	rg.DELETE("/proxy-hosts/:uuid", h.Delete)
}

// proxyHostRequest contains the fields user supplied when creating/updating a host.
type proxyHostRequest struct {
	Name         string `json:"name" binding:"required"`
	Domain       string `json:"domain" binding:"required"`
	TargetScheme string `json:"target_scheme" binding:"required,oneof=http https"`
	TargetHost   string `json:"target_host" binding:"required"`
	TargetPort   int    `json:"target_port" binding:"required,min=1,max=65535"`
	EnableTLS    bool   `json:"enable_tls"`
	EnableWS     bool   `json:"enable_websockets"`
}

// List returns every proxy host ordered by most recent update.
func (h *ProxyHostHandler) List(c *gin.Context) {
	var hosts []models.ProxyHost
	if err := h.db.Order("updated_at desc").Find(&hosts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch proxy hosts"})
		return
	}

	c.JSON(http.StatusOK, hosts)
}

// Create inserts a new proxy host into the datastore.
func (h *ProxyHostHandler) Create(c *gin.Context) {
	var req proxyHostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	host := models.ProxyHost{
		UUID:         uuid.NewString(),
		Name:         req.Name,
		Domain:       req.Domain,
		TargetScheme: req.TargetScheme,
		TargetHost:   req.TargetHost,
		TargetPort:   req.TargetPort,
		EnableTLS:    req.EnableTLS,
		EnableWS:     req.EnableWS,
	}

	if err := h.db.Create(&host).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create proxy host"})
		return
	}

	c.JSON(http.StatusCreated, host)
}

// Get returns a single host looked up by UUID.
func (h *ProxyHostHandler) Get(c *gin.Context) {
	uuidParam := c.Param("uuid")
	var host models.ProxyHost
	if err := h.db.First(&host, "uuid = ?", uuidParam).Error; err != nil {
		status := http.StatusInternalServerError
		if err == gorm.ErrRecordNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "proxy host not found"})
		return
	}

	c.JSON(http.StatusOK, host)
}

// Update mutates an existing host.
func (h *ProxyHostHandler) Update(c *gin.Context) {
	uuidParam := c.Param("uuid")
	var req proxyHostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var host models.ProxyHost
	if err := h.db.First(&host, "uuid = ?", uuidParam).Error; err != nil {
		status := http.StatusInternalServerError
		if err == gorm.ErrRecordNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "proxy host not found"})
		return
	}

	host.Name = req.Name
	host.Domain = req.Domain
	host.TargetScheme = req.TargetScheme
	host.TargetHost = req.TargetHost
	host.TargetPort = req.TargetPort
	host.EnableTLS = req.EnableTLS
	host.EnableWS = req.EnableWS

	if err := h.db.Save(&host).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update proxy host"})
		return
	}

	c.JSON(http.StatusOK, host)
}

// Delete removes a proxy host permanently.
func (h *ProxyHostHandler) Delete(c *gin.Context) {
	uuidParam := c.Param("uuid")
	if err := h.db.Where("uuid = ?", uuidParam).Delete(&models.ProxyHost{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete proxy host"})
		return
	}

	c.Status(http.StatusNoContent)
}
