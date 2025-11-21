package services

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/version"
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
		repoName:       "CaddyProxyManagerPlus",
		apiURL:         "https://api.github.com/repos/Wikid82/CaddyProxyManagerPlus/releases/latest",
	}
}

// SetAPIURL sets the GitHub API URL for testing.
func (s *UpdateService) SetAPIURL(url string) {
	s.apiURL = url
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

	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequest("GET", s.apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "CPMP-Update-Checker")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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
	if len(latest) > 0 && latest[0] == 'v' {
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
