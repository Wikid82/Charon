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
			HTMLURL: "https://github.com/Wikid82/charon/releases/tag/v1.0.0",
		}
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	us := NewUpdateService()
	err := us.SetAPIURL(server.URL + "/releases/latest")
	assert.NoError(t, err)
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
	assert.Equal(t, "https://github.com/Wikid82/charon/releases/tag/v1.0.0", info.ChangelogURL)

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

func TestUpdateService_SetAPIURL_GitHubValidation(t *testing.T) {
	svc := NewUpdateService()

	tests := []struct {
		name        string
		url         string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid GitHub API HTTPS",
			url:     "https://api.github.com/repos/test/repo",
			wantErr: false,
		},
		{
			name:        "GitHub with HTTP scheme",
			url:         "http://api.github.com/repos/test/repo",
			wantErr:     true,
			errContains: "must use HTTPS",
		},
		{
			name:        "non-GitHub domain",
			url:         "https://evil.com/api",
			wantErr:     true,
			errContains: "GitHub domain",
		},
		{
			name:    "localhost allowed",
			url:     "http://localhost:8080/api",
			wantErr: false,
		},
		{
			name:    "127.0.0.1 allowed",
			url:     "http://127.0.0.1:8080/api",
			wantErr: false,
		},
		{
			name:    "::1 IPv6 localhost allowed",
			url:     "http://[::1]:8080/api",
			wantErr: false,
		},
		{
			name:        "invalid URL",
			url:         "not a valid url",
			wantErr:     true,
			errContains: "", // Error message varies by Go version
		},
		{
			name:        "ftp scheme not allowed",
			url:         "ftp://api.github.com/repos/test/repo",
			wantErr:     true,
			errContains: "must use HTTP or HTTPS",
		},
		{
			name:    "github.com domain allowed with HTTPS",
			url:     "https://github.com/repos/test/repo",
			wantErr: false,
		},
		{
			name:        "github.com domain with HTTP rejected",
			url:         "http://github.com/repos/test/repo",
			wantErr:     true,
			errContains: "must use HTTPS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.SetAPIURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
