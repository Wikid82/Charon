package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
)

// EmergencyServer provides a minimal HTTP server for emergency operations.
// This server runs on a separate port with minimal security for failsafe access.
//
// Port Assignment:
// - Port 2019: Reserved for Caddy admin API
// - Port 2020: Emergency server (tier-2 break glass)
//
// Security Philosophy:
// - Separate port bypasses Caddy/CrowdSec/WAF entirely
// - Optional Basic Auth (configurable via env)
// - Should ONLY be accessible via VPN/SSH tunnel
// - Default bind to localhost IPv4 (127.0.0.1:2020) for safety
// - IPv6 support available via config (e.g., 0.0.0.0:2020 or [::]:2020 for dual-stack)
//
// Use Cases:
// - Layer 7 reverse proxy blocking requests (CrowdSec bouncer at Caddy)
// - Caddy itself is down or misconfigured
// - Emergency access when main application port is unreachable
type EmergencyServer struct {
	server   *http.Server
	listener net.Listener
	db       *gorm.DB
	cfg      config.EmergencyConfig
	cerberus handlers.CacheInvalidator
	caddy    handlers.CaddyConfigManager
}

// NewEmergencyServer creates a new emergency server instance
func NewEmergencyServer(db *gorm.DB, cfg config.EmergencyConfig) *EmergencyServer {
	return NewEmergencyServerWithDeps(db, cfg, nil, nil)
}

// NewEmergencyServerWithDeps creates a new emergency server instance with optional dependencies.
func NewEmergencyServerWithDeps(db *gorm.DB, cfg config.EmergencyConfig, caddyManager handlers.CaddyConfigManager, cerberus handlers.CacheInvalidator) *EmergencyServer {
	return &EmergencyServer{
		db:       db,
		cfg:      cfg,
		caddy:    caddyManager,
		cerberus: cerberus,
	}
}

// Start initializes and starts the emergency server
func (s *EmergencyServer) Start() error {
	if !s.cfg.Enabled {
		logger.Log().Info("Emergency server disabled (CHARON_EMERGENCY_SERVER_ENABLED=false)")
		return nil
	}

	// CRITICAL: Validate emergency token is configured (fail-fast)
	emergencyToken := os.Getenv(handlers.EmergencyTokenEnvVar)
	if emergencyToken == "" || len(strings.TrimSpace(emergencyToken)) == 0 {
		logger.Log().Error("FATAL: CHARON_EMERGENCY_SERVER_ENABLED=true but CHARON_EMERGENCY_TOKEN is empty or whitespace. Emergency server cannot start without a valid token.")
		return fmt.Errorf("emergency token not configured")
	}

	// Validate token meets minimum length requirement
	if len(emergencyToken) < handlers.MinTokenLength {
		logger.Log().WithField("length", len(emergencyToken)).Warn("⚠️  WARNING: CHARON_EMERGENCY_TOKEN is shorter than 32 bytes (weak security)")
	}

	// Log token initialization with redaction
	redactedToken := redactToken(emergencyToken)
	logger.Log().WithFields(map[string]interface{}{
		"token": redactedToken,
	}).Info("Emergency server initialized with token")

	// Security warning if no authentication configured
	if s.cfg.BasicAuthUsername == "" || s.cfg.BasicAuthPassword == "" {
		logger.Log().Warn("⚠️  SECURITY WARNING: Emergency server has NO authentication configured")
		logger.Log().Warn("⚠️  Ensure port is accessible ONLY via VPN/SSH tunnel")
		logger.Log().Warn("⚠️  Set CHARON_EMERGENCY_USERNAME and CHARON_EMERGENCY_PASSWORD")
	}

	// Configure Gin for minimal logging (not production mode to preserve logs)
	router := gin.New()

	// Middleware 1: Recovery (panic handler)
	router.Use(gin.Recovery())

	// Middleware 2: Simple request logging (minimal)
	router.Use(func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start).Milliseconds()
		status := c.Writer.Status()

		logger.Log().WithFields(map[string]interface{}{
			"server":  "emergency",
			"method":  method,
			"path":    path,
			"status":  status,
			"latency": fmt.Sprintf("%dms", latency),
			"ip":      c.ClientIP(),
		}).Info("Emergency server request")
	})

	// Emergency endpoints only
	emergencyHandler := handlers.NewEmergencyHandlerWithDeps(s.db, s.caddy, s.cerberus)

	// GET /health - Health check endpoint (NO AUTH - must be accessible for monitoring)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"server": "emergency",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Middleware 3: Basic Auth (if configured)
	// Applied AFTER /health endpoint so health checks don't require auth
	if s.cfg.BasicAuthUsername != "" && s.cfg.BasicAuthPassword != "" {
		accounts := gin.Accounts{
			s.cfg.BasicAuthUsername: s.cfg.BasicAuthPassword,
		}
		router.Use(gin.BasicAuth(accounts))
		logger.Log().WithField("username", s.cfg.BasicAuthUsername).Info("Emergency server Basic Auth enabled")
	}

	// POST /emergency/security-reset - Disable all security modules
	router.POST("/emergency/security-reset", emergencyHandler.SecurityReset)

	// Create HTTP server with sensible timeouts
	s.server = &http.Server{
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Create listener (this allows us to get the actual port when using :0 for testing)
	listener, err := net.Listen("tcp", s.cfg.BindAddress)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	s.listener = listener

	// Start server in goroutine
	go func() {
		logger.Log().WithFields(map[string]interface{}{
			"address":  listener.Addr().String(),
			"auth":     s.cfg.BasicAuthUsername != "",
			"endpoint": "/emergency/security-reset",
		}).Info("Starting emergency server (Tier 2 break glass)")

		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Log().WithError(err).Error("Emergency server failed")
		}
	}()

	return nil
}

// Stop gracefully shuts down the emergency server
func (s *EmergencyServer) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	logger.Log().Info("Stopping emergency server")
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("emergency server shutdown: %w", err)
	}

	logger.Log().Info("Emergency server stopped")
	return nil
}

// GetAddr returns the actual bind address (useful for tests with :0)
func (s *EmergencyServer) GetAddr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// redactToken returns a redacted version of the token showing only first/last 4 characters
// Format: [EMERGENCY_TOKEN:f51d...346b]
func redactToken(token string) string {
	if token == "" {
		return "[EMERGENCY_TOKEN:empty]"
	}
	if len(token) <= 8 {
		return "[EMERGENCY_TOKEN:***]"
	}
	return fmt.Sprintf("[EMERGENCY_TOKEN:%s...%s]", token[:4], token[len(token)-4:])
}
