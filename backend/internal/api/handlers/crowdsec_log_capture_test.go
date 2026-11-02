package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/logger"
)

// logCaptureFullKey is a 32-character key made of distinct characters, so every
// 4-character window of it is unique and a leaked fragment is detectable.
const logCaptureFullKey = "Qz7kP2mV9xR4tN6bL1wH8sJ3cF5gD0aY"

// captureDebugLogs routes the global logger to a buffer at Debug level and
// subscribes a listener on the broadcast hook. It returns the buffer and the
// listener channel so a test can prove Debug entries were delivered.
func captureDebugLogs(t *testing.T) (buf *bytes.Buffer, entries <-chan *logrus.Entry) {
	t.Helper()
	buf = &bytes.Buffer{}
	logger.Init(true, buf)
	hook := logger.GetBroadcastHook()
	id := t.Name()
	entries = hook.Subscribe(id)
	t.Cleanup(func() {
		hook.Unsubscribe(id)
		logger.Init(false, io.Discard)
	})
	return buf, entries
}

// lapiStatusServer answers every request with the given status code.
func lapiStatusServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestCrowdSecLogsCarryNoKeyText runs every key-bearing log path at Debug level
// and fails if any 4-character window of the key appears in the captured output.
func TestCrowdSecLogsCarryNoKeyText(t *testing.T) {
	buf, entries := captureDebugLogs(t)

	t.Run("LAPI key check", func(t *testing.T) {
		for _, status := range []int{http.StatusOK, http.StatusForbidden, http.StatusInternalServerError} {
			srv := lapiStatusServer(t, status)
			h := newPrecedenceHandler(t, srv.URL, t.TempDir(), noCSCLIMock())
			h.testKeyAgainstLAPI(context.Background(), logCaptureFullKey)
		}
	})

	t.Run("bouncer banner", func(t *testing.T) {
		h := newPrecedenceHandler(t, "http://127.0.0.1:1", t.TempDir(), noCSCLIMock())
		h.logBouncerKeyBanner(nil)
		h.logBouncerKeyBanner(errors.New("disk full"))
	})

	t.Run("registration outcomes", func(t *testing.T) {
		h := newPrecedenceHandler(t, "http://127.0.0.1:1", t.TempDir(), noCSCLIMock())
		h.handleBouncerRegistration(bouncerkey.EnsureResult{
			Key:            logCaptureFullKey,
			Outcome:        bouncerkey.OutcomeRegistered,
			EnvKey:         logCaptureFullKey,
			EnvKeyRejected: true,
		}, nil)
		h.handleBouncerRegistration(bouncerkey.EnsureResult{
			Key:            logCaptureFullKey,
			Outcome:        bouncerkey.OutcomeRegistered,
			EnvKey:         logCaptureFullKey,
			EnvKeyRejected: true,
			SaveErr:        errors.New("disk full"),
		}, nil)
		h.handleBouncerRegistration(bouncerkey.EnsureResult{
			EnvKey:         logCaptureFullKey,
			EnvKeyRejected: true,
		}, errors.New("bouncer registration failed"))
	})

	t.Run("hook received a debug entry", func(t *testing.T) {
		var sawDebug bool
		for drained := false; !drained; {
			select {
			case e := <-entries:
				if e.Level == logrus.DebugLevel {
					sawDebug = true
				}
			default:
				drained = true
			}
		}
		require.True(t, sawDebug, "the capture hook must observe Debug entries")
	})

	out := buf.String()
	for i := 0; i+4 <= len(logCaptureFullKey); i++ {
		assert.NotContains(t, out, logCaptureFullKey[i:i+4], "log output carries key text at offset %d", i)
	}
}
