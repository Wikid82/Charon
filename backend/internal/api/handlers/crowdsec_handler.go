package handlers

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"strings"

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

// ExportConfig creates a tar.gz archive of the CrowdSec data directory and streams it
// back to the client as a downloadable file.
func (h *CrowdsecHandler) ExportConfig(c *gin.Context) {
	// Ensure DataDir exists
	if _, err := os.Stat(h.DataDir); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "crowdsec config not found"})
		return
	}

	// Create a gzip writer and tar writer that stream directly to the response
	c.Header("Content-Type", "application/gzip")
	filename := fmt.Sprintf("crowdsec-config-%s.tar.gz", time.Now().Format("20060102-150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	gw := gzip.NewWriter(c.Writer)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Walk the DataDir and add files to the archive
	err := filepath.Walk(h.DataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(h.DataDir, path)
		if err != nil {
			return err
		}
		// Open file
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		hdr := &tar.Header{
			Name:    rel,
			Size:    info.Size(),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := io.Copy(tw, f); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		// If any error occurred while creating the archive, return 500
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
}

// ListFiles returns a flat list of files under the CrowdSec DataDir.
func (h *CrowdsecHandler) ListFiles(c *gin.Context) {
	var files []string
	if _, err := os.Stat(h.DataDir); os.IsNotExist(err) {
		c.JSON(http.StatusOK, gin.H{"files": files})
		return
	}
	err := filepath.Walk(h.DataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, err := filepath.Rel(h.DataDir, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}

// ReadFile returns the contents of a specific file under DataDir. Query param 'path' required.
func (h *CrowdsecHandler) ReadFile(c *gin.Context) {
	rel := c.Query("path")
	if rel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}
	clean := filepath.Clean(rel)
	// prevent directory traversal
	p := filepath.Join(h.DataDir, clean)
	if !strings.HasPrefix(p, filepath.Clean(h.DataDir)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": string(data)})
}

// WriteFile writes content to a file under the CrowdSec DataDir, creating a backup before doing so.
// JSON body: { "path": "relative/path.conf", "content": "..." }
func (h *CrowdsecHandler) WriteFile(c *gin.Context) {
	var payload struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if payload.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}
	clean := filepath.Clean(payload.Path)
	p := filepath.Join(h.DataDir, clean)
	if !strings.HasPrefix(p, filepath.Clean(h.DataDir)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	// Backup existing DataDir
	backupDir := h.DataDir + ".backup." + time.Now().Format("20060102-150405")
	if _, err := os.Stat(h.DataDir); err == nil {
		if err := os.Rename(h.DataDir, backupDir); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create backup"})
			return
		}
	}
	// Recreate DataDir and write file
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare dir"})
		return
	}
	if err := os.WriteFile(p, []byte(payload.Content), 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write file"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "written", "backup": backupDir})
}

// RegisterRoutes registers crowdsec admin routes under protected group
func (h *CrowdsecHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/admin/crowdsec/start", h.Start)
	rg.POST("/admin/crowdsec/stop", h.Stop)
	rg.GET("/admin/crowdsec/status", h.Status)
	rg.POST("/admin/crowdsec/import", h.ImportConfig)
	rg.GET("/admin/crowdsec/export", h.ExportConfig)
	rg.GET("/admin/crowdsec/files", h.ListFiles)
	rg.GET("/admin/crowdsec/file", h.ReadFile)
	rg.POST("/admin/crowdsec/file", h.WriteFile)
}
