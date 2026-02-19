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
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
)

const (
	consoleStatusNotEnrolled       = "not_enrolled"
	consoleStatusEnrolling         = "enrolling"
	consoleStatusPendingAcceptance = "pending_acceptance"
	consoleStatusEnrolled          = "enrolled"
	consoleStatusFailed            = "failed"

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
func NewConsoleEnrollmentService(db *gorm.DB, executor EnvCommandExecutor, dataDir, secret string) *ConsoleEnrollmentService {
	return &ConsoleEnrollmentService{
		db:      db,
		exec:    executor,
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

	// CRITICAL: Check that LAPI is running before attempting enrollment
	// Console enrollment requires an active LAPI connection to register with crowdsec.net
	if checkErr := s.checkLAPIAvailable(ctx); checkErr != nil {
		return ConsoleEnrollmentStatus{}, checkErr
	}

	if ensureErr := s.ensureCAPIRegistered(ctx); ensureErr != nil {
		return ConsoleEnrollmentStatus{}, ensureErr
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
	// If already enrolled or pending acceptance, skip unless Force is set
	if (rec.Status == consoleStatusEnrolled || rec.Status == consoleStatusPendingAcceptance) && !req.Force {
		logger.Log().WithFields(map[string]any{
			"status":     rec.Status,
			"agent_name": rec.AgentName,
			"tenant":     rec.Tenant,
		}).Info("console enrollment skipped: already enrolled or pending acceptance - use force=true to re-enroll")
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

	// Add tenant as a tag if provided
	if tenant != "" {
		args = append(args, "--tags", fmt.Sprintf("tenant:%s", tenant))
	}

	// Add overwrite flag if force is requested
	if req.Force {
		args = append(args, "--overwrite")
	}

	// Add config path
	configPath := s.findConfigPath()
	if configPath != "" {
		args = append([]string{"-c", configPath}, args...)
	}

	// Token is the last positional argument
	args = append(args, token)

	logger.Log().Info("starting crowdsec console enrollment")
	out, cmdErr := s.exec.ExecuteWithEnv(cmdCtx, "cscli", args, nil)

	// Log command output for debugging (redacting the token)
	redactedOut := redactSecret(string(out), token)
	if cmdErr != nil {
		rec.Status = consoleStatusFailed
		// Redact token from both output and error message
		redactedErr := redactSecret(cmdErr.Error(), token)
		// Extract the meaningful error message from cscli output
		userMessage := extractCscliErrorMessage(redactedOut)
		if userMessage == "" {
			userMessage = redactedOut
		}
		rec.LastError = userMessage
		_ = s.db.WithContext(ctx).Save(rec)
		logger.Log().WithField("error", util.SanitizeForLog(redactedErr)).WithField("correlation_id", rec.LastCorrelationID).WithField("tenant", util.SanitizeForLog(tenant)).WithField("output", util.SanitizeForLog(redactedOut)).Warn("crowdsec console enrollment failed")
		return s.statusFromModel(rec), fmt.Errorf("%s", userMessage)
	}

	logger.Log().WithField("correlation_id", rec.LastCorrelationID).WithField("output", util.SanitizeForLog(redactedOut)).Debug("cscli console enroll command output")

	// Enrollment request was sent successfully, but user must still accept it on crowdsec.net.
	// cscli console enroll returns exit code 0 when the request is sent, NOT when enrollment is complete.
	// The CrowdSec help explicitly states: "After running this command your will need to validate the enrollment in the webapp."
	complete := s.nowFn().UTC()
	rec.Status = consoleStatusPendingAcceptance
	rec.LastAttemptAt = &complete
	rec.LastError = ""
	if err := s.db.WithContext(ctx).Save(rec).Error; err != nil {
		return ConsoleEnrollmentStatus{}, err
	}

	logger.Log().WithField("tenant", util.SanitizeForLog(tenant)).WithField("agent", util.SanitizeForLog(agent)).WithField("correlation_id", rec.LastCorrelationID).Info("crowdsec console enrollment request sent - pending acceptance on crowdsec.net")
	return s.statusFromModel(rec), nil
}

// checkLAPIAvailable verifies that CrowdSec Local API is running and reachable.
// This is critical for console enrollment as the enrollment process requires LAPI.
// It retries up to 5 times with exponential backoff (3s, 6s, 12s, 24s) to handle LAPI initialization timing.
// Total wait time: ~45 seconds max.
func (s *ConsoleEnrollmentService) checkLAPIAvailable(ctx context.Context) error {
	maxRetries := 5
	baseDelay := 3 * time.Second

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		args := []string{"lapi", "status"}
		configPath := s.findConfigPath()
		if configPath != "" {
			args = append([]string{"-c", configPath}, args...)
		}

		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		out, err := s.exec.ExecuteWithEnv(checkCtx, "cscli", args, nil)
		cancel()

		if err == nil {
			logger.Log().WithField("config", configPath).Debug("LAPI check succeeded")
			return nil // LAPI is available
		}

		lastErr = err
		if i < maxRetries-1 {
			// Exponential backoff: 3s, 6s, 12s, 24s
			delay := baseDelay * time.Duration(1<<uint(i))
			logger.Log().WithError(err).WithField("attempt", i+1).WithField("next_delay_s", delay.Seconds()).WithField("output", string(out)).Debug("LAPI not ready, retrying with exponential backoff")
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("CrowdSec Local API is not running after %d attempts (~45s total wait) - please wait for LAPI to initialize or check CrowdSec logs: %w", maxRetries, lastErr)
}

func (s *ConsoleEnrollmentService) ensureCAPIRegistered(ctx context.Context) error {
	// Check for credentials in config subdirectory first (standard layout),
	// then fall back to dataDir root for backward compatibility
	credsPath := filepath.Join(s.dataDir, "config", "online_api_credentials.yaml")
	if _, err := os.Stat(credsPath); err == nil {
		return nil
	}
	credsPath = filepath.Join(s.dataDir, "online_api_credentials.yaml")
	if _, err := os.Stat(credsPath); err == nil {
		return nil
	}

	logger.Log().Info("registering with crowdsec capi")
	args := []string{"capi", "register"}
	configPath := s.findConfigPath()
	if configPath != "" {
		args = append([]string{"-c", configPath}, args...)
	}

	out, err := s.exec.ExecuteWithEnv(ctx, "cscli", args, nil)
	if err != nil {
		return fmt.Errorf("capi register: %s: %w", string(out), err)
	}
	return nil
}

// findConfigPath returns the path to the CrowdSec config file, checking
// config subdirectory first (standard layout), then dataDir root.
// Returns empty string if no config file is found.
func (s *ConsoleEnrollmentService) findConfigPath() string {
	configPath := filepath.Join(s.dataDir, "config", "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return configPath
	}
	configPath = filepath.Join(s.dataDir, "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return configPath
	}
	return ""
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

// ClearEnrollment resets the enrollment state to allow fresh enrollment.
// This does NOT unenroll from crowdsec.net - that must be done manually on the console.
func (s *ConsoleEnrollmentService) ClearEnrollment(ctx context.Context) error {
	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var rec models.CrowdsecConsoleEnrollment
	if err := s.db.WithContext(ctx).First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // Already cleared
		}
		return fmt.Errorf("failed to find enrollment record: %w", err)
	}

	logger.Log().WithField("previous_status", rec.Status).Info("clearing console enrollment state")

	// Delete the record
	if err := s.db.WithContext(ctx).Delete(&rec).Error; err != nil {
		return fmt.Errorf("failed to delete enrollment record: %w", err)
	}

	return nil
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

// extractCscliErrorMessage extracts the meaningful error message from cscli output.
// CrowdSec outputs error messages in formats like:
// - "level=error msg=\"...\""
// - "ERRO[...] ..."
// - Plain error text
//
// It also translates common CrowdSec errors into user-friendly messages.
func extractCscliErrorMessage(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}

	lowerOutput := strings.ToLower(output)

	// Check for specific error patterns and provide actionable messages
	errorPatterns := map[string]string{
		"token is expired":          "Enrollment token has expired. Please generate a new token from crowdsec.net console.",
		"token is invalid":          "Enrollment token is invalid. Please verify the token from crowdsec.net console.",
		"already enrolled":          "Agent is already enrolled. Use force=true to re-enroll.",
		"lapi is not reachable":     "Cannot reach Local API. Ensure CrowdSec is running and LAPI is initialized.",
		"capi is not reachable":     "Cannot reach Central API. Check network connectivity to crowdsec.net.",
		"connection refused":        "CrowdSec Local API refused connection. Ensure CrowdSec is running.",
		"no such file or directory": "CrowdSec configuration file not found. Run CrowdSec initialization first.",
		"permission denied":         "Permission denied. Ensure the process has access to CrowdSec configuration.",
	}

	for pattern, message := range errorPatterns {
		if strings.Contains(lowerOutput, pattern) {
			return message
		}
	}

	// Try to extract from level=error msg="..." format
	msgPattern := regexp.MustCompile(`msg="([^"]+)"`)
	if matches := msgPattern.FindStringSubmatch(output); len(matches) > 1 {
		return matches[1]
	}

	// Try to extract from ERRO[...] format - get text after the timestamp bracket
	erroPattern := regexp.MustCompile(`ERRO\[[^\]]*\]\s*(.+)`)
	if matches := erroPattern.FindStringSubmatch(output); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Try to find any line containing "error" or "failed" (case-insensitive)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "invalid") {
			return strings.TrimSpace(line)
		}
	}

	// If no pattern matched, return the first non-empty line (often the most relevant)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}

	return output
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
