package handlers

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// A refused update puts the previous target back.
func TestRedirectionHostHandler_Update_ApplyConfigFailure_RestoresTarget(t *testing.T) {
	caddyServer := failingCaddyServer(t)
	router, db := setupRedirectionHostHandlerRouterWithCaddy(t, caddyServer.URL)
	host := models.RedirectionHost{
		UUID:        "rh-update-apply-fail",
		DomainNames: "update-apply-fail.example.com",
		TargetURL:   "https://before.example.com",
		StatusCode:  301,
		Enabled:     true,
	}
	require.NoError(t, db.Create(&host).Error)

	w := doRequest(router, http.MethodPut, "/redirection-hosts/"+host.UUID, map[string]any{"target_url": "https://refused.example.com"})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	result := decodeBody(t, w)
	assert.Equal(t, "proxy_apply_failed", result["error_code"])
	var stored models.RedirectionHost
	require.NoError(t, db.First(&stored, host.ID).Error)
	assert.Equal(t, "https://before.example.com", stored.TargetURL)
}

// A refused delete puts the host back with its id and uuid.
func TestRedirectionHostHandler_Delete_ApplyConfigFailure_ReinsertsWithSameID(t *testing.T) {
	caddyServer := failingCaddyServer(t)
	router, db := setupRedirectionHostHandlerRouterWithCaddy(t, caddyServer.URL)
	host := models.RedirectionHost{
		UUID:        "rh-reinsert-id",
		DomainNames: "reinsert-id.example.com",
		TargetURL:   "https://target.example.com",
		StatusCode:  301,
	}
	require.NoError(t, db.Create(&host).Error)
	originalID := host.ID

	w := doRequest(router, http.MethodDelete, "/redirection-hosts/"+host.UUID, nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var restored models.RedirectionHost
	require.NoError(t, db.Where("uuid = ?", host.UUID).First(&restored).Error)
	assert.Equal(t, originalID, restored.ID)
}
