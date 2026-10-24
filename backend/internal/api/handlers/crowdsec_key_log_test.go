package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
)

const logTestRejectedEnvKey = "rejected-env-key-log-0000000000008"

// rejectedEnvWarnLog runs handleBouncerRegistration for a rejected env key with the given save
// outcome and returns the key path it logs plus every captured log line.
func rejectedEnvWarnLog(t *testing.T, saveErr error) (keyPath, logs string) {
	t.Helper()
	buf := captureLogs(t)
	h := newPrecedenceHandler(t, "http://127.0.0.1:1", t.TempDir(), noCSCLIMock())
	res := bouncerkey.EnsureResult{
		Key:            precNewKey,
		Outcome:        bouncerkey.OutcomeRegistered,
		EnvKey:         logTestRejectedEnvKey,
		EnvKeyRejected: true,
		SaveErr:        saveErr,
	}

	h.handleBouncerRegistration(context.Background(), res, nil, nil)
	return h.bouncerKeyPath(), buf.String()
}

func TestHandleBouncerRegistration_RejectedEnvLogNamesSavedKeyOnSuccess(t *testing.T) {
	keyPath, out := rejectedEnvWarnLog(t, nil)

	assert.Contains(t, out, "Charon uses the key saved in "+keyPath)
	assert.NotContains(t, out, "could not save")
	assert.NotContains(t, out, "Failed to save bouncer key")
	assert.NotContains(t, out, logTestRejectedEnvKey, "the rejected key must never be logged")
}

func TestHandleBouncerRegistration_RejectedEnvLogOnSaveFailure(t *testing.T) {
	keyPath, out := rejectedEnvWarnLog(t, errors.New("disk full"))

	assert.Contains(t, out, "Charon could not save its new key to "+keyPath)
	assert.Contains(t, out, "valid for this session only")
	assert.NotContains(t, out, "uses the key saved in", "a failed save must not claim the file holds the key in use")
	assert.Contains(t, out, `"level":"error"`, "the save failure must be logged at Error")
	assert.Contains(t, out, "Failed to save bouncer key to file")
	assert.NotContains(t, out, logTestRejectedEnvKey, "the rejected key must never be logged")
}

func TestRejectedEnvLogMessage_Variants(t *testing.T) {
	const path = "/app/data/crowdsec/bouncer_key"

	require.Contains(t, rejectedEnvLogMessage(path, nil), "uses the key saved in "+path)
	require.Contains(t, rejectedEnvLogMessage(path, errors.New("x")), "could not save its new key to "+path)
}

func TestLogBouncerKeyBanner_SaveSucceededNamesSavedPath(t *testing.T) {
	buf := captureLogs(t)
	h := newPrecedenceHandler(t, "http://127.0.0.1:1", t.TempDir(), noCSCLIMock())

	h.logBouncerKeyBanner(precNewKey, nil)

	out := buf.String()
	assert.Contains(t, out, "Saved To:     "+h.bouncerKeyPath())
	assert.Contains(t, out, "Full API key saved to file")
	assert.NotContains(t, out, "could not save")
	assert.NotContains(t, out, precNewKey, "the full key must never be logged")
}

func TestLogBouncerKeyBanner_SaveFailedSaysSessionOnly(t *testing.T) {
	buf := captureLogs(t)
	h := newPrecedenceHandler(t, "http://127.0.0.1:1", t.TempDir(), noCSCLIMock())

	h.logBouncerKeyBanner(precNewKey, errors.New("disk full"))

	out := buf.String()
	assert.Contains(t, out, "could not save to "+h.bouncerKeyPath())
	assert.Contains(t, out, "valid for this session only")
	assert.NotContains(t, out, "Saved To:", "a failed save must not print a saved path")
	assert.NotContains(t, out, "Full API key saved to file", "a failed save must not claim the file holds the key")
	assert.NotContains(t, out, precNewKey, "the full key must never be logged")
}

func TestRejectedEnvKeyMessage_Variants(t *testing.T) {
	const path = "/app/data/crowdsec/bouncer_key"

	saved := rejectedEnvKeyMessage(path, false)
	require.Contains(t, saved, "Charon saved a new key to "+path+" and uses it")
	require.NotContains(t, saved, "could not save")

	failed := rejectedEnvKeyMessage(path, true)
	require.Contains(t, failed, "could not save its new key to "+path)
	require.Contains(t, failed, "valid for this session only")
	require.NotContains(t, failed, "saved a new key")
}
