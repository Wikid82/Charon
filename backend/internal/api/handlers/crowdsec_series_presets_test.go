package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/crowdsec/hubcachetest"
	"github.com/Wikid82/charon/backend/internal/models"
)

const (
	presetsURL     = "/api/v1/admin/crowdsec/presets"
	itemCRSRules   = "name: crowdsecurity/crs\ntype: appsec-rule\nrules: []\n"
	itemCRSConfigs = "name: crowdsecurity/crs\ntype: appsec-config\ndefault_remediation: ban\n"
	itemNginx      = "name: crowdsecurity/nginx\ndescription: nginx\n"
)

// seriesItems maps the download path of the raw index excerpt (testdata/hub_index_ambiguous.json) to its item.
var seriesItems = map[string]string{
	"/appsec-rules/crowdsecurity/crs.yaml":   itemCRSRules,
	"/appsec-configs/crowdsecurity/crs.yaml": itemCRSConfigs,
	"/collections/crowdsecurity/nginx.yaml":  itemNginx,
}

// presetEnv is a hub whose index is the recorded raw index excerpt (crowdsecurity/crs exists in two sections),
// with a scripted cscli.
type presetEnv struct {
	t        *testing.T
	dataDir  string
	cacheDir string
	hub      *crowdsec.HubService
	r        *gin.Engine
	h        *CrowdsecHandler

	mu    sync.Mutex
	calls []string
	// cscli answers a command (arguments joined by spaces); nil means the healthy default.
	cscli func(cmd string) ([]byte, error)
	// network, when set, answers every hub request instead of the index and items.
	network func(*http.Request) (*http.Response, error)
}

func (e *presetEnv) Execute(_ context.Context, _ string, args ...string) ([]byte, error) {
	cmd := strings.Join(args, " ")
	e.mu.Lock()
	e.calls = append(e.calls, cmd)
	script := e.cscli
	e.mu.Unlock()
	if script != nil {
		if out, err := script(cmd); out != nil || err != nil {
			return out, err
		}
	}
	switch {
	case cmd == "version":
		return []byte("version: v1.8.2-rc1"), nil
	case strings.HasSuffix(cmd, " inspect crowdsecurity/crs -o json"):
		typ := strings.Fields(cmd)[0]
		item := itemCRSRules
		if typ == "appsec-configs" {
			item = itemCRSConfigs
		}
		return fmt.Appendf(nil, `{"installed":true,"tainted":false,"up_to_date":true,"local_hash":%q}`, hubcachetest.SHA256Hex([]byte(item))), nil
	case strings.HasSuffix(cmd, " inspect crowdsecurity/nginx -o json"):
		return fmt.Appendf(nil, `{"installed":true,"tainted":false,"up_to_date":true,"local_hash":%q}`, hubcachetest.SHA256Hex([]byte(itemNginx))), nil
	case cmd == "hub list -o json":
		return []byte(`{"collections":[{"name":"crowdsecurity/nginx","status":"enabled"}],"appsec-rules":[{"name":"crowdsecurity/crs","status":"enabled"}]}`), nil
	}
	return []byte("ok"), nil
}

func (e *presetEnv) ran(cmd string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, c := range e.calls {
		if c == cmd {
			return true
		}
	}
	return false
}

func newPresetEnv(t *testing.T) *presetEnv {
	t.Helper()
	e := &presetEnv{t: t, dataDir: t.TempDir(), cacheDir: filepath.Join(t.TempDir(), "hub_cache")}
	writeTree(t, e.dataDir, map[string]string{"config/config.yaml": "original: true\n"})
	index, err := os.ReadFile(filepath.Join("..", "..", "crowdsec", "testdata", "hub_index_ambiguous.json"))
	require.NoError(t, err)

	cache, err := crowdsec.NewHubCache(e.cacheDir, time.Hour)
	require.NoError(t, err)
	e.hub = crowdsec.NewHubService(e, cache, e.dataDir)
	e.hub.HubBaseURL = "https://hub.crowdsec.net"
	e.hub.ApplyTimeout = 10 * time.Second
	e.hub.HTTPClient = &http.Client{Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
		if e.network != nil {
			return e.network(req)
		}
		respond := func(code int, body string) (*http.Response, error) {
			return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		if req.URL.Path == "/api/index.json" {
			return respond(http.StatusOK, string(index))
		}
		if item, ok := seriesItems[req.URL.Path]; ok {
			return respond(http.StatusOK, item)
		}
		return respond(http.StatusNotFound, "")
	})}

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecPresetEvent{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.Setting{Key: "feature.cerberus.enabled", Value: "true"}).Error)
	e.h = newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", e.dataDir)
	e.h.Hub = e.hub
	e.r = gin.New()
	e.h.RegisterRoutes(e.r.Group("/api/v1"))
	return e
}

func (e *presetEnv) post(path string, payload map[string]string) (w *httptest.ResponseRecorder, body map[string]any) {
	e.t.Helper()
	w = postPresetJSON(e.r, presetsURL+path, payload)
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w, body
}

func typesOf(v any) []string {
	var out []string
	if list, ok := v.([]any); ok {
		for _, item := range list {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// --- a name that exists in several sections ---

func TestListPresets_OneEntryPerTypeAndName(t *testing.T) {
	t.Parallel()
	e := newPresetEnv(t)

	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, presetsURL, http.NoBody))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body struct {
		Presets []map[string]any `json:"presets"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	var types []string
	for _, p := range body.Presets {
		if p["slug"] == "crowdsecurity/crs" {
			typ, _ := p["type"].(string)
			types = append(types, typ)
		}
	}
	assert.ElementsMatch(t, []string{"appsec-configs", "appsec-rules"}, types, "one entry per section, with its type")
}

func TestPullPreset_AmbiguousNameAsksForTheType(t *testing.T) {
	t.Parallel()
	e := newPresetEnv(t)

	w, body := e.post("/pull", map[string]string{"slug": "crowdsecurity/crs"})

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, "preset_type_ambiguous", body["error_code"])
	assert.ElementsMatch(t, []string{"appsec-configs", "appsec-rules"}, typesOf(body["types"]))
	assert.NoDirExists(t, filepath.Join(e.cacheDir, "crowdsecurity"), "nothing was cached")
}

func TestPullPreset_WithATypePullsThatItem(t *testing.T) {
	t.Parallel()
	e := newPresetEnv(t)
	want := map[string]string{"appsec-rules": itemCRSRules, "appsec-configs": itemCRSConfigs}

	for typ, item := range want {
		w, body := e.post("/pull", map[string]string{"slug": "crowdsecurity/crs", "type": typ})

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, item, body["preview"], typ)
		assert.Equal(t, typ, body["type"], "the pull result shows the item type")
		assert.FileExists(t, filepath.Join(e.cacheDir, typ, "crowdsecurity", "crs", "item.yaml"))
	}
}

func TestApplyPreset_InstallsTheTypeThatWasPulled(t *testing.T) {
	t.Parallel()
	e := newPresetEnv(t)
	_, pulled := e.post("/pull", map[string]string{"slug": "crowdsecurity/crs", "type": "appsec-rules"})
	key, _ := pulled["cache_key"].(string)
	require.NotEmpty(t, key)

	w, body := e.post("/apply", map[string]string{"slug": "crowdsecurity/crs", "type": "appsec-rules", "cache_key": key})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "appsec-rules", body["type"], "the apply result shows the item type")
	assert.True(t, e.ran("appsec-rules install crowdsecurity/crs"))
	assert.False(t, e.ran("appsec-configs install crowdsecurity/crs"))
}

func TestPullPreset_UnambiguousNameNeedsNoType(t *testing.T) {
	t.Parallel()
	e := newPresetEnv(t)

	w, body := e.post("/pull", map[string]string{"slug": "crowdsecurity/nginx"})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, itemNginx, body["preview"])
}

// Entries of the old cache layout (no type folder) are a miss and are removed by the next pull.
func TestPullPreset_SweepsCacheEntriesOfTheOldLayout(t *testing.T) {
	t.Parallel()
	e := newPresetEnv(t)
	item := []byte("name: legacy/old\n")
	digest := hubcachetest.SHA256Hex(item)
	now := time.Now().UTC()
	meta, err := json.Marshal(crowdsec.CachedPreset{
		Slug: "legacy/old", CacheKey: fmt.Sprintf("legacy/old-%d-%s", now.Unix(), digest), Etag: "e", Source: "hub",
		RetrievedAt: now, Type: "collections", Name: "legacy/old", ItemSHA256: digest, SizeBytes: int64(len(item)),
	})
	require.NoError(t, err)
	writeTree(t, e.cacheDir, map[string]string{"legacy/old/item.yaml": string(item), "legacy/old/metadata.json": string(meta)})

	w, _ := e.post("/pull", map[string]string{"slug": "crowdsecurity/nginx"})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NoDirExists(t, filepath.Join(e.cacheDir, "legacy"))
	assert.FileExists(t, filepath.Join(e.cacheDir, "collections", "crowdsecurity", "nginx", "item.yaml"))
}

// --- an unreachable hub ---

const msgHubUnreachableWords = "could not reach the hub"

func TestPullPreset_OfflineAnswersHubUnreachable(t *testing.T) {
	t.Parallel()
	offline := func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial tcp: lookup hub.crowdsec.net on 127.0.0.11:53: no such host")
	}
	cases := map[string]struct {
		out []byte
		err error
	}{
		"cscli fails too":      {cscliFixture(t, "hub_update_offline"), errors.New("exit status 1")},
		"cscli prints no json": {[]byte("Error: nothing to list"), nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newPresetEnv(t)
			e.network = offline
			e.cscli = func(cmd string) ([]byte, error) {
				if strings.HasPrefix(cmd, "hub list") {
					return tc.out, tc.err
				}
				return nil, nil
			}

			w, body := e.post("/pull", map[string]string{"slug": "crowdsecurity/nginx"})

			require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
			assert.Equal(t, "hub_unreachable", body["error_code"])
			msg, _ := body["error"].(string)
			assert.Contains(t, msg, msgHubUnreachableWords)
			for _, internal := range []string{"fallback failed", "dial tcp", "lookup", "127.0.0.11", "hub.crowdsec.net"} {
				assert.NotContains(t, msg, internal, "the joined internal text is not shown")
			}
			assert.NotEmpty(t, body["hub_endpoints"], "the endpoints stay for troubleshooting")
		})
	}
}

// The cscli index fallback works, but the preset is not cached and the fallback has no download location for it.
func TestPullPreset_OfflineWithAWorkingIndexFallbackAnswersHubUnreachable(t *testing.T) {
	t.Parallel()
	index, err := os.ReadFile(filepath.Join("..", "..", "crowdsec", "testdata", "cscli", "hub_list_all.json"))
	require.NoError(t, err)
	e := newPresetEnv(t)
	e.network = func(*http.Request) (*http.Response, error) { return nil, errors.New("dial tcp: no such host") }
	e.cscli = func(cmd string) ([]byte, error) {
		if strings.HasPrefix(cmd, "hub list") {
			return index, nil
		}
		return nil, nil
	}

	w, body := e.post("/pull", map[string]string{"slug": "crowdsecurity/sshd"})

	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	assert.Equal(t, "hub_unreachable", body["error_code"])
	msg, _ := body["error"].(string)
	assert.Contains(t, msg, msgHubUnreachableWords)
	assert.NotContains(t, msg, "download location")
}

func TestPullPreset_ANameMissingFromAReachableHubIsNotAnOutage(t *testing.T) {
	t.Parallel()
	e := newPresetEnv(t)

	w, body := e.post("/pull", map[string]string{"slug": "crowdsecurity/nothing-here"})

	assert.NotEqual(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	assert.NotEqual(t, "hub_unreachable", body["error_code"])
}

// An install that fails because the hub cannot be reached is a 503 with its own code, and the tree is restored.
func TestApplyPreset_OfflineInstallAnswersHubUnreachable(t *testing.T) {
	t.Parallel()
	recorded := []string{}
	for _, kind := range []string{"offline", "internal"} {
		for _, cmd := range []string{"collections_install", "parsers_install", "appsec_rules_install"} {
			recorded = append(recorded, cmd+"_"+kind)
		}
	}
	// The recorded text is what matters, not the command it came from.
	for _, fixture := range recorded {
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()
			e := newPresetEnv(t)
			out := cscliFixture(t, fixture)
			e.cscli = func(cmd string) ([]byte, error) {
				switch cmd {
				case "hub update":
					return cscliFixture(t, "hub_update_offline"), errors.New("exit status 1")
				case "collections install crowdsecurity/nginx":
					writeTree(t, e.dataDir, map[string]string{"config/config.yaml": "original: false\n", "config/leftover.yaml": "partial"})
					return out, errors.New("exit status 1")
				}
				return nil, nil
			}
			_, pulled := e.post("/pull", map[string]string{"slug": "crowdsecurity/nginx"})
			key, _ := pulled["cache_key"].(string)
			require.NotEmpty(t, key)

			w, body := e.post("/apply", map[string]string{"slug": "crowdsecurity/nginx", "cache_key": key})

			require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
			assert.Equal(t, "hub_unreachable", body["error_code"])
			msg, _ := body["error"].(string)
			assert.Contains(t, msg, msgHubUnreachableWords)
			assert.NotContains(t, msg, "cdn-hub", "the download address is not shown")
			assert.Equal(t, "original: true\n", readTreeFile(t, filepath.Join(e.dataDir, "config", "config.yaml")))
			assert.NoFileExists(t, filepath.Join(e.dataDir, "config", "leftover.yaml"))
		})
	}
}

func TestApplyPreset_NetworkMarkersMeanUnreachable(t *testing.T) {
	t.Parallel()
	for _, marker := range []string{"dial tcp", "lookup hub-data.crowdsec.net on 127.0.0.11:53", "network is unreachable", "connection refused", "i/o timeout", "no such host", "unable to send request", "unable to retrieve latest crowdsec version"} {
		t.Run(marker, func(t *testing.T) {
			t.Parallel()
			e := newPresetEnv(t)
			e.cscli = func(cmd string) ([]byte, error) {
				if cmd == "collections install crowdsecurity/nginx" {
					return []byte("Error: cscli collections install: " + marker + " while fetching"), errors.New("exit status 1")
				}
				return nil, nil
			}
			_, pulled := e.post("/pull", map[string]string{"slug": "crowdsecurity/nginx"})
			key, _ := pulled["cache_key"].(string)

			w, body := e.post("/apply", map[string]string{"slug": "crowdsecurity/nginx", "cache_key": key})

			require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
			assert.Equal(t, "hub_unreachable", body["error_code"])
		})
	}
}

// Failures that are not about the network keep their answers.
func TestApplyPreset_OtherInstallFailuresKeepTheirAnswers(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		out    string
		status int
		code   string
	}{
		"unrelated failure": {"Error: cscli collections install: disk full", http.StatusInternalServerError, ""},
		"unknown item":      {"Error: cscli collections install: can't find 'crowdsecurity/nginx' in collections", http.StatusConflict, "preset_cache_stale"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newPresetEnv(t)
			e.cscli = func(cmd string) ([]byte, error) {
				if cmd == "collections install crowdsecurity/nginx" {
					return []byte(tc.out), errors.New("exit status 1")
				}
				return nil, nil
			}
			_, pulled := e.post("/pull", map[string]string{"slug": "crowdsecurity/nginx"})
			key, _ := pulled["cache_key"].(string)

			w, body := e.post("/apply", map[string]string{"slug": "crowdsecurity/nginx", "cache_key": key})

			assert.Equal(t, tc.status, w.Code, w.Body.String())
			assert.Equal(t, tc.code, func() string { s, _ := body["error_code"].(string); return s }())
		})
	}
}

// --- previewing a cached item by type ---

func (e *presetEnv) getCached(slug, query string) (w *httptest.ResponseRecorder, body map[string]any) {
	e.t.Helper()
	w = httptest.NewRecorder()
	e.r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, presetsURL+"/cache/"+slug+query, http.NoBody))
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w, body
}

func TestGetCachedPreset_TypeSelectsTheSection(t *testing.T) {
	t.Parallel()
	e := newPresetEnv(t)
	for _, typ := range []string{"appsec-rules", "appsec-configs"} {
		w, _ := e.post("/pull", map[string]string{"slug": "crowdsecurity/crs", "type": typ})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}

	w, body := e.getCached("crowdsecurity/crs", "?type=appsec-configs")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "appsec-configs", body["type"])
	assert.Equal(t, itemCRSConfigs, body["preview"])
}

func TestGetCachedPreset_SeveralSectionsNeedAType(t *testing.T) {
	t.Parallel()
	e := newPresetEnv(t)
	for _, typ := range []string{"appsec-rules", "appsec-configs"} {
		e.post("/pull", map[string]string{"slug": "crowdsecurity/crs", "type": typ})
	}

	w, body := e.getCached("crowdsecurity/crs", "")

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, "preset_type_ambiguous", body["error_code"])
	assert.ElementsMatch(t, []string{"appsec-configs", "appsec-rules"}, typesOf(body["types"]))
}

func TestGetCachedPreset_OnlyEntryNeedsNoType(t *testing.T) {
	t.Parallel()
	e := newPresetEnv(t)
	e.post("/pull", map[string]string{"slug": "crowdsecurity/nginx"})

	w, body := e.getCached("crowdsecurity/nginx", "")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "collections", body["type"])
	assert.Equal(t, itemNginx, body["preview"])
}

func TestGetCachedPreset_MissAndBadType(t *testing.T) {
	t.Parallel()
	e := newPresetEnv(t)

	w, _ := e.getCached("crowdsecurity/nginx", "")
	assert.Equal(t, http.StatusNotFound, w.Code, "nothing cached")
	w, _ = e.getCached("crowdsecurity/nginx", "?type=collections")
	assert.Equal(t, http.StatusNotFound, w.Code, "nothing cached for the type")
	w, _ = e.getCached("crowdsecurity/nginx", "?type=links")
	assert.Equal(t, http.StatusBadRequest, w.Code, "not a hub section")
	w, _ = e.getCached("a..b", "")
	assert.Equal(t, http.StatusNotFound, w.Code, "malformed slug")
}

func TestPullAndApplyPreset_RejectAnUnknownType(t *testing.T) {
	t.Parallel()
	e := newPresetEnv(t)
	for _, path := range []string{"/pull", "/apply"} {
		w, body := e.post(path, map[string]string{"slug": "crowdsecurity/nginx", "type": "links", "cache_key": "k"})
		require.Equal(t, http.StatusBadRequest, w.Code, path+" "+w.Body.String())
		assert.Equal(t, "invalid type", body["error"])
	}
}
