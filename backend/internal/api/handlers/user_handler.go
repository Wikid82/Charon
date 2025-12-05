package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

type UserHandler struct {
	DB          *gorm.DB
	MailService *services.MailService
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{
		DB:          db,
		MailService: services.NewMailService(db),
	}
}

func (h *UserHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/setup", h.GetSetupStatus)
	r.POST("/setup", h.Setup)
	r.GET("/profile", h.GetProfile)
	r.POST("/regenerate-api-key", h.RegenerateAPIKey)
	r.PUT("/profile", h.UpdateProfile)

	// User management (admin only)
	r.GET("/users", h.ListUsers)
	r.POST("/users", h.CreateUser)
	r.POST("/users/invite", h.InviteUser)
	r.GET("/users/:id", h.GetUser)
	r.PUT("/users/:id", h.UpdateUser)
	r.DELETE("/users/:id", h.DeleteUser)
	r.PUT("/users/:id/permissions", h.UpdateUserPermissions)

	// Invite acceptance (public)
	r.GET("/invite/validate", h.ValidateInvite)
	r.POST("/invite/accept", h.AcceptInvite)
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
		Email:   strings.ToLower(req.Email),
		Role:    "admin",
		Enabled: true,
		APIKey:  uuid.New().String(),
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

// RegenerateAPIKey generates a new API key for the authenticated user.
func (h *UserHandler) RegenerateAPIKey(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	apiKey := uuid.New().String()

	if err := h.DB.Model(&models.User{}).Where("id = ?", userID).Update("api_key", apiKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update API key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"api_key": apiKey})
}

// GetProfile returns the current user's profile including API key.
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      user.ID,
		"email":   user.Email,
		"name":    user.Name,
		"role":    user.Role,
		"api_key": user.APIKey,
	})
}

type UpdateProfileRequest struct {
	Name            string `json:"name" binding:"required"`
	Email           string `json:"email" binding:"required,email"`
	CurrentPassword string `json:"current_password"`
}

// UpdateProfile updates the authenticated user's profile.
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get current user
	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if email is already taken by another user
	req.Email = strings.ToLower(req.Email)
	var count int64
	if err := h.DB.Model(&models.User{}).Where("email = ? AND id != ?", req.Email, userID).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check email availability"})
		return
	}

	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already in use"})
		return
	}

	// If email is changing, verify password
	if req.Email != user.Email {
		if req.CurrentPassword == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Current password is required to change email"})
			return
		}
		if !user.CheckPassword(req.CurrentPassword) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
			return
		}
	}

	if err := h.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"name":  req.Name,
		"email": req.Email,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

// ListUsers returns all users (admin only).
func (h *UserHandler) ListUsers(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	var users []models.User
	if err := h.DB.Preload("PermittedHosts").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	// Return users with safe fields only
	result := make([]gin.H, len(users))
	for i, u := range users {
		result[i] = gin.H{
			"id":              u.ID,
			"uuid":            u.UUID,
			"email":           u.Email,
			"name":            u.Name,
			"role":            u.Role,
			"enabled":         u.Enabled,
			"last_login":      u.LastLogin,
			"invite_status":   u.InviteStatus,
			"invited_at":      u.InvitedAt,
			"permission_mode": u.PermissionMode,
			"created_at":      u.CreatedAt,
			"updated_at":      u.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, result)
}

// CreateUserRequest represents the request body for creating a user.
type CreateUserRequest struct {
	Email          string `json:"email" binding:"required,email"`
	Name           string `json:"name" binding:"required"`
	Password       string `json:"password" binding:"required,min=8"`
	Role           string `json:"role"`
	PermissionMode string `json:"permission_mode"`
	PermittedHosts []uint `json:"permitted_hosts"`
}

// CreateUser creates a new user with a password (admin only).
func (h *UserHandler) CreateUser(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default role to "user"
	if req.Role == "" {
		req.Role = "user"
	}

	// Default permission mode to "allow_all"
	if req.PermissionMode == "" {
		req.PermissionMode = "allow_all"
	}

	// Check if email already exists
	var count int64
	if err := h.DB.Model(&models.User{}).Where("email = ?", strings.ToLower(req.Email)).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check email"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already in use"})
		return
	}

	user := models.User{
		UUID:           uuid.New().String(),
		Email:          strings.ToLower(req.Email),
		Name:           req.Name,
		Role:           req.Role,
		Enabled:        true,
		APIKey:         uuid.New().String(),
		PermissionMode: models.PermissionMode(req.PermissionMode),
	}

	if err := user.SetPassword(req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}

		// Add permitted hosts if specified
		if len(req.PermittedHosts) > 0 {
			var hosts []models.ProxyHost
			if err := tx.Where("id IN ?", req.PermittedHosts).Find(&hosts).Error; err != nil {
				return err
			}
			if err := tx.Model(&user).Association("PermittedHosts").Replace(hosts); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":    user.ID,
		"uuid":  user.UUID,
		"email": user.Email,
		"name":  user.Name,
		"role":  user.Role,
	})
}

// InviteUserRequest represents the request body for inviting a user.
type InviteUserRequest struct {
	Email          string `json:"email" binding:"required,email"`
	Role           string `json:"role"`
	PermissionMode string `json:"permission_mode"`
	PermittedHosts []uint `json:"permitted_hosts"`
}

// generateSecureToken creates a cryptographically secure random token.
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// InviteUser creates a new user with an invite token and sends an email (admin only).
func (h *UserHandler) InviteUser(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	inviterID, _ := c.Get("userID")

	var req InviteUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default role to "user"
	if req.Role == "" {
		req.Role = "user"
	}

	// Default permission mode to "allow_all"
	if req.PermissionMode == "" {
		req.PermissionMode = "allow_all"
	}

	// Check if email already exists
	var existingUser models.User
	if err := h.DB.Where("email = ?", strings.ToLower(req.Email)).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already in use"})
		return
	}

	// Generate invite token
	inviteToken, err := generateSecureToken(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate invite token"})
		return
	}

	// Set invite expiration (48 hours)
	inviteExpires := time.Now().Add(48 * time.Hour)
	invitedAt := time.Now()
	inviterIDUint := inviterID.(uint)

	user := models.User{
		UUID:           uuid.New().String(),
		Email:          strings.ToLower(req.Email),
		Role:           req.Role,
		Enabled:        false, // Disabled until invite is accepted
		APIKey:         uuid.New().String(),
		PermissionMode: models.PermissionMode(req.PermissionMode),
		InviteToken:    inviteToken,
		InviteExpires:  &inviteExpires,
		InvitedAt:      &invitedAt,
		InvitedBy:      &inviterIDUint,
		InviteStatus:   "pending",
	}

	err = h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}

		// Explicitly disable user (bypass GORM's default:true)
		if err := tx.Model(&user).Update("enabled", false).Error; err != nil {
			return err
		}

		// Add permitted hosts if specified
		if len(req.PermittedHosts) > 0 {
			var hosts []models.ProxyHost
			if err := tx.Where("id IN ?", req.PermittedHosts).Find(&hosts).Error; err != nil {
				return err
			}
			if err := tx.Model(&user).Association("PermittedHosts").Replace(hosts); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user: " + err.Error()})
		return
	}

	// Try to send invite email
	emailSent := false
	if h.MailService.IsConfigured() {
		baseURL := getBaseURL(c)
		appName := getAppName(h.DB)
		if err := h.MailService.SendInvite(user.Email, inviteToken, appName, baseURL); err == nil {
			emailSent = true
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":           user.ID,
		"uuid":         user.UUID,
		"email":        user.Email,
		"role":         user.Role,
		"invite_token": inviteToken, // Return token in case email fails
		"email_sent":   emailSent,
		"expires_at":   inviteExpires,
	})
}

// getBaseURL extracts the base URL from the request.
func getBaseURL(c *gin.Context) string {
	scheme := "https"
	if c.Request.TLS == nil {
		// Check for X-Forwarded-Proto header
		if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + c.Request.Host
}

// getAppName retrieves the application name from settings or returns a default.
func getAppName(db *gorm.DB) string {
	var setting models.Setting
	if err := db.Where("key = ?", "app_name").First(&setting).Error; err == nil && setting.Value != "" {
		return setting.Value
	}
	return "Charon"
}

// GetUser returns a single user by ID (admin only).
func (h *UserHandler) GetUser(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user models.User
	if err := h.DB.Preload("PermittedHosts").First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Build permitted host IDs list
	permittedHostIDs := make([]uint, len(user.PermittedHosts))
	for i, host := range user.PermittedHosts {
		permittedHostIDs[i] = host.ID
	}

	c.JSON(http.StatusOK, gin.H{
		"id":              user.ID,
		"uuid":            user.UUID,
		"email":           user.Email,
		"name":            user.Name,
		"role":            user.Role,
		"enabled":         user.Enabled,
		"last_login":      user.LastLogin,
		"invite_status":   user.InviteStatus,
		"invited_at":      user.InvitedAt,
		"permission_mode": user.PermissionMode,
		"permitted_hosts": permittedHostIDs,
		"created_at":      user.CreatedAt,
		"updated_at":      user.UpdatedAt,
	})
}

// UpdateUserRequest represents the request body for updating a user.
type UpdateUserRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	Enabled *bool  `json:"enabled"`
}

// UpdateUser updates an existing user (admin only).
func (h *UserHandler) UpdateUser(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["name"] = req.Name
	}

	if req.Email != "" {
		email := strings.ToLower(req.Email)
		// Check if email is taken by another user
		var count int64
		if err := h.DB.Model(&models.User{}).Where("email = ? AND id != ?", email, id).Count(&count).Error; err == nil && count > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already in use"})
			return
		}
		updates["email"] = email
	}

	if req.Role != "" {
		updates["role"] = req.Role
	}

	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	if len(updates) > 0 {
		if err := h.DB.Model(&user).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

// DeleteUser deletes a user (admin only).
func (h *UserHandler) DeleteUser(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	currentUserID, _ := c.Get("userID")

	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Prevent self-deletion
	if uint(id) == currentUserID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete your own account"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Clear associations first
	if err := h.DB.Model(&user).Association("PermittedHosts").Clear(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear user associations"})
		return
	}

	if err := h.DB.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// UpdateUserPermissionsRequest represents the request body for updating user permissions.
type UpdateUserPermissionsRequest struct {
	PermissionMode string `json:"permission_mode" binding:"required,oneof=allow_all deny_all"`
	PermittedHosts []uint `json:"permitted_hosts"`
}

// UpdateUserPermissions updates a user's permission mode and host exceptions (admin only).
func (h *UserHandler) UpdateUserPermissions(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var req UpdateUserPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.DB.Transaction(func(tx *gorm.DB) error {
		// Update permission mode
		if err := tx.Model(&user).Update("permission_mode", req.PermissionMode).Error; err != nil {
			return err
		}

		// Update permitted hosts
		var hosts []models.ProxyHost
		if len(req.PermittedHosts) > 0 {
			if err := tx.Where("id IN ?", req.PermittedHosts).Find(&hosts).Error; err != nil {
				return err
			}
		}

		if err := tx.Model(&user).Association("PermittedHosts").Replace(hosts); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update permissions: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permissions updated successfully"})
}

// ValidateInvite validates an invite token (public endpoint).
func (h *UserHandler) ValidateInvite(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token required"})
		return
	}

	var user models.User
	if err := h.DB.Where("invite_token = ?", token).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid or expired invite token"})
		return
	}

	// Check if token is expired
	if user.InviteExpires != nil && user.InviteExpires.Before(time.Now()) {
		c.JSON(http.StatusGone, gin.H{"error": "Invite token has expired"})
		return
	}

	// Check if already accepted
	if user.InviteStatus != "pending" {
		c.JSON(http.StatusConflict, gin.H{"error": "Invite has already been accepted"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"email": user.Email,
	})
}

// AcceptInviteRequest represents the request body for accepting an invite.
type AcceptInviteRequest struct {
	Token    string `json:"token" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// AcceptInvite accepts an invitation and sets the user's password (public endpoint).
func (h *UserHandler) AcceptInvite(c *gin.Context) {
	var req AcceptInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := h.DB.Where("invite_token = ?", req.Token).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid or expired invite token"})
		return
	}

	// Check if token is expired
	if user.InviteExpires != nil && user.InviteExpires.Before(time.Now()) {
		// Mark as expired
		h.DB.Model(&user).Update("invite_status", "expired")
		c.JSON(http.StatusGone, gin.H{"error": "Invite token has expired"})
		return
	}

	// Check if already accepted
	if user.InviteStatus != "pending" {
		c.JSON(http.StatusConflict, gin.H{"error": "Invite has already been accepted"})
		return
	}

	// Set password and activate user
	if err := user.SetPassword(req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set password"})
		return
	}

	if err := h.DB.Model(&user).Updates(map[string]interface{}{
		"name":           req.Name,
		"password_hash":  user.PasswordHash,
		"enabled":        true,
		"invite_token":   "",  // Clear token
		"invite_expires": nil, // Clear expiration
		"invite_status":  "accepted",
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to accept invite"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Invite accepted successfully",
		"email":   user.Email,
	})
}
