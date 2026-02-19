package handlers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetBouncerAPIKeyFromEnv(t *testing.T) {
	envKeys := []string{
		"CROWDSEC_API_KEY",
		"CROWDSEC_BOUNCER_API_KEY",
		"CERBERUS_SECURITY_CROWDSEC_API_KEY",
		"CHARON_SECURITY_CROWDSEC_API_KEY",
		"CPM_SECURITY_CROWDSEC_API_KEY",
	}

	tests := []struct {
		name        string
		envVars     map[string]string
		expectedKey string
	}{
		{
			name: "CROWDSEC_BOUNCER_API_KEY set",
			envVars: map[string]string{
				"CROWDSEC_BOUNCER_API_KEY": "test-bouncer-key-123",
			},
			expectedKey: "test-bouncer-key-123",
		},
		{
			name: "CROWDSEC_API_KEY set",
			envVars: map[string]string{
				"CROWDSEC_API_KEY": "fallback-key-456",
			},
			expectedKey: "fallback-key-456",
		},
		{
			name: "CROWDSEC_API_KEY takes priority over CROWDSEC_BOUNCER_API_KEY",
			envVars: map[string]string{
				"CROWDSEC_BOUNCER_API_KEY": "bouncer-key",
				"CROWDSEC_API_KEY":         "priority-key",
			},
			expectedKey: "priority-key",
		},
		{
			name:        "no env vars set",
			envVars:     map[string]string{},
			expectedKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range envKeys {
				t.Setenv(key, "")
			}

			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			key := getBouncerAPIKeyFromEnv()
			if key != tt.expectedKey {
				t.Errorf("getBouncerAPIKeyFromEnv() key = %q, want %q", key, tt.expectedKey)
			}
		})
	}
}

func TestSaveAndReadKeyFromFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "crowdsec-bouncer-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	keyFile := filepath.Join(tmpDir, "subdir", "bouncer_key")
	testKey := "test-api-key-789"

	// Test saveKeyToFile creates directories and saves key
	if saveErr := saveKeyToFile(keyFile, testKey); saveErr != nil {
		t.Fatalf("saveKeyToFile() error = %v", saveErr)
	}

	// Verify file was created
	info, err := os.Stat(keyFile)
	if err != nil {
		t.Fatalf("key file not created: %v", err)
	}

	// Verify permissions (0600)
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("saveKeyToFile() file permissions = %o, want 0600", perm)
	}

	// Test readKeyFromFile
	readKey := readKeyFromFile(keyFile)
	if readKey != testKey {
		t.Errorf("readKeyFromFile() = %q, want %q", readKey, testKey)
	}
}

func TestReadKeyFromFile_NotExist(t *testing.T) {
	key := readKeyFromFile("/nonexistent/path/bouncer_key")
	if key != "" {
		t.Errorf("readKeyFromFile() = %q, want empty string for nonexistent file", key)
	}
}

func TestSaveKeyToFile_EmptyKey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "crowdsec-bouncer-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	keyFile := filepath.Join(tmpDir, "bouncer_key")

	// Should return error for empty key
	if err := saveKeyToFile(keyFile, ""); err == nil {
		t.Error("saveKeyToFile() expected error for empty key")
	}
}

func TestReadKeyFromFile_WhitespaceHandling(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "crowdsec-bouncer-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	keyFile := filepath.Join(tmpDir, "bouncer_key")
	testKey := "  key-with-whitespace  \n"

	// Write key with whitespace directly
	if err := os.WriteFile(keyFile, []byte(testKey), 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	// readKeyFromFile should trim whitespace
	readKey := readKeyFromFile(keyFile)
	if readKey != "key-with-whitespace" {
		t.Errorf("readKeyFromFile() = %q, want trimmed key", readKey)
	}
}
