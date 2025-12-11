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
	out       []byte
	err       error
	callCount int
	lastEnv   map[string]string
}

func (s *stubEnvExecutor) ExecuteWithEnv(ctx context.Context, name string, args []string, env map[string]string) ([]byte, error) {
	s.callCount++
	s.lastEnv = env
	return s.out, s.err
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
	exec := &stubEnvExecutor{}
	svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "super-secret")
	svc.nowFn = func() time.Time { return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC) }

	status, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{EnrollmentKey: "abc123def4g", Tenant: "tenant-a", AgentName: "agent-one"})
	require.NoError(t, err)
	require.Equal(t, consoleStatusEnrolled, status.Status)
	require.True(t, status.KeyPresent)
	require.NotEmpty(t, status.CorrelationID)
	require.Equal(t, 1, exec.callCount)
	require.Equal(t, "abc123def4g", exec.lastEnv["CROWDSEC_CONSOLE_ENROLL_KEY"])

	var rec models.CrowdsecConsoleEnrollment
	require.NoError(t, db.First(&rec).Error)
	require.NotEqual(t, "abc123def4g", rec.EncryptedEnrollKey)
	plain, decErr := svc.decrypt(rec.EncryptedEnrollKey)
	require.NoError(t, decErr)
	require.Equal(t, "abc123def4g", plain)
}

func TestConsoleEnrollFailureRedactsSecret(t *testing.T) {
	db := openConsoleTestDB(t)
	exec := &stubEnvExecutor{out: []byte("invalid secretKEY123"), err: fmt.Errorf("bad key secretKEY123")}
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
	require.Equal(t, 1, exec.callCount)

	status, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{EnrollmentKey: "ignoredignored", Tenant: "tenant", AgentName: "agent"})
	require.NoError(t, err)
	require.Equal(t, consoleStatusEnrolled, status.Status)
	require.Equal(t, 1, exec.callCount, "second call should be idempotent")
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
	require.Equal(t, 0, exec.callCount)
}

func TestConsoleEnrollNormalizesFullCommand(t *testing.T) {
	db := openConsoleTestDB(t)
	exec := &stubEnvExecutor{}
	svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "secret")

	status, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{EnrollmentKey: "sudo cscli console enroll cmj0r0uer000202lebd5luvxh", Tenant: "tenant", AgentName: "agent"})
	require.NoError(t, err)
	require.Equal(t, consoleStatusEnrolled, status.Status)
	require.Equal(t, "cmj0r0uer000202lebd5luvxh", exec.lastEnv["CROWDSEC_CONSOLE_ENROLL_KEY"])
}

func TestConsoleEnrollRejectsUnsafeInput(t *testing.T) {
	db := openConsoleTestDB(t)
	exec := &stubEnvExecutor{}
	svc := NewConsoleEnrollmentService(db, exec, t.TempDir(), "secret")

	_, err := svc.Enroll(context.Background(), ConsoleEnrollRequest{EnrollmentKey: "cscli console enroll cmj0r0uer000202lebd5luvxh; rm -rf /", Tenant: "tenant", AgentName: "agent"})
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "invalid enrollment key")
	require.Equal(t, 0, exec.callCount)
}
