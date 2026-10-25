package crowdsec

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/logger"
)

//go:embed testdata/cscli
var cscliFixtures embed.FS

const (
	rawIndexURL   = "https://raw.githubusercontent.com/crowdsecurity/hub/master/.index.json"
	rawItemPrefix = "https://raw.githubusercontent.com/crowdsecurity/hub/master/"
	hubDataIndex  = "https://hub-data.crowdsec.net/api/index.json"
)

func readCSCLIFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := cscliFixtures.ReadFile(filepath.ToSlash(filepath.Join("testdata", "cscli", name)))
	require.NoError(t, err)
	return string(data)
}

// hubTransport serves the given URLs and answers 404 for everything else; it records request order.
type hubTransport struct {
	mu       sync.Mutex
	requests []string
	bodies   map[string]string
	status   map[string]int
}

func (h *hubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	url := req.URL.String()
	h.mu.Lock()
	h.requests = append(h.requests, url)
	h.mu.Unlock()
	if code, ok := h.status[url]; ok {
		return newResponse(code, ""), nil
	}
	if body, ok := h.bodies[url]; ok {
		resp := newResponse(http.StatusOK, body)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	}
	return newResponse(http.StatusNotFound, ""), nil
}

func (h *hubTransport) seen() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.requests...)
}

func itemNames(idx HubIndex) []string {
	names := make([]string, 0, len(idx.Items))
	for _, it := range idx.Items {
		names = append(names, it.Name)
	}
	return names
}

func TestFetchIndexListsPresetsThatAreNotInstalled(t *testing.T) {
	exec := &recordingExec{outputs: map[string][]byte{
		"cscli hub list -o json": []byte(readCSCLIFixture(t, "hub_list_installed.json")),
	}}
	transport := &hubTransport{bodies: map[string]string{rawIndexURL: readCSCLIFixture(t, "hub_index_raw_sample.json")}}
	svc := NewHubService(exec, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: transport}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.Contains(t, itemNames(idx), "crowdsecurity/sshd")
	for _, it := range idx.Items {
		if it.Name == "crowdsecurity/sshd" {
			require.Equal(t, rawItemPrefix+"collections/crowdsecurity/sshd.yaml", it.DownloadURL)
		}
	}
	require.NotContains(t, exec.calls, "cscli hub list -o json", "the installed-only listing is not an index")
}

func TestFetchIndexFallsBackToFullCSCLIListingOffline(t *testing.T) {
	exec := &recordingExec{outputs: map[string][]byte{
		"cscli hub list -a -o json": []byte(readCSCLIFixture(t, "hub_list_all.json")),
	}}
	svc := NewHubService(exec, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network unreachable")
	})}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"crowdsecurity/base-http-scenarios", "crowdsecurity/sshd"}, itemNames(idx))
	require.Equal(t, []string{"cscli hub list -a -o json"}, exec.calls)
}

func TestFetchIndexTriesCDNFirst(t *testing.T) {
	transport := &hubTransport{bodies: map[string]string{cdnIndexURL: string(readCDNFixture(t, "hub_index_cdn_compact.json"))}}
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: transport}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.Len(t, idx.Items, 6)
	require.Equal(t, []string{cdnIndexURL}, transport.seen(), "the CDN answers first and nothing else is asked")
}

// An operator-set HUB_BASE_URL is tried before the CDN and, when it answers, nothing else is asked.
func TestFetchIndexHonorsOperatorOverrideBeforeCDN(t *testing.T) {
	override := "https://hub.crowdsec.net"
	transport := &hubTransport{bodies: map[string]string{
		override + defaultHubIndexPath: `{"items":[{"name":"crowdsecurity/demo","type":"collections"}]}`,
	}}
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HubBaseURL = override
	svc.HTTPClient = &http.Client{Transport: transport}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"crowdsecurity/demo"}, itemNames(idx))
	require.Equal(t, []string{override + defaultHubIndexPath}, transport.seen(), "the operator's hub answers first and the CDN is not asked")
}

// The legacy hub-data host answers 404 for its index path. As an explicit override it is tried first, and the CDN
// answers once it has failed.
func TestFetchIndexFallsThroughNotFoundOverrideToCDN(t *testing.T) {
	transport := &hubTransport{bodies: map[string]string{cdnIndexURL: string(readCDNFixture(t, "hub_index_cdn_compact.json"))}}
	svc := NewHubService(nil, nil, t.TempDir())
	svc.HubBaseURL = "https://hub-data.crowdsec.net"
	svc.HTTPClient = &http.Client{Transport: transport}

	idx, err := svc.FetchIndex(context.Background())
	require.NoError(t, err)
	require.Len(t, idx.Items, 6)
	require.Equal(t, []string{hubDataIndex, cdnIndexURL}, transport.seen())
}

func TestFindIndexEntryMatchesExactNameOnly(t *testing.T) {
	idx := HubIndex{Items: []HubIndexEntry{
		{Name: "crowdsecurity/sshd", Title: "SSH daemon", Type: "collections"},
		{Name: "Azlaroc/sftpgo", Title: "sftpgo", Type: "collections"},
		{Name: "crowdsecurity/crs", Title: "rules", Type: "appsec-rules"},
		{Name: "crowdsecurity/crs", Title: "config", Type: "appsec-configs"},
	}}

	entry, ok := findIndexEntry(idx, "collections", "crowdsecurity/sshd")
	require.True(t, ok)
	require.Equal(t, "crowdsecurity/sshd", entry.Name)
	entry, ok = findIndexEntry(idx, "collections", "Azlaroc/sftpgo")
	require.True(t, ok)
	require.Equal(t, "Azlaroc/sftpgo", entry.Name)

	for _, query := range []string{"SSH daemon", "sshd", "sftpgo", "azlaroc/sftpgo", "Crowdsecurity/sshd", ""} {
		_, found := findIndexEntry(idx, "collections", query)
		require.False(t, found, "%q must not match", query)
	}

	for _, typ := range []string{"appsec-rules", "appsec-configs"} {
		entry, ok = findIndexEntry(idx, typ, "crowdsecurity/crs")
		require.True(t, ok, typ)
		require.Equal(t, typ, entry.Type, "the entry of the requested section is returned")
	}
	_, ok = findIndexEntry(idx, "collections", "crowdsecurity/crs")
	require.False(t, ok, "the name exists in other sections only")
}

func TestPullFindsPresetsThatAreNotInstalled(t *testing.T) {
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	exec := &recordingExec{outputs: map[string][]byte{
		"cscli hub list -o json": []byte(readCSCLIFixture(t, "hub_list_installed.json")),
	}}
	transport := &hubTransport{bodies: map[string]string{
		rawIndexURL: readCSCLIFixture(t, "hub_index_raw_sample.json"),
		rawItemPrefix + "collections/crowdsecurity/sshd.yaml": "name: crowdsecurity/sshd\n",
	}}
	svc := NewHubService(exec, cache, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: transport}

	res, err := svc.Pull(context.Background(), "crowdsecurity/sshd", "")
	require.NoError(t, err)
	require.Equal(t, "crowdsecurity/sshd", res.Meta.Slug)
	require.Contains(t, res.Preview, "crowdsecurity/sshd")
}

// Hub item names keep their case end to end (cscli names are case-sensitive).
func TestPullKeepsMixedCaseSlug(t *testing.T) {
	cache, err := NewHubCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	transport := &hubTransport{bodies: map[string]string{
		rawIndexURL: `{"collections":{"Azlaroc/sftpgo":{"path":"collections/Azlaroc/sftpgo.yaml","version":"0.1","description":"sftpgo"}}}`,
		rawItemPrefix + "collections/Azlaroc/sftpgo.yaml": "name: Azlaroc/sftpgo\n",
	}}
	svc := NewHubService(nil, cache, t.TempDir())
	svc.HTTPClient = &http.Client{Transport: transport}

	res, err := svc.Pull(context.Background(), "Azlaroc/sftpgo", "")
	require.NoError(t, err)
	require.Equal(t, "Azlaroc/sftpgo", res.Meta.Slug)
}

// fieldHook keeps the message and fields of every log entry.
type fieldHook struct {
	mu      sync.Mutex
	entries []*logrus.Entry
}

func (h *fieldHook) Levels() []logrus.Level { return logrus.AllLevels }

func (h *fieldHook) Fire(e *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = append(h.entries, e)
	return nil
}

// Not parallel: it switches the shared logger to debug level to see the message.
func TestIndexParsersCountSkippedNamesWithoutLoggingThem(t *testing.T) {
	hook := &fieldHook{}
	logger.Log().Logger.AddHook(hook)
	previous := logger.Log().Logger.GetLevel()
	logger.Log().Logger.SetLevel(logrus.DebugLevel)
	t.Cleanup(func() { logger.Log().Logger.SetLevel(previous) })

	_, err := parseCSCLIIndex([]byte(`{"collections":[{"name":"crowdsecurity/ok"},{"name":"bad name"},{"name":"solo"}]}`))
	require.NoError(t, err)
	_, err = parseRawIndex([]byte(`{"collections":{"crowdsecurity/ok":{"path":"p"},"bad name":{"path":"q"}}}`), "https://hub.crowdsec.net")
	require.NoError(t, err)

	hook.mu.Lock()
	defer hook.mu.Unlock()
	got := map[string]any{}
	for _, e := range hook.entries {
		if e.Message == "hub index entries with unusable names skipped" {
			got[e.Data["source"].(string)] = e.Data["dropped"]
			for _, v := range e.Data {
				require.NotContains(t, fmt.Sprint(v), "bad name")
			}
		}
	}
	require.Equal(t, map[string]any{"cscli": 2, "raw": 1}, got)
}
