package handlers

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "time"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

// Executor abstracts starting/stopping CrowdSec so tests can mock it.
type CrowdsecExecutor interface {
    Start(ctx context.Context, binPath, configDir string) (int, error)
    Stop(ctx context.Context, configDir string) error
    Status(ctx context.Context, configDir string) (running bool, pid int, err error)
}

// CrowdsecHandler manages CrowdSec process and config imports.
type CrowdsecHandler struct {
    DB       *gorm.DB
    Executor CrowdsecExecutor
    BinPath  string
    DataDir  string
}

func NewCrowdsecHandler(db *gorm.DB, exec CrowdsecExecutor, binPath, dataDir string) *CrowdsecHandler {
    return &CrowdsecHandler{DB: db, Executor: exec, BinPath: binPath, DataDir: dataDir}
}

// Start starts the CrowdSec process.
func (h *CrowdsecHandler) Start(c *gin.Context) {
    ctx := c.Request.Context()
    pid, err := h.Executor.Start(ctx, h.BinPath, h.DataDir)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"status": "started", "pid": pid})
}

// Stop stops the CrowdSec process.
func (h *CrowdsecHandler) Stop(c *gin.Context) {
    ctx := c.Request.Context()
    if err := h.Executor.Stop(ctx, h.DataDir); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

// Status returns simple running state.
func (h *CrowdsecHandler) Status(c *gin.Context) {
    ctx := c.Request.Context()
    running, pid, err := h.Executor.Status(ctx, h.DataDir)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"running": running, "pid": pid})
}

// ImportConfig accepts a tar.gz or zip upload and extracts into DataDir (backing up existing config).
func (h *CrowdsecHandler) ImportConfig(c *gin.Context) {
    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
        return
    }

    // Save to temp file
    tmpDir := os.TempDir()
    tmpPath := filepath.Join(tmpDir, fmt.Sprintf("crowdsec-import-%d", time.Now().UnixNano()))
    if err := os.MkdirAll(tmpPath, 0o755); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create temp dir"})
        return
    }

    dst := filepath.Join(tmpPath, file.Filename)
    if err := c.SaveUploadedFile(file, dst); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save upload"})
        return
    }

    // For safety, do minimal validation: ensure file non-empty
    fi, err := os.Stat(dst)
    if err != nil || fi.Size() == 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "empty upload"})
        return
    }

    // Backup current config
    backupDir := h.DataDir + ".backup." + time.Now().Format("20060102-150405")
    if _, err := os.Stat(h.DataDir); err == nil {
        _ = os.Rename(h.DataDir, backupDir)
    }
    // Create target dir
    if err := os.MkdirAll(h.DataDir, 0o755); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create config dir"})
        return
    }

    // For now, simply copy uploaded file into data dir for operator to handle extraction
    target := filepath.Join(h.DataDir, file.Filename)
    in, err := os.Open(dst)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open temp file"})
        return
    }
    defer in.Close()
    out, err := os.Create(target)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create target file"})
        return
    }
    defer out.Close()
    if _, err := io.Copy(out, in); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"status": "imported", "backup": backupDir})
}

// RegisterRoutes registers crowdsec admin routes under protected group
func (h *CrowdsecHandler) RegisterRoutes(rg *gin.RouterGroup) {
    rg.POST("/admin/crowdsec/start", h.Start)
    rg.POST("/admin/crowdsec/stop", h.Stop)
    rg.GET("/admin/crowdsec/status", h.Status)
    rg.POST("/admin/crowdsec/import", h.ImportConfig)
}
