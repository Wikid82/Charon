package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
)

type CertificateHandler struct {
	service *services.CertificateService
}

func NewCertificateHandler(service *services.CertificateService) *CertificateHandler {
	return &CertificateHandler{service: service}
}

func (h *CertificateHandler) List(c *gin.Context) {
	certs, err := h.service.ListCertificates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, certs)
}

type UploadCertificateRequest struct {
	Name        string `form:"name" binding:"required"`
	Certificate string `form:"certificate"` // PEM content
	PrivateKey  string `form:"private_key"` // PEM content
}

func (h *CertificateHandler) Upload(c *gin.Context) {
	// Handle multipart form
	name := c.PostForm("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	// Read files
	certFile, err := c.FormFile("certificate_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "certificate_file is required"})
		return
	}

	keyFile, err := c.FormFile("key_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key_file is required"})
		return
	}

	// Open and read content
	certSrc, err := certFile.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open cert file"})
		return
	}
	defer certSrc.Close()

	keySrc, err := keyFile.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open key file"})
		return
	}
	defer keySrc.Close()

	// Read to string
	// Limit size to avoid DoS (e.g. 1MB)
	certBytes := make([]byte, 1024*1024)
	n, _ := certSrc.Read(certBytes)
	certPEM := string(certBytes[:n])

	keyBytes := make([]byte, 1024*1024)
	n, _ = keySrc.Read(keyBytes)
	keyPEM := string(keyBytes[:n])

	cert, err := h.service.UploadCertificate(name, certPEM, keyPEM)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, cert)
}

func (h *CertificateHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.DeleteCertificate(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "certificate deleted"})
}
