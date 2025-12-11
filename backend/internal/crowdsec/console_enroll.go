package crowdsec

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

const (
	consoleStatusNotEnrolled = "not_enrolled"
	consoleStatusEnrolling   = "enrolling"
	consoleStatusEnrolled    = "enrolled"
	consoleStatusFailed      = "failed"

	defaultEnrollTimeout = 45 * time.Second
)

var namePattern = regexp.MustCompile(`^[A-Za-z0-9_.\-]{1,64}$`)
var enrollmentTokenPattern = regexp.MustCompile(`^[A-Za-z0-9]{10,64}$`)

// EnvCommandExecutor executes commands with optional environment overrides.
type EnvCommandExecutor interface {
	ExecuteWithEnv(ctx context.Context, name string, args []string, env map[string]string) ([]byte, error)
}

// SecureCommandExecutor is the production executor that avoids leaking args by passing secrets via env.
type SecureCommandExecutor struct{}

// ExecuteWithEnv runs the command with provided env merged onto the current environment.
func (r *SecureCommandExecutor) ExecuteWithEnv(ctx context.Context, name string, args []string, env map[string]string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), formatEnv(env)...)
	return cmd.CombinedOutput()
}

func formatEnv(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

// ConsoleEnrollRequest captures enrollment input.
type ConsoleEnrollRequest struct {
	EnrollmentKey string
	Tenant        string
	AgentName     string
	Force         bool
}

// ConsoleEnrollmentStatus is the safe, redacted status view.
type ConsoleEnrollmentStatus struct {
	Status          string     `json:"status"`
	Tenant          string     `json:"tenant"`
	AgentName       string     `json:"agent_name"`
	LastError       string     `json:"last_error,omitempty"`
	LastAttemptAt   *time.Time `json:"last_attempt_at,omitempty"`
	EnrolledAt      *time.Time `json:"enrolled_at,omitempty"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	KeyPresent      bool       `json:"key_present"`
	CorrelationID   string     `json:"correlation_id,omitempty"`
}

// ConsoleEnrollmentService manages console enrollment lifecycle and persistence.
type ConsoleEnrollmentService struct {
	db      *gorm.DB
	exec    EnvCommandExecutor
	dataDir string
	key     []byte
	nowFn   func() time.Time
	mu      sync.Mutex
	timeout time.Duration
}

// NewConsoleEnrollmentService constructs a service using the supplied secret material for encryption.
func NewConsoleEnrollmentService(db *gorm.DB, exec EnvCommandExecutor, dataDir, secret string) *ConsoleEnrollmentService {
	return &ConsoleEnrollmentService{
		db:      db,
		exec:    exec,
		dataDir: dataDir,
		key:     deriveKey(secret),
		nowFn:   time.Now,
		timeout: defaultEnrollTimeout,
	}
}

// Status returns the current enrollment state.
func (s *ConsoleEnrollmentService) Status(ctx context.Context) (ConsoleEnrollmentStatus, error) {
	rec, err := s.load(ctx)
	if err != nil {
		return ConsoleEnrollmentStatus{}, err
	}
	return s.statusFromModel(rec), nil
}

// Enroll performs an enrollment attempt. It is idempotent when already enrolled unless Force is set.
func (s *ConsoleEnrollmentService) Enroll(ctx context.Context, req ConsoleEnrollRequest) (ConsoleEnrollmentStatus, error) {
	agent := strings.TrimSpace(req.AgentName)
	if agent == "" {
		return ConsoleEnrollmentStatus{}, fmt.Errorf("agent_name required")
	}
	if !namePattern.MatchString(agent) {
		return ConsoleEnrollmentStatus{}, fmt.Errorf("agent_name may only include letters, numbers, dot, dash, underscore")
	}
	tenant := strings.TrimSpace(req.Tenant)
	if tenant != "" && !namePattern.MatchString(tenant) {
		return ConsoleEnrollmentStatus{}, fmt.Errorf("tenant may only include letters, numbers, dot, dash, underscore")
	}
	token, err := normalizeEnrollmentKey(req.EnrollmentKey)
	if err != nil {
		return ConsoleEnrollmentStatus{}, err
	}
	if s.exec == nil {
		return ConsoleEnrollmentStatus{}, fmt.Errorf("executor unavailable")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rec, err := s.load(ctx)
	if err != nil {
		return ConsoleEnrollmentStatus{}, err
	}

	if rec.Status == consoleStatusEnrolling {
		return s.statusFromModel(rec), fmt.Errorf("enrollment already in progress")
	}
	if rec.Status == consoleStatusEnrolled && !req.Force {
		return s.statusFromModel(rec), nil
	}

	now := s.nowFn().UTC()
	rec.Status = consoleStatusEnrolling
	rec.AgentName = agent
	rec.Tenant = tenant
	rec.LastAttemptAt = &now
	rec.LastError = ""
	rec.LastCorrelationID = uuid.NewString()

	encryptedKey, err := s.encrypt(token)
	if err != nil {
		return ConsoleEnrollmentStatus{}, fmt.Errorf("protect secret: %w", err)
	}
	rec.EncryptedEnrollKey = encryptedKey

	if err := s.db.WithContext(ctx).Save(rec).Error; err != nil {
		return ConsoleEnrollmentStatus{}, err
	}

	cmdCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	args := []string{"console", "enroll", "--name", agent}
	if tenant != "" {
		args = append(args, "--tenant", tenant)
	}

	logger.Log().WithField("tenant", tenant).WithField("agent", agent).WithField("correlation_id", rec.LastCorrelationID).Info("starting crowdsec console enrollment")
	out, cmdErr := s.exec.ExecuteWithEnv(cmdCtx, "cscli", args, map[string]string{"CROWDSEC_CONSOLE_ENROLL_KEY": token})

	if cmdErr != nil {
		rec.Status = consoleStatusFailed
		rec.LastError = redactSecret(string(out)+": "+cmdErr.Error(), token)
		_ = s.db.WithContext(ctx).Save(rec)
		logger.Log().WithError(cmdErr).WithField("correlation_id", rec.LastCorrelationID).WithField("tenant", tenant).Warn("crowdsec console enrollment failed")
		return s.statusFromModel(rec), fmt.Errorf("console enrollment failed: %s", rec.LastError)
	}

	complete := s.nowFn().UTC()
	rec.Status = consoleStatusEnrolled
	rec.EnrolledAt = &complete
	rec.LastHeartbeatAt = &complete
	rec.LastError = ""
	if err := s.db.WithContext(ctx).Save(rec).Error; err != nil {
		return ConsoleEnrollmentStatus{}, err
	}

	logger.Log().WithField("tenant", tenant).WithField("agent", agent).WithField("correlation_id", rec.LastCorrelationID).Info("crowdsec console enrollment succeeded")
	return s.statusFromModel(rec), nil
}

func (s *ConsoleEnrollmentService) load(ctx context.Context) (*models.CrowdsecConsoleEnrollment, error) {
	var rec models.CrowdsecConsoleEnrollment
	err := s.db.WithContext(ctx).First(&rec).Error
	if err == nil {
		return &rec, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	now := s.nowFn().UTC()
	rec = models.CrowdsecConsoleEnrollment{
		UUID:      uuid.NewString(),
		Status:    consoleStatusNotEnrolled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&rec).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

func (s *ConsoleEnrollmentService) statusFromModel(rec *models.CrowdsecConsoleEnrollment) ConsoleEnrollmentStatus {
	if rec == nil {
		return ConsoleEnrollmentStatus{Status: consoleStatusNotEnrolled}
	}
	return ConsoleEnrollmentStatus{
		Status:          firstNonEmpty(rec.Status, consoleStatusNotEnrolled),
		Tenant:          rec.Tenant,
		AgentName:       rec.AgentName,
		LastError:       rec.LastError,
		LastAttemptAt:   rec.LastAttemptAt,
		EnrolledAt:      rec.EnrolledAt,
		LastHeartbeatAt: rec.LastHeartbeatAt,
		KeyPresent:      rec.EncryptedEnrollKey != "",
		CorrelationID:   rec.LastCorrelationID,
	}
}

func (s *ConsoleEnrollmentService) encrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(value), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// decrypt is only used in tests to validate encryption roundtrips.
func (s *ConsoleEnrollmentService) decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	ciphertext, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func deriveKey(secret string) []byte {
	if secret == "" {
		secret = "charon-console-enroll-default"
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func redactSecret(msg, secret string) string {
	if secret == "" {
		return msg
	}
	return strings.ReplaceAll(msg, secret, "<redacted>")
}

func normalizeEnrollmentKey(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("enrollment_key required")
	}
	if enrollmentTokenPattern.MatchString(trimmed) {
		return trimmed, nil
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid enrollment key")
	}
	if strings.EqualFold(parts[0], "sudo") {
		parts = parts[1:]
	}

	if len(parts) == 4 && parts[0] == "cscli" && parts[1] == "console" && parts[2] == "enroll" {
		token := parts[3]
		if enrollmentTokenPattern.MatchString(token) {
			return token, nil
		}
	}

	return "", fmt.Errorf("invalid enrollment key")
}
