package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// uploadPEMPair posts a certificate and key as multipart form files.
func uploadPEMPair(t *testing.T, router *gin.Engine, certPEM, keyPEM string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("name", "needs-key"))
	part, err := writer.CreateFormFile("certificate_file", "cert.pem")
	require.NoError(t, err)
	_, err = part.Write([]byte(certPEM))
	require.NoError(t, err)
	keyPart, err := writer.CreateFormFile("key_file", "key.pem")
	require.NoError(t, err)
	_, err = keyPart.Write([]byte(keyPEM))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/certificates", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestCertificateHandler_Upload_RefusesKeyWithoutEncryptionKey(t *testing.T) {
	// E23 at the HTTP layer: a 400 with a stable error code and nothing stored.
	db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))

	svc := services.NewCertificateService(t.TempDir(), db, nil)
	h := NewCertificateHandler(svc, nil, nil)
	r := gin.New()
	r.Use(mockAuthMiddleware())
	r.POST("/api/certificates", h.Upload)

	certPEM, keyPEM, err := generateSelfSignedCertPEM()
	require.NoError(t, err)
	w := uploadPEMPair(t, r, certPEM, keyPEM)

	require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	var body struct {
		Error     string `json:"error"`
		ErrorCode string `json:"error_code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "Set a valid encryption key (CHARON_ENCRYPTION_KEY) before uploading a certificate with a private key.", body.Error)
	assert.Equal(t, "encryption_key_required", body.ErrorCode)

	var count int64
	require.NoError(t, db.Model(&models.SSLCertificate{}).Count(&count).Error)
	assert.Zero(t, count, "a refused upload must store nothing")
}
