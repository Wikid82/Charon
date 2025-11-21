package handlers

import (
	"net/http"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DomainHandler struct {
	DB *gorm.DB
}

func NewDomainHandler(db *gorm.DB) *DomainHandler {
	return &DomainHandler{DB: db}
}

func (h *DomainHandler) List(c *gin.Context) {
	var domains []models.Domain
	if err := h.DB.Order("name asc").Find(&domains).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domains"})
		return
	}
	c.JSON(http.StatusOK, domains)
}

func (h *DomainHandler) Create(c *gin.Context) {
	var input struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	domain := models.Domain{
		Name: input.Name,
	}

	if err := h.DB.Create(&domain).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create domain"})
		return
	}

	c.JSON(http.StatusCreated, domain)
}

func (h *DomainHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.DB.Where("uuid = ?", id).Delete(&models.Domain{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete domain"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Domain deleted"})
}
