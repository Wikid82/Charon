package crowdsec

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

type stubEnvExecutor struct {
	responses []struct {
		out []byte
		err error
	}
	defaultResponse struct {
		out []byte
		err error
	}
	calls []struct {
		name string
		args []string
		env  map[string]string
	}
}

func (s *stubEnvExecutor) ExecuteWithEnv(ctx context.Context, name string, args []string, env map[string]string) ([]byte, error) {
	s.calls = append(s.calls, struct {
		name string
		args []string
		env  map[string]string
	}{name, args, env})

	if len(s.calls) <= len(s.responses) {
		resp := s.responses[len(s.calls)-1]
		return resp.out, resp.err
	}
	return s.defaultResponse.out, s.defaultResponse.err
}

func (s *stubEnvExecutor) callCount() int {
	return len(s.calls)
}

func (s *stubEnvExecutor) lastArgs() []string {
	if len(s.calls) == 0 {
		return nil
	}
	return s.calls[len(s.calls)-1].args
}

func openConsoleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecConsoleEnrollment{}))
	return db
}

func TestConsoleEnrollSuccess(t *testing.T) {
	db := openConsoleTestDB(t)
	exec := &stubEnvExecutor{} // Default success
	svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "super-secret")
	svc.nowFn = func() time.Time { return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC) }

	status, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{EnrollmentKey: "abc123def4g", Tenant: "tenant-a", AgentName: "agent-one"})
	require.NoError(t, err)
	require.Equal(t, consoleStatusEnrolled, status.Status)
	require.True(t, status.KeyPresent)
	require.NotEmpty(t, status.CorrelationID)

	// Expect 3 calls: lapi status, capi register, then console enroll
	require.Equal(t, 3, exec.callCount())
	require.Contains(t, exec.calls[0].args, "lapi")
	require.Equal(t, []string{"capi", "register"}, exec.calls[1].args)
	require.Equal(t, "abc123def4g", exec.lastArgs()[len(exec.lastArgs())-1])

	var rec models.CrowdsecConsoleEnrollment
	require.NoError(t, db.First(&rec).Error)
	require.NotEqual(t, "abc123def4g", rec.EncryptedEnrollKey)
	plain, decErr := svc.decrypt(rec.EncryptedEnrollKey)
	require.NoError(t, decErr)
	require.Equal(t, "abc123def4g", plain)
}

func TestConsoleEnrollFailureRedactsSecret(t *testing.T) {
	db := openConsoleTestDB(t)
	exec := &stubEnvExecutor{
		responses: []struct {
			out []byte
			err error
		}{
			{out: nil, err: nil}, // lapi status success
			{out: nil, err: nil}, // capi register success
			{out: []byte("invalid secretKEY123"), err: fmt.Errorf("bad key secretKEY123")}, // enroll failure
		},
	}
	svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "redactme")

	status, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{EnrollmentKey: "secretKEY123", Tenant: "tenant", AgentName: "agent"})
	require.Error(t, err)
	require.Equal(t, consoleStatusFailed, status.Status)
	require.NotContains(t, status.LastError, "secretKEY123")
	require.NotContains(t, err.Error(), "secretKEY123")
}

func TestConsoleEnrollIdempotentWhenAlreadyEnrolled(t *testing.T) {
	db := openConsoleTestDB(t)
	exec := &stubEnvExecutor{}
	svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "secret")

	_, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{EnrollmentKey: "abc123def4g", Tenant: "tenant", AgentName: "agent"})
	require.NoError(t, err)
	require.Equal(t, 3, exec.callCount()) // lapi status + capi register + enroll

	status, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{EnrollmentKey: "ignoredignored", Tenant: "tenant", AgentName: "agent"})
	require.NoError(t, err)
	require.Equal(t, consoleStatusEnrolled, status.Status)
	// Should call lapi status and capi register again, but then stop because already enrolled
	require.Equal(t, 5, exec.callCount(), "second call should check lapi, then capi, then stop")
	require.Equal(t, []string{"capi", "register"}, exec.lastArgs())
}

func TestConsoleEnrollBlockedWhenInProgress(t *testing.T) {
	db := openConsoleTestDB(t)
	rec := models.CrowdsecConsoleEnrollment{UUID: "u1", Status: consoleStatusEnrolling}
	require.NoError(t, db.Create(&rec).Error)

	exec := &stubEnvExecutor{}
	svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "secret")
	status, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{EnrollmentKey: "abc123def4g", Tenant: "tenant", AgentName: "agent"})
	require.Error(t, err)
	require.Equal(t, consoleStatusEnrolling, status.Status)
	// lapi status and capi register are called before status check blocks enrollment
	require.Equal(t, 2, exec.callCount())
	require.Contains(t, exec.calls[0].args, "lapi")
	require.Contains(t, exec.calls[0].args, "status")
	require.Equal(t, []string{"capi", "register"}, exec.calls[1].args)
}

func TestConsoleEnrollNormalizesFullCommand(t *testing.T) {
	db := openConsoleTestDB(t)
	exec := &stubEnvExecutor{}
	svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "secret")

	status, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{EnrollmentKey: "sudo cscli console enroll cmj0r0uer000202lebd5luvxh", Tenant: "tenant", AgentName: "agent"})
	require.NoError(t, err)
	require.Equal(t, consoleStatusEnrolled, status.Status)
	require.Equal(t, 3, exec.callCount()) // lapi status + capi register + enroll
	require.Equal(t, "cmj0r0uer000202lebd5luvxh", exec.lastArgs()[len(exec.lastArgs())-1])
}

func TestConsoleEnrollRejectsUnsafeInput(t *testing.T) {
	db := openConsoleTestDB(t)
	exec := &stubEnvExecutor{}
	svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "secret")

	_, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{EnrollmentKey: "cscli console enroll cmj0r0uer000202lebd5luvxh; rm -rf /", Tenant: "tenant", AgentName: "agent"})
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "invalid enrollment key")
	require.Equal(t, 0, exec.callCount())
}

func TestConsoleEnrollDoesNotPassTenant(t *testing.T) {
	db := openConsoleTestDB(t)
	exec := &stubEnvExecutor{}
	svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "secret")

	// Even if tenant is provided in the request
	req := ConsoleEnrollRequest{
		EnrollmentKey: "abc123def4g",
		Tenant:        "some-tenant-id",
		AgentName:     "agent-one",
	}

	status, err := svc.Enroll(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, consoleStatusEnrolled, status.Status)

	// Verify that --tenant is NOT passed to the command arguments
	require.Equal(t, 3, exec.callCount()) // lapi status + capi register + enroll
	require.NotContains(t, exec.lastArgs(), "--tenant")
	// Also verify that the tenant value itself is not passed as a standalone arg just in case
	require.NotContains(t, exec.lastArgs(), "some-tenant-id")
}

// ============================================
// SecureCommandExecutor Tests
// ============================================

func TestSecureCommandExecutorExecuteWithEnv(t *testing.T) {
	// Skip this test if not on a system with echo command
	exec := &SecureCommandExecutor{}
	ctx := context.Background()

	t.Run("executes command successfully", func(t *testing.T) {
		output, err := exec.ExecuteWithEnv(ctx, "echo", []string{"hello"}, nil)
		require.NoError(t, err)
		require.Contains(t, string(output), "hello")
	})

	t.Run("passes environment variables", func(t *testing.T) {
		output, err := exec.ExecuteWithEnv(ctx, "sh", []string{"-c", "echo $TEST_VAR"}, map[string]string{"TEST_VAR": "test_value"})
		require.NoError(t, err)
		require.Contains(t, string(output), "test_value")
	})

	t.Run("handles empty env map", func(t *testing.T) {
		output, err := exec.ExecuteWithEnv(ctx, "echo", []string{"no-env"}, map[string]string{})
		require.NoError(t, err)
		require.Contains(t, string(output), "no-env")
	})

	t.Run("handles command failure", func(t *testing.T) {
		_, err := exec.ExecuteWithEnv(ctx, "false", nil, nil)
		require.Error(t, err)
	})

	t.Run("handles context timeout", func(t *testing.T) {
		ctxTimeout, cancel := context.WithTimeout(ctx, time.Nanosecond)
		defer cancel()
		time.Sleep(time.Millisecond) // ensure timeout

		_, err := exec.ExecuteWithEnv(ctxTimeout, "sleep", []string{"10"}, nil)
		require.Error(t, err)
	})
}

// ============================================
// formatEnv Tests
// ============================================

func TestFormatEnv(t *testing.T) {
	t.Run("formats single env var", func(t *testing.T) {
		result := formatEnv(map[string]string{"KEY": "value"})
		require.Len(t, result, 1)
		require.Equal(t, "KEY=value", result[0])
	})

	t.Run("formats multiple env vars", func(t *testing.T) {
		result := formatEnv(map[string]string{
			"KEY1": "value1",
			"KEY2": "value2",
		})
		require.Len(t, result, 2)
		require.Contains(t, result, "KEY1=value1")
		require.Contains(t, result, "KEY2=value2")
	})

	t.Run("handles empty map", func(t *testing.T) {
		result := formatEnv(map[string]string{})
		require.Nil(t, result)
	})

	t.Run("handles nil map", func(t *testing.T) {
		result := formatEnv(nil)
		require.Nil(t, result)
	})

	t.Run("handles special characters", func(t *testing.T) {
		result := formatEnv(map[string]string{"KEY": "value with spaces"})
		require.Len(t, result, 1)
		require.Equal(t, "KEY=value with spaces", result[0])
	})
}

// ============================================
// ConsoleEnrollmentService.Status Tests
// ============================================

func TestConsoleEnrollmentStatus(t *testing.T) {
	t.Run("returns not_enrolled for new service", func(t *testing.T) {
		db := openConsoleTestDB(t)
		exec := &stubEnvExecutor{}
		svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "secret")

		status, err := svc.Status(context.Background())
		require.NoError(t, err)
		require.Equal(t, consoleStatusNotEnrolled, status.Status)
	})

	t.Run("returns enrolled status after enrollment", func(t *testing.T) {
		db := openConsoleTestDB(t)
		exec := &stubEnvExecutor{}
		svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "secret")

		// First enroll
		_, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{
			EnrollmentKey: "abc123def4g",
			AgentName:     "test-agent",
		})
		require.NoError(t, err)

		// Then check status
		status, err := svc.Status(context.Background())
		require.NoError(t, err)
		require.Equal(t, consoleStatusEnrolled, status.Status)
		require.Equal(t, "test-agent", status.AgentName)
		require.True(t, status.KeyPresent)
		require.NotNil(t, status.EnrolledAt)
	})

	t.Run("returns failed status after failed enrollment", func(t *testing.T) {
		db := openConsoleTestDB(t)
		exec := &stubEnvExecutor{
			responses: []struct {
				out []byte
				err error
			}{
				{out: nil, err: nil}, // lapi status success
				{out: nil, err: nil}, // capi register success
				{out: []byte("error"), err: fmt.Errorf("enroll failed")}, // enroll failure
			},
		}
		svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "secret")

		// Attempt enrollment (will fail)
		_, _ = svc.Enroll(context.Background(), ConsoleEnrollRequest{
			EnrollmentKey: "abc123def4g",
			AgentName:     "test-agent",
		})

		// Check status
		status, err := svc.Status(context.Background())
		require.NoError(t, err)
		require.Equal(t, consoleStatusFailed, status.Status)
		require.NotEmpty(t, status.LastError)
	})
}

// ============================================
// deriveKey Tests
// ============================================

func TestDeriveKey(t *testing.T) {
	t.Run("derives consistent key", func(t *testing.T) {
		key1 := deriveKey("secret")
		key2 := deriveKey("secret")
		require.Equal(t, key1, key2)
	})

	t.Run("derives different keys for different secrets", func(t *testing.T) {
		key1 := deriveKey("secret1")
		key2 := deriveKey("secret2")
		require.NotEqual(t, key1, key2)
	})

	t.Run("uses default for empty secret", func(t *testing.T) {
		key := deriveKey("")
		require.Len(t, key, 32) // SHA-256 output
	})
}

// ============================================
// normalizeEnrollmentKey Tests
// ============================================

func TestNormalizeEnrollmentKey(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{
			name:     "valid raw key",
			input:    "abc123def4g",
			expected: "abc123def4g",
		},
		{
			name:     "full command with sudo",
			input:    "sudo cscli console enroll abc123def4g",
			expected: "abc123def4g",
		},
		{
			name:     "full command without sudo",
			input:    "cscli console enroll abc123def4g",
			expected: "abc123def4g",
		},
		{
			name:     "key with whitespace",
			input:    "  abc123def4g  ",
			expected: "abc123def4g",
		},
		{
			name:      "empty key",
			input:     "",
			expectErr: true,
		},
		{
			name:      "only whitespace",
			input:     "   ",
			expectErr: true,
		},
		{
			name:      "invalid format",
			input:     "invalid key format here",
			expectErr: true,
		},
		{
			name:      "injection attempt",
			input:     "abc123; rm -rf /",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := normalizeEnrollmentKey(tc.input)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, result)
			}
		})
	}
}

// ============================================
// redactSecret Tests
// ============================================

func TestRedactSecret(t *testing.T) {
	t.Run("redacts secret from message", func(t *testing.T) {
		result := redactSecret("Error: invalid token secretKEY123", "secretKEY123")
		require.Equal(t, "Error: invalid token <redacted>", result)
	})

	t.Run("handles empty secret", func(t *testing.T) {
		result := redactSecret("Error message", "")
		require.Equal(t, "Error message", result)
	})

	t.Run("handles secret not in message", func(t *testing.T) {
		result := redactSecret("Error message", "secret")
		require.Equal(t, "Error message", result)
	})

	t.Run("redacts multiple occurrences", func(t *testing.T) {
		result := redactSecret("Token ABC123 failed, please retry with ABC123", "ABC123")
		require.Equal(t, "Token <redacted> failed, please retry with <redacted>", result)
	})
}

// ============================================
// Encryption Tests
// ============================================

func TestEncryptDecrypt(t *testing.T) {
	db := openConsoleTestDB(t)
	svc := NewConsoleEnrollmentService(db, &stubEnvExecutor{}, t.TempDir(), "test-secret")

	t.Run("encrypts and decrypts successfully", func(t *testing.T) {
		original := "sensitive-enrollment-key"
		encrypted, err := svc.encrypt(original)
		require.NoError(t, err)
		require.NotEqual(t, original, encrypted)

		decrypted, err := svc.decrypt(encrypted)
		require.NoError(t, err)
		require.Equal(t, original, decrypted)
	})

	t.Run("handles empty string", func(t *testing.T) {
		encrypted, err := svc.encrypt("")
		require.NoError(t, err)
		require.Empty(t, encrypted)

		decrypted, err := svc.decrypt("")
		require.NoError(t, err)
		require.Empty(t, decrypted)
	})

	t.Run("different encryptions produce different ciphertext", func(t *testing.T) {
		original := "test-key"
		encrypted1, _ := svc.encrypt(original)
		encrypted2, _ := svc.encrypt(original)
		require.NotEqual(t, encrypted1, encrypted2, "encryptions should use different nonces")
	})
}

// ============================================
// LAPI Availability Check Tests
// ============================================

// TestEnroll_RequiresLAPI verifies that enrollment fails with proper error when LAPI is not running.
// This ensures users get clear feedback to enable CrowdSec via GUI before attempting enrollment.
func TestEnroll_RequiresLAPI(t *testing.T) {
	db := openConsoleTestDB(t)
	exec := &stubEnvExecutor{
		responses: []struct {
			out []byte
			err error
		}{
			{out: nil, err: fmt.Errorf("dial tcp 127.0.0.1:8085: connection refused")}, // lapi status fails
		},
	}
	svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "secret")

	_, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{
		EnrollmentKey: "test123token",
		AgentName:     "agent",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "Local API is not running")
	require.Contains(t, err.Error(), "Security dashboard")

	// Verify that we called lapi status (first call)
	require.Equal(t, 1, exec.callCount())
	require.Contains(t, exec.calls[0].args, "lapi")
	require.Contains(t, exec.calls[0].args, "status")
}
