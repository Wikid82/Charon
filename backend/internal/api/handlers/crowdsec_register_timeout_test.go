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
)

const registerBouncerURL = "/api/v1/admin/crowdsec/bouncer/register"

func registerHandler(t *testing.T, ex CommandExecutor) *CrowdsecHandler {
	t.Helper()
	script := filepath.Join(t.TempDir(), "register_bouncer.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\n"), 0o600))
	h := pr4Handler(t, ex, t.TempDir())
	h.registerScriptPath = script
	return h
}

func TestRegisterBouncer_ScriptRunsUnderADeadline(t *testing.T) {
	t.Parallel()
	for _, plain := range []bool{true, false} {
		ex := pr4Blocking(plain)
		h := registerHandler(t, ex)
		h.registerScriptTimeout = 100 * time.Millisecond
		start := time.Now()
		w := pr4Serve(pr4Router(h), http.MethodPost, registerBouncerURL, "")
		assert.Less(t, time.Since(start), 5*time.Second)
		require.Equal(t, http.StatusGatewayTimeout, w.Code, w.Body.String())
		body := pr4Body(t, w)
		assert.Equal(t, "cscli_timeout", body["error_code"])
		assert.Equal(t, "CrowdSec did not answer in time", body["error"])
		assert.Equal(t, 1, ex.callCount(), "the bouncer list is not asked after a timed-out script")
	}
}

func TestRegisterBouncer_DefaultScriptDeadline(t *testing.T) {
	t.Parallel()
	ex := &pr4Exec{}
	w := pr4Serve(pr4Router(registerHandler(t, ex)), http.MethodPost, registerBouncerURL, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 2, ex.callCount())
	assert.Positive(t, ex.remaining[0])
	assert.LessOrEqual(t, ex.remaining[0], registerScriptTimeoutDefault)
}

func TestRegisterBouncer_ScriptFailureIsNotATimeout(t *testing.T) {
	t.Parallel()
	ex := &pr4Exec{respond: func(context.Context, []string) ([]byte, error) { return []byte("boom"), errors.New("exit status 1") }}
	w := pr4Serve(pr4Router(registerHandler(t, ex)), http.MethodPost, registerBouncerURL, "")
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Equal(t, "failed to register bouncer", pr4Body(t, w)["error"])
}

func TestRegisterBouncer_ListRunsThroughCSCLIWithADeadline(t *testing.T) {
	t.Parallel()
	ex := &pr4Exec{respond: func(_ context.Context, args []string) ([]byte, error) {
		if len(args) > 0 && args[len(args)-1] == "json" {
			return []byte(`[{"name":"caddy-bouncer"}]`), nil
		}
		return []byte("ok\n"), nil
	}}
	h := registerHandler(t, ex)
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

func TestRegisterBouncer_ListTimeoutKeepsTheRegistrationResult(t *testing.T) {
	t.Parallel()
	ex := &pr4Exec{respond: func(ctx context.Context, args []string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "bouncers list") {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return []byte("ok\n"), nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := registerHandler(t, ex)
	// the request context ends while the list runs: the script result still stands
	h.registerScriptTimeout = time.Minute
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
