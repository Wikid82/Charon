package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

type UserHandler struct {
	DB *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{DB: db}
}

func (h *UserHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/setup", h.GetSetupStatus)
	r.POST("/setup", h.Setup)
}

// GetSetupStatus checks if the application needs initial setup (i.e., no users exist).
func (h *UserHandler) GetSetupStatus(c *gin.Context) {
	var count int64
	if err := h.DB.Model(&models.User{}).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check setup status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"setupRequired": count == 0,
	})
}

type SetupRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// Setup creates the initial admin user and configures the ACME email.
func (h *UserHandler) Setup(c *gin.Context) {
	// 1. Check if setup is allowed
	var count int64
	if err := h.DB.Model(&models.User{}).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check setup status"})
		return
	}

	if count > 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Setup already completed"})
		return
	}

	// 2. Parse request
	var req SetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 3. Create User
	user := models.User{
		UUID:    uuid.New().String(),
		Name:    req.Name,
		Email:   req.Email,
		Role:    "admin",
		Enabled: true,
	}

	if err := user.SetPassword(req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// 4. Create Setting for ACME Email
	acmeEmailSetting := models.Setting{
		Key:      "caddy.acme_email",
		Value:    req.Email,
		Type:     "string",
		Category: "caddy",
	}

	// Transaction to ensure both succeed
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		// Use Save to update if exists (though it shouldn't in fresh setup) or create
		if err := tx.Where(models.Setting{Key: "caddy.acme_email"}).Assign(models.Setting{Value: req.Email}).FirstOrCreate(&acmeEmailSetting).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete setup: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Setup completed successfully",
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
		},
	})
}
