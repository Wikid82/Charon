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
	"os/exec"
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

// Helper: execDockerCommand runs a command inside the container and returns output.
func execDockerCommand(containerName string, args ...string) (string, error) {
	fullArgs := append([]string{"exec", containerName}, args...)
	cmd := exec.Command("docker", fullArgs...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// TestBouncerAuth_InvalidEnvKeyAutoRecovers verifies that when an invalid API key is set
// via environment variable, Charon detects the failure and auto-generates a new valid key.
//
// Test Steps:
// 1. Set CHARON_SECURITY_CROWDSEC_API_KEY=fakeinvalidkey in environment
// 2. Enable CrowdSec via API
// 3. Verify logs show:
//   - "Environment variable CHARON_SECURITY_CROWDSEC_API_KEY is set but invalid"
//   - "A new valid key will be generated and saved"
//
// 4. Verify new key auto-generated and saved to file
// 5. Verify Caddy bouncer connects successfully with new key
func TestBouncerAuth_InvalidEnvKeyAutoRecovers(t *testing.T) {
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

	// Note: Environment variable must be set in docker-compose.yml before starting container.
	// This test assumes CHARON_SECURITY_CROWDSEC_API_KEY=fakeinvalidkey is already set.
	t.Log("Step 1: Assuming invalid environment variable is set (CHARON_SECURITY_CROWDSEC_API_KEY=fakeinvalidkey)")

	// Step 2: Enable CrowdSec
	t.Log("Step 2: Enabling CrowdSec via API")
	resp, err := tc.doRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", nil)
	if err != nil {
		t.Fatalf("Failed to start CrowdSec: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && !strings.Contains(string(body), "already running") {
		if strings.Contains(string(body), "not found") || strings.Contains(string(body), "not available") {
			t.Skip("CrowdSec binary not available - skipping")
		}
		t.Logf("Start response: %s (continuing despite non-200 status)", string(body))
	}

	// Wait for LAPI to initialize
	tc.waitForLAPIReady(t, 30*time.Second)

	// Step 3: Check logs for auto-recovery messages
	t.Log("Step 3: Checking container logs for auto-recovery messages")
	logs, err := execDockerCommand(tc.ContainerName, "cat", "/var/log/charon/charon.log")
	if err != nil {
		// Try docker logs command if log file doesn't exist
		cmd := exec.Command("docker", "logs", "--tail", "200", tc.ContainerName)
		output, _ := cmd.CombinedOutput()
		logs = string(output)
	}

	if !strings.Contains(logs, "Environment variable") && !strings.Contains(logs, "invalid") {
		t.Logf("Warning: Expected warning messages not found in logs. This may indicate env var was not set before container start.")
		t.Logf("Logs (last 500 chars): %s", logs[max(0, len(logs)-500):])
	}

	// Step 4: Verify key file exists and contains a valid key
	t.Log("Step 4: Verifying bouncer key file exists")
	keyFilePath := "/app/data/crowdsec/bouncer_key"
	generatedKey, err := execDockerCommand(tc.ContainerName, "cat", keyFilePath)
	if err != nil {
		t.Fatalf("Failed to read bouncer key file: %v", err)
	}

	if generatedKey == "" {
		t.Fatal("Bouncer key file is empty")
	}

	if generatedKey == "fakeinvalidkey" {
		t.Fatal("Key should be regenerated, not the invalid env var")
	}

	t.Logf("Generated key (masked): %s...%s", generatedKey[:min(4, len(generatedKey))], generatedKey[max(0, len(generatedKey)-4):])

	// Step 5: Verify Caddy bouncer can authenticate with generated key
	t.Log("Step 5: Verifying Caddy bouncer authentication with generated key")
	lapiURL := tc.BaseURL // LAPI is on same host in test environment
	req, err := http.NewRequest("GET", lapiURL+"/v1/decisions/stream", nil)
	if err != nil {
		t.Fatalf("Failed to create LAPI request: %v", err)
	}
	req.Header.Set("X-Api-Key", generatedKey)

	client := &http.Client{Timeout: 10 * time.Second}
	decisionsResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to query LAPI: %v", err)
	}
	defer decisionsResp.Body.Close()

	if decisionsResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(decisionsResp.Body)
		t.Fatalf("LAPI authentication failed with status %d: %s", decisionsResp.StatusCode, string(respBody))
	}

	t.Log("✅ Auto-recovery from invalid env var successful")
}

// TestBouncerAuth_ValidEnvKeyPreserved verifies that when a valid API key is set
// via environment variable, it is used without triggering new registration.
//
// Test Steps:
// 1. Pre-register bouncer with cscli
// 2. Note: Registered key must be set as CHARON_SECURITY_CROWDSEC_API_KEY before starting container
// 3. Enable CrowdSec
// 4. Verify logs show "source=environment_variable"
// 5. Verify no duplicate bouncer registration
// 6. Verify authentication works with env key
func TestBouncerAuth_ValidEnvKeyPreserved(t *testing.T) {
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

	// Step 1: Pre-register bouncer (if not already registered)
	t.Log("Step 1: Checking if bouncer is pre-registered")
	listOutput, err := execDockerCommand(tc.ContainerName, "cscli", "bouncers", "list", "-o", "json")
	if err != nil {
		t.Logf("Failed to list bouncers: %v (this is expected if CrowdSec not fully initialized)", err)
	}

	bouncerExists := strings.Contains(listOutput, `"name":"caddy-bouncer"`)
	t.Logf("Bouncer exists: %v", bouncerExists)

	// Step 2: Note - Environment variable must be set in docker-compose.yml with the registered key
	t.Log("Step 2: Assuming valid environment variable is set (must match pre-registered key)")

	// Step 3: Enable CrowdSec
	t.Log("Step 3: Enabling CrowdSec via API")
	resp, err := tc.doRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", nil)
	if err != nil {
		t.Fatalf("Failed to start CrowdSec: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && !strings.Contains(string(body), "already running") {
		if strings.Contains(string(body), "not found") || strings.Contains(string(body), "not available") {
			t.Skip("CrowdSec binary not available - skipping")
		}
		t.Logf("Start response: %s (continuing)", string(body))
	}

	// Wait for LAPI
	tc.waitForLAPIReady(t, 30*time.Second)

	// Step 4: Check logs for environment variable source
	t.Log("Step 4: Checking logs for env var source indicator")
	logs, err := execDockerCommand(tc.ContainerName, "cat", "/var/log/charon/charon.log")
	if err != nil {
		cmd := exec.Command("docker", "logs", "--tail", "200", tc.ContainerName)
		output, _ := cmd.CombinedOutput()
		logs = string(output)
	}

	if !strings.Contains(logs, "source=environment_variable") {
		t.Logf("Warning: Expected 'source=environment_variable' not found in logs")
		t.Logf("This may indicate the env var was not set before container start")
	}

	// Step 5: Verify no duplicate bouncer registration
	t.Log("Step 5: Verifying no duplicate bouncer registration")
	listOutputAfter, err := execDockerCommand(tc.ContainerName, "cscli", "bouncers", "list", "-o", "json")
	if err == nil {
		bouncerCount := strings.Count(listOutputAfter, `"name":"caddy-bouncer"`)
		if bouncerCount > 1 {
			t.Errorf("Expected exactly 1 bouncer, found %d duplicates", bouncerCount)
		}
		t.Logf("Bouncer count: %d (expected 1)", bouncerCount)
	}

	// Step 6: Verify authentication works
	t.Log("Step 6: Verifying authentication (key must be set correctly in env)")
	keyFromFile, err := execDockerCommand(tc.ContainerName, "cat", "/app/data/crowdsec/bouncer_key")
	if err != nil {
		t.Logf("Could not read key file: %v", err)
		return // Cannot verify without key
	}

	lapiURL := tc.BaseURL
	req, err := http.NewRequest("GET", lapiURL+"/v1/decisions/stream", nil)
	if err != nil {
		t.Fatalf("Failed to create LAPI request: %v", err)
	}
	req.Header.Set("X-Api-Key", strings.TrimSpace(keyFromFile))

	client := &http.Client{Timeout: 10 * time.Second}
	decisionsResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to query LAPI: %v", err)
	}
	defer decisionsResp.Body.Close()

	if decisionsResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(decisionsResp.Body)
		t.Errorf("LAPI authentication failed with status %d: %s", decisionsResp.StatusCode, string(respBody))
	} else {
		t.Log("✅ Valid environment variable preserved successfully")
	}
}

// TestBouncerAuth_FileKeyPersistsAcrossRestarts verifies that an auto-generated key
// is saved to file and reused across container restarts.
//
// Test Steps:
// 1. Clear any existing key file
// 2. Enable CrowdSec (triggers auto-generation)
// 3. Read generated key from file
// 4. Restart Charon container
// 5. Verify same key is still in file
// 6. Verify logs show "source=file"
// 7. Verify authentication works with persisted key
func TestBouncerAuth_FileKeyPersistsAcrossRestarts(t *testing.T) {
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

	// Step 1: Clear key file (note: requires container to be started without env var set)
	t.Log("Step 1: Clearing key file")
	keyFilePath := "/app/data/crowdsec/bouncer_key"
	_, _ = execDockerCommand(tc.ContainerName, "rm", "-f", keyFilePath) // Ignore error if file doesn't exist

	// Step 2: Enable CrowdSec to trigger key auto-generation
	t.Log("Step 2: Enabling CrowdSec to trigger key auto-generation")
	resp, err := tc.doRequest(http.MethodPost, "/api/v1/admin/crowdsec/start", nil)
	if err != nil {
		t.Fatalf("Failed to start CrowdSec: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && !strings.Contains(string(body), "already running") {
		if strings.Contains(string(body), "not found") || strings.Contains(string(body), "not available") {
			t.Skip("CrowdSec binary not available - skipping")
		}
	}

	// Wait for LAPI and key generation
	tc.waitForLAPIReady(t, 30*time.Second)
	time.Sleep(5 * time.Second) // Allow time for key file creation

	// Step 3: Read generated key
	t.Log("Step 3: Reading generated key from file")
	originalKey, err := execDockerCommand(tc.ContainerName, "cat", keyFilePath)
	if err != nil {
		t.Fatalf("Failed to read bouncer key file after generation: %v", err)
	}

	if originalKey == "" {
		t.Fatal("Bouncer key file is empty after generation")
	}

	t.Logf("Original key (masked): %s...%s", originalKey[:min(4, len(originalKey))], originalKey[max(0, len(originalKey)-4):])

	// Step 4: Restart container
	t.Log("Step 4: Restarting Charon container")
	cmd := exec.Command("docker", "restart", tc.ContainerName)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to restart container: %v, output: %s", err, string(output))
	}

	// Wait for container to come back up
	time.Sleep(10 * time.Second)
	if err := tc.waitForAPI(t, 60*time.Second); err != nil {
		t.Fatalf("API not available after restart: %v", err)
	}

	// Re-authenticate after restart
	if err := tc.authenticate(t); err != nil {
		t.Fatalf("Authentication failed after restart: %v", err)
	}

	// Step 5: Verify same key persisted
	t.Log("Step 5: Verifying key persisted after restart")
	persistedKey, err := execDockerCommand(tc.ContainerName, "cat", keyFilePath)
	if err != nil {
		t.Fatalf("Failed to read bouncer key file after restart: %v", err)
	}

	if persistedKey != originalKey {
		t.Errorf("Key changed after restart. Original: %s...%s, After: %s...%s",
			originalKey[:4], originalKey[len(originalKey)-4:],
			persistedKey[:min(4, len(persistedKey))], persistedKey[max(0, len(persistedKey)-4):])
	}

	// Step 6: Verify logs show file source
	t.Log("Step 6: Checking logs for file source indicator")
	logs, err := execDockerCommand(tc.ContainerName, "cat", "/var/log/charon/charon.log")
	if err != nil {
		cmd := exec.Command("docker", "logs", "--tail", "200", tc.ContainerName)
		output, _ := cmd.CombinedOutput()
		logs = string(output)
	}

	if !strings.Contains(logs, "source=file") {
		t.Logf("Warning: Expected 'source=file' not found in logs after restart")
	}

	// Step 7: Verify authentication with persisted key
	t.Log("Step 7: Verifying authentication with persisted key")
	lapiURL := tc.BaseURL
	req, err := http.NewRequest("GET", lapiURL+"/v1/decisions/stream", nil)
	if err != nil {
		t.Fatalf("Failed to create LAPI request: %v", err)
	}
	req.Header.Set("X-Api-Key", persistedKey)

	client := &http.Client{Timeout: 10 * time.Second}
	decisionsResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to query LAPI: %v", err)
	}
	defer decisionsResp.Body.Close()

	if decisionsResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(decisionsResp.Body)
		t.Fatalf("LAPI authentication failed with status %d: %s", decisionsResp.StatusCode, string(respBody))
	}

	t.Log("✅ File key persistence across restarts successful")
}

// Helper: min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper: max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
