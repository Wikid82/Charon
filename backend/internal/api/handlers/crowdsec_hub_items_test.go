package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/stretchr/testify/require"
)

// hubLink creates the symlink an installed hub item leaves behind.
func hubLink(t *testing.T, dir, rel, target string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
	require.NoError(t, os.Symlink(target, p))
}

type rawEntry struct {
	name string
	body string
	flag byte
}

// rawArchive builds a tar.gz with exactly the given entry names, in order.
func rawArchive(t *testing.T, entries []rawEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, e := range entries {
		flag := e.flag
		if flag == 0 {
			flag = tar.TypeReg
		}
		hdr := &tar.Header{Name: e.name, Mode: 0o600, Typeflag: flag}
		if flag == tar.TypeReg {
			hdr.Size = int64(len(e.body))
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if flag == tar.TypeReg {
			_, err := tw.Write([]byte(e.body))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func importHubResponse(t *testing.T, r http.Handler, archive []byte) map[string]json.RawMessage {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, importBytesRequest(t, archive))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.JSONEq(t, `"imported"`, string(body["status"]))
	return body
}

func TestExportThenImportReportsMissingHubItems(t *testing.T) {
	t.Parallel()
	const hub = "/etc/crowdsec/hub/"
	src := t.TempDir()
	seedRealLayout(t, src)
	hubLink(t, src, "config/collections/sftpgo.yaml", hub+"collections/Azlaroc/sftpgo.yaml")
	hubLink(t, src, "config/scenarios/CVE-2022-26134.yaml", hub+"scenarios/crowdsecurity/CVE-2022-26134.yaml")
	hubLink(t, src, "config/parsers/s02-enrich/geoip-enrich.yaml", hub+"parsers/s02-enrich/crowdsecurity/geoip-enrich.yaml")
	_, srcRouter := newImportRouter(t, src)
	exported := exportBytes(t, srcRouter)
	require.Contains(t, archiveNames(t, exported), crowdsec.ManifestEntryName)

	// The target only has the geoip parser (and a link of its own that is not in the export).
	dst := t.TempDir()
	seedRealLayout(t, dst)
	hubLink(t, dst, "config/parsers/s02-enrich/geoip-enrich.yaml", hub+"parsers/s02-enrich/crowdsecurity/geoip-enrich.yaml")
	hubLink(t, dst, "config/scenarios/only-here.yaml", hub+"scenarios/target/only-here.yaml")
	_, dstRouter := newImportRouter(t, dst)

	body := importHubResponse(t, dstRouter, exported)
	require.JSONEq(t, `{"in_export":5,"skipped_invalid":0,"missing":[`+
		`{"type":"collections","name":"Azlaroc/sftpgo"},`+
		`{"type":"scenarios","name":"crowdsecurity/CVE-2022-26134"}]}`, string(body["hub_items"]))
	require.NoFileExists(t, filepath.Join(dst, crowdsec.ManifestEntryName))
	require.FileExists(t, filepath.Join(dst, "config", "config.yaml"))
	assertTargetLinkKept(t, dst)
}

func assertTargetLinkKept(t *testing.T, dst string) {
	t.Helper()
	target, err := os.Readlink(filepath.Join(dst, "config", "scenarios", "only-here.yaml"))
	require.NoError(t, err)
	require.Equal(t, "/etc/crowdsec/hub/scenarios/target/only-here.yaml", target)
}

func TestImportConfigNothingMissingReportsEmptyList(t *testing.T) {
	t.Parallel()
	dst := t.TempDir()
	seedRealLayout(t, dst)
	hubLink(t, dst, "config/collections/a.yaml", "/etc/crowdsec/hub/collections/o/a.yaml")
	_, r := newImportRouter(t, dst)
	body := importHubResponse(t, r, rawArchive(t, []rawEntry{
		{name: "config/config.yaml", body: importableConfig},
		{name: crowdsec.ManifestEntryName, body: `{"version":1,"items":[{"type":"collections","name":"o/a"}]}`},
	}))
	require.JSONEq(t, `{"in_export":1,"skipped_invalid":0,"missing":[]}`, string(body["hub_items"]))
}

func TestImportConfigLegacyArchiveHasNoHubItems(t *testing.T) {
	t.Parallel()
	dst := t.TempDir()
	seedRealLayout(t, dst)
	_, r := newImportRouter(t, dst)
	body := importHubResponse(t, r, rawArchive(t, []rawEntry{{name: "config/config.yaml", body: importableConfig}}))
	require.NotContains(t, body, "hub_items")
	require.Contains(t, body, "backup")
}

func TestImportConfigUnusableManifestIsIgnored(t *testing.T) {
	t.Parallel()
	good := `{"version":1,"items":[{"type":"collections","name":"o/a"}]}`
	cfg := rawEntry{name: "config/config.yaml", body: importableConfig}
	overCap := make([]string, 0, crowdsec.MaxHubItems+1)
	for i := 0; i <= crowdsec.MaxHubItems; i++ {
		overCap = append(overCap, fmt.Sprintf(`{"type":"collections","name":"o%x/%x"}`, i*7919, i*104729))
	}
	cases := map[string][]rawEntry{
		"malformed":       {cfg, {name: crowdsec.ManifestEntryName, body: `{"version":`}},
		"unknown version": {cfg, {name: crowdsec.ManifestEntryName, body: `{"version":2,"items":[]}`}},
		"over the cap":    {cfg, {name: crowdsec.ManifestEntryName, body: `{"version":1,"items":[` + strings.Join(overCap, ",") + `]}`}},
		"duplicate":       {cfg, {name: crowdsec.ManifestEntryName, body: good}, {name: "./" + crowdsec.ManifestEntryName, body: good}},
		"symlink entry":   {cfg, {name: crowdsec.ManifestEntryName, flag: tar.TypeSymlink}},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dst := t.TempDir()
			seedRealLayout(t, dst)
			_, r := newImportRouter(t, dst)
			body := importHubResponse(t, r, rawArchive(t, entries))
			require.NotContains(t, body, "hub_items")
			require.NoFileExists(t, filepath.Join(dst, crowdsec.ManifestEntryName))
		})
	}
}

func TestImportConfigCountsInvalidManifestItems(t *testing.T) {
	t.Parallel()
	dst := t.TempDir()
	seedRealLayout(t, dst)
	_, r := newImportRouter(t, dst)
	body := importHubResponse(t, r, rawArchive(t, []rawEntry{
		{name: "config/config.yaml", body: importableConfig},
		{name: crowdsec.ManifestEntryName, body: `{"version":1,"items":[` +
			`{"type":"collections","name":"crowdsecurity/CVE-2022-26134"},{"type":"collections","name":"--help"},` +
			`{"type":"appsec-rules","name":"a/b"},{"type":"parsers","name":"a/../b"}]}`},
	}))
	require.JSONEq(t, `{"in_export":1,"skipped_invalid":3,"missing":[{"type":"collections","name":"crowdsecurity/CVE-2022-26134"}]}`, string(body["hub_items"]))
}

func TestImportConfigManifestSpellingsNeverReachDisk(t *testing.T) {
	t.Parallel()
	good := `{"version":1,"items":[{"type":"collections","name":"o/a"}]}`
	for _, spelling := range []string{crowdsec.ManifestEntryName, "./" + crowdsec.ManifestEntryName, "a/../" + crowdsec.ManifestEntryName, "config/../" + crowdsec.ManifestEntryName} {
		t.Run(spelling, func(t *testing.T) {
			t.Parallel()
			dst := t.TempDir()
			seedRealLayout(t, dst)
			writeTree(t, dst, map[string]string{crowdsec.ManifestEntryName: "stale live file"})
			_, r := newImportRouter(t, dst)
			body := importHubResponse(t, r, rawArchive(t, []rawEntry{
				{name: "config/config.yaml", body: importableConfig},
				{name: spelling, body: good},
			}))
			require.JSONEq(t, `{"in_export":1,"skipped_invalid":0,"missing":[{"type":"collections","name":"o/a"}]}`, string(body["hub_items"]))
			require.NoFileExists(t, filepath.Join(dst, crowdsec.ManifestEntryName))
			require.NoDirExists(t, filepath.Join(dst, "a"))
		})
	}
}

func TestImportConfigNestedManifestNameIsAnOrdinaryFile(t *testing.T) {
	t.Parallel()
	dst := t.TempDir()
	seedRealLayout(t, dst)
	_, r := newImportRouter(t, dst)
	body := importHubResponse(t, r, rawArchive(t, []rawEntry{
		{name: "config/config.yaml", body: importableConfig},
		{name: "config/" + crowdsec.ManifestEntryName, body: "ordinary"},
	}))
	require.NotContains(t, body, "hub_items")
	require.Equal(t, "ordinary", readTreeFile(t, filepath.Join(dst, "config", crowdsec.ManifestEntryName)))
	require.NoFileExists(t, filepath.Join(dst, crowdsec.ManifestEntryName))
}

func TestSkipArchiveEntrySkipsOnlyTheRootManifest(t *testing.T) {
	t.Parallel()
	keep := func(string) bool { return false }
	for _, typeflag := range []byte{tar.TypeReg, tar.TypeDir, tar.TypeSymlink} {
		require.True(t, skipArchiveEntry(map[string]struct{}{}, crowdsec.ManifestEntryName, typeflag, keep))
	}
	require.False(t, skipArchiveEntry(map[string]struct{}{}, "config/"+crowdsec.ManifestEntryName, tar.TypeReg, keep))
}

func TestAddHubItemsReportOmitsFieldWhenScanFails(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	dir := t.TempDir()
	bad := filepath.Join(dir, "config", "scenarios")
	require.NoError(t, os.MkdirAll(bad, 0o750))
	require.NoError(t, os.Chmod(bad, 0o000))       //nolint:gosec // G302: test makes a directory unreadable
	t.Cleanup(func() { _ = os.Chmod(bad, 0o700) }) //nolint:gosec // G302: restore so TempDir cleanup works
	h, _ := newImportRouter(t, dir)

	resp := map[string]any{}
	h.addHubItemsReport(resp, &crowdsec.ArchiveHubItems{Items: []crowdsec.HubItem{{Type: "collections", Name: "o/a"}}})
	require.NotContains(t, resp, "hub_items")

	h.addHubItemsReport(resp, nil)
	require.Empty(t, resp)
}
