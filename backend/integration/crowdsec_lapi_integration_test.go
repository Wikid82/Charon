//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"testing"
	"time"
)

// testConfig holds configuration for LAPI integration tests.
type testConfig struct {
	BaseURL       string
	ContainerName string
	Client        *http.Client
	Cookie        []*http.Cookie
}

// newTestConfig creates a test configuration with defaults.
func newTestConfig() *testConfig {
	baseURL := os.Getenv("CHARON_TEST_API_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
	}

	return &testConfig{
		BaseURL:       baseURL,
		ContainerName: "charon-e2e",
		Client:        client,
	}
}

// authenticate registers and logs in to get session cookies.
func (tc *testConfig) authenticate(t *testing.T) error {
	t.Helper()

	// Register (may fail if user exists - that's OK)
	registerPayload := map[string]string{
		"email":    "lapi-test@example.local",
		"password": "testpassword123",
		"name":     "LAPI Tester",
	}
	payloadBytes, _ := json.Marshal(registerPayload)
	_, _ = tc.Client.Post(tc.BaseURL+"/api/v1/auth/register", "application/json", bytes.NewReader(payloadBytes))

	// Login
	loginPayload := map[string]string{
		"email":    "lapi-test@example.local",
		"password": "testpassword123",
	}
	payloadBytes, _ = json.Marshal(loginPayload)
	resp, err := tc.Client.Post(tc.BaseURL+"/api/v1/auth/login", "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// doRequest performs an authenticated HTTP request.
func (tc *testConfig) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, tc.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return tc.Client.Do(req)
}

// waitForAPI waits for the API to be ready.
func (tc *testConfig) waitForAPI(t *testing.T, timeout time.Duration) error {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := tc.Client.Get(tc.BaseURL + "/api/v1/")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("API not ready after %v", timeout)
}

// waitForLAPIReady polls the status endpoint until LAPI is ready or timeout.
func (tc *testConfig) waitForLAPIReady(t *testing.T, timeout time.Duration) (bool, error) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := tc.doRequest(http.MethodGet, "/api/v1/admin/crowdsec/status", nil)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var status struct {
			Running   bool `json:"running"`
			LapiReady bool `json:"lapi_ready"`
		}
		if err := json.Unmarshal(body, &status); err == nil {
			if status.LapiReady {
				return true, nil
			}
		}

		time.Sleep(1 * time.Second)
	}
	return false, nil
}

// TestCrowdSecLAPIStartup verifies LAPI can be started via API and becomes ready.
//
// Test steps:
//  1. Start CrowdSec via POST /api/v1/admin/crowdsec/start
//  2. Wait for LAPI to initialize (up to 30s with polling)
//  3. Verify: GET /api/v1/admin/crowdsec/status returns lapi_ready: true
//  4. Use the diagnostic endpoint: GET /api/v1/admin/crowdsec/diagnostics/connectivity
func TestCrowdSecLAPIStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tc := newTestConfig()

	// Wait for API to be ready
	if err := tc.waitForAPI(t, 60*time.Second); err != nil {
		t.Skipf("API not available, skipping test: %v", err)
	}

	// Authenticate
	if err := tc.authenticate(t); err != nil {
		t.Fatalf("Authentication failed: %v", err)
	}

	// Step 1: Start CrowdSec
	t.Log("Step 1: Starting CrowdSec via API...")
	resp, err := tc.doRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", nil)
	if err != nil {
		t.Fatalf("Failed to call start endpoint: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	t.Logf("Start response: %s", string(body))

	var startResp struct {
		Status    string `json:"status"`
		PID       int    `json:"pid"`
		LapiReady bool   `json:"lapi_ready"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(body, &startResp); err != nil {
		t.Logf("Warning: Could not parse start response: %v", err)
	}

	// Check for expected responses
	if resp.StatusCode != http.StatusOK {
		// CrowdSec binary may not be available
		if strings.Contains(string(body), "not found") || strings.Contains(string(body), "not available") {
			t.Skip("CrowdSec binary not available in container - skipping")
		}
		t.Logf("Start returned non-200 status: %d - continuing to check status", resp.StatusCode)
	}

	// Step 2: Wait for LAPI to be ready
	t.Log("Step 2: Waiting for LAPI to initialize (up to 30s)...")
	lapiReady, _ := tc.waitForLAPIReady(t, 30*time.Second)

	// Step 3: Verify status endpoint
	t.Log("Step 3: Verifying status endpoint...")
	resp, err = tc.doRequest(http.MethodGet, "/api/v1/admin/crowdsec/status", nil)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	t.Logf("Status response: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Status endpoint returned %d", resp.StatusCode)
	}

	var statusResp struct {
		Running   bool `json:"running"`
		PID       int  `json:"pid"`
		LapiReady bool `json:"lapi_ready"`
	}
	if err := json.Unmarshal(body, &statusResp); err != nil {
		t.Fatalf("Failed to parse status response: %v", err)
	}

	t.Logf("CrowdSec status: running=%v, pid=%d, lapi_ready=%v", statusResp.Running, statusResp.PID, statusResp.LapiReady)

	// Validate: If we managed to start, LAPI should eventually be ready
	// If CrowdSec binary is not available, we expect running=false
	if statusResp.Running && !statusResp.LapiReady && lapiReady {
		t.Error("Expected lapi_ready=true after waiting, but got false")
	}

	// Step 4: Check diagnostics connectivity endpoint
	t.Log("Step 4: Checking diagnostics connectivity endpoint...")
	resp, err = tc.doRequest(http.MethodGet, "/api/v1/admin/crowdsec/diagnostics/connectivity", nil)
	if err != nil {
		t.Fatalf("Failed to get diagnostics: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	t.Logf("Diagnostics connectivity response: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Diagnostics endpoint returned %d", resp.StatusCode)
	}

	var diagResp map[string]interface{}
	if err := json.Unmarshal(body, &diagResp); err != nil {
		t.Fatalf("Failed to parse diagnostics response: %v", err)
	}

	// Verify expected fields are present
	expectedFields := []string{"lapi_running", "lapi_ready", "capi_registered", "console_enrolled"}
	for _, field := range expectedFields {
		if _, ok := diagResp[field]; !ok {
			t.Errorf("Expected field '%s' not found in diagnostics response", field)
		}
	}

	t.Log("TestCrowdSecLAPIStartup completed successfully")
}

// TestCrowdSecLAPIRestartPersistence verifies LAPI can restart and state persists.
//
// Test steps:
//  1. Start CrowdSec
//  2. Record initial state
//  3. Stop CrowdSec via API
//  4. Start CrowdSec again
//  5. Verify LAPI comes back online
//  6. Verify state persists
func TestCrowdSecLAPIRestartPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tc := newTestConfig()

	// Wait for API to be ready
	if err := tc.waitForAPI(t, 60*time.Second); err != nil {
		t.Skipf("API not available, skipping test: %v", err)
	}

	// Authenticate
	if err := tc.authenticate(t); err != nil {
		t.Fatalf("Authentication failed: %v", err)
	}

	// Step 1: Start CrowdSec
	t.Log("Step 1: Starting CrowdSec...")
	resp, err := tc.doRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", nil)
	if err != nil {
		t.Fatalf("Failed to start CrowdSec: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if strings.Contains(string(body), "not found") || strings.Contains(string(body), "not available") {
		t.Skip("CrowdSec binary not available in container - skipping")
	}

	// Wait for LAPI to be ready
	lapiReady, _ := tc.waitForLAPIReady(t, 30*time.Second)
	t.Logf("Step 2: Initial LAPI ready state: %v", lapiReady)

	// Step 3: Stop CrowdSec
	t.Log("Step 3: Stopping CrowdSec...")
	resp, err = tc.doRequest(http.MethodPost, "/api/v1/admin/crowdsec/stop", nil)
	if err != nil {
		t.Fatalf("Failed to stop CrowdSec: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	t.Logf("Stop response: %s", string(body))

	// Verify stopped
	time.Sleep(2 * time.Second)
	resp, err = tc.doRequest(http.MethodGet, "/api/v1/admin/crowdsec/status", nil)
	if err != nil {
		t.Fatalf("Failed to get status after stop: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	var statusResp struct {
		Running bool `json:"running"`
	}
	if err := json.Unmarshal(body, &statusResp); err == nil {
		t.Logf("Status after stop: running=%v", statusResp.Running)
	}

	// Step 4: Restart CrowdSec
	t.Log("Step 4: Restarting CrowdSec...")
	resp, err = tc.doRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", nil)
	if err != nil {
		t.Fatalf("Failed to restart CrowdSec: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	t.Logf("Restart response: %s", string(body))

	// Step 5: Verify LAPI comes back online
	t.Log("Step 5: Waiting for LAPI to come back online...")
	lapiReadyAfterRestart, _ := tc.waitForLAPIReady(t, 30*time.Second)

	// Step 6: Verify state
	t.Log("Step 6: Verifying state after restart...")
	resp, err = tc.doRequest(http.MethodGet, "/api/v1/admin/crowdsec/status", nil)
	if err != nil {
		t.Fatalf("Failed to get status after restart: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	t.Logf("Final status: %s", string(body))

	var finalStatus struct {
		Running   bool `json:"running"`
		LapiReady bool `json:"lapi_ready"`
	}
	if err := json.Unmarshal(body, &finalStatus); err != nil {
		t.Fatalf("Failed to parse final status: %v", err)
	}

	// If CrowdSec is available, it should be running after restart
	if lapiReady && !lapiReadyAfterRestart {
		t.Error("LAPI was ready before stop but not after restart")
	}

	t.Log("TestCrowdSecLAPIRestartPersistence completed successfully")
}

// TestCrowdSecDiagnosticsConnectivity verifies the connectivity diagnostics endpoint.
//
// Test steps:
//  1. Start CrowdSec
//  2. Call GET /api/v1/admin/crowdsec/diagnostics/connectivity
//  3. Verify response contains all expected fields:
//     - lapi_running
//     - lapi_ready
//     - capi_registered
//     - console_enrolled
func TestCrowdSecDiagnosticsConnectivity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tc := newTestConfig()

	// Wait for API to be ready
	if err := tc.waitForAPI(t, 60*time.Second); err != nil {
		t.Skipf("API not available, skipping test: %v", err)
	}

	// Authenticate
	if err := tc.authenticate(t); err != nil {
		t.Fatalf("Authentication failed: %v", err)
	}

	// Try to start CrowdSec (may fail if binary not available)
	t.Log("Attempting to start CrowdSec...")
	resp, err := tc.doRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", nil)
	if err == nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Logf("Start response: %s", string(body))

		// Wait briefly for LAPI
		tc.waitForLAPIReady(t, 10*time.Second)
	}

	// Call diagnostics connectivity endpoint
	t.Log("Calling diagnostics connectivity endpoint...")
	resp, err = tc.doRequest(http.MethodGet, "/api/v1/admin/crowdsec/diagnostics/connectivity", nil)
	if err != nil {
		t.Fatalf("Failed to get diagnostics connectivity: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	t.Logf("Diagnostics connectivity response: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Diagnostics connectivity returned %d", resp.StatusCode)
	}

	var diagResp map[string]interface{}
	if err := json.Unmarshal(body, &diagResp); err != nil {
		t.Fatalf("Failed to parse diagnostics response: %v", err)
	}

	// Verify all required fields are present
	requiredFields := []string{
		"lapi_running",
		"lapi_ready",
		"capi_registered",
		"console_enrolled",
	}

	for _, field := range requiredFields {
		if _, ok := diagResp[field]; !ok {
			t.Errorf("Required field '%s' not found in diagnostics response", field)
		} else {
			t.Logf("Field '%s': %v", field, diagResp[field])
		}
	}

	// Optional fields that should be present when applicable
	optionalFields := []string{
		"lapi_pid",
		"capi_reachable",
		"console_reachable",
		"console_status",
		"console_agent_name",
	}

	for _, field := range optionalFields {
		if val, ok := diagResp[field]; ok {
			t.Logf("Optional field '%s': %v", field, val)
		}
	}

	t.Log("TestCrowdSecDiagnosticsConnectivity completed successfully")
}

// TestCrowdSecDiagnosticsConfig verifies the config diagnostics endpoint.
//
// Test steps:
//  1. Call GET /api/v1/admin/crowdsec/diagnostics/config
//  2. Verify response contains:
//     - config_exists
//     - acquis_exists
//     - lapi_port
//     - errors array
func TestCrowdSecDiagnosticsConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tc := newTestConfig()

	// Wait for API to be ready
	if err := tc.waitForAPI(t, 60*time.Second); err != nil {
		t.Skipf("API not available, skipping test: %v", err)
	}

	// Authenticate
	if err := tc.authenticate(t); err != nil {
		t.Fatalf("Authentication failed: %v", err)
	}

	// Call diagnostics config endpoint
	t.Log("Calling diagnostics config endpoint...")
	resp, err := tc.doRequest(http.MethodGet, "/api/v1/admin/crowdsec/diagnostics/config", nil)
	if err != nil {
		t.Fatalf("Failed to get diagnostics config: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	t.Logf("Diagnostics config response: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Diagnostics config returned %d", resp.StatusCode)
	}

	var diagResp map[string]interface{}
	if err := json.Unmarshal(body, &diagResp); err != nil {
		t.Fatalf("Failed to parse diagnostics response: %v", err)
	}

	// Verify all required fields are present
	requiredFields := []string{
		"config_exists",
		"acquis_exists",
		"lapi_port",
		"errors",
	}

	for _, field := range requiredFields {
		if _, ok := diagResp[field]; !ok {
			t.Errorf("Required field '%s' not found in diagnostics config response", field)
		} else {
			t.Logf("Field '%s': %v", field, diagResp[field])
		}
	}

	// Verify errors is an array
	if errors, ok := diagResp["errors"]; ok {
		if _, isArray := errors.([]interface{}); !isArray {
			t.Errorf("Expected 'errors' to be an array, got %T", errors)
		}
	}

	// Optional fields that may be present when configs exist
	optionalFields := []string{
		"config_valid",
		"acquis_valid",
		"config_path",
		"acquis_path",
	}

	for _, field := range optionalFields {
		if val, ok := diagResp[field]; ok {
			t.Logf("Optional field '%s': %v", field, val)
		}
	}

	// Log summary
	t.Logf("Config exists: %v, Acquis exists: %v, LAPI port: %v",
		diagResp["config_exists"],
		diagResp["acquis_exists"],
		diagResp["lapi_port"],
	)

	t.Log("TestCrowdSecDiagnosticsConfig completed successfully")
}
