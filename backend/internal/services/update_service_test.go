package services

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/network"
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
	us.SetHTTPClient(server.Client())
	us.SetCurrentVersion("0.9.0")

	// Test Update Available
	info, err := us.CheckForUpdates()
	assert.NoError(t, err)
	assert.True(t, info.Available)
	assert.Equal(t, "v1.0.0", info.LatestVersion)
	assert.Equal(t, "https://github.com/Wikid82/charon/releases/tag/v1.0.0", info.ChangelogURL)
	assert.Equal(t, "0.9.0", info.CurrentVersion)

	// Test No Update Available
	us.SetCurrentVersion("1.0.0")
	us.ClearCache()

	info, err = us.CheckForUpdates()
	assert.NoError(t, err)
	assert.False(t, info.Available)
	assert.Equal(t, "v1.0.0", info.LatestVersion)
	assert.Equal(t, "1.0.0", info.CurrentVersion)

	// A second call within the TTL returns the cached result.
	info2, err := us.CheckForUpdates()
	assert.NoError(t, err)
	assert.Equal(t, info, info2)

	// A refused connection surfaces as an error.
	server.Close()
	us.ClearCache()

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

func TestUpdateService_DefaultClientRejectsLoopback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v1.0.0"})
	}))
	defer server.Close()

	us := NewUpdateService()
	assert.NoError(t, us.SetAPIURL(server.URL))

	// No injected client: the default safe client must refuse loopback targets.
	_, err := us.CheckForUpdates()
	if assert.Error(t, err) {
		assert.ErrorIs(t, err, network.ErrBlockedAddress)
	}
}

func TestUpdateService_CheckForUpdates_RejectsReservedDestinations(t *testing.T) {
	for _, target := range []string{"http://100.64.0.1:9/", "http://198.18.0.1:9/", "http://[2002::1]:9/"} {
		svc := NewUpdateService()
		svc.apiURL = target
		_, err := svc.CheckForUpdates()
		if err == nil || !errors.Is(err, network.ErrBlockedAddress) {
			t.Errorf("%s: expected policy rejection, got %v", target, err)
		}
	}
}

func TestReleaseURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "valid GitHub release URL", raw: "https://github.com/Wikid82/charon/releases/tag/v1.0.0", want: "https://github.com/Wikid82/charon/releases/tag/v1.0.0"},
		{name: "explicit default HTTPS port kept", raw: "https://github.com:443/Wikid82/charon/releases/tag/v1.0.0", want: "https://github.com:443/Wikid82/charon/releases/tag/v1.0.0"},
		{name: "http scheme rejected", raw: "http://github.com/Wikid82/charon/releases/tag/v1.0.0", want: ""},
		{name: "javascript scheme rejected", raw: "javascript:alert(document.domain)", want: ""},
		{name: "data scheme rejected", raw: "data:text/html,<script>alert(1)</script>", want: ""},
		{name: "other host rejected", raw: "https://example.com/Wikid82/charon/releases/tag/v1.0.0", want: ""},
		{name: "api subdomain rejected", raw: "https://api.github.com/repos/Wikid82/charon/releases/latest", want: ""},
		{name: "suffix lookalike host rejected", raw: "https://github.com.evil.example/Wikid82/charon", want: ""},
		{name: "prefix lookalike host rejected", raw: "https://evilgithub.com/Wikid82/charon", want: ""},
		{name: "uppercase host rejected by exact match", raw: "https://GITHUB.COM/Wikid82/charon", want: ""},
		{name: "userinfo with github host rejected", raw: "https://user@github.com/Wikid82/charon", want: ""},
		{name: "userinfo with password rejected", raw: "https://user:pass@github.com/Wikid82/charon", want: ""},
		{name: "userinfo spoofing github host rejected", raw: "https://github.com@evil.example/Wikid82/charon", want: ""},
		{name: "non-default port accepted by hostname check", raw: "https://github.com:8443/Wikid82/charon", want: "https://github.com:8443/Wikid82/charon"},
		{name: "empty string rejected", raw: "", want: ""},
		{name: "unparseable URL rejected", raw: "https://[::1", want: ""},
		{name: "scheme-relative URL rejected", raw: "//github.com/Wikid82/charon", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, releaseURL(tt.raw))
		})
	}
}

func TestUpdateService_CheckForUpdates_SanitizesChangelogURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(githubRelease{
			TagName: "v1.0.0",
			HTMLURL: "javascript:alert(1)",
		})
	}))
	defer server.Close()

	us := NewUpdateService()
	assert.NoError(t, us.SetAPIURL(server.URL+"/releases/latest"))
	us.SetHTTPClient(server.Client())
	us.SetCurrentVersion("0.9.0")

	info, err := us.CheckForUpdates()
	assert.NoError(t, err)
	assert.Equal(t, "", info.ChangelogURL)
	assert.True(t, info.Available)
	assert.Equal(t, "0.9.0", info.CurrentVersion)
}

func TestUpdateService_CheckForUpdates_NonOKCarriesCurrentVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	us := NewUpdateService()
	assert.NoError(t, us.SetAPIURL(server.URL+"/releases/latest"))
	us.SetHTTPClient(server.Client())
	us.SetCurrentVersion("0.9.0")

	info, err := us.CheckForUpdates()
	assert.NoError(t, err)
	assert.False(t, info.Available)
	assert.Equal(t, "", info.ChangelogURL)
	assert.Equal(t, "0.9.0", info.CurrentVersion)
}
