package services

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
)

// bouncerCommandCount counts the bouncer cscli calls the scripted executor recorded.
func bouncerCommandCount(e *scriptedBouncerExecutor) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, c := range e.calls {
		if strings.Contains(strings.Join(c, " "), "bouncers") {
			n++
		}
	}
	return n
}

// TestEnsureBouncerRegistrationHeldKeySurvivesSecondBoot: a registered key that cannot be saved is held, and
// a second boot in the same process validates it and registers nothing new.
func TestEnsureBouncerRegistrationHeldKeySurvivesSecondBoot(t *testing.T) {
	h := newBouncerHarness(t)
	h.writeFileKey(t, "rejected-saved-key-000000000001")
	keyDir := filepath.Dir(h.keyFile)
	require.NoError(t, os.Chmod(h.keyFile, 0o400))
	require.NoError(t, os.Chmod(keyDir, 0o500))
	t.Cleanup(func() {
		_ = os.Chmod(keyDir, 0o700)
		_ = os.Chmod(h.keyFile, 0o600)
		bouncerkey.Release(h.keyFile)
	})
	exec := &scriptedBouncerExecutor{addOutput: []byte(testGeneratedKey)}

	require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))
	require.Equal(t, testGeneratedKey, bouncerkey.Held(h.keyFile), "the unsaved key is held")
	afterFirst := bouncerCommandCount(exec)
	require.Equal(t, 2, afterFirst, "the first boot deletes and adds the bouncer once")

	require.NoError(t, ensureBouncerRegistration(h.dataDir, h.keyFile, exec))
	require.Equal(t, afterFirst, bouncerCommandCount(exec), "the second boot must not delete or add a bouncer")
	require.Equal(t, testGeneratedKey, bouncerkey.Held(h.keyFile))
}

// TestLogStartupKeyDecisionHeldKeyIsInfoWithoutKey: a held key that validates logs one Info line and no key text.
func TestLogStartupKeyDecisionHeldKeyIsInfoWithoutKey(t *testing.T) {
	h := newBouncerHarness(t)

	logStartupKeyDecision(bouncerkey.EnsureResult{
		Outcome:     bouncerkey.OutcomeHeldKeyValid,
		Key:         testGeneratedKey,
		HeldSaveErr: errors.New("read-only file system"),
	}, h.keyFile, nil)

	out := h.channels(h.drain())
	require.Contains(t, strings.Join(out, "\n"), "still in use")
	assertLogsCarryNoKeyText(t, out, testGeneratedKey)
}
