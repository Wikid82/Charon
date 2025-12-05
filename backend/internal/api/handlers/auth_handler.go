package handlers

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthHandler struct {
	authService *services.AuthService
	db          *gorm.DB
}

func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// NewAuthHandlerWithDB creates an AuthHandler with database access for forward auth.
func NewAuthHandlerWithDB(authService *services.AuthService, db *gorm.DB) *AuthHandler {
	return &AuthHandler{authService: authService, db: db}
}

// isProduction checks if we're running in production mode
func isProduction() bool {
	env := os.Getenv("CHARON_ENV")
	return env == "production" || env == "prod"
}

// setSecureCookie sets an auth cookie with security best practices
// - HttpOnly: prevents JavaScript access (XSS protection)
// - Secure: only sent over HTTPS (in production)
// - SameSite=Strict: prevents CSRF attacks
func setSecureCookie(c *gin.Context, name, value string, maxAge int) {
	secure := isProduction()
	sameSite := http.SameSiteStrictMode

	// Use the host without port for domain
	domain := ""

	c.SetSameSite(sameSite)
	c.SetCookie(
		name,   // name
		value,  // value
		maxAge, // maxAge in seconds
		"/",    // path
		domain, // domain (empty = current host)
		secure, // secure (HTTPS only in production)
		true,   // httpOnly (no JS access)
	)
}

// clearSecureCookie removes a cookie with the same security settings
func clearSecureCookie(c *gin.Context, name string) {
	setSecureCookie(c, name, "", -1)
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Set secure cookie (HttpOnly, Secure in prod, SameSite=Strict)
	setSecureCookie(c, "auth_token", token, 3600*24)

	c.JSON(http.StatusOK, gin.H{"token": token})
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.authService.Register(req.Email, req.Password, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	clearSecureCookie(c, "auth_token")
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")

	u, err := h.authService.GetUserByID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id": userID,
		"role":    role,
		"name":    u.Name,
		"email":   u.Email,
	})
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if err := h.authService.ChangePassword(userID.(uint), req.OldPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}

// Verify is the forward auth endpoint for Caddy.
// It validates the user's session and checks access permissions for the requested host.
// Used by Caddy's forward_auth directive.
//
// Expected headers from Caddy:
//   - X-Forwarded-Host: The original host being accessed
//   - X-Forwarded-Uri: The original URI being accessed
//
// Response headers on success (200):
//   - X-Forwarded-User: The user's email
//   - X-Forwarded-Groups: The user's role (for future RBAC)
//
// Response on failure:
//   - 401: Not authenticated (redirect to login)
//   - 403: Authenticated but not authorized for this host
func (h *AuthHandler) Verify(c *gin.Context) {
	// Extract token from cookie or Authorization header
	var tokenString string

	// Try cookie first (most common for browser requests)
	if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
		tokenString = cookie
	}

	// Fall back to Authorization header
	if tokenString == "" {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	// No token found - not authenticated
	if tokenString == "" {
		c.Header("X-Auth-Redirect", "/login")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Validate token
	claims, err := h.authService.ValidateToken(tokenString)
	if err != nil {
		c.Header("X-Auth-Redirect", "/login")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Get user details
	user, err := h.authService.GetUserByID(claims.UserID)
	if err != nil || !user.Enabled {
		c.Header("X-Auth-Redirect", "/login")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Get the forwarded host from Caddy
	forwardedHost := c.GetHeader("X-Forwarded-Host")
	if forwardedHost == "" {
		forwardedHost = c.GetHeader("X-Original-Host")
	}

	// If we have a database reference and a forwarded host, check permissions
	if h.db != nil && forwardedHost != "" {
		// Find the proxy host for this domain
		var proxyHost models.ProxyHost
		err := h.db.Where("domain_names LIKE ?", "%"+forwardedHost+"%").First(&proxyHost).Error

		if err == nil && proxyHost.ForwardAuthEnabled {
			// Load user's permitted hosts for permission check
			var userWithHosts models.User
			if err := h.db.Preload("PermittedHosts").First(&userWithHosts, user.ID).Error; err == nil {
				// Check if user can access this host
				if !userWithHosts.CanAccessHost(proxyHost.ID) {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"error": "Access denied to this application",
					})
					return
				}
			}
		}
	}

	// Set headers for downstream services
	c.Header("X-Forwarded-User", user.Email)
	c.Header("X-Forwarded-Groups", user.Role)
	c.Header("X-Forwarded-Name", user.Name)

	// Return 200 OK - access granted
	c.Status(http.StatusOK)
}

// VerifyStatus returns the current auth status without triggering a redirect.
// Useful for frontend to check if user is logged in.
func (h *AuthHandler) VerifyStatus(c *gin.Context) {
	// Extract token
	var tokenString string

	if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
		tokenString = cookie
	}

	if tokenString == "" {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if tokenString == "" {
		c.JSON(http.StatusOK, gin.H{
			"authenticated": false,
		})
		return
	}

	claims, err := h.authService.ValidateToken(tokenString)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"authenticated": false,
		})
		return
	}

	user, err := h.authService.GetUserByID(claims.UserID)
	if err != nil || !user.Enabled {
		c.JSON(http.StatusOK, gin.H{
			"authenticated": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
			"role":  user.Role,
		},
	})
}

// GetAccessibleHosts returns the list of proxy hosts the authenticated user can access.
func (h *AuthHandler) GetAccessibleHosts(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not available"})
		return
	}

	// Load user with permitted hosts
	var user models.User
	if err := h.db.Preload("PermittedHosts").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get all enabled proxy hosts
	var allHosts []models.ProxyHost
	if err := h.db.Where("enabled = ?", true).Find(&allHosts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch hosts"})
		return
	}

	// Filter to accessible hosts
	accessibleHosts := make([]gin.H, 0)
	for _, host := range allHosts {
		if user.CanAccessHost(host.ID) {
			accessibleHosts = append(accessibleHosts, gin.H{
				"id":           host.ID,
				"name":         host.Name,
				"domain_names": host.DomainNames,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"hosts":           accessibleHosts,
		"permission_mode": user.PermissionMode,
	})
}

// CheckHostAccess checks if the current user can access a specific host.
func (h *AuthHandler) CheckHostAccess(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	hostIDStr := c.Param("hostId")
	hostID, err := strconv.ParseUint(hostIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid host ID"})
		return
	}

	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not available"})
		return
	}

	// Load user with permitted hosts
	var user models.User
	if err := h.db.Preload("PermittedHosts").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	canAccess := user.CanAccessHost(uint(hostID))

	c.JSON(http.StatusOK, gin.H{
		"host_id":    hostID,
		"can_access": canAccess,
	})
}
