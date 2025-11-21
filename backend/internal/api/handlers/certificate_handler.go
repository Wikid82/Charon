package handlers

import (
	"net/http"

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
