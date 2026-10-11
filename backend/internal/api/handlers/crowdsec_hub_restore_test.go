package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

const restorePath = "/api/v1/admin/crowdsec/hub-items/restore"

// hubFakeExec is a scripted cscli. A successful install creates the link a real one would, so the
// installed-item diff sees it.
type hubFakeExec struct {
	mu      sync.Mutex
	calls   []string
	dataDir string

	update    func(ctx context.Context) ([]byte, error)
	install   func(ctx context.Context, typ, name string) ([]byte, error)
	inspect   func(ctx context.Context, typ, name string) ([]byte, error)
	deps      map[string][]crowdsec.HubItem // links created together with an install
	noLinkFor map[string]bool
}

func (f *hubFakeExec) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, strings.Join(append([]string{name}, args...), " "))
	f.mu.Unlock()
	switch {
	case name != "cscli":
		return nil, fmt.Errorf("unexpected binary %q", name)
	case len(args) == 2 && args[0] == "hub" && args[1] == "update":
		if f.update != nil {
			return f.update(ctx)
		}
		return []byte("ok"), nil
	case len(args) == 3 && args[1] == "install":
		if f.install != nil {
			if out, err := f.install(ctx, args[0], args[2]); err != nil {
				return out, err
			}
		}
		it := crowdsec.HubItem{Type: args[0], Name: args[2]}
		if !f.noLinkFor[it.Name] {
			linkHubItem(f.dataDir, it)
		}
		for _, dep := range f.deps[it.Name] {
			linkHubItem(f.dataDir, dep)
		}
		return []byte("ok"), nil
	case len(args) == 5 && args[1] == "inspect":
		if f.inspect != nil {
			return f.inspect(ctx, args[0], args[2])
		}
		return []byte(`{"installed":true,"tainted":false}`), nil
	}
	return nil, fmt.Errorf("unexpected call %v", args)
}

func (f *hubFakeExec) callLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *hubFakeExec) installCalls() []string {
	var out []string
	for _, c := range f.callLog() {
		if strings.Contains(c, " install ") {
			out = append(out, c)
		}
	}
	return out
}

// linkHubItem creates the symlink an installed item is made of.
func linkHubItem(dataDir string, it crowdsec.HubItem) {
	owner, base, _ := strings.Cut(it.Name, "/")
	stage := ""
	if it.Type == "parsers" || it.Type == "postoverflows" {
		stage = "s01-parse"
	}
	dir := filepath.Join(dataDir, "config", it.Type, stage)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		panic(err)
	}
	target := path.Join("/etc/crowdsec/hub", it.Type, stage, owner, base+".yaml")
	if err := os.Symlink(target, filepath.Join(dir, owner+"-"+base+".yaml")); err != nil && !errors.Is(err, os.ErrExist) {
		panic(err)
	}
}

type hubHarness struct {
	h      *CrowdsecHandler
	r      *gin.Engine
	exec   *hubFakeExec
	dir    string
	signal atomic.Int32
}

func newHubHarness(t *testing.T) *hubHarness {
	t.Helper()
	dir := t.TempDir()
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{started: true}, "/bin/false", dir)
	ex := &hubFakeExec{dataDir: dir}
	h.CmdExec = ex
	hh := &hubHarness{h: h, exec: ex, dir: dir}
	h.signalProcess = func(int, syscall.Signal) error {
		hh.signal.Add(1)
		return nil
	}
	hh.r = gin.New()
	h.RegisterRoutes(hh.r.Group("/api/v1"))
	return hh
}

func hubItems(pairs ...string) []crowdsec.HubItem {
	var out []crowdsec.HubItem
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, crowdsec.HubItem{Type: pairs[i], Name: pairs[i+1]})
	}
	return out
}

func restoreBody(items ...crowdsec.HubItem) []byte {
	b, _ := json.Marshal(map[string]any{"items": items})
	return b
}

func (hh *hubHarness) post(body []byte) *httptest.ResponseRecorder {
	return hh.postCtx(context.Background(), body)
}

func (hh *hubHarness) postCtx(ctx context.Context, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, restorePath, bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	hh.r.ServeHTTP(w, req)
	return w
}

func decodeRestore(t *testing.T, w *httptest.ResponseRecorder) hubRestoreResponse {
	t.Helper()
	var out hubRestoreResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), w.Body.String())
	return out
}

func restoreErr(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var out struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), w.Body.String())
	return out.Error
}

func TestRestoreHubItemsRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	valid := crowdsec.HubItem{Type: "collections", Name: "crowdsecurity/sshd"}
	long := func(n int) string { return "a/" + strings.Repeat("b", n) }
	bad := map[string]crowdsec.HubItem{
		"flag":              {Type: "collections", Name: "--help"},
		"flag in name":      {Type: "collections", Name: "-x/y"},
		"traversal":         {Type: "collections", Name: "../x"},
		"dotdot middle":     {Type: "collections", Name: "a/../b"},
		"metachar":          {Type: "collections", Name: "a;b/c"},
		"trailing newline":  {Type: "collections", Name: "a/b\n"},
		"space":             {Type: "collections", Name: "a/b c"},
		"empty owner":       {Type: "collections", Name: "/b"},
		"empty name":        {Type: "collections", Name: "a/"},
		"empty":             {Type: "collections", Name: ""},
		"no slash":          {Type: "collections", Name: "sshd"},
		"two slashes":       {Type: "collections", Name: "a/b/c"},
		"unicode":           {Type: "collections", Name: "a/bé"},
		"over-long name":    {Type: "collections", Name: long(129)},
		"wrong type":        {Type: "bouncers", Name: "a/b"},
		"empty type":        {Type: "", Name: "a/b"},
		"flag-like type":    {Type: "--help", Name: "a/b"},
		"type with space":   {Type: "collections install", Name: "a/b"},
		"upper-case type":   {Type: "Collections", Name: "a/b"},
		"command separator": {Type: "collections", Name: "a/b&&c"},
	}
	for name, item := range bad {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			hh := newHubHarness(t)
			w := hh.post(restoreBody(valid, item))
			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Equal(t, "invalid hub item", restoreErr(t, w))
			require.Empty(t, hh.exec.callLog(), "a rejected request runs nothing, valid items included")
		})
	}

	t.Run("more than 1000 items", func(t *testing.T) {
		t.Parallel()
		hh := newHubHarness(t)
		items := make([]crowdsec.HubItem, 0, crowdsec.MaxHubItems+1)
		for i := 0; i <= crowdsec.MaxHubItems; i++ {
			items = append(items, crowdsec.HubItem{Type: "scenarios", Name: fmt.Sprintf("o/n%d", i)})
		}
		w := hh.post(restoreBody(items...))
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Equal(t, "invalid hub item", restoreErr(t, w))
		require.Empty(t, hh.exec.callLog())
	})

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()
		hh := newHubHarness(t)
		w := hh.post([]byte(`{"items":[]}`))
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Equal(t, "no hub items given", restoreErr(t, w))
		require.Empty(t, hh.exec.callLog())
	})

	t.Run("malformed JSON", func(t *testing.T) {
		t.Parallel()
		hh := newHubHarness(t)
		w := hh.post([]byte(`{"items":`))
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Equal(t, "invalid request", restoreErr(t, w))
		require.Empty(t, hh.exec.callLog())
	})

	t.Run("oversized body", func(t *testing.T) {
		t.Parallel()
		hh := newHubHarness(t)
		body := append([]byte(`{"items":[],"pad":"`), bytes.Repeat([]byte("a"), 2<<20)...)
		body = append(body, []byte(`"}`)...)
		w := hh.post(body)
		require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
		require.Equal(t, "request too large", restoreErr(t, w))
		require.Empty(t, hh.exec.callLog())
	})
}

func TestRestoreHubItemsInstallsAndVerifies(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	items := hubItems("collections", "Azlaroc/sftpgo", "parsers", "crowdsecurity/CVE-2022-26134", "scenarios", "Dominic-Wagner/vaultwarden")

	// Duplicates collapse to one install.
	w := hh.post(restoreBody(append(items, items[0])...))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	res := decodeRestore(t, w)
	require.Equal(t, items, res.Installed)
	require.Empty(t, res.Failed)
	require.Empty(t, res.AlreadyInstalled)
	require.Zero(t, res.Remaining)
	require.Zero(t, res.SkippedInvalid)
	require.True(t, res.Reloaded)
	require.EqualValues(t, 1, hh.signal.Load())

	require.Equal(t, []string{
		"cscli hub update",
		"cscli collections install Azlaroc/sftpgo",
		"cscli collections inspect Azlaroc/sftpgo -o json",
		"cscli parsers install crowdsecurity/CVE-2022-26134",
		"cscli parsers inspect crowdsecurity/CVE-2022-26134 -o json",
		"cscli scenarios install Dominic-Wagner/vaultwarden",
		"cscli scenarios inspect Dominic-Wagner/vaultwarden -o json",
	}, hh.exec.callLog())

	// Arrays are present (not null) even when empty.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	require.JSONEq(t, `[]`, string(raw["failed"]))
	require.JSONEq(t, `[]`, string(raw["already_installed"]))
}

func TestRestoreHubItemsSkipsInstalled(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	have := crowdsec.HubItem{Type: "scenarios", Name: "crowdsecurity/ssh-bf"}
	linkHubItem(hh.dir, have)

	w := hh.post(restoreBody(have, crowdsec.HubItem{Type: "scenarios", Name: "crowdsecurity/other"}))
	require.Equal(t, http.StatusOK, w.Code)
	res := decodeRestore(t, w)
	require.Equal(t, []crowdsec.HubItem{have}, res.AlreadyInstalled)
	require.Equal(t, hubItems("scenarios", "crowdsecurity/other"), res.Installed)
	require.Equal(t, []string{"cscli scenarios install crowdsecurity/other"}, hh.exec.installCalls())
}

func TestRestoreHubItemsCollectionsFirstThenReDiff(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	dep := crowdsec.HubItem{Type: "scenarios", Name: "crowdsecurity/dep"}
	hh.exec.deps = map[string][]crowdsec.HubItem{"crowdsecurity/coll": {dep}}

	// The scenario is listed first but only diffed after the collection pass pulled it in.
	other := crowdsec.HubItem{Type: "parsers", Name: "crowdsecurity/solo"}
	coll := crowdsec.HubItem{Type: "collections", Name: "crowdsecurity/coll"}
	w := hh.post(restoreBody(dep, other, coll))
	require.Equal(t, http.StatusOK, w.Code)
	res := decodeRestore(t, w)
	require.Equal(t, []crowdsec.HubItem{coll, other}, res.Installed)
	require.Equal(t, []crowdsec.HubItem{dep}, res.AlreadyInstalled)
	require.Equal(t, []string{
		"cscli collections install crowdsecurity/coll",
		"cscli parsers install crowdsecurity/solo",
	}, hh.exec.installCalls())
}

func TestRestoreHubItemsReasons(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	hh.exec.noLinkFor = map[string]bool{"o/linkless": true}
	hh.exec.install = func(_ context.Context, _, name string) ([]byte, error) {
		switch name {
		case "o/missing":
			return []byte("Error: can't find 'o/missing' in scenarios at /secret/path"), errors.New("exit status 1")
		case "o/broken":
			return []byte("Error: download failed /secret/path token=abc"), errors.New("exit status 1")
		}
		return nil, nil
	}
	hh.exec.inspect = func(_ context.Context, _, name string) ([]byte, error) {
		switch name {
		case "o/tainted":
			return []byte(`{"installed":true,"tainted":true}`), nil
		case "o/notinstalled":
			return []byte(`{"installed":false}`), nil
		case "o/garbage":
			return []byte(`not json /secret/path`), nil
		}
		return []byte(`{"installed":true,"tainted":false}`), nil
	}

	w := hh.post(restoreBody(hubItems("scenarios", "o/missing", "scenarios", "o/broken", "scenarios", "o/tainted",
		"scenarios", "o/notinstalled", "scenarios", "o/garbage", "scenarios", "o/fine")...))
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "secret")
	require.NotContains(t, w.Body.String(), "token")
	res := decodeRestore(t, w)
	require.Equal(t, hubItems("scenarios", "o/fine"), res.Installed)
	require.Equal(t, []hubRestoreFailure{
		{"scenarios", "o/missing", "not_found"},
		{"scenarios", "o/broken", "install_failed"},
		{"scenarios", "o/tainted", "verify_failed"},
		{"scenarios", "o/notinstalled", "verify_failed"},
		{"scenarios", "o/garbage", "verify_failed"},
	}, res.Failed)
	require.Zero(t, res.Remaining, "permanent failures are not retryable")
}

func TestRestoreHubItemsHubUpdateFailure(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	hh.exec.update = func(context.Context) ([]byte, error) {
		return []byte("Error: Get https://hub.example/secret: no route"), errors.New("exit status 1")
	}
	w := hh.post(restoreBody(hubItems("collections", "a/b")...))
	require.Equal(t, http.StatusBadGateway, w.Code)
	require.Equal(t, "the CrowdSec hub could not be reached; nothing was installed", restoreErr(t, w))
	require.NotContains(t, w.Body.String(), "secret")
	require.Equal(t, []string{"cscli hub update"}, hh.exec.callLog())
	require.Zero(t, hh.signal.Load())
	require.False(t, hh.h.restore.running.Load(), "the running flag is cleared after a failure")
}

func TestRestoreHubItemsDeadlineLeavesNotAttempted(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	hh.h.restore.limits.overall = 100 * time.Millisecond
	hh.exec.update = func(context.Context) ([]byte, error) {
		time.Sleep(200 * time.Millisecond) // the budget is gone before the first item
		return []byte("ok"), nil
	}
	items := hubItems("collections", "a/one", "scenarios", "a/two", "parsers", "a/three")
	w := hh.post(restoreBody(items...))
	require.Equal(t, http.StatusOK, w.Code)
	res := decodeRestore(t, w)
	require.Empty(t, res.Installed)
	require.Equal(t, 3, res.Remaining)
	require.Len(t, res.Failed, 3)
	for _, f := range res.Failed {
		require.Equal(t, "not_attempted", f.Reason)
	}
	require.Empty(t, hh.exec.installCalls())
	require.False(t, res.Reloaded)
}

func TestRestoreHubItemsClientCancelBetweenItems(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// The client leaves while the first item is being verified.
	hh.exec.inspect = func(context.Context, string, string) ([]byte, error) {
		cancel()
		return []byte(`{"installed":true,"tainted":false}`), nil
	}
	w := hh.postCtx(ctx, restoreBody(hubItems("scenarios", "a/one", "scenarios", "a/two", "scenarios", "a/three")...))
	require.Equal(t, http.StatusOK, w.Code)
	res := decodeRestore(t, w)
	require.Equal(t, hubItems("scenarios", "a/one"), res.Installed)
	require.Equal(t, []hubRestoreFailure{{"scenarios", "a/two", "not_attempted"}, {"scenarios", "a/three", "not_attempted"}}, res.Failed)
	require.Equal(t, 2, res.Remaining)
	require.Len(t, hh.exec.installCalls(), 1)
	// The install that did succeed is still reloaded although the request context is cancelled.
	require.True(t, res.Reloaded)
	require.EqualValues(t, 1, hh.signal.Load())
}

func TestRestoreHubItemsReloadOutcomesAreIgnored(t *testing.T) {
	t.Parallel()
	items := restoreBody(hubItems("scenarios", "a/one")...)

	t.Run("not running", func(t *testing.T) {
		t.Parallel()
		hh := newHubHarness(t)
		hh.h.Executor = &fakeExec{}
		w := hh.post(items)
		require.Equal(t, http.StatusOK, w.Code)
		res := decodeRestore(t, w)
		require.Len(t, res.Installed, 1)
		require.False(t, res.Reloaded)
	})
	t.Run("status error", func(t *testing.T) {
		t.Parallel()
		hh := newHubHarness(t)
		hh.h.Executor = &fakeExecWithError{statusError: errors.New("boom")}
		w := hh.post(items)
		require.Equal(t, http.StatusOK, w.Code)
		res := decodeRestore(t, w)
		require.Len(t, res.Installed, 1)
		require.False(t, res.Reloaded)
	})
	t.Run("signal error", func(t *testing.T) {
		t.Parallel()
		hh := newHubHarness(t)
		hh.h.signalProcess = func(int, syscall.Signal) error { return errors.New("denied") }
		w := hh.post(items)
		require.Equal(t, http.StatusOK, w.Code)
		require.False(t, decodeRestore(t, w).Reloaded)
	})
	t.Run("nothing installed means no reload", func(t *testing.T) {
		t.Parallel()
		hh := newHubHarness(t)
		linkHubItem(hh.dir, crowdsec.HubItem{Type: "scenarios", Name: "a/one"})
		w := hh.post(items)
		require.Equal(t, http.StatusOK, w.Code)
		require.False(t, decodeRestore(t, w).Reloaded)
		require.Zero(t, hh.signal.Load())
	})
}

// blockingInstall makes install calls wait for a release, reporting each start.
type blockingInstall struct {
	started chan string
	release chan struct{}
}

func newBlockingInstall() *blockingInstall {
	return &blockingInstall{started: make(chan string, 16), release: make(chan struct{}, 16)}
}

func (b *blockingInstall) fn(ctx context.Context, _, name string) ([]byte, error) {
	b.started <- name
	select {
	case <-b.release:
		time.Sleep(20 * time.Millisecond) // the install keeps working a moment, so queued writers get scheduled
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func awaitInstall(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the install to start")
		return ""
	}
}

func TestRestoreHubItemsConcurrentCallIsRefused(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	b := newBlockingInstall()
	hh.exec.install = b.fn

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- hh.post(restoreBody(hubItems("scenarios", "a/one")...)) }()
	awaitInstall(t, b.started)

	w := hh.post(restoreBody(hubItems("scenarios", "a/two")...))
	require.Equal(t, http.StatusConflict, w.Code)
	require.Equal(t, "a restore is already running", restoreErr(t, w))

	b.release <- struct{}{}
	require.Equal(t, http.StatusOK, (<-done).Code)
	require.False(t, hh.h.restore.running.Load())

	// The flag is cleared: the next call is served.
	b.release <- struct{}{}
	require.Equal(t, http.StatusOK, hh.post(restoreBody(hubItems("scenarios", "a/two")...)).Code)
}

func TestRestoreHubItemsHungInstallFreesTheLockWithinTheItemTimeout(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	hh.h.restore.limits.item = 150 * time.Millisecond
	b := newBlockingInstall()
	hh.exec.install = func(ctx context.Context, typ, name string) ([]byte, error) {
		if name == "a/hung" { // never released: only its context ends the call
			return b.fn(ctx, typ, name)
		}
		return nil, nil
	}

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- hh.post(restoreBody(hubItems("scenarios", "a/hung", "scenarios", "a/next")...)) }()
	awaitInstall(t, b.started)

	acquired := make(chan struct{})
	start := time.Now()
	go func() {
		holdLock(&hh.h.dataMu)
		close(acquired)
		hh.h.dataMu.Unlock()
	}()
	select {
	case <-acquired:
		require.Less(t, time.Since(start), 3*time.Second)
	case <-time.After(5 * time.Second):
		t.Fatal("dataMu was not released within the per-item timeout")
	}

	w := <-done
	res := decodeRestore(t, w)
	require.Equal(t, []hubRestoreFailure{{"scenarios", "a/hung", "install_failed"}}, res.Failed)
	require.Equal(t, hubItems("scenarios", "a/next"), res.Installed)
}

// TestRestoreHubItemsExcludesImport proves an import started during an install waits for that one
// item, never overlaps it, and cannot remove the link the install created.
func TestRestoreHubItemsExcludesImport(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	seedRealLayout(t, hh.dir)
	b := newBlockingInstall()
	var seq, installDone, importDone atomic.Int64
	hh.exec.install = func(ctx context.Context, typ, name string) ([]byte, error) {
		out, err := b.fn(ctx, typ, name)
		installDone.Store(seq.Add(1)) // before the link is written and the lock released
		return out, err
	}
	item := crowdsec.HubItem{Type: "collections", Name: "crowdsecurity/coll"}

	restoreDone := make(chan *httptest.ResponseRecorder, 1)
	go func() { restoreDone <- hh.post(restoreBody(item)) }()
	awaitInstall(t, b.started)

	importResult := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		hh.r.ServeHTTP(w, importRequest(t, map[string]string{"config/config.yaml": importableConfig}))
		importDone.Store(seq.Add(1))
		importResult <- w.Code
	}()

	select {
	case <-importResult:
		t.Fatal("the import ran while an install held the data lock")
	case <-time.After(300 * time.Millisecond):
	}

	b.release <- struct{}{}
	require.Equal(t, http.StatusOK, (<-restoreDone).Code)
	require.Equal(t, http.StatusOK, <-importResult)
	require.Less(t, installDone.Load(), importDone.Load(), "the import completed after the install")

	installed, err := crowdsec.InstalledHubItems(hh.dir)
	require.NoError(t, err)
	require.Contains(t, installed, item, "the link the install created survived the import")
}

// TestRestoreHubItemsReleasesTheLockBetweenItems proves another writer gets in after every item.
func TestRestoreHubItemsReleasesTheLockBetweenItems(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	b := newBlockingInstall()
	hh.exec.install = b.fn

	items := hubItems("scenarios", "a/one", "scenarios", "a/two", "scenarios", "a/three")
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- hh.post(restoreBody(items...)) }()

	awaitInstall(t, b.started)
	var startedAtAcquire atomic.Int64
	acquired := make(chan struct{})
	go func() { // another writer queues behind the first item
		holdLock(&hh.h.dataMu)
		startedAtAcquire.Store(int64(len(hh.exec.installCalls())))
		close(acquired)
		hh.h.dataMu.Unlock()
	}()
	time.Sleep(50 * time.Millisecond)
	b.release <- struct{}{}

	// It gets in no later than the end of the next item, long before the action is over.
	deadline := time.After(5 * time.Second)
	for got := false; !got; {
		select {
		case <-acquired:
			got = true
		case <-b.started:
			b.release <- struct{}{}
		case <-deadline:
			t.Fatal("another writer was starved by the restore")
		}
	}
	require.Less(t, int(startedAtAcquire.Load()), len(items), "the writer only got in after the last item started")

	// Let the remaining items finish.
	var w *httptest.ResponseRecorder
	for w == nil {
		select {
		case <-b.started:
			b.release <- struct{}{}
		case w = <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("restore stalled")
		}
	}
	res := decodeRestore(t, w)
	require.Equal(t, items, res.Installed)
}

// TestRestoreHubItemsSerializesWithPresetApply proves ApplyCurated cannot overlap an install.
func TestRestoreHubItemsSerializesWithPresetApply(t *testing.T) {
	c := newCuratedHarness(t, nil)
	ex := &hubFakeExec{dataDir: t.TempDir()}
	b := newBlockingInstall()
	ex.install = b.fn
	c.handler.CmdExec = ex
	c.handler.DataDir = ex.dataDir

	restoreDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest(http.MethodPost, restorePath, bytes.NewReader(restoreBody(hubItems("scenarios", "a/one")...)))
		w := httptest.NewRecorder()
		c.router.ServeHTTP(w, req)
		restoreDone <- w
	}()
	awaitInstall(t, b.started)

	applyDone := make(chan int, 1)
	go func() {
		code, _ := c.apply(t, "honeypot-friendly-defaults")
		applyDone <- code
	}()
	select {
	case <-applyDone:
		t.Fatal("the preset apply ran during an install")
	case <-time.After(300 * time.Millisecond):
	}
	c.exec.mu.Lock()
	require.Empty(t, c.exec.calls, "no hub call of the preset apply while the install runs")
	c.exec.mu.Unlock()

	b.release <- struct{}{}
	require.Equal(t, http.StatusOK, (<-restoreDone).Code)
	require.Equal(t, http.StatusOK, <-applyDone)
}

func TestRestoreHubItemsMaximumRequest(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	owner := "o" + strings.Repeat("w", 63)
	items := make([]crowdsec.HubItem, 0, crowdsec.MaxHubItems)
	for i := 0; i < crowdsec.MaxHubItems; i++ {
		name := fmt.Sprintf("n%04d", i)
		items = append(items, crowdsec.HubItem{Type: "postoverflows", Name: owner + "/" + name + strings.Repeat("x", 128-len(name))})
	}
	body := restoreBody(items...)
	require.Less(t, len(body), 1<<20)

	w := hh.post(body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	res := decodeRestore(t, w)
	require.Len(t, res.Installed, crowdsec.MaxHubItems)
	require.Empty(t, res.Failed)
}

func TestHubItemNotFound(t *testing.T) {
	t.Parallel()
	// Verbatim cscli v1.8.1 output for an item the hub does not know.
	for _, typ := range crowdsec.HubItemTypes {
		it := crowdsec.HubItem{Type: typ, Name: "x/y"}
		out := []byte("Error: cscli " + typ + " install: can't find 'x/y' in " + typ)
		require.True(t, hubItemNotFound(out, it), typ)
	}
	it := crowdsec.HubItem{Type: "collections", Name: "x/y"}
	require.False(t, hubItemNotFound([]byte("Error: cscli collections install: can't find 'other/item' in collections"), it), "another item's name")
	require.False(t, hubItemNotFound([]byte("Error: cscli collections install: can't find 'x/y' in parsers"), it), "another type")
	require.False(t, hubItemNotFound([]byte("exec: \"cscli\": executable file not found in $PATH"), it))
	require.False(t, hubItemNotFound([]byte("item NOT FOUND"), it))
	require.False(t, hubItemNotFound([]byte("download failed"), it))
	require.False(t, hubItemNotFound(nil, it))
}

func TestRestoreHubItemsMissingItemOutputIsClassified(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	hh.exec.install = func(_ context.Context, typ, name string) ([]byte, error) {
		switch name {
		case "x/gone":
			return []byte("Error: cscli " + typ + " install: can't find 'x/gone' in " + typ), errors.New("exit status 1")
		case "x/nobinary":
			return []byte("exec: \"cscli\": executable file not found in $PATH"), errors.New("exit status 1")
		}
		return nil, nil
	}
	w := hh.post(restoreBody(hubItems("collections", "x/gone", "parsers", "x/gone", "scenarios", "x/nobinary")...))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []hubRestoreFailure{
		{"collections", "x/gone", "not_found"},
		{"parsers", "x/gone", "not_found"},
		{"scenarios", "x/nobinary", "install_failed"},
	}, decodeRestore(t, w).Failed)
}

// breakInstalledScan makes the next read of the installed items fail: "config" becomes a file, so the
// hub directories below it cannot be scanned.
func breakInstalledScan(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.RemoveAll(filepath.Join(dir, "config")))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config"), []byte("x"), 0o600))
}

func TestRestoreHubItemsSecondReadFailureKeepsThePartialResult(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	hh.exec.noLinkFor = map[string]bool{"x/coll": true}
	hh.exec.install = func(_ context.Context, typ, _ string) ([]byte, error) {
		if typ == "collections" {
			breakInstalledScan(t, hh.dir)
		}
		return nil, nil
	}
	pre := crowdsec.HubItem{Type: "collections", Name: "x/already"}
	coll := crowdsec.HubItem{Type: "collections", Name: "x/coll"}
	linkHubItem(hh.dir, pre)

	w := hh.post(restoreBody(pre, coll, crowdsec.HubItem{Type: "parsers", Name: "x/solo"}))
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	var body struct {
		Error            string              `json:"error"`
		Installed        []crowdsec.HubItem  `json:"installed"`
		AlreadyInstalled []crowdsec.HubItem  `json:"already_installed"`
		Failed           []hubRestoreFailure `json:"failed"`
		Reloaded         bool                `json:"reloaded"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, msgRestoreReadFailedPartial, body.Error)
	require.Equal(t, []crowdsec.HubItem{coll}, body.Installed)
	require.Equal(t, []crowdsec.HubItem{pre}, body.AlreadyInstalled)
	require.Empty(t, body.Failed, "no item is reported as not attempted")
	require.True(t, body.Reloaded, "what pass one installed still takes effect")
	require.EqualValues(t, 1, hh.signal.Load())
	require.Equal(t, []string{"cscli collections install x/coll"}, hh.exec.installCalls())
}

func TestRestoreHubItemsSecondReadFailureWithoutInstallsDoesNotReload(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	// The collection fails to install and the installed items cannot be read afterwards.
	hh.exec.install = func(_ context.Context, _, _ string) ([]byte, error) {
		breakInstalledScan(t, hh.dir)
		return []byte("Error: download failed"), errors.New("exit status 1")
	}
	w := hh.post(restoreBody(hubItems("collections", "x/coll", "parsers", "x/solo")...))
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.Equal(t, msgRestoreReadFailedPartial, restoreErr(t, w))
	require.Zero(t, hh.signal.Load())
}

func TestRestoreHubItemsFirstReadFailureInstallsNothing(t *testing.T) {
	t.Parallel()
	hh := newHubHarness(t)
	breakInstalledScan(t, hh.dir)
	w := hh.post(restoreBody(hubItems("collections", "x/coll")...))
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, msgRestoreReadFailed, restoreErr(t, w))
	require.Empty(t, hh.exec.installCalls())
	require.Zero(t, hh.signal.Load())
}

func TestHubRestoreLimitsDefaults(t *testing.T) {
	t.Parallel()
	l := hubRestoreLimits{}.withDefaults()
	require.Equal(t, hubRestoreOverall, l.overall)
	require.Equal(t, hubRestoreItemTimeout, l.item)
	require.Equal(t, hubRestoreUpdateTimeout, l.update)
	require.Equal(t, time.Second, hubRestoreLimits{overall: time.Second}.withDefaults().overall)
}

func TestHubLogDetailTruncatesAndSanitizes(t *testing.T) {
	t.Parallel()
	got := hubLogDetail("a\nb" + strings.Repeat("x", 1000))
	require.Len(t, got, hubRestoreLogLimit)
	require.NotContains(t, got, "\n")
}
