package handlers

import (
	"net/http"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AuthUserHandler handles auth user operations
type AuthUserHandler struct {
	db *gorm.DB
}

// NewAuthUserHandler creates a new auth user handler
func NewAuthUserHandler(db *gorm.DB) *AuthUserHandler {
	return &AuthUserHandler{db: db}
}

// List returns all auth users
func (h *AuthUserHandler) List(c *gin.Context) {
	var users []models.AuthUser
	if err := h.db.Order("created_at desc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

// Get returns a single auth user by UUID
func (h *AuthUserHandler) Get(c *gin.Context) {
	uuid := c.Param("uuid")
	var user models.AuthUser
	if err := h.db.Where("uuid = ?", uuid).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

// CreateRequest represents the request body for creating an auth user
type CreateAuthUserRequest struct {
	Username   string `json:"username" binding:"required"`
	Email      string `json:"email" binding:"required,email"`
	Name       string `json:"name"`
	Password   string `json:"password" binding:"required,min=8"`
	Roles      string `json:"roles"`
	MFAEnabled bool   `json:"mfa_enabled"`
}

// Create creates a new auth user
func (h *AuthUserHandler) Create(c *gin.Context) {
	var req CreateAuthUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := models.AuthUser{
		Username:   req.Username,
		Email:      req.Email,
		Name:       req.Name,
		Roles:      req.Roles,
		MFAEnabled: req.MFAEnabled,
		Enabled:    true,
	}

	if err := user.SetPassword(req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// UpdateRequest represents the request body for updating an auth user
type UpdateAuthUserRequest struct {
	Email      *string `json:"email,omitempty"`
	Name       *string `json:"name,omitempty"`
	Password   *string `json:"password,omitempty"`
	Roles      *string `json:"roles,omitempty"`
	Enabled    *bool   `json:"enabled,omitempty"`
	MFAEnabled *bool   `json:"mfa_enabled,omitempty"`
}

// Update updates an existing auth user
func (h *AuthUserHandler) Update(c *gin.Context) {
	uuid := c.Param("uuid")
	var user models.AuthUser
	if err := h.db.Where("uuid = ?", uuid).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req UpdateAuthUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Password != nil {
		if err := user.SetPassword(*req.Password); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
	}
	if req.Roles != nil {
		user.Roles = *req.Roles
	}
	if req.Enabled != nil {
		user.Enabled = *req.Enabled
	}
	if req.MFAEnabled != nil {
		user.MFAEnabled = *req.MFAEnabled
	}

	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// Delete deletes an auth user
func (h *AuthUserHandler) Delete(c *gin.Context) {
	uuid := c.Param("uuid")

	// Prevent deletion of the last admin
	var user models.AuthUser
	if err := h.db.Where("uuid = ?", uuid).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check if this is an admin user
	if user.HasRole("admin") {
		var adminCount int64
		h.db.Model(&models.AuthUser{}).Where("roles LIKE ?", "%admin%").Count(&adminCount)
		if adminCount <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete the last admin user"})
			return
		}
	}

	if err := h.db.Where("uuid = ?", uuid).Delete(&models.AuthUser{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// Stats returns statistics about auth users
func (h *AuthUserHandler) Stats(c *gin.Context) {
	var total int64
	var enabled int64
	var withMFA int64

	h.db.Model(&models.AuthUser{}).Count(&total)
	h.db.Model(&models.AuthUser{}).Where("enabled = ?", true).Count(&enabled)
	h.db.Model(&models.AuthUser{}).Where("mfa_enabled = ?", true).Count(&withMFA)

	c.JSON(http.StatusOK, gin.H{
		"total":    total,
		"enabled":  enabled,
		"with_mfa": withMFA,
	})
}

// AuthProviderHandler handles auth provider operations
type AuthProviderHandler struct {
	db *gorm.DB
}

// NewAuthProviderHandler creates a new auth provider handler
func NewAuthProviderHandler(db *gorm.DB) *AuthProviderHandler {
	return &AuthProviderHandler{db: db}
}

// List returns all auth providers
func (h *AuthProviderHandler) List(c *gin.Context) {
	var providers []models.AuthProvider
	if err := h.db.Order("created_at desc").Find(&providers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, providers)
}

// Get returns a single auth provider by UUID
func (h *AuthProviderHandler) Get(c *gin.Context) {
	uuid := c.Param("uuid")
	var provider models.AuthProvider
	if err := h.db.Where("uuid = ?", uuid).First(&provider).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, provider)
}

// CreateProviderRequest represents the request body for creating an auth provider
type CreateProviderRequest struct {
	Name         string `json:"name" binding:"required"`
	Type         string `json:"type" binding:"required"`
	ClientID     string `json:"client_id" binding:"required"`
	ClientSecret string `json:"client_secret" binding:"required"`
	IssuerURL    string `json:"issuer_url"`
	AuthURL      string `json:"auth_url"`
	TokenURL     string `json:"token_url"`
	UserInfoURL  string `json:"user_info_url"`
	Scopes       string `json:"scopes"`
	RoleMapping  string `json:"role_mapping"`
	IconURL      string `json:"icon_url"`
	DisplayName  string `json:"display_name"`
}

// Create creates a new auth provider
func (h *AuthProviderHandler) Create(c *gin.Context) {
	var req CreateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	provider := models.AuthProvider{
		Name:         req.Name,
		Type:         req.Type,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		IssuerURL:    req.IssuerURL,
		AuthURL:      req.AuthURL,
		TokenURL:     req.TokenURL,
		UserInfoURL:  req.UserInfoURL,
		Scopes:       req.Scopes,
		RoleMapping:  req.RoleMapping,
		IconURL:      req.IconURL,
		DisplayName:  req.DisplayName,
		Enabled:      true,
	}

	if err := h.db.Create(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, provider)
}

// UpdateProviderRequest represents the request body for updating an auth provider
type UpdateProviderRequest struct {
	Name         *string `json:"name,omitempty"`
	Type         *string `json:"type,omitempty"`
	ClientID     *string `json:"client_id,omitempty"`
	ClientSecret *string `json:"client_secret,omitempty"`
	IssuerURL    *string `json:"issuer_url,omitempty"`
	AuthURL      *string `json:"auth_url,omitempty"`
	TokenURL     *string `json:"token_url,omitempty"`
	UserInfoURL  *string `json:"user_info_url,omitempty"`
	Scopes       *string `json:"scopes,omitempty"`
	RoleMapping  *string `json:"role_mapping,omitempty"`
	IconURL      *string `json:"icon_url,omitempty"`
	DisplayName  *string `json:"display_name,omitempty"`
	Enabled      *bool   `json:"enabled,omitempty"`
}

// Update updates an existing auth provider
func (h *AuthProviderHandler) Update(c *gin.Context) {
	uuid := c.Param("uuid")
	var provider models.AuthProvider
	if err := h.db.Where("uuid = ?", uuid).First(&provider).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req UpdateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != nil {
		provider.Name = *req.Name
	}
	if req.Type != nil {
		provider.Type = *req.Type
	}
	if req.ClientID != nil {
		provider.ClientID = *req.ClientID
	}
	if req.ClientSecret != nil {
		provider.ClientSecret = *req.ClientSecret
	}
	if req.IssuerURL != nil {
		provider.IssuerURL = *req.IssuerURL
	}
	if req.AuthURL != nil {
		provider.AuthURL = *req.AuthURL
	}
	if req.TokenURL != nil {
		provider.TokenURL = *req.TokenURL
	}
	if req.UserInfoURL != nil {
		provider.UserInfoURL = *req.UserInfoURL
	}
	if req.Scopes != nil {
		provider.Scopes = *req.Scopes
	}
	if req.RoleMapping != nil {
		provider.RoleMapping = *req.RoleMapping
	}
	if req.IconURL != nil {
		provider.IconURL = *req.IconURL
	}
	if req.DisplayName != nil {
		provider.DisplayName = *req.DisplayName
	}
	if req.Enabled != nil {
		provider.Enabled = *req.Enabled
	}

	if err := h.db.Save(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// Delete deletes an auth provider
func (h *AuthProviderHandler) Delete(c *gin.Context) {
	uuid := c.Param("uuid")
	if err := h.db.Where("uuid = ?", uuid).Delete(&models.AuthProvider{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Provider deleted successfully"})
}

// AuthPolicyHandler handles auth policy operations
type AuthPolicyHandler struct {
	db *gorm.DB
}

// NewAuthPolicyHandler creates a new auth policy handler
func NewAuthPolicyHandler(db *gorm.DB) *AuthPolicyHandler {
	return &AuthPolicyHandler{db: db}
}

// List returns all auth policies
func (h *AuthPolicyHandler) List(c *gin.Context) {
	var policies []models.AuthPolicy
	if err := h.db.Order("created_at desc").Find(&policies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policies)
}

// Get returns a single auth policy by UUID
func (h *AuthPolicyHandler) Get(c *gin.Context) {
	uuid := c.Param("uuid")
	var policy models.AuthPolicy
	if err := h.db.Where("uuid = ?", uuid).First(&policy).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Policy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

// GetByID returns a single auth policy by ID
func (h *AuthPolicyHandler) GetByID(id uint) (*models.AuthPolicy, error) {
	var policy models.AuthPolicy
	if err := h.db.First(&policy, id).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}

// CreatePolicyRequest represents the request body for creating an auth policy
type CreatePolicyRequest struct {
	Name           string `json:"name" binding:"required"`
	Description    string `json:"description"`
	AllowedRoles   string `json:"allowed_roles"`
	AllowedUsers   string `json:"allowed_users"`
	AllowedDomains string `json:"allowed_domains"`
	RequireMFA     bool   `json:"require_mfa"`
	SessionTimeout int    `json:"session_timeout"`
}

// Create creates a new auth policy
func (h *AuthPolicyHandler) Create(c *gin.Context) {
	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy := models.AuthPolicy{
		Name:           req.Name,
		Description:    req.Description,
		AllowedRoles:   req.AllowedRoles,
		AllowedUsers:   req.AllowedUsers,
		AllowedDomains: req.AllowedDomains,
		RequireMFA:     req.RequireMFA,
		SessionTimeout: req.SessionTimeout,
		Enabled:        true,
	}

	if err := h.db.Create(&policy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, policy)
}

// UpdatePolicyRequest represents the request body for updating an auth policy
type UpdatePolicyRequest struct {
	Name           *string `json:"name,omitempty"`
	Description    *string `json:"description,omitempty"`
	AllowedRoles   *string `json:"allowed_roles,omitempty"`
	AllowedUsers   *string `json:"allowed_users,omitempty"`
	AllowedDomains *string `json:"allowed_domains,omitempty"`
	RequireMFA     *bool   `json:"require_mfa,omitempty"`
	SessionTimeout *int    `json:"session_timeout,omitempty"`
	Enabled        *bool   `json:"enabled,omitempty"`
}

// Update updates an existing auth policy
func (h *AuthPolicyHandler) Update(c *gin.Context) {
	uuid := c.Param("uuid")
	var policy models.AuthPolicy
	if err := h.db.Where("uuid = ?", uuid).First(&policy).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Policy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req UpdatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != nil {
		policy.Name = *req.Name
	}
	if req.Description != nil {
		policy.Description = *req.Description
	}
	if req.AllowedRoles != nil {
		policy.AllowedRoles = *req.AllowedRoles
	}
	if req.AllowedUsers != nil {
		policy.AllowedUsers = *req.AllowedUsers
	}
	if req.AllowedDomains != nil {
		policy.AllowedDomains = *req.AllowedDomains
	}
	if req.RequireMFA != nil {
		policy.RequireMFA = *req.RequireMFA
	}
	if req.SessionTimeout != nil {
		policy.SessionTimeout = *req.SessionTimeout
	}
	if req.Enabled != nil {
		policy.Enabled = *req.Enabled
	}

	if err := h.db.Save(&policy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)
}

// Delete deletes an auth policy
func (h *AuthPolicyHandler) Delete(c *gin.Context) {
	uuid := c.Param("uuid")

	// Get the policy first to get its ID
	var policy models.AuthPolicy
	if err := h.db.Where("uuid = ?", uuid).First(&policy).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Policy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check if any proxy hosts are using this policy
	var count int64
	h.db.Model(&models.ProxyHost{}).Where("auth_policy_id = ?", policy.ID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete policy that is in use by proxy hosts"})
		return
	}

	if err := h.db.Delete(&policy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Policy deleted successfully"})
}
