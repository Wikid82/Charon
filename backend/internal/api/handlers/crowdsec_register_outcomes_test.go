package handlers

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/util"
)

// These tests drive POST /admin/crowdsec/bouncer/register through the shared key decision (Ensure). They
// answer the key check from a fixed verdict per key, and the cscli commands from a fake executor.

const (
	regSavedKey = "saved-registration-key-0001"
	regNewKey   = "new-registration-key-000002"
	regSentinel = "ZZregisterSentinel0123456789abcdefghijkl"
)

// keyVerdicts answers the key check from one verdict per key. A key that is not listed is Unknown (LAPI down).
func keyVerdicts(verdicts map[string]bouncerkey.Verdict) func(context.Context, bouncerkey.Doer, string, bouncerkey.CheckOptions) bouncerkey.Verdict {
	return func(_ context.Context, _ bouncerkey.Doer, key string, _ bouncerkey.CheckOptions) bouncerkey.Verdict {
		if v, ok := verdicts[key]; ok {
			return v
		}
		return bouncerkey.VerdictUnknown
	}
}

// registerCSCLI answers the bouncer commands: delete succeeds, add prints add (or fails with addErr) and list
// prints list (or fails with listErr).
func registerCSCLI(add string, addErr error, list string, listErr error) *pr4Exec {
	return &pr4Exec{respond: func(_ context.Context, args []string) ([]byte, error) {
		a := pr4StripConfig(args)
		if len(a) < 2 {
			return nil, errors.New("unexpected command")
		}
		switch a[1] {
		case "delete":
			return []byte("deleted\n"), nil
		case "add":
			return []byte(add + "\n"), addErr
		case "list":
			return []byte(list), listErr
		}
		return nil, errors.New("unexpected command")
	}}
}

// cscliVerbs returns the bouncers subcommand of every cscli call, in order.
func cscliVerbs(ex *pr4Exec) []string {
	ex.mu.Lock()
	defer ex.mu.Unlock()
	var verbs []string
	for _, c := range ex.calls {
		if a := pr4StripConfig(c); len(a) >= 2 {
			verbs = append(verbs, a[1])
		}
	}
	return verbs
}

// newRegisterFixture returns a handler in a fresh data directory whose cscli commands are ex and whose key checks
// answer from verdicts. The key variables are cleared, so only the saved key file decides.
func newRegisterFixture(t *testing.T, ex *pr4Exec, verdicts map[string]bouncerkey.Verdict) *CrowdsecHandler {
	t.Helper()
	clearKeyEnv(t, "")
	h := pr4Handler(t, ex, t.TempDir())
	h.keyCheck = keyVerdicts(verdicts)
	return h
}

func bodyText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.TrimSpace(string(b))
}

func TestRegisterBouncerKeepsASavedKeyThatCrowdSecAccepts(t *testing.T) {
	ex := registerCSCLI("", nil, `[{"name":"caddy-bouncer"}]`, nil)
	h := newRegisterFixture(t, ex, map[string]bouncerkey.Verdict{regSavedKey: bouncerkey.VerdictAccepted})
	writeSavedKey(t, h.bouncerKeyPath(), regSavedKey)

	w := pr4Serve(pr4Router(h), http.MethodPost, registerBouncerURL, "")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := pr4Body(t, w)
	assert.Equal(t, "registered", body["status"])
	assert.Equal(t, "caddy-bouncer", body["bouncer_name"])
	assert.Equal(t, util.MaskAPIKey(regSavedKey), body["api_key_preview"])
	assert.Equal(t, true, body["registered"])
	assert.Equal(t, regSavedKey, bodyText(t, h.bouncerKeyPath()), "the saved key file is untouched")
	assert.Equal(t, []string{"list"}, cscliVerbs(ex), "a working key is never deleted or replaced")
}

func TestRegisterBouncerReplacesASavedKeyThatCrowdSecRejects(t *testing.T) {
	ex := registerCSCLI(regNewKey, nil, `[{"name":"caddy-bouncer"}]`, nil)
	h := newRegisterFixture(t, ex, map[string]bouncerkey.Verdict{regSavedKey: bouncerkey.VerdictRejected})
	writeSavedKey(t, h.bouncerKeyPath(), regSavedKey)

	w := pr4Serve(pr4Router(h), http.MethodPost, registerBouncerURL, "")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := pr4Body(t, w)
	assert.Equal(t, "registered", body["status"])
	assert.Equal(t, util.MaskAPIKey(regNewKey), body["api_key_preview"])
	assert.Equal(t, true, body["registered"])
	assert.Equal(t, regNewKey, bodyText(t, h.bouncerKeyPath()), "the new key is saved")
	assert.Equal(t, []string{"delete", "add", "list"}, cscliVerbs(ex))
}

func TestRegisterBouncerKeepsTheSavedKeyWhenLAPIIsDown(t *testing.T) {
	ex := registerCSCLI(regNewKey, nil, `[{"name":"caddy-bouncer"}]`, nil)
	h := newRegisterFixture(t, ex, nil) // every key check is Unknown
	writeSavedKey(t, h.bouncerKeyPath(), regSavedKey)

	w := pr4Serve(pr4Router(h), http.MethodPost, registerBouncerURL, "")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := pr4Body(t, w)
	assert.Equal(t, "unverified", body["status"])
	assert.Equal(t, util.MaskAPIKey(regSavedKey), body["api_key_preview"])
	assert.Equal(t, regSavedKey, bodyText(t, h.bouncerKeyPath()), "the key file is untouched while LAPI cannot answer")
	assert.Equal(t, []string{"list"}, cscliVerbs(ex), "no bouncer is deleted or added while LAPI cannot answer")
}

// TestRegisterBouncerReportsAnUnknownRegistrationAsNotRegistered: the list cannot be read, so the state is unknown
// and registered is false, even though the key check accepted the saved key.
func TestRegisterBouncerReportsAnUnknownRegistrationAsNotRegistered(t *testing.T) {
	ex := registerCSCLI("", nil, "", errors.New("exit status 1"))
	h := newRegisterFixture(t, ex, map[string]bouncerkey.Verdict{regSavedKey: bouncerkey.VerdictAccepted})
	writeSavedKey(t, h.bouncerKeyPath(), regSavedKey)

	w := pr4Serve(pr4Router(h), http.MethodPost, registerBouncerURL, "")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := pr4Body(t, w)
	assert.Equal(t, "registered", body["status"])
	assert.Equal(t, false, body["registered"], "an unknown registration is never reported as registered")
}

func TestRegisterBouncerAnswersBusyWhileAJobRuns(t *testing.T) {
	ex := registerCSCLI(regNewKey, nil, "", nil)
	h := newRegisterFixture(t, ex, nil)
	setBackgroundInstallActive(t, h)

	w := pr4Serve(pr4Router(h), http.MethodPost, registerBouncerURL, "")

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	body := pr4Body(t, w)
	assert.Equal(t, codeCrowdsecBusy, body["error_code"])
	assert.Equal(t, busyJobID, body["job_id"])
	assert.Empty(t, cscliVerbs(ex), "a refused registration runs no cscli command")
	assert.NoFileExists(t, h.bouncerKeyPath())
}

func TestRegisterBouncerRefusesOnceShutdownBegan(t *testing.T) {
	ex := registerCSCLI(regNewKey, nil, "", nil)
	h := newRegisterFixture(t, ex, nil)
	require.NoError(t, h.WaitRegistrations(context.Background()))

	w := pr4Serve(pr4Router(h), http.MethodPost, registerBouncerURL, "")

	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	assert.Equal(t, "shutting_down", pr4Body(t, w)["error_code"])
	assert.Empty(t, cscliVerbs(ex))
}

func TestRegisterBouncerFailureIsA500WithoutCommandOutput(t *testing.T) {
	ex := registerCSCLI(regSentinel, errors.New("exit status 1"), "", nil)
	h := newRegisterFixture(t, ex, nil)

	w := pr4Serve(pr4Router(h), http.MethodPost, registerBouncerURL, "")

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	body := pr4Body(t, w)
	assert.Equal(t, "failed to register bouncer", body["error"])
	assert.Equal(t, "see server logs", body["details"])
	assert.NotContains(t, w.Body.String(), regSentinel)
	assert.NoFileExists(t, h.bouncerKeyPath())
}

func TestRegisterBouncerTimeoutIsA504(t *testing.T) {
	ex := registerCSCLI("", context.DeadlineExceeded, "", nil)
	h := newRegisterFixture(t, ex, nil)

	w := pr4Serve(pr4Router(h), http.MethodPost, registerBouncerURL, "")

	require.Equal(t, http.StatusGatewayTimeout, w.Code, w.Body.String())
	assert.Equal(t, "cscli_timeout", pr4Body(t, w)["error_code"])
}

// TestRegisterBouncerReplaceBudgetExpiryIsA504 hangs every add, so the swap ends on its total budget (shortened
// here) and not on a step cap. The error must still map to the coded 504, not a 500.
func TestRegisterBouncerReplaceBudgetExpiryIsA504(t *testing.T) {
	ex := &pr4Exec{respond: func(ctx context.Context, args []string) ([]byte, error) {
		a := pr4StripConfig(args)
		if len(a) >= 2 && a[1] == "add" {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return []byte("deleted\n"), nil
	}}
	h := newRegisterFixture(t, ex, nil)
	h.replaceTimings = crowdsec.ReplaceTimings{
		Delete:      10 * time.Millisecond,
		Add:         100 * time.Millisecond,
		RetryDelete: 10 * time.Millisecond,
		RetryAdd:    100 * time.Millisecond,
		Total:       50 * time.Millisecond,
	}

	w := pr4Serve(pr4Router(h), http.MethodPost, registerBouncerURL, "")

	require.Equal(t, http.StatusGatewayTimeout, w.Code, w.Body.String())
	assert.Equal(t, "cscli_timeout", pr4Body(t, w)["error_code"])
	assert.NoFileExists(t, h.bouncerKeyPath())
}

func TestRegistrationPreviewIsEmptyWithoutAKey(t *testing.T) {
	assert.Empty(t, registrationPreview(""))
	assert.Equal(t, util.MaskAPIKey(regNewKey), registrationPreview(regNewKey))
}

func TestRegisterBouncerAnswersBusyWhileAStartOrStopRuns(t *testing.T) {
	ex := registerCSCLI(regNewKey, nil, "", nil)
	h := newRegisterFixture(t, ex, nil)
	h.lifecycleMu.Lock() // a Start or Stop holds the lifecycle for its whole run
	defer h.lifecycleMu.Unlock()

	w := pr4Serve(pr4Router(h), http.MethodPost, registerBouncerURL, "")

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, codeCrowdsecBusy, pr4Body(t, w)["error_code"])
	assert.Empty(t, cscliVerbs(ex), "a refused registration runs no cscli command")
}

// TestRegisterBouncerRefusedWhenShutdownBeginsMidRequest closes the registration gate from inside the key check,
// so the registration itself is refused after the request already passed the entry gate.
func TestRegisterBouncerRefusedWhenShutdownBeginsMidRequest(t *testing.T) {
	ex := registerCSCLI(regNewKey, nil, "", nil)
	h := newRegisterFixture(t, ex, nil)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	h.keyCheck = func(context.Context, bouncerkey.Doer, string, bouncerkey.CheckOptions) bouncerkey.Verdict {
		// A cancelled context closes the gate without waiting for the registration that is in flight.
		_ = h.WaitRegistrations(cancelled)
		return bouncerkey.VerdictRejected
	}
	writeSavedKey(t, h.bouncerKeyPath(), regSavedKey)

	w := pr4Serve(pr4Router(h), http.MethodPost, registerBouncerURL, "")

	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	assert.Equal(t, "shutting_down", pr4Body(t, w)["error_code"])
	assert.Equal(t, regSavedKey, bodyText(t, h.bouncerKeyPath()), "shutdown never rotates the saved key")
	assert.Empty(t, cscliVerbs(ex))
}
