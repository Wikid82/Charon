package handlers

import (
	"net/http"
	"strconv"

	"github.com/Wikid82/charon/backend/internal/services"
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
		responses[i] = services.DNSProviderResponse{
			DNSProvider:    p,
			HasCredentials: p.CredentialsEncrypted != "",
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"providers": responses,
		"total":     len(responses),
	})
}

// Get handles GET /api/v1/dns-providers/:id
// Returns a single DNS provider without exposing credentials.
func (h *DNSProviderHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	provider, err := h.service.Get(c.Request.Context(), uint(id))
	if err != nil {
		if err == services.ErrDNSProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "DNS provider not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve DNS provider"})
		return
	}

	response := services.DNSProviderResponse{
		DNSProvider:    *provider,
		HasCredentials: provider.CredentialsEncrypted != "",
	}

	c.JSON(http.StatusOK, response)
}

// Create handles POST /api/v1/dns-providers
// Creates a new DNS provider with encrypted credentials.
func (h *DNSProviderHandler) Create(c *gin.Context) {
	var req services.CreateDNSProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	response := services.DNSProviderResponse{
		DNSProvider:    *provider,
		HasCredentials: provider.CredentialsEncrypted != "",
	}

	c.JSON(http.StatusCreated, response)
}

// Update handles PUT /api/v1/dns-providers/:id
// Updates an existing DNS provider.
func (h *DNSProviderHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	var req services.UpdateDNSProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	provider, err := h.service.Update(c.Request.Context(), uint(id), req)
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

	response := services.DNSProviderResponse{
		DNSProvider:    *provider,
		HasCredentials: provider.CredentialsEncrypted != "",
	}

	c.JSON(http.StatusOK, response)
}

// Delete handles DELETE /api/v1/dns-providers/:id
// Deletes a DNS provider.
func (h *DNSProviderHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	err = h.service.Delete(c.Request.Context(), uint(id))
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
func (h *DNSProviderHandler) Test(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	result, err := h.service.Test(c.Request.Context(), uint(id))
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
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
func (h *DNSProviderHandler) GetTypes(c *gin.Context) {
	types := []gin.H{
		{
			"type": "cloudflare",
			"name": "Cloudflare",
			"fields": []gin.H{
				{
					"name":     "api_token",
					"label":    "API Token",
					"type":     "password",
					"required": true,
					"hint":     "Token with Zone:DNS:Edit permissions",
				},
			},
			"documentation_url": "https://developers.cloudflare.com/api/tokens/",
		},
		{
			"type": "route53",
			"name": "Amazon Route 53",
			"fields": []gin.H{
				{
					"name":     "access_key_id",
					"label":    "Access Key ID",
					"type":     "text",
					"required": true,
				},
				{
					"name":     "secret_access_key",
					"label":    "Secret Access Key",
					"type":     "password",
					"required": true,
				},
				{
					"name":     "region",
					"label":    "AWS Region",
					"type":     "text",
					"required": true,
					"default":  "us-east-1",
				},
			},
			"documentation_url": "https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/dns-routing-traffic.html",
		},
		{
			"type": "digitalocean",
			"name": "DigitalOcean",
			"fields": []gin.H{
				{
					"name":     "auth_token",
					"label":    "API Token",
					"type":     "password",
					"required": true,
					"hint":     "Personal Access Token with read/write scope",
				},
			},
			"documentation_url": "https://docs.digitalocean.com/reference/api/api-reference/",
		},
		{
			"type": "googleclouddns",
			"name": "Google Cloud DNS",
			"fields": []gin.H{
				{
					"name":     "service_account_json",
					"label":    "Service Account JSON",
					"type":     "textarea",
					"required": true,
					"hint":     "JSON key file for service account with DNS Administrator role",
				},
				{
					"name":     "project",
					"label":    "Project ID",
					"type":     "text",
					"required": true,
				},
			},
			"documentation_url": "https://cloud.google.com/dns/docs/",
		},
		{
			"type": "namecheap",
			"name": "Namecheap",
			"fields": []gin.H{
				{
					"name":     "api_user",
					"label":    "API Username",
					"type":     "text",
					"required": true,
				},
				{
					"name":     "api_key",
					"label":    "API Key",
					"type":     "password",
					"required": true,
				},
				{
					"name":     "client_ip",
					"label":    "Client IP Address",
					"type":     "text",
					"required": true,
					"hint":     "Your server's public IP address (whitelisted in Namecheap)",
				},
			},
			"documentation_url": "https://www.namecheap.com/support/api/intro/",
		},
		{
			"type": "godaddy",
			"name": "GoDaddy",
			"fields": []gin.H{
				{
					"name":     "api_key",
					"label":    "API Key",
					"type":     "text",
					"required": true,
				},
				{
					"name":     "api_secret",
					"label":    "API Secret",
					"type":     "password",
					"required": true,
				},
			},
			"documentation_url": "https://developer.godaddy.com/",
		},
		{
			"type": "azure",
			"name": "Azure DNS",
			"fields": []gin.H{
				{
					"name":     "tenant_id",
					"label":    "Tenant ID",
					"type":     "text",
					"required": true,
				},
				{
					"name":     "client_id",
					"label":    "Client ID",
					"type":     "text",
					"required": true,
				},
				{
					"name":     "client_secret",
					"label":    "Client Secret",
					"type":     "password",
					"required": true,
				},
				{
					"name":     "subscription_id",
					"label":    "Subscription ID",
					"type":     "text",
					"required": true,
				},
				{
					"name":     "resource_group",
					"label":    "Resource Group",
					"type":     "text",
					"required": true,
				},
			},
			"documentation_url": "https://docs.microsoft.com/en-us/azure/dns/",
		},
		{
			"type": "hetzner",
			"name": "Hetzner",
			"fields": []gin.H{
				{
					"name":     "api_key",
					"label":    "API Key",
					"type":     "password",
					"required": true,
				},
			},
			"documentation_url": "https://docs.hetzner.com/dns-console/dns/general/dns-overview/",
		},
		{
			"type": "vultr",
			"name": "Vultr",
			"fields": []gin.H{
				{
					"name":     "api_key",
					"label":    "API Key",
					"type":     "password",
					"required": true,
				},
			},
			"documentation_url": "https://www.vultr.com/api/",
		},
		{
			"type": "dnsimple",
			"name": "DNSimple",
			"fields": []gin.H{
				{
					"name":     "oauth_token",
					"label":    "OAuth Token",
					"type":     "password",
					"required": true,
				},
				{
					"name":     "account_id",
					"label":    "Account ID",
					"type":     "text",
					"required": true,
				},
			},
			"documentation_url": "https://developer.dnsimple.com/",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"types": types,
	})
}
