package handlers

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestSecurityHeadersCreate_IgnoresPresetMarkers describes a create that sends the preset markers: the client cannot make
// a profile a preset, so the stored profile carries no preset marker.
func TestSecurityHeadersCreate_IgnoresPresetMarkers(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityHeaderProfile{}))
	router := roleStampRouter("admin")
	handler := NewSecurityHeadersHandler(db, nil)
	router.POST("/api/v1/security/headers/profiles", handler.CreateProfile)

	w := serveJSON(t, router, http.MethodPost, "/api/v1/security/headers/profiles",
		`{"name":"sneaky","is_preset":true,"preset_type":"strict"}`)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var stored models.SecurityHeaderProfile
	require.NoError(t, db.Where("name = ?", "sneaky").First(&stored).Error)
	assert.False(t, stored.IsPreset)
	assert.Equal(t, "", stored.PresetType)
}

// TestSecurityHeadersUpdate_KeepsCreatedAtAndPresetMarkers describes an update that sends new preset markers and a new
// creation time: the stored creation time and markers are kept, and the other fields are updated.
func TestSecurityHeadersUpdate_KeepsCreatedAtAndPresetMarkers(t *testing.T) {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityHeaderProfile{}))
	require.NoError(t, db.Create(&models.SecurityHeaderProfile{UUID: "keep-markers", Name: "before", PresetType: "custom-marker"}).Error)
	var original models.SecurityHeaderProfile
	require.NoError(t, db.Where("uuid = ?", "keep-markers").First(&original).Error)

	router := roleStampRouter("admin")
	handler := NewSecurityHeadersHandler(db, nil)
	router.PUT("/api/v1/security/headers/profiles/:id", handler.UpdateProfile)

	w := serveJSON(t, router, http.MethodPut, "/api/v1/security/headers/profiles/"+strconv.FormatUint(uint64(original.ID), 10),
		`{"name":"after","is_preset":true,"preset_type":"strict","created_at":"2001-01-01T00:00:00Z"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var stored models.SecurityHeaderProfile
	require.NoError(t, db.First(&stored, original.ID).Error)
	assert.Equal(t, "after", stored.Name)
	assert.False(t, stored.IsPreset)
	assert.Equal(t, "custom-marker", stored.PresetType)
	assert.True(t, original.CreatedAt.Equal(stored.CreatedAt), "created_at is kept: got %s want %s", stored.CreatedAt, original.CreatedAt)
}
