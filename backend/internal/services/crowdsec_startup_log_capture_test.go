package services

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/logger"
)

// Keys made of distinct characters: every 4-character window is unique, so a
// key text in the captured output is detectable.
const (
	startupCaptureEnvKey = "Qz7kP2mV9xR4tN6bL1wH8sJ3cF5gD0aY"
	startupCaptureGenKey = "Mn3pQ8rS1tU6vW0xY4zA9bC2dE7fG5hJ"
)

// TestCrowdSecLogsCarryNoKeyText runs every startup key-bearing log path at Debug
// level and fails if any 4-character window of either key appears in the output.
func TestCrowdSecLogsCarryNoKeyText(t *testing.T) {
	for _, name := range bouncerkey.EnvNames() {
		t.Setenv(name, "")
	}
	lapi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") == startupCaptureEnvKey {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(lapi.Close)
	pointStartupKeyChecksAt(t, lapi.URL)

	buf := &bytes.Buffer{}
	logger.Init(true, buf)
	hook := logger.GetBroadcastHook()
	hookID := t.Name()
	entries := hook.Subscribe(hookID)
	t.Cleanup(func() {
		hook.Unsubscribe(hookID)
		logger.Init(false, io.Discard)
	})

	t.Run("rejected env key and registration", func(t *testing.T) {
		dir := t.TempDir()
		keyFile := filepath.Join(dir, "crowdsec", "bouncer_key")
		t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", startupCaptureEnvKey)
		exec := &scriptedBouncerExecutor{addOutput: []byte(startupCaptureGenKey + "\n")}
		require.NoError(t, ensureBouncerRegistration(dir, keyFile, exec))
	})

	t.Run("saved file key", func(t *testing.T) {
		dir := t.TempDir()
		keyFile := filepath.Join(dir, "crowdsec", "bouncer_key")
		require.NoError(t, os.MkdirAll(filepath.Dir(keyFile), 0o700))
		require.NoError(t, os.WriteFile(keyFile, []byte(startupCaptureGenKey), 0o600))
		exec := &scriptedBouncerExecutor{}
		require.NoError(t, ensureBouncerRegistration(dir, keyFile, exec))
	})

	t.Run("decision logging for every outcome", func(t *testing.T) {
		for _, res := range []bouncerkey.EnsureResult{
			{Key: startupCaptureGenKey, Outcome: bouncerkey.OutcomeRegistered, EnvKey: startupCaptureEnvKey, EnvKeyRejected: true},
			{Key: startupCaptureGenKey, Outcome: bouncerkey.OutcomeRegistered, EnvKey: startupCaptureEnvKey, EnvKeyRejected: true, SaveErr: errors.New("disk full")},
			{Key: startupCaptureGenKey, Outcome: bouncerkey.OutcomeRegistered},
			{Key: startupCaptureEnvKey, Outcome: bouncerkey.OutcomeEnvKeySaved},
			{Key: startupCaptureEnvKey, Outcome: bouncerkey.OutcomeEnvKeySaved, SaveErr: errors.New("disk full")},
			{EnvKey: startupCaptureEnvKey, EnvKeyRejected: true},
		} {
			logStartupKeyDecision(res, "/data/crowdsec/bouncer_key", nil)
		}
		logStartupKeyDecision(bouncerkey.EnsureResult{EnvKey: startupCaptureEnvKey, EnvKeyRejected: true}, "/data/crowdsec/bouncer_key", errors.New("register failed"))
	})

	t.Run("debug probe reaches the capture", func(t *testing.T) {
		logger.Log().Debug("capture probe")
		var sawProbe bool
		for drained := false; !drained; {
			select {
			case e := <-entries:
				if e.Level == logrus.DebugLevel && e.Message == "capture probe" {
					sawProbe = true
				}
			default:
				drained = true
			}
		}
		require.True(t, sawProbe, "the capture hook must observe Debug entries")
	})

	out := buf.String()
	require.True(t, strings.Contains(out, "CrowdSec bouncer"), "startup logs must be captured")
	for _, key := range []string{startupCaptureEnvKey, startupCaptureGenKey} {
		for i := 0; i+4 <= len(key); i++ {
			assert.NotContains(t, out, key[i:i+4], "log output carries key text at offset %d", i)
		}
	}
}
