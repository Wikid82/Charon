package handlers

import (
	"net"
	"net/http"
	"net/url"
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

func requestScheme(c *gin.Context) string {
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		// Honor first entry in a comma-separated header
		parts := strings.Split(proto, ",")
		return strings.ToLower(strings.TrimSpace(parts[0]))
	}
	if c.Request != nil && c.Request.TLS != nil {
		return "https"
	}
	if c.Request != nil && c.Request.URL != nil && c.Request.URL.Scheme != "" {
		return strings.ToLower(c.Request.URL.Scheme)
	}
	return "http"
}

func normalizeHost(rawHost string) string {
	host := strings.TrimSpace(rawHost)
	if host == "" {
		return ""
	}

	if strings.Contains(host, ":") {
		if parsedHost, _, err := net.SplitHostPort(host); err == nil {
			host = parsedHost
		}
	}

	return strings.Trim(host, "[]")
}

func originHost(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	return normalizeHost(parsedURL.Host)
}

func isLocalOrPrivateHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}

	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate()) {
		return true
	}

	return false
}

func isLocalRequest(c *gin.Context) bool {
	candidates := []string{}

	if c.Request != nil {
		candidates = append(candidates, normalizeHost(c.Request.Host))

		if c.Request.URL != nil {
			candidates = append(candidates, normalizeHost(c.Request.URL.Host))
		}

		candidates = append(candidates,
			originHost(c.Request.Header.Get("Origin")),
			originHost(c.Request.Header.Get("Referer")),
		)
	}

	if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
		parts := strings.Split(forwardedHost, ",")
		for _, part := range parts {
			candidates = append(candidates, normalizeHost(part))
		}
	}

	for _, host := range candidates {
		if host == "" {
			continue
		}

		if isLocalOrPrivateHost(host) {
			return true
		}
	}

	return false
}

// setSecureCookie sets an auth cookie with security best practices
// - HttpOnly: prevents JavaScript access (XSS protection)
// - Secure: always true (all major browsers honour Secure on localhost HTTP;
//   HTTP-on-private-IP without TLS is an unsupported deployment)
// - SameSite: Lax for any local/private-network request (regardless of scheme),
//   Strict otherwise (public HTTPS only)
func setSecureCookie(c *gin.Context, name, value string, maxAge int) {
	scheme := requestScheme(c)
	sameSite := http.SameSiteStrictMode
	if scheme != "https" {
		sameSite = http.SameSiteLaxMode
	}

	if isLocalRequest(c) {
		sameSite = http.SameSiteLaxMode
	}

	// Use the host without port for domain
	domain := ""

	c.SetSameSite(sameSite)
	c.SetCookie(
		name,   // name
		value,  // value
		maxAge, // maxAge in seconds
		"/",    // path
		domain, // domain (empty = current host)
		true,   // secure
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

	// Set secure cookie (scheme-aware) and return token for header fallback
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
	if userIDValue, exists := c.Get("userID"); exists {
		if userID, ok := userIDValue.(uint); ok && userID > 0 {
			if err := h.authService.InvalidateSessions(userID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to invalidate session"})
				return
			}
		}
	}

	clearSecureCookie(c, "auth_token")
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

// Refresh creates a new token for the authenticated user.
// Must be called with a valid existing token.
// Supports long-running test sessions by allowing token refresh before expiry.
func (h *AuthHandler) Refresh(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	user, err := h.authService.GetUserByID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	token, err := h.authService.GenerateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Set secure cookie and return new token
	setSecureCookie(c, "auth_token", token, 3600*24)

	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userID, ok := userIDValue.(uint)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	role, _ := c.Get("role")

	u, err := h.authService.GetUserByID(userID)
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
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenString = strings.TrimPrefix(authHeader, "Bearer ")
	}

	// Fall back to cookie (most common for browser requests)
	if tokenString == "" {
		if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
			tokenString = cookie
		}
	}

	// No token found - not authenticated
	if tokenString == "" {
		c.Header("X-Auth-Redirect", "/login")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Validate token
	user, _, err := h.authService.AuthenticateToken(tokenString)
	if err != nil {
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
	c.Header("X-Forwarded-Groups", string(user.Role))
	c.Header("X-Forwarded-Name", user.Name)

	// Return 200 OK - access granted
	c.Status(http.StatusOK)
}

// VerifyStatus returns the current auth status without triggering a redirect.
// Useful for frontend to check if user is logged in.
func (h *AuthHandler) VerifyStatus(c *gin.Context) {
	// Extract token
	var tokenString string
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenString = strings.TrimPrefix(authHeader, "Bearer ")
	}

	if tokenString == "" {
		if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
			tokenString = cookie
		}
	}

	if tokenString == "" {
		c.JSON(http.StatusOK, gin.H{
			"authenticated": false,
		})
		return
	}

	user, _, err := h.authService.AuthenticateToken(tokenString)
	if err != nil {
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
