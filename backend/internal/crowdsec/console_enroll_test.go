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

	// Expect 2 calls: capi register, then console enroll
	require.Equal(t, 2, exec.callCount())
	require.Equal(t, []string{"capi", "register"}, exec.calls[0].args)
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
	require.Equal(t, 2, exec.callCount()) // capi register + enroll

	status, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{EnrollmentKey: "ignoredignored", Tenant: "tenant", AgentName: "agent"})
	require.NoError(t, err)
	require.Equal(t, consoleStatusEnrolled, status.Status)
	// Should call capi register again (because file missing in temp dir), but then stop because already enrolled
	require.Equal(t, 3, exec.callCount(), "second call should check capi then stop")
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
	// capi register is called before status check
	require.Equal(t, 1, exec.callCount())
	require.Equal(t, []string{"capi", "register"}, exec.lastArgs())
}

func TestConsoleEnrollNormalizesFullCommand(t *testing.T) {
	db := openConsoleTestDB(t)
	exec := &stubEnvExecutor{}
	svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "secret")

	status, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{EnrollmentKey: "sudo cscli console enroll cmj0r0uer000202lebd5luvxh", Tenant: "tenant", AgentName: "agent"})
	require.NoError(t, err)
	require.Equal(t, consoleStatusEnrolled, status.Status)
	require.Equal(t, 2, exec.callCount())
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
	require.Equal(t, 2, exec.callCount())
	require.NotContains(t, exec.lastArgs(), "--tenant")
	// Also verify that the tenant value itself is not passed as a standalone arg just in case
	require.NotContains(t, exec.lastArgs(), "some-tenant-id")
}
