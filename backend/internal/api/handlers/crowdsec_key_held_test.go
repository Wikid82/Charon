package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/util"
)

// heldKeyCmd answers every cscli call and gives bouncers add the key precNewKey. It records the bouncer calls
// so a test can see whether a Start deleted or added a bouncer.
type heldKeyCmd struct {
	mu    sync.Mutex
	calls []string
}

func (c *heldKeyCmd) Execute(_ context.Context, _ string, args ...string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, strings.Join(args, " "))
	switch {
	case slices.Contains(args, "bouncers") && slices.Contains(args, "add"):
		return []byte(precNewKey), nil
	case slices.Contains(args, "lapi"):
		return []byte("ok"), nil
	default:
		return nil, nil
	}
}

func (c *heldKeyCmd) bouncerCalls() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, call := range c.calls {
		if strings.Contains(call, "bouncers") {
			out = append(out, call)
		}
	}
	return out
}

// unwritableKeyHandler builds a Start handler whose key directory and saved key file are read-only. The saved
// key is rejected by LAPI and the registered key (precNewKey) is accepted, so the first Start registers a key
// it cannot save and holds it in memory.
func unwritableKeyHandler(t *testing.T) (h *CrowdsecHandler, keyPath string, cmd *heldKeyCmd, applier *recordingApplier) {
	t.Helper()
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	keyPath = filepath.Join(dataDir, "bouncer_key")
	writeSavedKey(t, keyPath, precSavedKey)
	require.NoError(t, os.Chmod(keyPath, 0o400))
	require.NoError(t, os.Chmod(dataDir, 0o500)) //nolint:gosec // G302: test makes its temp directory read-only on purpose
	t.Cleanup(func() {
		_ = os.Chmod(dataDir, 0o700) //nolint:gosec // G302: test restores its temp directory so cleanup can remove it
		_ = os.Chmod(keyPath, 0o600)
		bouncerkey.Release(keyPath)
	})

	lapiURL, _ := newKeyLAPI(t, precNewKey)
	cmd = &heldKeyCmd{}
	applier = &recordingApplier{}
	h = newStartApplyHandler(t, lapiURL, dataDir, cmd, &fakeExec{}, applier)
	h.keyCheckWait = 300 * time.Millisecond
	return h, keyPath, cmd, applier
}

// TestStartHoldsUnsavedKeyAndSecondStartKeepsBouncer: a registered key that cannot be saved is held and used.
// A second Start with the file still read-only validates the held key and does not delete or add a bouncer.
func TestStartHoldsUnsavedKeyAndSecondStartKeepsBouncer(t *testing.T) {
	h, keyPath, cmd, applier := unwritableKeyHandler(t)

	first := startRequest(t, h, context.Background())
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	require.Equal(t, precNewKey, bouncerkey.Held(keyPath), "the unsaved key is held in memory")
	afterFirst := cmd.bouncerCalls()
	require.Len(t, afterFirst, 2, "the first Start deletes and adds the bouncer once")

	second := startRequest(t, h, context.Background())
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	require.Equal(t, afterFirst, cmd.bouncerCalls(), "the second Start must not delete or add a bouncer")
	require.Equal(t, precNewKey, bouncerkey.Held(keyPath), "a still-unwritable file keeps the hold")
	require.Equal(t, 2, applier.count(), "each Start applies the proxy config once")

	var body map[string]any
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &body))
	require.Empty(t, body["warning"], "a held key that validates is not an unverified key")
}

// TestGetKeyStatusHeldKeyReportsMemoryWithoutFullKey: a held key is reported as memory, with a preview only.
func TestGetKeyStatusHeldKeyReportsMemoryWithoutFullKey(t *testing.T) {
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	lapiURL, _ := newKeyLAPI(t, precNewKey)
	h := newKeyCheckHandler(t, lapiURL, dataDir, noCSCLIMock())
	keyPath := h.bouncerKeyPath()
	bouncerkey.Hold(keyPath, precNewKey)
	t.Cleanup(func() { bouncerkey.Release(keyPath) })

	body := keyStatusFor(t, h)

	require.Equal(t, "memory", body["key_source"])
	require.Equal(t, "accepted", body["key_check"])
	require.Equal(t, true, body["valid"])
	require.NotContains(t, body, "full_key", "a held key is never returned in full")
	require.Equal(t, util.MaskAPIKey(precNewKey), body["current_key_preview"])
	require.Equal(t, bouncerkey.SaveFailedNotice(keyPath), body["message"])
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), precNewKey)
}

// TestBouncerInfoAndKeyReportMemorySource: the bouncer info and the key endpoint use the held key and say so.
func TestBouncerInfoAndKeyReportMemorySource(t *testing.T) {
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	lapiURL, _ := newKeyLAPI(t, precNewKey)
	h := newKeyCheckHandler(t, lapiURL, dataDir, &heldKeyCmd{})
	keyPath := h.bouncerKeyPath()
	bouncerkey.Hold(keyPath, precNewKey)
	t.Cleanup(func() { bouncerkey.Release(keyPath) })

	infoW := httptest.NewRecorder()
	infoCtx, _ := gin.CreateTestContext(infoW)
	infoCtx.Request = httptest.NewRequest(http.MethodGet, "/admin/crowdsec/bouncer", http.NoBody)
	h.GetBouncerInfo(infoCtx)
	require.Equal(t, http.StatusOK, infoW.Code)
	var info map[string]any
	require.NoError(t, json.Unmarshal(infoW.Body.Bytes(), &info))
	require.Equal(t, "memory", info["key_source"])

	keyW := httptest.NewRecorder()
	keyCtx, _ := gin.CreateTestContext(keyW)
	keyCtx.Request = httptest.NewRequest(http.MethodGet, "/admin/crowdsec/bouncer/key", http.NoBody)
	h.GetBouncerKey(keyCtx)
	require.Equal(t, http.StatusOK, keyW.Code)
	var key map[string]any
	require.NoError(t, json.Unmarshal(keyW.Body.Bytes(), &key))
	require.Equal(t, precNewKey, key["key"])
	require.Equal(t, "memory", key["source"])
}

// TestEnsureBouncerRegistrationHeldKeyKeepsRejectionFlags: a held key that validates leaves the rejection state
// as it was (the earlier save failure is still true).
func TestEnsureBouncerRegistrationHeldKeyKeepsRejectionFlags(t *testing.T) {
	clearKeyEnv(t, "")
	dataDir := t.TempDir()
	lapiURL, _ := newKeyLAPI(t, precNewKey)
	h := newKeyCheckHandler(t, lapiURL, dataDir, noCSCLIMock())
	keyPath := h.bouncerKeyPath()
	bouncerkey.Hold(keyPath, precNewKey)
	t.Cleanup(func() { bouncerkey.Release(keyPath) })
	h.envKeyRejected = true
	h.rejectedEnvKey = precEnvKeyOther
	h.rejectedEnvSaveFailed = true

	res, err := h.ensureBouncerRegistration(context.Background())

	require.NoError(t, err)
	require.Equal(t, bouncerkey.OutcomeHeldKeyValid, res.Outcome)
	require.Equal(t, precNewKey, res.Key)
	require.True(t, h.envKeyRejected)
	require.Equal(t, precEnvKeyOther, h.rejectedEnvKey)
	require.True(t, h.rejectedEnvSaveFailed)
}

// TestHandleBouncerRegistrationHeldKeyLogsInfoWithoutKey: a held key that validates is one Info line, with no
// key text and no Error or Warn.
func TestHandleBouncerRegistrationHeldKeyLogsInfoWithoutKey(t *testing.T) {
	cases := []struct {
		name    string
		saveErr error
		want    string
	}{
		{name: "saved", saveErr: nil, want: "was saved to the key file"},
		{name: "still unwritable", saveErr: os.ErrPermission, want: "could not be written"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := captureLogs(t)
			h := newPrecedenceHandler(t, "http://127.0.0.1:1", t.TempDir(), noCSCLIMock())
			res := bouncerkey.EnsureResult{Key: precNewKey, Outcome: bouncerkey.OutcomeHeldKeyValid, HeldSaveErr: tc.saveErr}

			h.handleBouncerRegistration(res, nil)

			out := buf.String()
			require.Contains(t, out, tc.want)
			require.Contains(t, out, `"level":"info"`)
			require.NotContains(t, out, `"level":"warning"`)
			require.NotContains(t, out, `"level":"error"`)
			require.NotContains(t, out, precNewKey, "the held key must never be logged")
		})
	}
}
