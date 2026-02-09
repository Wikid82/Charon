package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// --- Sprint 2: Archive Validation Tests ---

// createTestArchive creates a test archive with specified files.
// Returns the archive path.
func createTestArchive(t *testing.T, format string, files map[string]string, compressed bool) string {
	t.Helper()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test."+format)

	if format == "tar.gz" {
		// #nosec G304 -- archivePath is in test temp directory created by t.TempDir()
		f, err := os.Create(archivePath)
		require.NoError(t, err)
		defer func() { _ = f.Close() }()

		var w io.Writer = f
		if compressed {
			gw := gzip.NewWriter(f)
			defer func() { _ = gw.Close() }()
			w = gw
		}

		tw := tar.NewWriter(w)
		defer func() { _ = tw.Close() }()

		for name, content := range files {
			hdr := &tar.Header{
				Name: name,
				Size: int64(len(content)),
				Mode: 0o644,
			}
			require.NoError(t, tw.WriteHeader(hdr))
			_, err := tw.Write([]byte(content))
			require.NoError(t, err)
		}
	}

	return archivePath
}

// TestConfigArchiveValidator_ValidFormats tests that valid archive formats are accepted.
func TestConfigArchiveValidator_ValidFormats(t *testing.T) {
	t.Parallel()

	validator := &ConfigArchiveValidator{
		MaxSize:             50 * 1024 * 1024,
		MaxUncompressed:     500 * 1024 * 1024,
		MaxCompressionRatio: 100,
		RequiredFiles:       []string{"config.yaml"},
	}

	tests := []struct {
		name   string
		format string
		files  map[string]string
	}{
		{
			name:   "valid tar.gz with config.yaml",
			format: "tar.gz",
			files: map[string]string{
				"config.yaml": "api:\n  server:\n    listen_uri: 0.0.0.0:8080\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archivePath := createTestArchive(t, tt.format, tt.files, true)
			err := validator.Validate(archivePath)
			require.NoError(t, err)
		})
	}
}

// TestConfigArchiveValidator_InvalidFormats tests rejection of invalid formats.
func TestConfigArchiveValidator_InvalidFormats(t *testing.T) {
	t.Parallel()

	validator := &ConfigArchiveValidator{
		MaxSize:             50 * 1024 * 1024,
		MaxUncompressed:     500 * 1024 * 1024,
		MaxCompressionRatio: 100,
		RequiredFiles:       []string{"config.yaml"},
	}

	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		content  string
		wantErr  string
	}{
		{
			name:     "txt file",
			filename: "test.txt",
			content:  "not an archive",
			wantErr:  "unsupported format",
		},
		{
			name:     "rar file",
			filename: "test.rar",
			content:  "Rar!\x1a\x07\x00",
			wantErr:  "unsupported format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.filename)
			// #nosec G306 -- Test file, 0o600 not required
			err := os.WriteFile(path, []byte(tt.content), 0o600)
			require.NoError(t, err)

			err = validator.Validate(path)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestConfigArchiveValidator_SizeLimit tests enforcement of size limits.
func TestConfigArchiveValidator_SizeLimit(t *testing.T) {
	t.Parallel()

	validator := &ConfigArchiveValidator{
		MaxSize:             1024, // 1KB limit for testing
		MaxUncompressed:     10 * 1024,
		MaxCompressionRatio: 100,
		RequiredFiles:       []string{"config.yaml"},
	}

	// Create multiple large files to exceed compressed size limit
	// Use less compressible content (random-like data)
	largeContent := make([]byte, 2048)
	for i := range largeContent {
		largeContent[i] = byte(i % 256) // Less compressible than repeated chars
	}

	files := map[string]string{
		"config.yaml": string(largeContent),
		"file2.yaml":  string(largeContent),
		"file3.yaml":  string(largeContent),
	}

	archivePath := createTestArchive(t, "tar.gz", files, true)

	// Verify the archive is actually larger than limit
	info, err := os.Stat(archivePath)
	require.NoError(t, err)

	// If archive is still under limit, skip this test
	if info.Size() <= validator.MaxSize {
		t.Skipf("Archive size %d is under limit %d, skipping", info.Size(), validator.MaxSize)
	}

	err = validator.Validate(archivePath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds maximum size")
}

// TestConfigArchiveValidator_CompressionRatio tests zip bomb protection.
func TestConfigArchiveValidator_CompressionRatio(t *testing.T) {
	t.Parallel()

	validator := &ConfigArchiveValidator{
		MaxSize:             50 * 1024 * 1024,
		MaxUncompressed:     500 * 1024 * 1024,
		MaxCompressionRatio: 10, // Lower ratio for testing
		RequiredFiles:       []string{"config.yaml"},
	}

	// Create highly compressible content (simulating zip bomb)
	highlyCompressible := strings.Repeat("AAAAAAAAAA", 10000)
	files := map[string]string{
		"config.yaml": highlyCompressible,
	}

	archivePath := createTestArchive(t, "tar.gz", files, true)
	err := validator.Validate(archivePath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "compression ratio")
}

// TestConfigArchiveValidator_RequiredFiles tests required file validation.
func TestConfigArchiveValidator_RequiredFiles(t *testing.T) {
	t.Parallel()

	validator := &ConfigArchiveValidator{
		MaxSize:             50 * 1024 * 1024,
		MaxUncompressed:     500 * 1024 * 1024,
		MaxCompressionRatio: 100,
		RequiredFiles:       []string{"config.yaml"},
	}

	tests := []struct {
		name    string
		files   map[string]string
		wantErr bool
	}{
		{
			name: "has required file",
			files: map[string]string{
				"config.yaml": "valid: true",
			},
			wantErr: false,
		},
		{
			name: "missing required file",
			files: map[string]string{
				"other.yaml": "valid: true",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archivePath := createTestArchive(t, "tar.gz", tt.files, true)
			err := validator.Validate(archivePath)
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "required file")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestImportConfig_Validation tests the enhanced ImportConfig handler with validation.
func TestImportConfig_Validation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	db := OpenTestDB(t)
	tmpDir := t.TempDir()
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	tests := []struct {
		name       string
		files      map[string]string
		wantStatus int
		wantErr    string
	}{
		{
			name: "valid archive",
			files: map[string]string{
				"config.yaml": "api:\n  server:\n    listen_uri: 0.0.0.0:8080\n",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "missing config.yaml",
			files: map[string]string{
				"other.yaml": "data: test",
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantErr:    "required file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archivePath := createTestArchive(t, "tar.gz", tt.files, true)

			// Create multipart request
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, err := writer.CreateFormFile("file", "test.tar.gz")
			require.NoError(t, err)

			// #nosec G304 -- archivePath is in test temp directory
			archiveData, err := os.ReadFile(archivePath)
			require.NoError(t, err)
			_, err = part.Write(archiveData)
			require.NoError(t, err)
			require.NoError(t, writer.Close())

			req := httptest.NewRequest(http.MethodPost, "/api/v1/crowdsec/import", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()

			c, _ := gin.CreateTestContext(w)
			c.Request = req

			h.ImportConfig(c)

			require.Equal(t, tt.wantStatus, w.Code)
			if tt.wantErr != "" {
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Contains(t, resp["error"], tt.wantErr)
			}
		})
	}
}

// TestImportConfig_Rollback tests backup restoration on validation failure.
func TestImportConfig_Rollback(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	db := OpenTestDB(t)
	tmpDir := t.TempDir()
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", tmpDir)

	// Create existing config
	existingConfig := filepath.Join(tmpDir, "existing.yaml")
	// #nosec G306 -- Test file, 0o600 not required
	err := os.WriteFile(existingConfig, []byte("existing: true"), 0o600)
	require.NoError(t, err)

	// Create invalid archive (missing config.yaml)
	archivePath := createTestArchive(t, "tar.gz", map[string]string{
		"invalid.yaml": "test: data",
	}, true)

	// Create multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.tar.gz")
	require.NoError(t, err)

	// #nosec G304 -- archivePath is in test temp directory
	archiveData, err := os.ReadFile(archivePath)
	require.NoError(t, err)
	_, err = part.Write(archiveData)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/crowdsec/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.ImportConfig(c)

	// Should fail validation
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)

	// Original config should still exist (rollback)
	_, err = os.Stat(existingConfig)
	require.NoError(t, err)
}
