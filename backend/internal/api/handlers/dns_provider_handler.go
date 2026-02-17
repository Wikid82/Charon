package handlers

import (
	"context"
	"net/http"
	"sort"
	"strconv"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
	"github.com/gin-gonic/gin"
)

// DNSProviderHandler handles DNS provider API requests.
type DNSProviderHandler struct {
	service services.DNSProviderService
}

// NewDNSProviderHandler creates a new DNS provider handler.
func NewDNSProviderHandler(service services.DNSProviderService) *DNSProviderHandler {
	return &DNSProviderHandler{
		service: service,
	}
}

// resolveProvider resolves a DNS provider by either numeric ID or UUID.
// It first attempts to parse as uint (backward compatibility), then tries UUID.
func (h *DNSProviderHandler) resolveProvider(ctx context.Context, idOrUUID string) (*models.DNSProvider, error) {
	// Try parsing as numeric ID first (backward compatibility)
	if id, err := strconv.ParseUint(idOrUUID, 10, 32); err == nil {
		return h.service.Get(ctx, uint(id))
	}

	// Empty string check
	if idOrUUID == "" {
		return nil, services.ErrDNSProviderNotFound
	}

	// Try as UUID
	return h.service.GetByUUID(ctx, idOrUUID)
}

// List handles GET /api/v1/dns-providers
// Returns all DNS providers without exposing credentials.
func (h *DNSProviderHandler) List(c *gin.Context) {
	providers, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list DNS providers"})
		return
	}

	// Convert to response format with has_credentials indicator
	responses := make([]services.DNSProviderResponse, len(providers))
	for i, p := range providers {
		pCopy := p // Create a copy to take address of
		responses[i] = services.NewDNSProviderResponse(&pCopy)
	}

	c.JSON(http.StatusOK, gin.H{
		"providers": responses,
		"total":     len(responses),
	})
}

// Get handles GET /api/v1/dns-providers/:id
// Returns a single DNS provider without exposing credentials.
// Accepts either numeric ID or UUID for flexibility.
func (h *DNSProviderHandler) Get(c *gin.Context) {
	provider, err := h.resolveProvider(c.Request.Context(), c.Param("id"))
	if err != nil {
		if err == services.ErrDNSProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "DNS provider not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve DNS provider"})
		return
	}

	response := services.NewDNSProviderResponse(provider)

	c.JSON(http.StatusOK, response)
}

// Create handles POST /api/v1/dns-providers
// Creates a new DNS provider with encrypted credentials.
func (h *DNSProviderHandler) Create(c *gin.Context) {
	var req services.CreateDNSProviderRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
		return
	}

	provider, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		statusCode := http.StatusBadRequest
		errorMessage := err.Error()

		switch err {
		case services.ErrInvalidProviderType:
			errorMessage = "Unsupported DNS provider type"
		case services.ErrInvalidCredentials:
			errorMessage = "Invalid credentials: missing required fields"
		case services.ErrEncryptionFailed:
			statusCode = http.StatusInternalServerError
			errorMessage = "Failed to encrypt credentials"
		}

		c.JSON(statusCode, gin.H{"error": errorMessage})
		return
	}

	response := services.NewDNSProviderResponse(provider)

	c.JSON(http.StatusCreated, response)
}

// Update handles PUT /api/v1/dns-providers/:id
// Updates an existing DNS provider.
// Accepts either numeric ID or UUID for flexibility.
func (h *DNSProviderHandler) Update(c *gin.Context) {
	// Resolve provider first to get internal ID
	provider, err := h.resolveProvider(c.Request.Context(), c.Param("id"))
	if err != nil {
		if err == services.ErrDNSProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "DNS provider not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	var req services.UpdateDNSProviderRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
		return
	}

	updatedProvider, err := h.service.Update(c.Request.Context(), provider.ID, req)
	if err != nil {
		statusCode := http.StatusBadRequest
		errorMessage := err.Error()

		switch err {
		case services.ErrDNSProviderNotFound:
			statusCode = http.StatusNotFound
			errorMessage = "DNS provider not found"
		case services.ErrInvalidCredentials:
			errorMessage = "Invalid credentials: missing required fields"
		case services.ErrEncryptionFailed:
			statusCode = http.StatusInternalServerError
			errorMessage = "Failed to encrypt credentials"
		}

		c.JSON(statusCode, gin.H{"error": errorMessage})
		return
	}

	response := services.NewDNSProviderResponse(updatedProvider)

	c.JSON(http.StatusOK, response)
}

// Delete handles DELETE /api/v1/dns-providers/:id
// Deletes a DNS provider.
// Accepts either numeric ID or UUID for flexibility.
func (h *DNSProviderHandler) Delete(c *gin.Context) {
	// Resolve provider first to get internal ID
	provider, err := h.resolveProvider(c.Request.Context(), c.Param("id"))
	if err != nil {
		if err == services.ErrDNSProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "DNS provider not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	err = h.service.Delete(c.Request.Context(), provider.ID)
	if err != nil {
		if err == services.ErrDNSProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "DNS provider not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete DNS provider"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "DNS provider deleted successfully"})
}

// Test handles POST /api/v1/dns-providers/:id/test
// Tests a saved DNS provider's credentials.
// Accepts either numeric ID or UUID for flexibility.
func (h *DNSProviderHandler) Test(c *gin.Context) {
	// Resolve provider first to get internal ID
	provider, err := h.resolveProvider(c.Request.Context(), c.Param("id"))
	if err != nil {
		if err == services.ErrDNSProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "DNS provider not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	result, err := h.service.Test(c.Request.Context(), provider.ID)
	if err != nil {
		if err == services.ErrDNSProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "DNS provider not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to test DNS provider"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// TestCredentials handles POST /api/v1/dns-providers/test
// Tests DNS provider credentials without saving them.
func (h *DNSProviderHandler) TestCredentials(c *gin.Context) {
	var req services.CreateDNSProviderRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
		return
	}

	result, err := h.service.TestCredentials(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to test credentials"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetTypes handles GET /api/v1/dns-providers/types
// Returns the list of supported DNS provider types with their required fields.
// Types are sourced from the provider registry (built-in, custom, and external plugins).
func (h *DNSProviderHandler) GetTypes(c *gin.Context) {
	// Get all registered providers from the global registry
	providers := dnsprovider.Global().List()

	// Build response with provider metadata and fields
	types := make([]gin.H, 0, len(providers))
	for _, provider := range providers {
		metadata := provider.Metadata()

		// Combine required and optional fields
		requiredFields := provider.RequiredCredentialFields()
		optionalFields := provider.OptionalCredentialFields()

		// Convert fields to response format with required flag
		fields := make([]gin.H, 0, len(requiredFields)+len(optionalFields))

		for _, f := range requiredFields {
			field := gin.H{
				"name":     f.Name,
				"label":    f.Label,
				"type":     f.Type,
				"required": true,
			}
			if f.Placeholder != "" {
				field["placeholder"] = f.Placeholder
			}
			if f.Hint != "" {
				field["hint"] = f.Hint
			}
			if len(f.Options) > 0 {
				field["options"] = f.Options
			}
			fields = append(fields, field)
		}

		for _, f := range optionalFields {
			field := gin.H{
				"name":     f.Name,
				"label":    f.Label,
				"type":     f.Type,
				"required": false,
			}
			if f.Placeholder != "" {
				field["placeholder"] = f.Placeholder
			}
			if f.Hint != "" {
				field["hint"] = f.Hint
			}
			if len(f.Options) > 0 {
				field["options"] = f.Options
			}
			fields = append(fields, field)
		}

		providerType := gin.H{
			"type":        metadata.Type,
			"name":        metadata.Name,
			"description": metadata.Description,
			"is_built_in": metadata.IsBuiltIn,
			"fields":      fields,
		}

		if metadata.DocumentationURL != "" {
			providerType["documentation_url"] = metadata.DocumentationURL
		}

		types = append(types, providerType)
	}

	// Sort by type for stable, predictable output (registry.List already sorts, but explicit for safety)
	sort.Slice(types, func(i, j int) bool {
		return types[i]["type"].(string) < types[j]["type"].(string)
	})

	c.JSON(http.StatusOK, gin.H{
		"types": types,
	})
}
