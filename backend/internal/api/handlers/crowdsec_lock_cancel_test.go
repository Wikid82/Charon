package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// requireLockHeldThenReleasable asserts the holder's lock survived a cancelled waiter: it is still taken,
// and it can be released and taken again.
func requireLockHeldThenReleasable(t *testing.T, mu *ctxMutex) {
	t.Helper()
	require.False(t, mu.TryLock(), "the cancelled waiter must not have released the holder's lock")
	mu.Unlock()
	require.True(t, mu.TryLock(), "the lock must be free once the holder is done")
	mu.Unlock()
}

// TestCancelledWaitersReturnWithoutReleasingTheHoldersLock covers every gin site that waits for the data
// lock: while another request holds it, a request whose client leaves returns with no body.
func TestCancelledWaitersReturnWithoutReleasingTheHoldersLock(t *testing.T) {
	c := newCuratedHarness(t, nil)

	requests := map[string]func(ctx context.Context) *http.Request{
		"apply": func(ctx context.Context) *http.Request {
			body, _ := json.Marshal(map[string]string{"slug": "honeypot-friendly-defaults"})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", bytes.NewReader(body)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			return req
		},
		"write": func(ctx context.Context) *http.Request {
			body, _ := json.Marshal(map[string]string{"path": "locked.conf", "content": "x"})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/file", bytes.NewReader(body)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			return req
		},
		"import": func(ctx context.Context) *http.Request {
			return importRequest(t, map[string]string{"config.yaml": importableConfig}).WithContext(ctx)
		},
		"export": func(ctx context.Context) *http.Request {
			return httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/export", http.NoBody).WithContext(ctx)
		},
	}

	for name, newReq := range requests {
		t.Run(name, func(t *testing.T) {
			holdLock(&c.handler.dataMu)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				w := httptest.NewRecorder()
				c.router.ServeHTTP(w, newReq(ctx))
				done <- w
			}()

			select {
			case <-done:
				c.handler.dataMu.Unlock()
				t.Fatalf("%s returned while the lock was held and the client was still there", name)
			case <-time.After(100 * time.Millisecond):
			}
			cancel()
			select {
			case w := <-done:
				require.Empty(t, w.Body.String(), "a client that left gets no body")
			case <-time.After(5 * time.Second):
				c.handler.dataMu.Unlock()
				t.Fatalf("%s kept waiting after the request was cancelled", name)
			}
			requireLockHeldThenReleasable(t, &c.handler.dataMu)
		})
	}
}

// TestRestoreLockSitesGiveUpOnCancelledContext covers the three waits of the restore action.
func TestRestoreLockSitesGiveUpOnCancelledContext(t *testing.T) {
	t.Parallel()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	item := crowdsec.HubItem{Type: "scenarios", Name: "a/one"}

	t.Run("hub update", func(t *testing.T) {
		t.Parallel()
		hh := newHubHarness(t)
		holdLock(&hh.h.dataMu)
		require.ErrorIs(t, hh.h.hubUpdate(cancelled, time.Second), context.Canceled)
		require.Empty(t, hh.exec.callLog(), "no cscli call without the lock")
		requireLockHeldThenReleasable(t, &hh.h.dataMu)
	})
	t.Run("installed items", func(t *testing.T) {
		t.Parallel()
		hh := newHubHarness(t)
		holdLock(&hh.h.dataMu)
		_, err := hh.h.installedHubSet(cancelled)
		require.ErrorIs(t, err, context.Canceled)
		requireLockHeldThenReleasable(t, &hh.h.dataMu)
	})
	t.Run("install", func(t *testing.T) {
		t.Parallel()
		hh := newHubHarness(t)
		holdLock(&hh.h.dataMu)
		ctx, stop := context.WithCancel(context.Background())
		defer stop()
		done := make(chan string, 1)
		go func() { done <- hh.h.installHubItem(ctx, time.Second, item) }()
		select {
		case <-done:
			hh.h.dataMu.Unlock()
			t.Fatal("install ran while the lock was held")
		case <-time.After(100 * time.Millisecond):
		}
		stop()
		select {
		case reason := <-done:
			require.Equal(t, hubReasonNotAttempted, reason)
		case <-time.After(5 * time.Second):
			hh.h.dataMu.Unlock()
			t.Fatal("install kept waiting after the context ended")
		}
		require.Empty(t, hh.exec.callLog())
		requireLockHeldThenReleasable(t, &hh.h.dataMu)
	})
}

// TestRestoreWaitCutShortMarksItemsNotAttempted: when the lock wait of a pass ends with the context, the
// pass reports its items as not attempted rather than as an unreadable install list.
func TestRestoreWaitCutShortMarksItemsNotAttempted(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	holdLock(&hh.h.dataMu)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := &hubRestoreResponse{}
	items := hubItems("scenarios", "a/one", "parsers", "a/two")

	require.True(t, hh.h.restorePass(ctx, time.Second, items, res))
	require.Equal(t, []hubRestoreFailure{
		{"scenarios", "a/one", hubReasonNotAttempted},
		{"parsers", "a/two", hubReasonNotAttempted},
	}, res.Failed)
	requireLockHeldThenReleasable(t, &hh.h.dataMu)
}

// TestRestoreWhileDataLockHeldAnswersWhenClientLeaves: the whole action ends once the client is gone, even
// though an import-like holder keeps the lock.
func TestRestoreWhileDataLockHeldAnswersWhenClientLeaves(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	holdLock(&hh.h.dataMu)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	w := hh.postCtx(ctx, restoreBody(hubItems("scenarios", "a/one")...))
	require.Equal(t, http.StatusBadGateway, w.Code)
	require.Equal(t, msgRestoreHubDown, restoreErr(t, w))
	require.False(t, hh.h.restore.running.Load(), "the restore slot is free again")
	requireLockHeldThenReleasable(t, &hh.h.dataMu)
}
