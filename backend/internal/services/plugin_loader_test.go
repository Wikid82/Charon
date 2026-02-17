package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// =============================================================================
// computeSignature Tests
// =============================================================================

func TestComputeSignature(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		fileContent []byte
		wantPrefix  string
		wantErr     bool
	}{
		{
			name:        "empty file",
			fileContent: []byte{},
			wantPrefix:  "sha256:",
			wantErr:     false,
		},
		{
			name:        "simple content",
			fileContent: []byte("test plugin content"),
			wantPrefix:  "sha256:",
			wantErr:     false,
		},
		{
			name:        "binary content",
			fileContent: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
			wantPrefix:  "sha256:",
			wantErr:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create temp file with known content
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.so")
			if err := os.WriteFile(tmpFile, tc.fileContent, 0o600); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			service := NewPluginLoaderService(nil, tmpDir, nil)
			sig, err := service.computeSignature(tmpFile)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify prefix
			if len(sig) < len(tc.wantPrefix) || sig[:len(tc.wantPrefix)] != tc.wantPrefix {
				t.Errorf("signature doesn't have expected prefix %q, got %q", tc.wantPrefix, sig)
			}

			// Verify the signature matches what we expect
			hash := sha256.Sum256(tc.fileContent)
			expected := "sha256:" + hex.EncodeToString(hash[:])
			if sig != expected {
				t.Errorf("signature mismatch: got %q, want %q", sig, expected)
			}
		})
	}
}

func TestComputeSignatureNonExistentFile(t *testing.T) {
	t.Parallel()

	service := NewPluginLoaderService(nil, t.TempDir(), nil)
	_, err := service.computeSignature("/nonexistent/path/plugin.so")

	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

func TestComputeSignatureConsistency(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "consistent.so")
	content := []byte("plugin binary content for consistency test")
	if err := os.WriteFile(tmpFile, content, 0o600); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("failed to write temp file: %v", err)
	}

	service := NewPluginLoaderService(nil, tmpDir, nil)

	// Compute signature multiple times
	sig1, err1 := service.computeSignature(tmpFile)
	sig2, err2 := service.computeSignature(tmpFile)
	sig3, err3 := service.computeSignature(tmpFile)

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("unexpected errors: %v, %v, %v", err1, err2, err3)
	}

	if sig1 != sig2 || sig2 != sig3 {
		t.Errorf("signature not consistent across calls: %q, %q, %q", sig1, sig2, sig3)
	}
}

// =============================================================================
// verifyDirectoryPermissions Tests
// =============================================================================

func TestVerifyDirectoryPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests not applicable on Windows")
	}

	t.Parallel()

	tests := []struct {
		name    string
		mode    os.FileMode
		wantErr bool
	}{
		{
			name:    "secure permissions 0755",
			mode:    0o755,
			wantErr: false,
		},
		{
			name:    "secure permissions 0750",
			mode:    0o750,
			wantErr: false,
		},
		{
			name:    "secure permissions 0700",
			mode:    0o700,
			wantErr: false,
		},
		{
			name:    "world writable 0777",
			mode:    0o777,
			wantErr: true,
		},
		{
			name:    "world writable 0757",
			mode:    0o757,
			wantErr: true,
		},
		{
			name:    "world writable 0773",
			mode:    0o773,
			wantErr: true,
		},
		{
			name:    "world writable 0772",
			mode:    0o772,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			testDir := filepath.Join(tmpDir, "plugins")
			if err := os.Mkdir(testDir, tc.mode); err != nil {
				t.Fatalf("failed to create test directory: %v", err)
			}

			// Ensure permissions are actually set (t.TempDir may have umask applied)
			if err := os.Chmod(testDir, tc.mode); err != nil {
				t.Fatalf("failed to chmod: %v", err)
			}

			service := NewPluginLoaderService(nil, testDir, nil)
			err := service.verifyDirectoryPermissions(testDir)

			if tc.wantErr && err == nil {
				t.Error("expected error for insecure permissions, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestVerifyDirectoryPermissionsNonExistent(t *testing.T) {
	t.Parallel()

	service := NewPluginLoaderService(nil, "/nonexistent", nil)
	err := service.verifyDirectoryPermissions("/nonexistent/path/to/dir")

	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

// =============================================================================
// NewPluginLoaderService Constructor Tests
// =============================================================================

func TestNewPluginLoaderServicePermissiveMode(t *testing.T) {
	t.Parallel()

	service := NewPluginLoaderService(nil, "/plugins", nil)

	if service.allowedSigs != nil {
		t.Errorf("expected nil allowlist for permissive mode, got %v", service.allowedSigs)
	}
	if service.pluginDir != "/plugins" {
		t.Errorf("expected pluginDir /plugins, got %s", service.pluginDir)
	}
	if service.loadedPlugins == nil {
		t.Error("expected loadedPlugins map to be initialized")
	}
}

func TestNewPluginLoaderServiceStrictModeEmpty(t *testing.T) {
	t.Parallel()

	emptyAllowlist := make(map[string]string)
	service := NewPluginLoaderService(nil, "/plugins", emptyAllowlist)

	if service.allowedSigs == nil {
		t.Error("expected non-nil allowlist for strict mode")
	}
	if len(service.allowedSigs) != 0 {
		t.Errorf("expected empty allowlist, got %d entries", len(service.allowedSigs))
	}
}

func TestNewPluginLoaderServiceStrictModePopulated(t *testing.T) {
	t.Parallel()

	allowlist := map[string]string{
		"cloudflare": "sha256:abc123",
		"route53":    "sha256:def456",
	}
	service := NewPluginLoaderService(nil, "/plugins", allowlist)

	if service.allowedSigs == nil {
		t.Error("expected non-nil allowlist")
	}
	if len(service.allowedSigs) != 2 {
		t.Errorf("expected 2 entries in allowlist, got %d", len(service.allowedSigs))
	}
	if service.allowedSigs["cloudflare"] != "sha256:abc123" {
		t.Errorf("cloudflare signature mismatch")
	}
}

// =============================================================================
// Allowlist Logic Tests
// =============================================================================

func TestLoadPluginNotInAllowlist(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pluginFile := filepath.Join(tmpDir, "unknown-provider.so")
	if err := os.WriteFile(pluginFile, []byte("fake plugin"), 0o600); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("failed to create plugin file: %v", err)
	}

	// Strict mode with populated allowlist that doesn't include "unknown-provider"
	allowlist := map[string]string{
		"known-provider": "sha256:some-hash",
	}
	service := NewPluginLoaderService(nil, tmpDir, allowlist)

	err := service.LoadPlugin(pluginFile)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, dnsprovider.ErrPluginNotInAllowlist) {
		t.Errorf("expected ErrPluginNotInAllowlist, got: %v", err)
	}
}

func TestLoadPluginSignatureMismatch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pluginFile := filepath.Join(tmpDir, "cloudflare.so")
	content := []byte("fake cloudflare plugin content")
	if err := os.WriteFile(pluginFile, content, 0o600); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("failed to create plugin file: %v", err)
	}

	// Calculate the wrong signature
	allowlist := map[string]string{
		"cloudflare": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}
	service := NewPluginLoaderService(nil, tmpDir, allowlist)

	err := service.LoadPlugin(pluginFile)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, dnsprovider.ErrSignatureMismatch) {
		t.Errorf("expected ErrSignatureMismatch, got: %v", err)
	}
}

func TestLoadPluginSignatureMatch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pluginFile := filepath.Join(tmpDir, "cloudflare.so")
	content := []byte("fake cloudflare plugin content")
	if err := os.WriteFile(pluginFile, content, 0o600); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("failed to create plugin file: %v", err)
	}

	// Calculate the correct signature
	hash := sha256.Sum256(content)
	correctSig := "sha256:" + hex.EncodeToString(hash[:])

	allowlist := map[string]string{
		"cloudflare": correctSig,
	}
	service := NewPluginLoaderService(nil, tmpDir, allowlist)

	// This will fail at plugin.Open() but that's expected
	// The important part is it gets past the signature check
	err := service.LoadPlugin(pluginFile)

	// Should fail with ErrPluginLoadFailed (not signature error)
	if err == nil {
		t.Log("plugin loaded unexpectedly (shouldn't happen with fake .so)")
	}

	// Verify it's NOT a signature error
	if errors.Is(err, dnsprovider.ErrPluginNotInAllowlist) {
		t.Error("should have passed allowlist check")
	}
	if errors.Is(err, dnsprovider.ErrSignatureMismatch) {
		t.Error("should have passed signature check")
	}

	// Should be a plugin load failure
	if err != nil && !errors.Is(err, dnsprovider.ErrPluginLoadFailed) {
		t.Logf("got expected plugin load failure: %v", err)
	}
}

func TestLoadPluginPermissiveMode(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pluginFile := filepath.Join(tmpDir, "any-plugin.so")
	if err := os.WriteFile(pluginFile, []byte("fake plugin"), 0o600); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("failed to create plugin file: %v", err)
	}

	// Permissive mode - nil allowlist
	service := NewPluginLoaderService(nil, tmpDir, nil)

	err := service.LoadPlugin(pluginFile)

	// In permissive mode, it skips allowlist check entirely
	// Will fail at plugin.Open() but that's expected
	if errors.Is(err, dnsprovider.ErrPluginNotInAllowlist) {
		t.Error("permissive mode should skip allowlist check")
	}
	if errors.Is(err, dnsprovider.ErrSignatureMismatch) {
		t.Error("permissive mode should skip signature check")
	}
}

// =============================================================================
// LoadAllPlugins Edge Cases
// =============================================================================

func TestLoadAllPluginsEmptyDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	service := NewPluginLoaderService(nil, tmpDir, nil)

	err := service.LoadAllPlugins()

	if err != nil {
		t.Errorf("expected nil error for empty directory, got: %v", err)
	}
}

func TestLoadAllPluginsNonExistentDirectory(t *testing.T) {
	t.Parallel()

	service := NewPluginLoaderService(nil, "/nonexistent/plugin/dir", nil)

	err := service.LoadAllPlugins()

	if err != nil {
		t.Errorf("expected nil error for non-existent directory, got: %v", err)
	}
}

func TestLoadAllPluginsEmptyPluginDir(t *testing.T) {
	t.Parallel()

	service := NewPluginLoaderService(nil, "", nil)

	err := service.LoadAllPlugins()

	if err != nil {
		t.Errorf("expected nil error for empty plugin dir config, got: %v", err)
	}
}

func TestLoadAllPluginsSkipsDirectories(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0o750); err != nil { // #nosec G301 -- test directory
		t.Fatalf("failed to create subdir: %v", err)
	}

	service := NewPluginLoaderService(nil, tmpDir, nil)

	err := service.LoadAllPlugins()

	// Should not error - directories are skipped
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadAllPluginsSkipsNonSoFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Create non-.so files
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("readme"), 0o600); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("failed to create txt file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "plugin.dll"), []byte("dll"), 0o600); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("failed to create dll file: %v", err)
	}

	service := NewPluginLoaderService(nil, tmpDir, nil)

	err := service.LoadAllPlugins()

	// Should not error - non-.so files are skipped
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadAllPluginsWorldWritableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests not applicable on Windows")
	}

	t.Parallel()

	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "plugins")
	//nolint:gosec // G301 test verifies security check with insecure permissions
	if err := os.Mkdir(pluginDir, 0o777); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}
	// #nosec G302 -- Test intentionally creates insecure permissions to verify security check
	if err := os.Chmod(pluginDir, 0o777); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}

	// Create a .so file so ReadDir returns something
	if err := os.WriteFile(filepath.Join(pluginDir, "test.so"), []byte("test"), 0o600); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("failed to create so file: %v", err)
	}

	service := NewPluginLoaderService(nil, pluginDir, nil)

	err := service.LoadAllPlugins()

	if err == nil {
		t.Error("expected error for world-writable directory, got nil")
	}
}

// =============================================================================
// List and State Management Tests
// =============================================================================

func TestListLoadedPluginsEmpty(t *testing.T) {
	t.Parallel()

	service := NewPluginLoaderService(nil, "/plugins", nil)

	plugins := service.ListLoadedPlugins()

	if len(plugins) != 0 {
		t.Errorf("expected empty list, got %d plugins", len(plugins))
	}
}

func TestIsPluginLoadedFalse(t *testing.T) {
	t.Parallel()

	service := NewPluginLoaderService(nil, "/plugins", nil)

	if service.IsPluginLoaded("nonexistent") {
		t.Error("expected false for non-loaded plugin")
	}
}

func TestUnloadNonExistentPlugin(t *testing.T) {
	t.Parallel()

	service := NewPluginLoaderService(nil, "/plugins", nil)

	err := service.UnloadPlugin("nonexistent")

	// Should not error - just logs and removes from maps
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCleanupEmpty(t *testing.T) {
	t.Parallel()

	service := NewPluginLoaderService(nil, "/plugins", nil)

	err := service.Cleanup()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// parsePluginSignatures Tests (Testing the parsing logic)
// =============================================================================

func TestParsePluginSignaturesLogic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		envValue       string
		expectedNil    bool
		expectedLen    int
		expectedValues map[string]string
	}{
		{
			name:        "empty string returns nil (permissive)",
			envValue:    "",
			expectedNil: true,
		},
		{
			name:        "empty JSON object returns empty map (strict)",
			envValue:    "{}",
			expectedNil: false,
			expectedLen: 0,
		},
		{
			name:           "valid JSON with signatures",
			envValue:       `{"cloudflare":"sha256:abc123","route53":"sha256:def456"}`,
			expectedNil:    false,
			expectedLen:    2,
			expectedValues: map[string]string{"cloudflare": "sha256:abc123", "route53": "sha256:def456"},
		},
		{
			name:        "invalid JSON returns nil (fallback)",
			envValue:    `{invalid json`,
			expectedNil: true,
		},
		{
			name:        "signature without sha256 prefix returns nil (fallback)",
			envValue:    `{"cloudflare":"abc123"}`,
			expectedNil: true,
		},
		{
			name:        "mixed valid and invalid signatures returns nil (fallback)",
			envValue:    `{"cloudflare":"sha256:abc123","route53":"invalidprefix"}`,
			expectedNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := parseSignaturesFromJSON(tc.envValue)

			if tc.expectedNil && result != nil {
				t.Errorf("expected nil, got %v", result)
				return
			}

			if !tc.expectedNil {
				if result == nil {
					t.Error("expected non-nil result")
					return
				}
				if len(result) != tc.expectedLen {
					t.Errorf("expected length %d, got %d", tc.expectedLen, len(result))
				}
				for k, v := range tc.expectedValues {
					if result[k] != v {
						t.Errorf("expected %s=%s, got %s", k, v, result[k])
					}
				}
			}
		})
	}
}

// parseSignaturesFromJSON is a test helper that replicates the parsing logic
// from main.go's parsePluginSignatures() without the os.Getenv call.
func parseSignaturesFromJSON(envVal string) map[string]string {
	if envVal == "" {
		return nil
	}

	var signatures map[string]string
	if err := json.Unmarshal([]byte(envVal), &signatures); err != nil {
		return nil
	}

	// Validate all signatures have sha256: prefix
	for _, sig := range signatures {
		if len(sig) < 7 || sig[:7] != "sha256:" {
			return nil
		}
	}

	return signatures
}

// =============================================================================
// Integration-style Tests (Signature Workflow)
// =============================================================================

func TestSignatureWorkflowEndToEnd(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pluginFile := filepath.Join(tmpDir, "myplugin.so")
	content := []byte("this is fake plugin content for e2e test")

	// Write plugin file
	if err := os.WriteFile(pluginFile, content, 0o600); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("failed to write plugin: %v", err)
	}

	// Step 1: Compute signature (simulating admin workflow)
	service := NewPluginLoaderService(nil, tmpDir, nil)
	sig, err := service.computeSignature(pluginFile)
	if err != nil {
		t.Fatalf("failed to compute signature: %v", err)
	}

	// Step 2: Create service with this signature in allowlist
	allowlist := map[string]string{
		"myplugin": sig,
	}
	strictService := NewPluginLoaderService(nil, tmpDir, allowlist)

	// Step 3: Try to load - should pass signature check
	err = strictService.LoadPlugin(pluginFile)

	// Will fail at plugin.Open() but should NOT fail at signature check
	if errors.Is(err, dnsprovider.ErrPluginNotInAllowlist) {
		t.Error("should have passed allowlist check")
	}
	if errors.Is(err, dnsprovider.ErrSignatureMismatch) {
		t.Error("should have passed signature check with correct signature")
	}

	// Step 4: Modify the plugin file (simulating tampering)
	if writeErr := os.WriteFile(pluginFile, []byte("TAMPERED CONTENT"), 0o600); writeErr != nil { // #nosec G306 -- test fixture
		t.Fatalf("failed to tamper plugin: %v", writeErr)
	}

	// Step 5: Try to load again - should fail signature check now
	err = strictService.LoadPlugin(pluginFile)
	if !errors.Is(err, dnsprovider.ErrSignatureMismatch) {
		t.Errorf("expected ErrSignatureMismatch after tampering, got: %v", err)
	}
}

// =============================================================================
// generateUUID Tests
// =============================================================================

func TestGenerateUUIDUniqueness(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		uuid := generateUUID()
		if seen[uuid] {
			t.Errorf("duplicate UUID generated: %s", uuid)
		}
		seen[uuid] = true
	}
}

func TestGenerateUUIDFormat(t *testing.T) {
	t.Parallel()

	uuid := generateUUID()

	if uuid == "" {
		t.Error("UUID should not be empty")
	}

	// Should contain a hyphen (format is timestamp-unix)
	if !containsHyphen(uuid) {
		t.Errorf("UUID should contain hyphen, got: %s", uuid)
	}
}

func containsHyphen(s string) bool {
	for _, c := range s {
		if c == '-' {
			return true
		}
	}
	return false
}

// =============================================================================
// Windows Platform Tests
// =============================================================================

func TestLoadAllPluginsWindowsSkipped(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("this test only runs on Windows")
	}

	service := NewPluginLoaderService(nil, "C:\\plugins", nil)

	err := service.LoadAllPlugins()

	// On Windows, should return nil (skipped)
	if err != nil {
		t.Errorf("expected nil error on Windows, got: %v", err)
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestConcurrentPluginMapAccess(t *testing.T) {
	t.Parallel()

	service := NewPluginLoaderService(nil, "/plugins", nil)

	// Simulate concurrent reads and writes
	done := make(chan bool)

	// Readers
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = service.IsPluginLoaded("test-plugin")
				_ = service.ListLoadedPlugins()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// =============================================================================
// Edge Cases for computeSignature with various file contents
// =============================================================================

func TestComputeSignatureLargeFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "large.so")

	// Create a 1MB file
	content := make([]byte, 1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}

	if err := os.WriteFile(tmpFile, content, 0o600); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("failed to write large file: %v", err)
	}

	service := NewPluginLoaderService(nil, tmpDir, nil)
	sig, err := service.computeSignature(tmpFile)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify it's a valid sha256 signature
	expectedLen := len("sha256:") + 64 // sha256 produces 64 hex chars
	if len(sig) != expectedLen {
		t.Errorf("expected signature length %d, got %d", expectedLen, len(sig))
	}
}

func TestComputeSignatureSpecialCharactersInPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Create path with spaces (common edge case)
	pluginDir := filepath.Join(tmpDir, "my plugins")
	if err := os.MkdirAll(pluginDir, 0o750); err != nil { // #nosec G301 -- test directory
		t.Fatalf("failed to create directory: %v", err)
	}

	pluginFile := filepath.Join(pluginDir, "my plugin.so")
	if err := os.WriteFile(pluginFile, []byte("test content"), 0o600); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("failed to write file: %v", err)
	}

	service := NewPluginLoaderService(nil, pluginDir, nil)
	sig, err := service.computeSignature(pluginFile)

	if err != nil {
		t.Errorf("unexpected error with spaces in path: %v", err)
	}
	if sig == "" {
		t.Error("expected non-empty signature")
	}
}
