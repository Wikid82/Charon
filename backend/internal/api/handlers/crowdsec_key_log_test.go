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

	assert.Contains(t, out, "Charon could not save the new key to "+keyPath)
	assert.Contains(t, out, "CrowdSec blocking may not work until the key file can be written")
	assert.NotContains(t, out, "session", "a failed save must not claim the new key is valid for a session")
	assert.NotContains(t, out, "uses the key saved in", "a failed save must not claim the file holds the key in use")
	assert.Contains(t, out, `"level":"error"`, "the save failure must be logged at Error")
	assert.Contains(t, out, "Failed to save bouncer key to file")
	assert.NotContains(t, out, logTestRejectedEnvKey, "the rejected key must never be logged")
}

func TestHandleBouncerRegistration_RejectedEnvRegisterFailureLogsNoSuccess(t *testing.T) {
	buf := captureLogs(t)
	h := newPrecedenceHandler(t, "http://127.0.0.1:1", t.TempDir(), noCSCLIMock())
	res := bouncerkey.EnsureResult{EnvKey: logTestRejectedEnvKey, EnvKeyRejected: true}
	apply := &applyCounter{}

	h.handleBouncerRegistration(context.Background(), res, errors.New("bouncer registration failed"), apply.apply)

	out := buf.String()
	assert.Contains(t, out, "could not register a new key with CrowdSec, so no new key was saved to "+h.bouncerKeyPath())
	assert.Contains(t, out, "Failed to register bouncer")
	assert.NotContains(t, out, "uses the key saved in", "a failed registration must not claim the file holds the key in use")
	assert.NotContains(t, out, "Charon saved a new key", "a failed registration saved no new key")
	assert.NotContains(t, out, "validated and saved to key file")
	assert.NotContains(t, out, logTestRejectedEnvKey, "the rejected key must never be logged")
	assert.Zero(t, apply.calls, "a failed registration has no new key to re-apply to Caddy")
}

func TestHandleBouncerRegistration_EnvKeySavedSuccessLogsOnlyWhenSaved(t *testing.T) {
	const saved = "validated and saved to key file"

	t.Run("save succeeded", func(t *testing.T) {
		buf := captureLogs(t)
		h := newPrecedenceHandler(t, "http://127.0.0.1:1", t.TempDir(), noCSCLIMock())
		res := bouncerkey.EnsureResult{Key: precEnvKey, Outcome: bouncerkey.OutcomeEnvKeySaved, EnvKey: precEnvKey}

		h.handleBouncerRegistration(context.Background(), res, nil, nil)

		assert.Contains(t, buf.String(), saved)
	})

	t.Run("save failed", func(t *testing.T) {
		buf := captureLogs(t)
		h := newPrecedenceHandler(t, "http://127.0.0.1:1", t.TempDir(), noCSCLIMock())
		res := bouncerkey.EnsureResult{
			Key: precEnvKey, Outcome: bouncerkey.OutcomeEnvKeySaved, EnvKey: precEnvKey, SaveErr: errors.New("disk full"),
		}

		h.handleBouncerRegistration(context.Background(), res, nil, nil)

		assert.NotContains(t, buf.String(), saved, "a failed save must not log a success line")
		assert.Contains(t, buf.String(), "Failed to save bouncer key to file")
	})
}

func TestRejectedEnvLogMessage_Variants(t *testing.T) {
	const path = "/app/data/crowdsec/bouncer_key"

	require.Contains(t, rejectedEnvLogMessage(path, nil), "uses the key saved in "+path)
	require.Contains(t, rejectedEnvLogMessage(path, errors.New("x")), "could not save the new key to "+path)
	require.NotContains(t, rejectedEnvLogMessage(path, errors.New("x")), "session")
	require.Contains(t, rejectedEnvRegisterFailedLogMessage(path), "could not register a new key with CrowdSec")
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
	assert.Contains(t, out, "CrowdSec blocking may not work until it can be written")
	assert.NotContains(t, out, "session", "the banner must not claim the key is valid for a session")
	assert.NotContains(t, out, "Saved To:", "a failed save must not print a saved path")
	assert.NotContains(t, out, "Full API key saved to file", "a failed save must not claim the file holds the key")
	assert.NotContains(t, out, precNewKey, "the full key must never be logged")
}

func TestRejectedEnvKeyMessage_Variants(t *testing.T) {
	const path = "/app/data/crowdsec/bouncer_key"

	saved := rejectedEnvKeyMessage(path, false, false)
	require.Contains(t, saved, "Charon saved a new key to "+path+" and uses it")
	require.NotContains(t, saved, "could not save")

	failed := rejectedEnvKeyMessage(path, true, false)
	require.Contains(t, failed, "Charon could not save the new key to "+path)
	require.Contains(t, failed, "CrowdSec blocking may not work until the key file can be written")
	require.NotContains(t, failed, "session")
	require.NotContains(t, failed, "saved a new key")

	registerFailed := rejectedEnvKeyMessage(path, false, true)
	require.Contains(t, registerFailed, "could not register a new key with CrowdSec, so no new key was saved to "+path)
	require.NotContains(t, registerFailed, "saved a new key to")
	require.NotContains(t, registerFailed, "uses it")
}
