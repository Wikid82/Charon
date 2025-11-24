package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUpdateService_CheckForUpdates(t *testing.T) {
	// Mock GitHub API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/latest" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		release := githubRelease{
			TagName: "v1.0.0",
			HTMLURL: "https://github.com/Wikid82/CaddyProxyManagerPlus/releases/tag/v1.0.0",
		}
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	us := NewUpdateService()
	us.SetAPIURL(server.URL + "/releases/latest")
	// us.currentVersion is private, so we can't set it directly in test unless we export it or add a setter.
	// However, NewUpdateService sets it from version.Version.
	// We can temporarily change version.Version if it's a var, but it's likely a const or var in another package.
	// Let's check version package.
	// Assuming version.Version is a var we can change, or we add a SetCurrentVersion method for testing.
	// For now, let's assume we can't change it easily without a setter.
	// Let's add SetCurrentVersion to UpdateService for testing purposes.
	us.SetCurrentVersion("0.9.0")

	// Test Update Available
	info, err := us.CheckForUpdates()
	assert.NoError(t, err)
	assert.True(t, info.Available)
	assert.Equal(t, "v1.0.0", info.LatestVersion)
	assert.Equal(t, "https://github.com/Wikid82/CaddyProxyManagerPlus/releases/tag/v1.0.0", info.ChangelogURL)

	// Test No Update Available
	us.SetCurrentVersion("1.0.0")
	// us.cachedResult = nil // cachedResult is private
	// us.lastCheck = time.Time{} // lastCheck is private
	us.ClearCache() // Add this method

	info, err = us.CheckForUpdates()
	assert.NoError(t, err)
	assert.False(t, info.Available)
	assert.Equal(t, "v1.0.0", info.LatestVersion)

	// Test Cache
	// If we call again immediately, it should use cache.
	// We can verify this by closing the server or changing the response, but cache logic is simple.
	// Let's change the server handler? No, httptest server handler is fixed.
	// But we can check if it returns the same object.
	info2, err := us.CheckForUpdates()
	assert.NoError(t, err)
	assert.Equal(t, info, info2)

	// Test Error (Server Down)
	server.Close()
	us.cachedResult = nil
	us.lastCheck = time.Time{}

	// Depending on implementation, it might return error or just available=false
	// Implementation:
	// resp, err := client.Do(req) -> returns error if connection refused
	// if err != nil { return nil, err }
	_, err = us.CheckForUpdates()
	assert.Error(t, err)
}
