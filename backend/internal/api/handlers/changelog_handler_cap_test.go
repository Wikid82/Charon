package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/changelog"
	"github.com/Wikid82/charon/backend/internal/models"
)

// capHistory returns one entry per minor version 0.36.0 .. 0.46.0, the shape
// of real history for a deployment that has shipped data since v0.36.1.
func capHistory() []changelog.Entry {
	entries := make([]changelog.Entry, 0, 11)
	for minor := 36; minor <= 46; minor++ {
		entries = append(entries, changelog.Entry{
			Version:  fmt.Sprintf("0.%d.0", minor),
			Features: []string{"feature"},
		})
	}
	return entries
}

// TestChangelogHandler_Status_StaleUserGetsCappedAtThree is the F8 contract:
// a user seeded at 0.36.1 who never acked, on a 0.46.6 build, sees exactly
// the three newest entries rather than the whole history.
func TestChangelogHandler_Status_StaleUserGetsCappedAtThree(t *testing.T) {
	h, db, svc := setupChangelogHandler(t)
	svc.SetCurrentVersion("0.46.6")
	setChangelogEntriesForTest(t, svc, append(capHistory(), changelog.Entry{Version: "0.46.6"}))
	userID := createChangelogTestUser(t, db, "0.36.1", false)
	r := buildChangelogRouter(h, "user", userID)

	w := doJSON(t, r, http.MethodGet, "/changelog/status", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		ShowChangelog bool              `json:"show_changelog"`
		Versions      []changelog.Entry `json:"versions"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.ShowChangelog)
	got := make([]string, 0, len(resp.Versions))
	for _, e := range resp.Versions {
		got = append(got, e.Version)
	}
	assert.Equal(t, []string{"0.46.6", "0.46.0", "0.45.0"}, got)
}

func TestChangelogHandler_Status_NothingNewer_ReturnsEmptyArray(t *testing.T) {
	h, db, svc := setupChangelogHandler(t)
	svc.SetCurrentVersion("0.46.0")
	setChangelogEntriesForTest(t, svc, capHistory())
	userID := createChangelogTestUser(t, db, "0.46.0", false)
	r := buildChangelogRouter(h, "user", userID)

	w := doJSON(t, r, http.MethodGet, "/changelog/status", nil)

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"show_changelog":false,"versions":[]}`, w.Body.String())
}

func TestChangelogHandler_Ack_DismissPermanent_LastSeenNeverLowered(t *testing.T) {
	tests := []struct {
		name     string
		running  string
		stored   string
		expected string
	}{
		{"downgrade keeps the higher stored version", "0.46.5", "0.46.6", "0.46.6"},
		{"equal stored version is left as is", "0.46.6", "0.46.6", "0.46.6"},
		{"upgrade advances the stored version", "0.46.6", "0.46.5", "0.46.6"},
		{"empty stored version is advanced", "0.46.6", "", "0.46.6"},
		{"invalid stored version is replaced", "0.46.6", "nightly-a1b2c3d", "0.46.6"},
		{"stored with build metadata is compared by core version", "0.46.6", "0.46.6+nightly.abc1234", "0.46.6+nightly.abc1234"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, db, svc := setupChangelogHandler(t)
			svc.SetCurrentVersion(tt.running)
			userID := createChangelogTestUser(t, db, tt.stored, false)
			r := buildChangelogRouter(h, "user", userID)

			w := doJSON(t, r, http.MethodPost, "/changelog/ack", map[string]any{"action": "dismiss_permanent"})

			require.Equal(t, http.StatusOK, w.Code)
			var user models.User
			require.NoError(t, db.First(&user, userID).Error)
			assert.Equal(t, tt.expected, user.LastSeenVersion)
		})
	}
}

func TestChangelogHandler_Ack_DismissPermanent_WritesCanonicalAnchor(t *testing.T) {
	tests := []struct {
		name    string
		running string
		want    string
	}{
		{"build metadata is stripped", "0.46.5+nightly.a1b2c3d", "0.46.5"},
		{"override with metadata writes the bare version", "1.2.0+dev", "1.2.0"},
		{"prerelease is kept", "0.47.0-rc.1", "0.47.0-rc.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, db, svc := setupChangelogHandler(t)
			svc.SetCurrentVersion(tt.running)
			userID := createChangelogTestUser(t, db, "0.1.0", false)
			r := buildChangelogRouter(h, "user", userID)

			w := doJSON(t, r, http.MethodPost, "/changelog/ack", map[string]any{"action": "dismiss_permanent"})

			require.Equal(t, http.StatusOK, w.Code)
			var user models.User
			require.NoError(t, db.First(&user, userID).Error)
			assert.Equal(t, tt.want, user.LastSeenVersion)
		})
	}
}

func TestChangelogHandler_Ack_OptOutWithDowngrade_StillLowersNothing(t *testing.T) {
	h, db, svc := setupChangelogHandler(t)
	svc.SetCurrentVersion("0.46.5")
	userID := createChangelogTestUser(t, db, "0.46.6", false)
	r := buildChangelogRouter(h, "user", userID)

	w := doJSON(t, r, http.MethodPost, "/changelog/ack", map[string]any{
		"action": "dismiss_permanent", "opt_out": true,
	})

	require.Equal(t, http.StatusOK, w.Code)
	var user models.User
	require.NoError(t, db.First(&user, userID).Error)
	assert.Equal(t, "0.46.6", user.LastSeenVersion)
	assert.True(t, user.ChangelogOptOut, "opt_out must still apply when the anchor is not advanced")
}

// TestChangelogHandler_Ack_BindError_GenericMessage guards F16: a malformed
// body must return the fixed message, never validator or decoder internals.
func TestChangelogHandler_Ack_BindError_GenericMessage(t *testing.T) {
	bodies := map[string]string{
		"invalid action enum": `{"action":"nope"}`,
		"missing action":      `{"opt_out":true}`,
		"malformed json":      `{"action":`,
		"wrong type":          `{"action":42}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			h, db, _ := setupChangelogHandler(t)
			userID := createChangelogTestUser(t, db, "0.46.0", false)
			r := buildChangelogRouter(h, "user", userID)

			req := httptest.NewRequest(http.MethodPost, "/changelog/ack", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code)
			var resp map[string]string
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, map[string]string{"error": "invalid request"}, resp)
		})
	}
}
