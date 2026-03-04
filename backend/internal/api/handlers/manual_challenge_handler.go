package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
)

// ManualChallengeServiceInterface defines the interface for manual challenge operations.
// This allows for easier testing by enabling mock implementations.
type ManualChallengeServiceInterface interface {
	CreateChallenge(ctx context.Context, req services.CreateChallengeRequest) (*models.ManualChallenge, error)
	GetChallengeForUser(ctx context.Context, challengeID string, userID uint) (*models.ManualChallenge, error)
	ListChallengesForProvider(ctx context.Context, providerID, userID uint) ([]models.ManualChallenge, error)
	VerifyChallenge(ctx context.Context, challengeID string, userID uint) (*services.VerifyResult, error)
	PollChallengeStatus(ctx context.Context, challengeID string, userID uint) (*services.ChallengeStatusResponse, error)
	DeleteChallenge(ctx context.Context, challengeID string, userID uint) error
}

// DNSProviderServiceInterface defines the subset of DNSProviderService needed by ManualChallengeHandler.
type DNSProviderServiceInterface interface {
	Get(ctx context.Context, id uint) (*models.DNSProvider, error)
}

// ManualChallengeHandler handles manual DNS challenge API requests.
type ManualChallengeHandler struct {
	challengeService ManualChallengeServiceInterface
	providerService  DNSProviderServiceInterface
}

// NewManualChallengeHandler creates a new manual challenge handler.
func NewManualChallengeHandler(challengeService ManualChallengeServiceInterface, providerService DNSProviderServiceInterface) *ManualChallengeHandler {
	return &ManualChallengeHandler{
		challengeService: challengeService,
		providerService:  providerService,
	}
}

// ManualChallengeResponse represents the API response for a manual challenge.
type ManualChallengeResponse struct {
	ID                   string `json:"id"`
	ProviderID           uint   `json:"provider_id"`
	FQDN                 string `json:"fqdn"`
	Value                string `json:"value"`
	Status               string `json:"status"`
	DNSPropagated        bool   `json:"dns_propagated"`
	CreatedAt            string `json:"created_at"`
	ExpiresAt            string `json:"expires_at"`
	LastCheckAt          string `json:"last_check_at,omitempty"`
	TimeRemainingSeconds int    `json:"time_remaining_seconds"`
	ErrorMessage         string `json:"error_message,omitempty"`
}

// ErrorResponse represents an error response with a code.
type ErrorResponse struct {
	Success bool `json:"success"`
	Error   struct {
		Code    string                 `json:"code"`
		Message string                 `json:"message"`
		Details map[string]interface{} `json:"details,omitempty"`
	} `json:"error"`
}

// newErrorResponse creates a standardized error response.
func newErrorResponse(code, message string, details map[string]interface{}) ErrorResponse {
	resp := ErrorResponse{Success: false}
	resp.Error.Code = code
	resp.Error.Message = message
	resp.Error.Details = details
	return resp
}

// GetChallenge handles GET /api/v1/dns-providers/:id/manual-challenge/:challengeId
// Returns the status and details of a manual DNS challenge.
func (h *ManualChallengeHandler) GetChallenge(c *gin.Context) {
	providerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_PROVIDER_ID",
			"Invalid provider ID",
			nil,
		))
		return
	}

	challengeID := c.Param("challengeId")
	if challengeID == "" {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_CHALLENGE_ID",
			"Challenge ID is required",
			nil,
		))
		return
	}

	// Get user ID from context (set by auth middleware)
	userID := getUserIDFromContext(c)

	// Verify provider exists and user has access
	provider, err := h.providerService.Get(c.Request.Context(), uint(providerID))
	if err != nil {
		if errors.Is(err, services.ErrDNSProviderNotFound) {
			c.JSON(http.StatusNotFound, newErrorResponse(
				"PROVIDER_NOT_FOUND",
				"DNS provider not found",
				nil,
			))
			return
		}
		c.JSON(http.StatusInternalServerError, newErrorResponse(
			"INTERNAL_ERROR",
			"Failed to retrieve DNS provider",
			nil,
		))
		return
	}

	// Verify provider is manual type
	if provider.ProviderType != "manual" {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_PROVIDER_TYPE",
			"This endpoint is only available for manual DNS providers",
			nil,
		))
		return
	}

	// Get challenge
	challenge, err := h.challengeService.GetChallengeForUser(c.Request.Context(), challengeID, userID)
	if err != nil {
		if errors.Is(err, services.ErrChallengeNotFound) {
			c.JSON(http.StatusNotFound, newErrorResponse(
				"CHALLENGE_NOT_FOUND",
				"Challenge not found",
				map[string]interface{}{"challenge_id": challengeID},
			))
			return
		}
		if errors.Is(err, services.ErrUnauthorized) {
			c.JSON(http.StatusForbidden, newErrorResponse(
				"UNAUTHORIZED",
				"You do not have access to this challenge",
				nil,
			))
			return
		}
		c.JSON(http.StatusInternalServerError, newErrorResponse(
			"INTERNAL_ERROR",
			"Failed to retrieve challenge",
			nil,
		))
		return
	}

	// Verify challenge belongs to the specified provider
	if challenge.ProviderID != uint(providerID) {
		c.JSON(http.StatusNotFound, newErrorResponse(
			"CHALLENGE_NOT_FOUND",
			"Challenge not found for this provider",
			nil,
		))
		return
	}

	c.JSON(http.StatusOK, challengeToResponse(challenge))
}

// VerifyChallenge handles POST /api/v1/dns-providers/:id/manual-challenge/:challengeId/verify
// Triggers DNS verification for a challenge.
func (h *ManualChallengeHandler) VerifyChallenge(c *gin.Context) {
	providerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_PROVIDER_ID",
			"Invalid provider ID",
			nil,
		))
		return
	}

	challengeID := c.Param("challengeId")
	if challengeID == "" {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_CHALLENGE_ID",
			"Challenge ID is required",
			nil,
		))
		return
	}

	// Get user ID from context
	userID := getUserIDFromContext(c)

	// Verify provider exists and is manual type
	provider, err := h.providerService.Get(c.Request.Context(), uint(providerID))
	if err != nil {
		if errors.Is(err, services.ErrDNSProviderNotFound) {
			c.JSON(http.StatusNotFound, newErrorResponse(
				"PROVIDER_NOT_FOUND",
				"DNS provider not found",
				nil,
			))
			return
		}
		c.JSON(http.StatusInternalServerError, newErrorResponse(
			"INTERNAL_ERROR",
			"Failed to retrieve DNS provider",
			nil,
		))
		return
	}

	if provider.ProviderType != "manual" {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_PROVIDER_TYPE",
			"This endpoint is only available for manual DNS providers",
			nil,
		))
		return
	}

	// Verify ownership before verification
	challenge, err := h.challengeService.GetChallengeForUser(c.Request.Context(), challengeID, userID)
	if err != nil {
		if errors.Is(err, services.ErrChallengeNotFound) {
			c.JSON(http.StatusNotFound, newErrorResponse(
				"CHALLENGE_NOT_FOUND",
				"Challenge not found",
				map[string]interface{}{"challenge_id": challengeID},
			))
			return
		}
		if errors.Is(err, services.ErrUnauthorized) {
			c.JSON(http.StatusForbidden, newErrorResponse(
				"UNAUTHORIZED",
				"You do not have access to this challenge",
				nil,
			))
			return
		}
		c.JSON(http.StatusInternalServerError, newErrorResponse(
			"INTERNAL_ERROR",
			"Failed to retrieve challenge",
			nil,
		))
		return
	}

	if challenge.ProviderID != uint(providerID) {
		c.JSON(http.StatusNotFound, newErrorResponse(
			"CHALLENGE_NOT_FOUND",
			"Challenge not found for this provider",
			nil,
		))
		return
	}

	// Perform verification
	result, err := h.challengeService.VerifyChallenge(c.Request.Context(), challengeID, userID)
	if err != nil {
		if errors.Is(err, services.ErrChallengeExpired) {
			c.JSON(http.StatusGone, newErrorResponse(
				"CHALLENGE_EXPIRED",
				"Challenge has expired",
				map[string]interface{}{
					"challenge_id": challengeID,
					"expired_at":   challenge.ExpiresAt.Format("2006-01-02T15:04:05Z"),
				},
			))
			return
		}
		c.JSON(http.StatusInternalServerError, newErrorResponse(
			"INTERNAL_ERROR",
			"Failed to verify challenge",
			nil,
		))
		return
	}

	c.JSON(http.StatusOK, result)
}

// PollChallenge handles GET /api/v1/dns-providers/:id/manual-challenge/:challengeId/poll
// Returns the current status for polling.
func (h *ManualChallengeHandler) PollChallenge(c *gin.Context) {
	providerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_PROVIDER_ID",
			"Invalid provider ID",
			nil,
		))
		return
	}

	challengeID := c.Param("challengeId")
	if challengeID == "" {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_CHALLENGE_ID",
			"Challenge ID is required",
			nil,
		))
		return
	}

	userID := getUserIDFromContext(c)

	// Verify provider exists
	provider, err := h.providerService.Get(c.Request.Context(), uint(providerID))
	if err != nil {
		if errors.Is(err, services.ErrDNSProviderNotFound) {
			c.JSON(http.StatusNotFound, newErrorResponse(
				"PROVIDER_NOT_FOUND",
				"DNS provider not found",
				nil,
			))
			return
		}
		c.JSON(http.StatusInternalServerError, newErrorResponse(
			"INTERNAL_ERROR",
			"Failed to retrieve DNS provider",
			nil,
		))
		return
	}

	if provider.ProviderType != "manual" {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_PROVIDER_TYPE",
			"This endpoint is only available for manual DNS providers",
			nil,
		))
		return
	}

	// Get challenge status
	status, err := h.challengeService.PollChallengeStatus(c.Request.Context(), challengeID, userID)
	if err != nil {
		if errors.Is(err, services.ErrChallengeNotFound) {
			c.JSON(http.StatusNotFound, newErrorResponse(
				"CHALLENGE_NOT_FOUND",
				"Challenge not found",
				nil,
			))
			return
		}
		if errors.Is(err, services.ErrUnauthorized) {
			c.JSON(http.StatusForbidden, newErrorResponse(
				"UNAUTHORIZED",
				"You do not have access to this challenge",
				nil,
			))
			return
		}
		c.JSON(http.StatusInternalServerError, newErrorResponse(
			"INTERNAL_ERROR",
			"Failed to get challenge status",
			nil,
		))
		return
	}

	c.JSON(http.StatusOK, status)
}

// ListChallenges handles GET /api/v1/dns-providers/:id/manual-challenges
// Returns all challenges for a provider.
func (h *ManualChallengeHandler) ListChallenges(c *gin.Context) {
	providerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_PROVIDER_ID",
			"Invalid provider ID",
			nil,
		))
		return
	}

	userID := getUserIDFromContext(c)

	// Verify provider exists and is manual type
	provider, err := h.providerService.Get(c.Request.Context(), uint(providerID))
	if err != nil {
		if errors.Is(err, services.ErrDNSProviderNotFound) {
			c.JSON(http.StatusNotFound, newErrorResponse(
				"PROVIDER_NOT_FOUND",
				"DNS provider not found",
				nil,
			))
			return
		}
		c.JSON(http.StatusInternalServerError, newErrorResponse(
			"INTERNAL_ERROR",
			"Failed to retrieve DNS provider",
			nil,
		))
		return
	}

	if provider.ProviderType != "manual" {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_PROVIDER_TYPE",
			"This endpoint is only available for manual DNS providers",
			nil,
		))
		return
	}

	challenges, err := h.challengeService.ListChallengesForProvider(c.Request.Context(), uint(providerID), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, newErrorResponse(
			"INTERNAL_ERROR",
			"Failed to list challenges",
			nil,
		))
		return
	}

	responses := make([]ManualChallengeResponse, len(challenges))
	for i, ch := range challenges {
		responses[i] = *challengeToResponse(&ch)
	}

	c.JSON(http.StatusOK, gin.H{
		"challenges": responses,
		"total":      len(responses),
	})
}

// DeleteChallenge handles DELETE /api/v1/dns-providers/:id/manual-challenge/:challengeId
// Cancels/deletes a challenge.
func (h *ManualChallengeHandler) DeleteChallenge(c *gin.Context) {
	providerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_PROVIDER_ID",
			"Invalid provider ID",
			nil,
		))
		return
	}

	challengeID := c.Param("challengeId")
	if challengeID == "" {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_CHALLENGE_ID",
			"Challenge ID is required",
			nil,
		))
		return
	}

	userID := getUserIDFromContext(c)

	// Verify provider exists
	provider, err := h.providerService.Get(c.Request.Context(), uint(providerID))
	if err != nil {
		if errors.Is(err, services.ErrDNSProviderNotFound) {
			c.JSON(http.StatusNotFound, newErrorResponse(
				"PROVIDER_NOT_FOUND",
				"DNS provider not found",
				nil,
			))
			return
		}
		c.JSON(http.StatusInternalServerError, newErrorResponse(
			"INTERNAL_ERROR",
			"Failed to retrieve DNS provider",
			nil,
		))
		return
	}

	if provider.ProviderType != "manual" {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_PROVIDER_TYPE",
			"This endpoint is only available for manual DNS providers",
			nil,
		))
		return
	}

	err = h.challengeService.DeleteChallenge(c.Request.Context(), challengeID, userID)
	if err != nil {
		if errors.Is(err, services.ErrChallengeNotFound) {
			c.JSON(http.StatusNotFound, newErrorResponse(
				"CHALLENGE_NOT_FOUND",
				"Challenge not found",
				nil,
			))
			return
		}
		if errors.Is(err, services.ErrUnauthorized) {
			c.JSON(http.StatusForbidden, newErrorResponse(
				"UNAUTHORIZED",
				"You do not have access to this challenge",
				nil,
			))
			return
		}
		c.JSON(http.StatusInternalServerError, newErrorResponse(
			"INTERNAL_ERROR",
			"Failed to delete challenge",
			nil,
		))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Challenge deleted successfully",
	})
}

// CreateChallengeRequest represents the request to create a manual challenge.
type CreateChallengeRequest struct {
	FQDN  string `json:"fqdn" binding:"required"`
	Token string `json:"token"`
	Value string `json:"value" binding:"required"`
}

// CreateChallenge handles POST /api/v1/dns-providers/:id/manual-challenges
// Creates a new manual DNS challenge.
func (h *ManualChallengeHandler) CreateChallenge(c *gin.Context) {
	providerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_PROVIDER_ID",
			"Invalid provider ID",
			nil,
		))
		return
	}

	var req CreateChallengeRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_REQUEST",
			bindErr.Error(),
			nil,
		))
		return
	}

	userID := getUserIDFromContext(c)

	// Verify provider exists and is manual type
	provider, err := h.providerService.Get(c.Request.Context(), uint(providerID))
	if err != nil {
		if errors.Is(err, services.ErrDNSProviderNotFound) {
			c.JSON(http.StatusNotFound, newErrorResponse(
				"PROVIDER_NOT_FOUND",
				"DNS provider not found",
				nil,
			))
			return
		}
		c.JSON(http.StatusInternalServerError, newErrorResponse(
			"INTERNAL_ERROR",
			"Failed to retrieve DNS provider",
			nil,
		))
		return
	}

	if provider.ProviderType != "manual" {
		c.JSON(http.StatusBadRequest, newErrorResponse(
			"INVALID_PROVIDER_TYPE",
			"This endpoint is only available for manual DNS providers",
			nil,
		))
		return
	}

	challenge, err := h.challengeService.CreateChallenge(c.Request.Context(), services.CreateChallengeRequest{
		ProviderID: uint(providerID),
		UserID:     userID,
		FQDN:       req.FQDN,
		Token:      req.Token,
		Value:      req.Value,
	})
	if err != nil {
		if errors.Is(err, services.ErrChallengeInProgress) {
			c.JSON(http.StatusConflict, newErrorResponse(
				"CHALLENGE_IN_PROGRESS",
				"Another challenge is already in progress for this domain",
				map[string]interface{}{"fqdn": req.FQDN},
			))
			return
		}
		c.JSON(http.StatusInternalServerError, newErrorResponse(
			"INTERNAL_ERROR",
			"Failed to create challenge",
			nil,
		))
		return
	}

	c.JSON(http.StatusCreated, challengeToResponse(challenge))
}

// RegisterRoutes registers all manual challenge routes.
func (h *ManualChallengeHandler) RegisterRoutes(rg *gin.RouterGroup) {
	// Routes under /dns-providers/:id
	rg.GET("/dns-providers/:id/manual-challenges", h.ListChallenges)
	rg.POST("/dns-providers/:id/manual-challenges", h.CreateChallenge)
	rg.GET("/dns-providers/:id/manual-challenge/:challengeId", h.GetChallenge)
	rg.POST("/dns-providers/:id/manual-challenge/:challengeId/verify", h.VerifyChallenge)
	rg.GET("/dns-providers/:id/manual-challenge/:challengeId/poll", h.PollChallenge)
	rg.DELETE("/dns-providers/:id/manual-challenge/:challengeId", h.DeleteChallenge)
}

// Helper functions

func challengeToResponse(ch *models.ManualChallenge) *ManualChallengeResponse {
	resp := &ManualChallengeResponse{
		ID:                   ch.ID,
		ProviderID:           ch.ProviderID,
		FQDN:                 ch.FQDN,
		Value:                ch.Value,
		Status:               string(ch.Status),
		DNSPropagated:        ch.DNSPropagated,
		CreatedAt:            ch.CreatedAt.Format("2006-01-02T15:04:05Z"),
		ExpiresAt:            ch.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		TimeRemainingSeconds: int(ch.TimeRemaining().Seconds()),
		ErrorMessage:         ch.ErrorMessage,
	}

	if ch.LastCheckAt != nil {
		resp.LastCheckAt = ch.LastCheckAt.Format("2006-01-02T15:04:05Z")
	}

	return resp
}

// getUserIDFromContext extracts user ID from gin context.
func getUserIDFromContext(c *gin.Context) uint {
	// Try to get user_id from context (set by auth middleware)
	if userID, exists := c.Get("user_id"); exists {
		switch v := userID.(type) {
		case uint:
			return v
		case int:
			// Check for overflow when converting int -> uint
			if v < 0 {
				return 0 // Invalid negative ID
			}
			return uint(v) // #nosec G115 -- validated non-negative
		case int64:
			// Check for overflow when converting int64 -> uint
			// Use simple bounds check instead of complex expression
			if v < 0 || v > 4294967295 { // Max uint32, safe for most systems
				return 0 // Out of valid range
			}
			return uint(v) // #nosec G115 -- validated range
		case uint64:
			return uint(v)
		}
	}
	return 0
}
