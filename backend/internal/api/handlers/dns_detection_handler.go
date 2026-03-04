package handlers

import (
	"net/http"

	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
)

// DNSDetectionHandler handles DNS provider auto-detection API requests.
type DNSDetectionHandler struct {
	service services.DNSDetectionService
}

// NewDNSDetectionHandler creates a new DNS detection handler.
func NewDNSDetectionHandler(service services.DNSDetectionService) *DNSDetectionHandler {
	return &DNSDetectionHandler{
		service: service,
	}
}

// DetectRequest represents the request body for DNS provider detection.
type DetectRequest struct {
	Domain string `json:"domain" binding:"required"`
}

// Detect handles POST /api/v1/dns-providers/detect
// Performs DNS provider auto-detection for a given domain.
func (h *DNSDetectionHandler) Detect(c *gin.Context) {
	var req DetectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain is required"})
		return
	}

	// Perform detection
	result, err := h.service.DetectProvider(req.Domain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to detect DNS provider"})
		return
	}

	// If detected, try to find a matching configured provider
	if result.Detected {
		suggestedProvider, err := h.service.SuggestConfiguredProvider(c.Request.Context(), req.Domain)
		if err == nil && suggestedProvider != nil {
			result.SuggestedProvider = suggestedProvider
		}
	}

	c.JSON(http.StatusOK, result)
}

// GetPatterns handles GET /api/v1/dns-providers/detection-patterns
// Returns the current nameserver pattern database.
func (h *DNSDetectionHandler) GetPatterns(c *gin.Context) {
	patterns := h.service.GetNameserverPatterns()

	// Convert to structured response
	type ProviderPattern struct {
		Pattern      string `json:"pattern"`
		ProviderType string `json:"provider_type"`
	}

	patternsList := make([]ProviderPattern, 0, len(patterns))
	for pattern, providerType := range patterns {
		patternsList = append(patternsList, ProviderPattern{
			Pattern:      pattern,
			ProviderType: providerType,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"patterns": patternsList,
		"total":    len(patternsList),
	})
}
