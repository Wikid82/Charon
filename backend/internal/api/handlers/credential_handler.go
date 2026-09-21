package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
)

// CredentialHandler handles HTTP requests for DNS provider credentials.
type CredentialHandler struct {
	credentialService  services.CredentialService
	dnsProviderService services.DNSProviderService
}

// NewCredentialHandler creates a new credential handler.
func NewCredentialHandler(credentialService services.CredentialService, dnsProviderService services.DNSProviderService) *CredentialHandler {
	return &CredentialHandler{
		credentialService:  credentialService,
		dnsProviderService: dnsProviderService,
	}
}

// resolveProviderID resolves the :id path param — accepting either a legacy
// numeric ID (back-compat) or the DNS provider's UUID, since DNSProviderResponse
// never exposes the internal numeric ID to clients — to the provider's internal
// numeric ID. On failure it writes the appropriate error response itself and
// returns ok=false so the caller can return immediately: 400 for a value that
// is neither a valid numeric ID nor a syntactically valid UUID, 404 for a
// well-formed but nonexistent one, and 500 for any other (e.g. storage) error.
func (h *CredentialHandler) resolveProviderID(c *gin.Context) (uint, bool) {
	providerID, err := h.dnsProviderService.ResolveID(c.Request.Context(), c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrDNSProviderNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "DNS provider not found"})
		case errors.Is(err, services.ErrInvalidDNSProviderIdentifier):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return 0, false
	}
	return providerID, true
}

// List handles GET /api/v1/dns-providers/:id/credentials
func (h *CredentialHandler) List(c *gin.Context) {
	providerID, ok := h.resolveProviderID(c)
	if !ok {
		return
	}

	credentials, err := h.credentialService.List(c.Request.Context(), providerID)
	if err != nil {
		if err == services.ErrDNSProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "DNS provider not found"})
			return
		}
		if err == services.ErrMultiCredentialNotEnabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Multi-credential mode not enabled for this provider"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, credentials)
}

// Create handles POST /api/v1/dns-providers/:id/credentials
func (h *CredentialHandler) Create(c *gin.Context) {
	providerID, ok := h.resolveProviderID(c)
	if !ok {
		return
	}

	var req services.CreateCredentialRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
		return
	}

	credential, err := h.credentialService.Create(c.Request.Context(), providerID, req)
	if err != nil {
		if err == services.ErrDNSProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "DNS provider not found"})
			return
		}
		if err == services.ErrMultiCredentialNotEnabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Multi-credential mode not enabled for this provider"})
			return
		}
		if err == services.ErrInvalidProviderType || err == services.ErrInvalidCredentials {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err == services.ErrEncryptionFailed {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, credential)
}

// Get handles GET /api/v1/dns-providers/:id/credentials/:cred_id
func (h *CredentialHandler) Get(c *gin.Context) {
	providerID, ok := h.resolveProviderID(c)
	if !ok {
		return
	}

	credentialID, err := strconv.ParseUint(c.Param("cred_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid credential ID"})
		return
	}

	credential, err := h.credentialService.Get(c.Request.Context(), providerID, uint(credentialID))
	if err != nil {
		if err == services.ErrCredentialNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Credential not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, credential)
}

// Update handles PUT /api/v1/dns-providers/:id/credentials/:cred_id
func (h *CredentialHandler) Update(c *gin.Context) {
	providerID, ok := h.resolveProviderID(c)
	if !ok {
		return
	}

	credentialID, err := strconv.ParseUint(c.Param("cred_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid credential ID"})
		return
	}

	var req services.UpdateCredentialRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
		return
	}

	credential, err := h.credentialService.Update(c.Request.Context(), providerID, uint(credentialID), req)
	if err != nil {
		if err == services.ErrCredentialNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Credential not found"})
			return
		}
		if err == services.ErrInvalidProviderType || err == services.ErrInvalidCredentials {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err == services.ErrEncryptionFailed {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, credential)
}

// Delete handles DELETE /api/v1/dns-providers/:id/credentials/:cred_id
func (h *CredentialHandler) Delete(c *gin.Context) {
	providerID, ok := h.resolveProviderID(c)
	if !ok {
		return
	}

	credentialID, err := strconv.ParseUint(c.Param("cred_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid credential ID"})
		return
	}

	if err := h.credentialService.Delete(c.Request.Context(), providerID, uint(credentialID)); err != nil {
		if err == services.ErrCredentialNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Credential not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// Test handles POST /api/v1/dns-providers/:id/credentials/:cred_id/test
func (h *CredentialHandler) Test(c *gin.Context) {
	providerID, ok := h.resolveProviderID(c)
	if !ok {
		return
	}

	credentialID, err := strconv.ParseUint(c.Param("cred_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid credential ID"})
		return
	}

	result, err := h.credentialService.Test(c.Request.Context(), providerID, uint(credentialID))
	if err != nil {
		if err == services.ErrCredentialNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Credential not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// EnableMultiCredentials handles POST /api/v1/dns-providers/:id/enable-multi-credentials
func (h *CredentialHandler) EnableMultiCredentials(c *gin.Context) {
	providerID, ok := h.resolveProviderID(c)
	if !ok {
		return
	}

	if err := h.credentialService.EnableMultiCredentials(c.Request.Context(), providerID); err != nil {
		if err == services.ErrDNSProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "DNS provider not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Multi-credential mode enabled successfully"})
}
