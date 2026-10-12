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
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/crowdsec/hubcachetest"
	"github.com/Wikid82/charon/backend/internal/models"
)

const (
	laterCommitSkip   = "enabled by a later commit in this series"
	installSlug       = "crowdsecurity/base-http-scenarios"
	installMarkerFile = "installed-by-cscli.marker"
)

var (
	cmdVersion  = "cscli version"
	cmdUpdate   = "cscli hub update"
	cmdInstall  = "cscli collections install " + installSlug
	cmdInspect  = "cscli collections inspect " + installSlug + " -o json"
	cmdUpgrade  = "cscli collections upgrade " + installSlug
	cmdHubList  = "cscli hub list -o json"
	errExitCode = errors.New("exit status 1")
)

// readCSCLIFixture loads a recorded cscli output from the crowdsec package's testdata.
func readCSCLIFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "crowdsec", "testdata", "cscli", name)) // #nosec G304 -- fixed test fixture names
	require.NoError(t, err)
	return data
}

// inspectWithHash returns the recorded inspect output with local_hash replaced.
func inspectWithHash(t *testing.T, fixture, hash string) []byte {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(readCSCLIFixture(t, fixture), &doc))
	doc["local_hash"] = hash
	out, err := json.Marshal(doc)
	require.NoError(t, err)
	return out
}

type installHarness struct {
	router   *gin.Engine
	handler  *CrowdsecHandler
	hub      *crowdsec.HubService
	exec     *scriptedHubExec
	cacheDir string
	dataDir  string
	item     []byte
	itemSHA  string
	events   func() []models.CrowdsecPresetEvent

	reloads        atomic.Int32
	callsAtReload  atomic.Int32
	remoteRequests atomic.Int32
}

// reply is one scripted cscli answer; the zero value selects the healthy default.
type reply struct {
	out []byte
	err error
}

// installScript changes the healthy default cscli. replies maps a command to successive answers (the last
// one repeats); onCommand runs before a command is answered and may block or change state.
type installScript struct {
	replies   map[string][]reply
	onCommand func(ctx context.Context, h *installHarness, cmd string)
}

// newInstallHarness wires a handler to a scripted cscli whose healthy defaults mirror the recorded outputs.
func newInstallHarness(t *testing.T, script installScript) *installHarness {
	t.Helper()
	t.Setenv("FEATURE_CERBERUS_ENABLED", "true")

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecPresetEvent{}))

	h := &installHarness{
		cacheDir: filepath.Join(t.TempDir(), "cache"),
		dataDir:  t.TempDir(),
		item:     readCSCLIFixture(t, "hub_item_base_http_scenarios.yaml"),
	}
	h.itemSHA = hubcachetest.SHA256Hex(h.item)
	require.NoError(t, os.WriteFile(filepath.Join(h.dataDir, "config.yaml"), []byte("original: true\n"), 0o600))

	cache, err := crowdsec.NewHubCache(h.cacheDir, time.Hour)
	require.NoError(t, err)

	h.exec = &scriptedHubExec{}
	var mu sync.Mutex
	seen := map[string]int{}
	h.exec.fn = func(ctx context.Context, cmd string) ([]byte, error) {
		if script.onCommand != nil {
			script.onCommand(ctx, h, cmd)
		}
		mu.Lock()
		n := seen[cmd]
		seen[cmd]++
		mu.Unlock()
		if seq := script.replies[cmd]; len(seq) > 0 {
			if r := seq[min(n, len(seq)-1)]; r.out != nil || r.err != nil {
				return r.out, r.err
			}
		}
		return h.defaultOutput(t, cmd)
	}

	h.hub = crowdsec.NewHubService(h.exec, cache, h.dataDir)
	h.hub.ApplyTimeout = 5 * time.Second
	h.hub.HTTPClient = &http.Client{Transport: roundTripperStub(func(*http.Request) (*http.Response, error) {
		h.remoteRequests.Add(1)
		return nil, errors.New("remote hub must not be contacted by apply")
	})}
	h.hub.Reload = func(context.Context) error {
		h.reloads.Add(1)
		h.exec.mu.Lock()
		h.callsAtReload.Store(int32(len(h.exec.calls))) // #nosec G115 -- small test counter
		h.exec.mu.Unlock()
		return nil
	}

	h.handler = newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.handler.Hub = h.hub

	h.router = gin.New()
	h.handler.RegisterRoutes(h.router.Group("/api/v1"))
	h.events = func() []models.CrowdsecPresetEvent {
		var ev []models.CrowdsecPresetEvent
		require.NoError(t, db.Order("id").Find(&ev).Error)
		return ev
	}
	return h
}

// defaultOutput is a healthy cscli: the install changes the data dir, inspect reports the previewed hash.
func (h *installHarness) defaultOutput(t *testing.T, cmd string) ([]byte, error) {
	t.Helper()
	switch cmd {
	case cmdVersion:
		return []byte("version: v1.8.1"), nil
	case cmdUpdate:
		return []byte("Downloading /etc/crowdsec/hub/.index.json"), nil
	case cmdInstall:
		if err := os.WriteFile(filepath.Join(h.dataDir, installMarkerFile), []byte("x"), 0o600); err != nil {
			return nil, err
		}
		return readCSCLIFixture(t, "install_collection.txt"), nil
	case cmdInspect:
		return inspectWithHash(t, "inspect_installed.json", h.itemSHA), nil
	case cmdUpgrade:
		return []byte("ok"), nil
	case cmdHubList:
		return readCSCLIFixture(t, "hub_list_installed.json"), nil
	default:
		return nil, fmt.Errorf("unexpected command: %s", cmd)
	}
}

type roundTripperStub func(*http.Request) (*http.Response, error)

func (f roundTripperStub) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// store writes a valid cache entry for the previewed item and returns its key.
func (h *installHarness) store(t *testing.T, mutate func(*hubcachetest.Entry)) string {
	t.Helper()
	entry := hubcachetest.Entry{Slug: installSlug, Item: h.item}
	if mutate != nil {
		mutate(&entry)
	}
	return hubcachetest.Write(t, h.cacheDir, entry)
}

func (h *installHarness) apply(t *testing.T, slug, cacheKey string) (code int, resp map[string]any) {
	t.Helper()
	payload := map[string]string{"slug": slug}
	if cacheKey != "" {
		payload["cache_key"] = cacheKey
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.router.ServeHTTP(w, req)
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), w.Body.String())
	return w.Code, out
}

func (h *installHarness) calls() []string {
	h.exec.mu.Lock()
	defer h.exec.mu.Unlock()
	return append([]string(nil), h.exec.calls...)
}

func (h *installHarness) requireRolledBack(t *testing.T) {
	t.Helper()
	require.NoFileExists(t, filepath.Join(h.dataDir, installMarkerFile), "snapshot must be restored")
	require.FileExists(t, filepath.Join(h.dataDir, "config.yaml"))
	require.Zero(t, h.reloads.Load(), "no reload before the install is verified")
}

func (h *installHarness) entryExists(slug string) bool {
	_, err := os.Stat(filepath.Join(h.cacheDir, filepath.FromSlash(slug), "metadata.json"))
	return err == nil
}

func requireNoCSCLIInstall(t *testing.T, calls []string) {
	t.Helper()
	for _, c := range calls {
		require.NotContains(t, c, " install ", "no install may run")
	}
}

func TestApplyPresetRequiresCacheKey(t *testing.T) {
	t.Skip(laterCommitSkip)
	h := newInstallHarness(t, installScript{})
	h.store(t, nil)

	code, resp := h.apply(t, installSlug, "")
	require.Equal(t, http.StatusBadRequest, code)
	require.Equal(t, "preset_cache_key_required", resp["error_code"])
	require.Empty(t, h.calls())
}

func TestApplyPresetInstallsPreviewedItem(t *testing.T) {
	t.Skip(laterCommitSkip)
	h := newInstallHarness(t, installScript{})
	key := h.store(t, nil)

	code, resp := h.apply(t, installSlug, key)
	require.Equal(t, http.StatusOK, code, resp)
	require.Equal(t, "applied", resp["status"])
	require.Equal(t, true, resp["used_cscli"])
	require.Equal(t, installSlug, resp["slug"])

	calls := h.calls()
	require.Equal(t, []string{cmdVersion, cmdUpdate, cmdInstall, cmdInspect, cmdHubList}, calls)
	for _, c := range calls {
		require.NotContains(t, c, "hub install")
		require.NotContains(t, c, "--force")
	}
	require.EqualValues(t, 1, h.reloads.Load())
	require.EqualValues(t, len(calls), h.callsAtReload.Load(), "reload runs after verification")
	require.FileExists(t, filepath.Join(h.dataDir, installMarkerFile))
	require.Zero(t, h.remoteRequests.Load())
	require.True(t, h.entryExists(installSlug))
}

func TestApplyPresetKeepsHubSlugCase(t *testing.T) {
	t.Skip(laterCommitSkip)
	const slug = "Azlaroc/sftpgo"
	install := "cscli collections install " + slug
	inspect := "cscli collections inspect " + slug + " -o json"
	item := []byte("name: Azlaroc/sftpgo\n")
	h := newInstallHarness(t, installScript{replies: map[string][]reply{
		install:    {{out: []byte("ok")}},
		inspect:    {{out: inspectWithHash(t, "inspect_installed.json", hubcachetest.SHA256Hex(item))}},
		cmdHubList: {{out: []byte(`{"collections":[{"name":"Azlaroc/sftpgo","status":"enabled"}]}`)}},
	}})
	key := hubcachetest.Write(t, h.cacheDir, hubcachetest.Entry{Slug: slug, Item: item})

	code, resp := h.apply(t, slug, key)
	require.Equal(t, http.StatusOK, code, resp)
	require.Equal(t, slug, resp["slug"])
	require.Contains(t, h.calls(), install)
}

func TestApplyRejectsOutdatedCacheEntry(t *testing.T) {
	t.Skip(laterCommitSkip)
	unknownKey := installSlug + "-1-" + strings.Repeat("0", 64)
	cases := map[string]func(h *installHarness, t *testing.T) string{
		"different key": func(h *installHarness, t *testing.T) string {
			h.store(t, nil)
			return unknownKey
		},
		"expired entry": func(h *installHarness, t *testing.T) string {
			return h.store(t, func(e *hubcachetest.Entry) { e.RetrievedAt = time.Now().Add(-3 * time.Hour) })
		},
		"no entry": func(*installHarness, *testing.T) string { return unknownKey },
		"entry without digests": func(h *installHarness, t *testing.T) string {
			return h.store(t, func(e *hubcachetest.Entry) { e.Omit = []string{"item_sha256"} })
		},
		"content differs from recorded digest": func(h *installHarness, t *testing.T) string {
			key := h.store(t, nil)
			require.NoError(t, os.WriteFile(filepath.Join(h.cacheDir, filepath.FromSlash(installSlug), "item.yaml"), []byte("name: other\n"), 0o600))
			return key
		},
	}
	for name, prepare := range cases {
		t.Run(name, func(t *testing.T) {
			h := newInstallHarness(t, installScript{})
			key := prepare(h, t)

			code, resp := h.apply(t, installSlug, key)
			require.Equal(t, http.StatusConflict, code, resp)
			require.Equal(t, "preset_cache_stale", resp["error_code"])
			require.Empty(t, h.calls(), "no cscli command runs for an outdated entry")
			require.Zero(t, h.remoteRequests.Load(), "apply never refreshes from the remote hub")
			h.requireRolledBack(t)

			events := h.events()
			require.Len(t, events, 1)
			require.Equal(t, "failed", events[0].Status)
		})
	}
}

func TestApplyReportsLocallyEditedItem(t *testing.T) {
	t.Skip(laterCommitSkip)
	h := newInstallHarness(t, installScript{replies: map[string][]reply{
		// A locally edited item makes the install a no-op that still exits with status 0.
		cmdInstall: {{out: readCSCLIFixture(t, "install_locally_edited_noop.txt")}},
		cmdInspect: {{out: readCSCLIFixture(t, "inspect_locally_edited.json")}},
	}})
	key := h.store(t, nil)

	code, resp := h.apply(t, installSlug, key)
	require.Equal(t, http.StatusConflict, code, resp)
	require.Equal(t, "preset_local_modified", resp["error_code"])
	h.requireRolledBack(t)
	require.True(t, h.entryExists(installSlug), "the cache entry stays")
	for _, c := range h.calls() {
		require.NotContains(t, c, "--force")
	}
}

func TestApplyUpgradesOlderInstalledItem(t *testing.T) {
	t.Skip(laterCommitSkip)
	// Needs a check against a real cscli: the upgrade step for an older, unmodified item is not verified.
	h := newInstallHarness(t, installScript{replies: map[string][]reply{
		cmdInspect: {{out: readCSCLIFixture(t, "inspect_older_version.json")}, {}},
	}})
	key := h.store(t, nil)

	code, resp := h.apply(t, installSlug, key)
	require.Equal(t, http.StatusOK, code, resp)
	require.Equal(t, []string{cmdVersion, cmdUpdate, cmdInstall, cmdInspect, cmdUpgrade, cmdInspect, cmdHubList}, h.calls())
	require.EqualValues(t, 1, h.reloads.Load())
}

func TestApplyReportsHubStillUpdating(t *testing.T) {
	t.Skip(laterCommitSkip)
	h := newInstallHarness(t, installScript{replies: map[string][]reply{
		cmdInspect: {{out: readCSCLIFixture(t, "inspect_older_version.json")}}, // the hash never matches
	}})
	key := h.store(t, nil)

	code, resp := h.apply(t, installSlug, key)
	require.Equal(t, http.StatusConflict, code, resp)
	require.Equal(t, "preset_hub_updating", resp["error_code"])
	h.requireRolledBack(t)
	require.False(t, h.entryExists(installSlug), "the mismatching entry is evicted")
}

func TestApplyKeepsNewerCacheEntryOnMismatch(t *testing.T) {
	t.Skip(laterCommitSkip)
	var newerKey string
	h := newInstallHarness(t, installScript{
		replies: map[string][]reply{cmdInspect: {{out: readCSCLIFixture(t, "inspect_older_version.json")}}},
		onCommand: func(_ context.Context, h *installHarness, cmd string) {
			if cmd == cmdInstall {
				// A concurrent pull replaces the entry while apply is inside cscli.
				newerKey = h.store(t, func(e *hubcachetest.Entry) {
					e.Item = []byte("name: newer\n")
					e.RetrievedAt = time.Now().Add(time.Minute)
				})
			}
		},
	})
	key := h.store(t, nil)

	code, resp := h.apply(t, installSlug, key)
	require.Equal(t, http.StatusConflict, code, resp)
	require.NotEqual(t, key, newerKey)
	require.True(t, h.entryExists(installSlug), "the newer entry survives")
	raw, err := os.ReadFile(filepath.Join(h.cacheDir, filepath.FromSlash(installSlug), "metadata.json")) // #nosec G304 -- test path
	require.NoError(t, err)
	require.Contains(t, string(raw), newerKey)
}

func TestApplyReportsStaleWhenItemUnknownToCSCLI(t *testing.T) {
	t.Skip(laterCommitSkip)
	for fixture, name := range map[string]string{"install_unknown_item.txt": "unknown item", "install_missing_index.txt": "missing local index"} {
		t.Run(name, func(t *testing.T) {
			h := newInstallHarness(t, installScript{replies: map[string][]reply{
				cmdInstall: {{out: readCSCLIFixture(t, fixture), err: errExitCode}},
			}})
			key := h.store(t, nil)

			code, resp := h.apply(t, installSlug, key)
			require.Equal(t, http.StatusConflict, code, resp)
			require.Equal(t, "preset_cache_stale", resp["error_code"])
			h.requireRolledBack(t)
		})
	}
}

// hubListWith returns the recorded hub listing after mutate changed one section.
func hubListWith(t *testing.T, section string, mutate func(items []any) []any) []byte {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(readCSCLIFixture(t, "hub_list_installed.json"), &doc))
	items, ok := doc[section].([]any)
	require.True(t, ok)
	doc[section] = mutate(items)
	out, err := json.Marshal(doc)
	require.NoError(t, err)
	return out
}

func TestApplyRefusesMissingDependency(t *testing.T) {
	t.Skip(laterCommitSkip)
	h := newInstallHarness(t, installScript{replies: map[string][]reply{
		// The parser named by the item is not installed.
		cmdHubList: {{out: hubListWith(t, "parsers", func([]any) []any { return []any{} })}},
	}})
	key := h.store(t, nil)

	code, resp := h.apply(t, installSlug, key)
	require.Equal(t, http.StatusConflict, code, resp)
	require.Equal(t, "preset_cache_stale", resp["error_code"])
	h.requireRolledBack(t)
}

func TestApplyWarnsAboutLocallyEditedDependency(t *testing.T) {
	t.Skip(laterCommitSkip)
	edited := hubListWith(t, "collections", func(items []any) []any {
		for _, raw := range items {
			if item := raw.(map[string]any); item["name"] == "crowdsecurity/http-cve" {
				item["status"] = "enabled,tainted"
			}
		}
		return items
	})
	h := newInstallHarness(t, installScript{replies: map[string][]reply{cmdHubList: {{out: edited}}}})
	key := h.store(t, nil)

	code, resp := h.apply(t, installSlug, key)
	require.Equal(t, http.StatusOK, code, resp)
	require.NotEqual(t, "preset_local_modified", resp["error_code"])
	warnings, ok := resp["warnings"].([]any)
	require.True(t, ok, "response carries warnings")
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "crowdsecurity/http-cve")
	require.EqualValues(t, 1, h.reloads.Load())
}

func TestApplyReportsCSCLIUnavailable(t *testing.T) {
	t.Skip(laterCommitSkip)
	h := newInstallHarness(t, installScript{replies: map[string][]reply{
		cmdVersion: {{err: errors.New("cscli: not found")}},
	}})
	key := h.store(t, nil)

	code, resp := h.apply(t, installSlug, key)
	require.Equal(t, http.StatusServiceUnavailable, code, resp)
	require.Equal(t, "cscli_unavailable", resp["error_code"])
	requireNoCSCLIInstall(t, h.calls())
	require.True(t, h.entryExists(installSlug))
}

func TestApplyTimesOutWithoutEvictingEntry(t *testing.T) {
	t.Skip(laterCommitSkip)
	h := newInstallHarness(t, installScript{
		replies: map[string][]reply{cmdInstall: {{err: context.DeadlineExceeded}}},
		onCommand: func(ctx context.Context, h *installHarness, cmd string) {
			if cmd == cmdInstall {
				require.NoError(t, os.WriteFile(filepath.Join(h.dataDir, installMarkerFile), []byte("x"), 0o600))
				<-ctx.Done() // blocks until the apply deadline passes
			}
		},
	})
	h.hub.ApplyTimeout = 200 * time.Millisecond
	key := h.store(t, nil)

	code, resp := h.apply(t, installSlug, key)
	require.Equal(t, http.StatusGatewayTimeout, code, resp)
	require.Equal(t, "preset_apply_timeout", resp["error_code"])
	h.requireRolledBack(t)
	require.True(t, h.entryExists(installSlug), "a slow server does not evict a good entry")
}

func TestApplyCuratedPresetCacheKeyIsOptional(t *testing.T) {
	t.Skip(laterCommitSkip)
	h := newCuratedInstallHarness(t)

	code, resp := h.apply(t, "geoip-enrichment", "")
	require.Equal(t, http.StatusOK, code, resp)

	code, resp = h.apply(t, "geoip-enrichment", "curated-geoip-enrichment")
	require.Equal(t, http.StatusOK, code, resp)

	code, resp = h.apply(t, "geoip-enrichment", "some-other-key")
	require.Equal(t, http.StatusConflict, code, resp)
	require.Equal(t, "preset_cache_stale", resp["error_code"])
}

func TestApplyCuratedPresetMatchesSlugIgnoringCase(t *testing.T) {
	t.Skip(laterCommitSkip)
	h := newCuratedInstallHarness(t)

	code, resp := h.apply(t, "GeoIP-Enrichment", "")
	require.Equal(t, http.StatusOK, code, resp)
	require.Equal(t, "geoip-enrichment", resp["slug"])
}

// newCuratedInstallHarness answers every typed install and inspect of a curated preset as healthy.
func newCuratedInstallHarness(t *testing.T) *installHarness {
	t.Helper()
	return newInstallHarness(t, installScript{
		replies: map[string][]reply{
			"cscli parsers install crowdsecurity/geoip-enrich":         {{out: []byte("ok")}},
			"cscli parsers inspect crowdsecurity/geoip-enrich -o json": {{out: []byte(`{"installed":true,"tainted":false}`)}},
		},
	})
}
