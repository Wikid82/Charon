package services

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"fmt"
	"sync"

	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

// SetupCodeMinLength is the shortest operator-provided setup code accepted from
// CHARON_SETUP_TOKEN.
const SetupCodeMinLength = 32

// setupCodeEntropyBytes is the random entropy of a generated setup code.
const setupCodeEntropyBytes = 32

// SetupCodeIssue describes the first-run setup code issued for this start.
type SetupCodeIssue struct {
	// Pending reports whether a setup code is active for this start.
	Pending bool
	// Generated reports whether the code was generated for this start. False means
	// it came from CHARON_SETUP_TOKEN, which the operator already holds.
	Generated bool
	// Code is the plaintext of a generated code, so the caller can show it once.
	// It is empty for a preset: the operator's secret is never returned.
	Code string
}

// SetupCodeGuard checks the first-run setup code. It keeps only the SHA-256 digest
// of the code, so the plaintext exists only in the startup log line and the caller
// that issued it. A nil or discarded guard accepts no code.
type SetupCodeGuard struct {
	mu     sync.RWMutex
	digest []byte
}

// NewSetupCodeGuard returns a guard that accepts code. An empty code yields an
// inactive guard.
func NewSetupCodeGuard(code string) *SetupCodeGuard {
	g := &SetupCodeGuard{}
	if code != "" {
		sum := sha256.Sum256([]byte(code))
		g.digest = sum[:]
	}
	return g
}

// Active reports whether a code is still accepted.
func (g *SetupCodeGuard) Active() bool {
	if g == nil {
		return false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.digest != nil
}

// Verify reports whether candidate matches the active code. The comparison runs
// over fixed-size digests in constant time.
func (g *SetupCodeGuard) Verify(candidate string) bool {
	if g == nil {
		return false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.digest == nil {
		return false
	}
	sum := sha256.Sum256([]byte(candidate))
	return subtle.ConstantTimeCompare(g.digest, sum[:]) == 1
}

// Discard ends acceptance of the code. It runs after the first account exists.
func (g *SetupCodeGuard) Discard() {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.digest = nil
}

// ResolveSetupCode returns the operator preset when one is given, after checking
// its length, and otherwise a newly generated code.
func ResolveSetupCode(preset string) (string, error) {
	if err := validateSetupPreset(preset); err != nil {
		return "", err
	}
	if preset != "" {
		return preset, nil
	}
	return generateSetupCode()
}

// IssueStartupSetupCode decides the first-run setup code for this start. While the
// users table is empty it returns an active guard and a SetupCodeIssue. A generated
// code is included in the issue so the caller can show it once; a preset is only
// flagged as not generated. Once any user exists it returns an inactive guard and
// a zero SetupCodeIssue. An invalid preset is rejected in both cases so a
// misconfiguration fails at boot.
func IssueStartupSetupCode(db *gorm.DB, preset string) (*SetupCodeGuard, SetupCodeIssue, error) {
	if err := validateSetupPreset(preset); err != nil {
		return nil, SetupCodeIssue{}, err
	}

	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		return nil, SetupCodeIssue{}, fmt.Errorf("count users for setup code: %w", err)
	}
	if count > 0 {
		return NewSetupCodeGuard(""), SetupCodeIssue{}, nil
	}

	code, err := ResolveSetupCode(preset)
	if err != nil {
		return nil, SetupCodeIssue{}, err
	}
	issue := SetupCodeIssue{Pending: true, Generated: preset == ""}
	if issue.Generated {
		issue.Code = code
	}
	return NewSetupCodeGuard(code), issue, nil
}

func validateSetupPreset(preset string) error {
	if preset != "" && len(preset) < SetupCodeMinLength {
		return fmt.Errorf("CHARON_SETUP_TOKEN must be at least %d characters", SetupCodeMinLength)
	}
	return nil
}

func generateSetupCode() (string, error) {
	buf := make([]byte, setupCodeEntropyBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate setup code: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}
