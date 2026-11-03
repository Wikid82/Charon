package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
)

const registerBouncerURL = "/api/v1/admin/crowdsec/bouncer/register"

func TestRegisterBouncer_ListRunsThroughCSCLIWithADeadline(t *testing.T) {
	ex := registerCSCLI("", nil, `[{"name":"caddy-bouncer"}]`, nil)
	h := newRegisterFixture(t, ex, map[string]bouncerkey.Verdict{regSavedKey: bouncerkey.VerdictAccepted})
	writeSavedKey(t, h.bouncerKeyPath(), regSavedKey)
	// a config file in the data dir is put in front of the arguments, as for every other cscli command
	require.NoError(t, os.MkdirAll(filepath.Join(h.DataDir, "config"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(h.DataDir, "config", "config.yaml"), []byte("x: y\n"), 0o600))

	w := pr4Serve(pr4Router(h), http.MethodPost, registerBouncerURL, "")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, true, pr4Body(t, w)["registered"])
	last := ex.lastCall(t)
	assert.Equal(t, "-c", last[0])
	assert.Equal(t, []string{"bouncers", "list", "-o", "json"}, last[2:])
	assert.Positive(t, ex.lastRemaining(t))
	assert.LessOrEqual(t, ex.lastRemaining(t), cscliListTimeout)
}

func TestRegisterBouncer_ListTimeoutIsNotRegistered(t *testing.T) {
	ex := &pr4Exec{respond: func(ctx context.Context, args []string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "bouncers list") {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return []byte("ok\n"), nil
	}}
	h := newRegisterFixture(t, ex, map[string]bouncerkey.Verdict{regSavedKey: bouncerkey.VerdictAccepted})
	writeSavedKey(t, h.bouncerKeyPath(), regSavedKey)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() { time.Sleep(150 * time.Millisecond); cancel(); close(done) }()

	w := pr4ServeCtx(ctx, pr4Router(h), http.MethodPost, registerBouncerURL, "")
	<-done

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, false, pr4Body(t, w)["registered"])
}

func alertsServe(t *testing.T, ex CommandExecutor, ctx context.Context) map[string]any {
	t.Helper()
	h := pr4Handler(t, ex, t.TempDir())
	h.validateLAPIURL = func(string) (*url.URL, error) { return nil, errors.New("blocked in test") }
	w := pr4ServeCtx(ctx, pr4Router(h), http.MethodGet, "/api/v1/admin/crowdsec/alerts", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return pr4Body(t, w)
}

func TestListAlerts_SoftErrorField(t *testing.T) {
	t.Parallel()
	t.Run("success has no error field", func(t *testing.T) {
		t.Parallel()
		body := alertsServe(t, pr4Answer(t, "alerts_list_json"), context.Background())
		assert.NotContains(t, body, "error")
		assert.Equal(t, float64(3), body["total"])
	})
	t.Run("cscli failure", func(t *testing.T) {
		t.Parallel()
		ex := &pr4Exec{respond: func(context.Context, []string) ([]byte, error) { return nil, errors.New("exit status 1") }}
		body := alertsServe(t, ex, context.Background())
		assert.Equal(t, "cscli not available or failed", body["error"])
		assert.Equal(t, float64(0), body["total"])
		assert.Empty(t, body["alerts"])
	})
	t.Run("unparsable output", func(t *testing.T) {
		t.Parallel()
		ex := &pr4Exec{respond: func(context.Context, []string) ([]byte, error) { return []byte("not json"), nil }}
		assert.Equal(t, "cscli not available or failed", alertsServe(t, ex, context.Background())["error"])
	})
	t.Run("timeout", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		body := alertsServe(t, pr4Blocking(false), ctx)
		assert.Equal(t, "cscli alerts list timed out; the CrowdSec local API may be stopped", body["error"])
	})
	t.Run("a failed listing is not cached", func(t *testing.T) {
		t.Parallel()
		calls := 0
		ex := &pr4Exec{respond: func(context.Context, []string) ([]byte, error) {
			calls++
			if calls == 1 {
				return nil, errors.New("exit status 1")
			}
			return []byte("[]"), nil
		}}
		h := pr4Handler(t, ex, t.TempDir())
		r := pr4Router(h)
		first := pr4Body(t, pr4Serve(r, http.MethodGet, "/api/v1/admin/crowdsec/alerts", ""))
		second := pr4Body(t, pr4Serve(r, http.MethodGet, "/api/v1/admin/crowdsec/alerts", ""))
		assert.Contains(t, first, "error")
		assert.NotContains(t, second, "error")
	})
}
