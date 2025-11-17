package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/api/routes"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
)

// Server wraps the HTTP engine and shared dependencies for easier testing.
type Server struct {
	Engine *gin.Engine
	cfg    config.Config
}

// New wires up the HTTP router and registers versioned routes.
func New(db *gorm.DB, cfg config.Config) (*Server, error) {
	gin.SetMode(gin.ReleaseMode)
	if cfg.Environment == "development" {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	if err := routes.Register(router, db); err != nil {
		return nil, fmt.Errorf("register routes: %w", err)
	}

	attachFrontend(router, cfg.FrontendDir)

	return &Server{Engine: router, cfg: cfg}, nil
}

func attachFrontend(router *gin.Engine, frontendDir string) {
	if frontendDir == "" {
		return
	}

	info, err := os.Stat(frontendDir)
	if err != nil || !info.IsDir() {
		return
	}

	assetsDir := filepath.Join(frontendDir, "assets")
	if _, err := os.Stat(assetsDir); err == nil {
		router.StaticFS("/assets", gin.Dir(assetsDir, false))
	}

	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
			return
		}

		c.File(filepath.Join(frontendDir, "index.html"))
	})
}

// Run starts the HTTP server with proper shutdown semantics.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", s.cfg.HTTPPort),
		Handler: s.Engine,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
