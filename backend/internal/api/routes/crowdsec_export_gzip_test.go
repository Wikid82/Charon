package routes

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestCrowdsecExport_NotDoubleCompressed guards against the router-wide gzip middleware
// wrapping the export handler's own tar.gz stream a second time.
func TestCrowdsecExport_NotDoubleCompressed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)

	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "config"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "config", "config.yaml"), []byte("test: config\n"), 0o600))

	h := handlers.NewCrowdsecHandler(db, nil, "/bin/false", dataDir)
	t.Cleanup(func() {
		if h.Security != nil {
			h.Security.Close()
		}
	})

	r := gin.New()
	r.Use(compressionMiddleware())
	h.RegisterRoutes(r.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/export", http.NoBody)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/gzip", w.Header().Get("Content-Type"))
	require.Contains(t, w.Header().Get("Content-Disposition"), "attachment; filename=crowdsec-config-")

	// Decode transport compression exactly as a client would (honouring Content-Encoding).
	var body io.Reader = w.Body
	if w.Header().Get("Content-Encoding") == "gzip" {
		zr, zerr := gzip.NewReader(body)
		require.NoError(t, zerr)
		body = zr
	}
	// What remains must be the tar.gz file itself.
	gr, err := gzip.NewReader(body)
	require.NoError(t, err, "download is not a single-layer tar.gz")
	tr := tar.NewReader(gr)
	var names []string
	for {
		hdr, nerr := tr.Next()
		if nerr == io.EOF {
			break
		}
		require.NoError(t, nerr)
		names = append(names, hdr.Name)
	}
	require.Contains(t, names, filepath.Join("config", "config.yaml"))
}
