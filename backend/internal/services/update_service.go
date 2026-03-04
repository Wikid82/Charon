package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/version"
)

type UpdateService struct {
	currentVersion string
	repoOwner      string
	repoName       string
	lastCheck      time.Time
	cachedResult   *UpdateInfo
	apiURL         string // For testing
}

type UpdateInfo struct {
	Available     bool   `json:"available"`
	LatestVersion string `json:"latest_version"`
	ChangelogURL  string `json:"changelog_url"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func NewUpdateService() *UpdateService {
	return &UpdateService{
		currentVersion: version.Version,
		repoOwner:      "Wikid82",
		repoName:       "charon",
		apiURL:         "https://api.github.com/repos/Wikid82/charon/releases/latest",
	}
}

// SetAPIURL sets the GitHub API URL for testing.
// CRITICAL FIX: Added validation to prevent SSRF if this becomes user-exposed.
// This function returns an error if the URL is invalid or not a GitHub domain.
//
// Note: For testing purposes, this accepts HTTP URLs (for httptest.Server).
// In production, only HTTPS GitHub URLs should be used.
func (s *UpdateService) SetAPIURL(url string) error {
	parsed, err := neturl.Parse(url)
	if err != nil {
		return fmt.Errorf("invalid API URL: %w", err)
	}

	// Only allow HTTP/HTTPS
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("API URL must use HTTP or HTTPS")
	}

	// For test servers (127.0.0.1 or localhost), allow any URL
	// This is safe because test servers are never exposed to user input
	host := parsed.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		s.apiURL = url
		return nil
	}

	// For production, only allow GitHub domains
	allowedHosts := []string{
		"api.github.com",
		"github.com",
	}

	hostAllowed := false
	for _, allowed := range allowedHosts {
		if parsed.Host == allowed {
			hostAllowed = true
			break
		}
	}

	if !hostAllowed {
		return fmt.Errorf("API URL must be a GitHub domain (api.github.com or github.com) or localhost for testing, got: %s", parsed.Host)
	}

	// Enforce HTTPS for production GitHub URLs
	if parsed.Scheme != "https" {
		return fmt.Errorf("GitHub API URL must use HTTPS")
	}

	s.apiURL = url
	return nil
}

// SetCurrentVersion sets the current version for testing.
func (s *UpdateService) SetCurrentVersion(v string) {
	s.currentVersion = v
}

// ClearCache clears the update cache for testing.
func (s *UpdateService) ClearCache() {
	s.cachedResult = nil
	s.lastCheck = time.Time{}
}

func (s *UpdateService) CheckForUpdates() (*UpdateInfo, error) {
	// Cache for 1 hour
	if s.cachedResult != nil && time.Since(s.lastCheck) < 1*time.Hour {
		return s.cachedResult, nil
	}

	// Use SSRF-safe HTTP client for defense-in-depth
	// Note: SetAPIURL already validates the URL against github.com allowlist
	client := network.NewSafeHTTPClient(
		network.WithTimeout(5*time.Second),
		network.WithAllowLocalhost(), // Allow localhost for testing
	)

	req, err := http.NewRequest("GET", s.apiURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Charon-Update-Checker")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Log().WithError(err).Warn("failed to close update service response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		// If rate limited or not found, just return no update available
		return &UpdateInfo{Available: false}, nil
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	// Simple string comparison for now.
	// In production, use a semver library.
	// Assuming tags are "v0.1.0" and version is "0.1.0"
	latest := release.TagName
	if latest != "" && latest[0] == 'v' {
		latest = latest[1:]
	}

	info := &UpdateInfo{
		Available:     latest != s.currentVersion && latest != "",
		LatestVersion: release.TagName,
		ChangelogURL:  release.HTMLURL,
	}

	s.cachedResult = info
	s.lastCheck = time.Now()

	return info, nil
}
