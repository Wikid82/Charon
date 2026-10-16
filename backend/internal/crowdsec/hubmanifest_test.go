package crowdsec

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsManifestPath(t *testing.T) {
	t.Parallel()
	for _, rel := range []string{"charon-hub-items.json", "./charon-hub-items.json", "a/../charon-hub-items.json"} {
		require.True(t, IsManifestPath(rel), rel)
	}
	for _, rel := range []string{"", ".", "config/charon-hub-items.json", "Charon-Hub-Items.json", "charon-hub-items.json.bak", "x/charon-hub-items.json"} {
		require.False(t, IsManifestPath(rel), rel)
	}
}

func TestParseHubManifest(t *testing.T) {
	t.Parallel()
	many := make([]string, MaxHubItems+1)
	for i := range many {
		many[i] = fmt.Sprintf(`{"type":"collections","name":"o/c%d"}`, i)
	}
	atCap := strings.Join(many[:MaxHubItems], ",")
	cases := []struct {
		name        string
		body        string
		ignored     bool
		wantItems   []HubItem
		wantSkipped int
	}{
		{"sorted and deduplicated", `{"version":1,"items":[{"type":"scenarios","name":"b/b"},{"type":"collections","name":"z/z"},{"type":"collections","name":"Azlaroc/sftpgo"},{"type":"collections","name":"z/z"}]}`, false,
			[]HubItem{{"collections", "Azlaroc/sftpgo"}, {"collections", "z/z"}, {"scenarios", "b/b"}}, 0},
		{"mixed case survives", `{"version":1,"items":[{"type":"scenarios","name":"crowdsecurity/CVE-2022-26134"}]}`, false, []HubItem{{"scenarios", "crowdsecurity/CVE-2022-26134"}}, 0},
		{"empty list is usable", `{"version":1,"items":[]}`, false, []HubItem{}, 0},
		{"no items key is usable", `{"version":1}`, false, []HubItem{}, 0},
		{"invalid items dropped and counted", `{"version":1,"items":[{"type":"collections","name":"ok/ok"},{"type":"appsec-rules","name":"a/b"},{"type":"collections","name":"--help"},{"type":"collections","name":"a/../b"},{"type":5,"name":"a/b"},"str",null,{"type":"parsers","name":"a/b\n"}]}`, false,
			[]HubItem{{"collections", "ok/ok"}}, 7},
		{"exactly the cap", `{"version":1,"items":[` + atCap + `]}`, false, nil, 0},
		{"over the cap", `{"version":1,"items":[` + strings.Join(many, ",") + `]}`, true, nil, 0},
		{"unknown version", `{"version":2,"items":[{"type":"collections","name":"a/b"}]}`, true, nil, 0},
		{"missing version", `{"items":[]}`, true, nil, 0},
		{"malformed", `{"version":1,"items":[`, true, nil, 0},
		{"trailing data", `{"version":1,"items":[]} {}`, true, nil, 0},
		{"items not a list", `{"version":1,"items":{}}`, true, nil, 0},
		{"not an object", `[]`, true, nil, 0},
		{"empty", ``, true, nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseHubManifest([]byte(tc.body))
			if tc.ignored {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, tc.wantSkipped, got.SkippedInvalid)
			if tc.wantItems != nil {
				require.Equal(t, tc.wantItems, got.Items)
			} else {
				require.Len(t, got.Items, MaxHubItems)
			}
		})
	}
}

func TestMissingHubItemsAndReport(t *testing.T) {
	t.Parallel()
	m := &ArchiveHubItems{Items: []HubItem{{"collections", "a/a"}, {"collections", "b/b"}, {"parsers", "a/a"}}, SkippedInvalid: 2}
	installed := []HubItem{{"collections", "b/b"}, {"scenarios", "extra/only-here"}, {"scenarios", "a/a"}}
	require.Equal(t, []HubItem{{"collections", "a/a"}, {"parsers", "a/a"}}, m.MissingHubItems(installed))

	rep := m.Report(m.Items)
	require.Equal(t, HubItemsReport{InExport: 3, SkippedInvalid: 2, Missing: []HubItem{}}, rep)
	raw, err := json.Marshal(rep)
	require.NoError(t, err)
	require.JSONEq(t, `{"in_export":3,"skipped_invalid":2,"missing":[]}`, string(raw))
}

// realLayoutTree builds a data dir like a real install: hub links of every type plus ordinary files.
func realLayoutTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"config/config.yaml": "a: 1\n", "config/collections/mine.yaml": "user file"})
	const hub = "/etc/crowdsec/hub/"
	link(t, dir, "config/collections/sftpgo.yaml", hub+"collections/Azlaroc/sftpgo.yaml")
	link(t, dir, "config/collections/caddy.yaml", hub+"collections/crowdsecurity/caddy.yaml")
	link(t, dir, "config/scenarios/CVE-2022-26134.yaml", hub+"scenarios/crowdsecurity/CVE-2022-26134.yaml")
	link(t, dir, "config/parsers/s01-parse/caddy-logs.yaml", hub+"parsers/s01-parse/crowdsecurity/caddy-logs.yaml")
	link(t, dir, "config/postoverflows/s01-whitelist/rdns.yaml", hub+"postoverflows/s01-whitelist/crowdsecurity/rdns.yaml")
	link(t, dir, "config/contexts/http_base.yaml", hub+"contexts/crowdsecurity/http_base.yaml")
	link(t, dir, "config/collections/elsewhere.yaml", "/home/me/elsewhere.yaml")
	return dir
}

var realLayoutItems = []HubItem{
	{"collections", "Azlaroc/sftpgo"},
	{"collections", "crowdsecurity/caddy"},
	{"contexts", "crowdsecurity/http_base"},
	{"parsers", "crowdsecurity/caddy-logs"},
	{"postoverflows", "crowdsecurity/rdns"},
	{"scenarios", "crowdsecurity/CVE-2022-26134"},
}

func exportToFile(t *testing.T, dir string) (archivePath string, raw []byte) {
	t.Helper()
	var buf bytes.Buffer
	_, _, err := WriteExportArchive(context.Background(), dir, &buf)
	require.NoError(t, err)
	archivePath = filepath.Join(t.TempDir(), "export.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, buf.Bytes(), 0o600))
	return archivePath, buf.Bytes()
}

func TestExportManifestRoundTrip(t *testing.T) {
	t.Parallel()
	dir := realLayoutTree(t)
	archivePath, raw := exportToFile(t, dir)

	require.Equal(t, []string{"charon-hub-items.json", "config/collections/mine.yaml", "config/config.yaml"}, exportNames(t, raw))

	body := readTarEntry(t, raw, ManifestEntryName)
	var m hubManifest
	require.NoError(t, json.Unmarshal(body, &m))
	require.Equal(t, 1, m.Version)
	require.Equal(t, realLayoutItems, m.Items)
	require.JSONEq(t, `{"version":1,"items":[`+
		`{"type":"collections","name":"Azlaroc/sftpgo"},{"type":"collections","name":"crowdsecurity/caddy"},`+
		`{"type":"contexts","name":"crowdsecurity/http_base"},{"type":"parsers","name":"crowdsecurity/caddy-logs"},`+
		`{"type":"postoverflows","name":"crowdsecurity/rdns"},{"type":"scenarios","name":"crowdsecurity/CVE-2022-26134"}]}`, string(body))

	info, err := InspectArchive(archivePath)
	require.NoError(t, err)
	require.NotNil(t, info.HubItems)
	require.Equal(t, realLayoutItems, info.HubItems.Items)
	require.Zero(t, info.HubItems.SkippedInvalid)
	require.NotContains(t, info.Files, ManifestEntryName, "the manifest is not an inspected file")

	// Exports are deterministic in content.
	_, raw2 := exportToFile(t, dir)
	require.Equal(t, body, readTarEntry(t, raw2, ManifestEntryName))
}

func TestExportWithoutInstalledItemsHasNoManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"config/config.yaml": "a: 1\n"})
	link(t, dir, "config/collections/x.yaml", "/somewhere/else.yaml")
	archivePath, raw := exportToFile(t, dir)
	require.Equal(t, []string{"config/config.yaml"}, exportNames(t, raw))
	info, err := InspectArchive(archivePath)
	require.NoError(t, err)
	require.Nil(t, info.HubItems)
}

func TestExportNeverEmitsLiveFileWithManifestName(t *testing.T) {
	t.Parallel()
	t.Run("without items", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeExportTree(t, dir, map[string]string{"config/config.yaml": "a: 1\n", ManifestEntryName: `{"version":1,"items":[{"type":"collections","name":"evil/evil"}]}`})
		_, raw := exportToFile(t, dir)
		require.Equal(t, []string{"config/config.yaml"}, exportNames(t, raw))
	})
	t.Run("with items the generated one wins", func(t *testing.T) {
		t.Parallel()
		dir := realLayoutTree(t)
		writeExportTree(t, dir, map[string]string{ManifestEntryName: `{"version":1,"items":[{"type":"collections","name":"evil/evil"}]}`})
		_, raw := exportToFile(t, dir)
		names := exportNames(t, raw)
		require.Equal(t, 1, countOf(names, ManifestEntryName))
		var m hubManifest
		require.NoError(t, json.Unmarshal(readTarEntry(t, raw, ManifestEntryName), &m))
		require.Equal(t, realLayoutItems, m.Items)
	})
	t.Run("nested file is ordinary", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeExportTree(t, dir, map[string]string{"config/config.yaml": "a: 1\n", "config/" + ManifestEntryName: "ordinary"})
		_, raw := exportToFile(t, dir)
		require.Equal(t, []string{"config/charon-hub-items.json", "config/config.yaml"}, exportNames(t, raw))
	})
}

func TestExportManifestFailureModes(t *testing.T) {
	t.Parallel()
	t.Run("size cap counts the manifest", func(t *testing.T) {
		t.Parallel()
		dir := realLayoutTree(t)
		_, _, err := writeExportArchive(context.Background(), dir, io.Discard, 20, exportHooks{})
		require.ErrorIs(t, err, ErrExportTooLarge)
	})
	t.Run("manifest fits when only it is left", func(t *testing.T) {
		t.Parallel()
		dir := realLayoutTree(t)
		total, _, err := writeExportArchive(context.Background(), dir, io.Discard, MaxImportUncompressedBytes, exportHooks{})
		require.NoError(t, err)
		_, _, err = writeExportArchive(context.Background(), dir, io.Discard, total-1, exportHooks{})
		require.ErrorIs(t, err, ErrExportTooLarge)
		_, _, err = writeExportArchive(context.Background(), dir, io.Discard, total, exportHooks{})
		require.NoError(t, err)
	})
	t.Run("unreadable type directory omits the manifest", func(t *testing.T) {
		t.Parallel()
		skipIfRoot(t)
		dir := realLayoutTree(t)
		bad := filepath.Join(dir, "config", "scenarios")
		require.NoError(t, os.Chmod(bad, 0o000))       //nolint:gosec // G302: test makes a directory unreadable
		t.Cleanup(func() { _ = os.Chmod(bad, 0o700) }) //nolint:gosec // G302: restore so TempDir cleanup works
		_, raw := exportToFile(t, dir)
		require.NotContains(t, exportNames(t, raw), ManifestEntryName)
	})
	t.Run("tar write failure", func(t *testing.T) {
		t.Parallel()
		dir := realLayoutTree(t)
		_, _, err := WriteExportArchive(context.Background(), dir, failingWriter{err: errors.New("disk full")})
		require.Error(t, err)
	})
}

func TestWriteHubManifestWriterErrors(t *testing.T) {
	t.Parallel()
	dir := realLayoutTree(t)
	tw := tar.NewWriter(io.Discard)
	require.NoError(t, tw.Close())
	_, err := writeHubManifest(tw, dir, 0, MaxImportUncompressedBytes)
	require.Error(t, err, "a closed tar writer rejects the header")
}

func countOf(names []string, want string) int {
	n := 0
	for _, name := range names {
		if name == want {
			n++
		}
	}
	return n
}

func readTarEntry(t *testing.T, gz []byte, name string) []byte {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(gz))
	require.NoError(t, err)
	tr := tar.NewReader(gr)
	for {
		hdr, nextErr := tr.Next()
		require.NoError(t, nextErr, "entry %s not found", name)
		if hdr.Name == name {
			b, readErr := io.ReadAll(tr)
			require.NoError(t, readErr)
			return b
		}
	}
}

// manifestArchive writes the given raw entries into a tar.gz and returns its path.
func manifestArchive(t *testing.T, build func(add func(h *tar.Header, body string))) string {
	t.Helper()
	raw := tarGzRaw(t, func(tw *tar.Writer) {
		build(func(h *tar.Header, body string) {
			h.Size = int64(len(body))
			if h.Typeflag == 0 {
				h.Typeflag = tar.TypeReg
			}
			if h.Mode == 0 {
				h.Mode = 0o600
			}
			require.NoError(t, tw.WriteHeader(h))
			_, err := tw.Write([]byte(body))
			require.NoError(t, err)
		})
		require.NoError(t, tw.Close())
	})
	p := filepath.Join(t.TempDir(), "m.tar.gz")
	require.NoError(t, os.WriteFile(p, raw, 0o600))
	return p
}

const goodManifest = `{"version":1,"items":[{"type":"collections","name":"a/a"}]}`

func TestInspectArchiveManifest(t *testing.T) {
	t.Parallel()
	good := []HubItem{{"collections", "a/a"}}
	cases := []struct {
		name  string
		build func(add func(h *tar.Header, body string))
		want  []HubItem // nil means ignored
	}{
		{"root", func(add func(*tar.Header, string)) { add(&tar.Header{Name: ManifestEntryName}, goodManifest) }, good},
		{"dot slash spelling", func(add func(*tar.Header, string)) { add(&tar.Header{Name: "./" + ManifestEntryName}, goodManifest) }, good},
		{"dot dot spelling", func(add func(*tar.Header, string)) { add(&tar.Header{Name: "a/../" + ManifestEntryName}, goodManifest) }, good},
		{"nested is not the manifest", func(add func(*tar.Header, string)) {
			add(&tar.Header{Name: "config/" + ManifestEntryName}, goodManifest)
		}, nil},
		{"directory typeflag", func(add func(*tar.Header, string)) {
			add(&tar.Header{Name: ManifestEntryName, Typeflag: tar.TypeDir}, "")
		}, nil},
		{"symlink typeflag", func(add func(*tar.Header, string)) {
			add(&tar.Header{Name: ManifestEntryName, Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}, "")
		}, nil},
		{"non-regular entry does not count as a duplicate", func(add func(*tar.Header, string)) {
			add(&tar.Header{Name: ManifestEntryName, Typeflag: tar.TypeSymlink, Linkname: "x"}, "")
			add(&tar.Header{Name: ManifestEntryName}, goodManifest)
		}, good},
		{"duplicate exact", func(add func(*tar.Header, string)) {
			add(&tar.Header{Name: ManifestEntryName}, goodManifest)
			add(&tar.Header{Name: ManifestEntryName}, goodManifest)
		}, nil},
		{"duplicate through spellings", func(add func(*tar.Header, string)) {
			add(&tar.Header{Name: ManifestEntryName}, goodManifest)
			add(&tar.Header{Name: "./" + ManifestEntryName}, `{"version":1,"items":[]}`)
		}, nil},
		{"duplicate with nested is fine", func(add func(*tar.Header, string)) {
			add(&tar.Header{Name: ManifestEntryName}, goodManifest)
			add(&tar.Header{Name: "x/" + ManifestEntryName}, `garbage`)
		}, good},
		{"malformed", func(add func(*tar.Header, string)) { add(&tar.Header{Name: ManifestEntryName}, `{"version":`) }, nil},
		{"unknown version", func(add func(*tar.Header, string)) {
			add(&tar.Header{Name: ManifestEntryName}, `{"version":9,"items":[]}`)
		}, nil},
		{"over the byte cap", func(add func(*tar.Header, string)) {
			add(&tar.Header{Name: ManifestEntryName}, `{"version":1,"items":[],"pad":"`+strings.Repeat("x", int(maxManifestBytes))+`"}`)
		}, nil},
		{"escaping name", func(add func(*tar.Header, string)) { add(&tar.Header{Name: "../" + ManifestEntryName}, goodManifest) }, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			info, err := InspectArchive(manifestArchive(t, tc.build))
			require.NoError(t, err)
			require.False(t, info.Incomplete, "a manifest problem never makes the notification inspection incomplete")
			if tc.want == nil {
				require.Nil(t, info.HubItems)
				return
			}
			require.NotNil(t, info.HubItems)
			require.Equal(t, tc.want, info.HubItems.Items)
		})
	}
}

func TestInspectArchiveManifestSkippedInvalidAndLegacy(t *testing.T) {
	t.Parallel()
	info, err := InspectArchive(manifestArchive(t, func(add func(*tar.Header, string)) {
		add(&tar.Header{Name: "config/config.yaml"}, "a: 1\n")
		add(&tar.Header{Name: ManifestEntryName}, `{"version":1,"items":[{"type":"collections","name":"ok/ok"},{"type":"collections","name":"bad name"},{"type":"x","name":"a/b"}]}`)
	}))
	require.NoError(t, err)
	require.Equal(t, []HubItem{{"collections", "ok/ok"}}, info.HubItems.Items)
	require.Equal(t, 2, info.HubItems.SkippedInvalid)

	legacy, err := InspectArchive(manifestArchive(t, func(add func(*tar.Header, string)) {
		add(&tar.Header{Name: "config/config.yaml"}, "a: 1\n")
	}))
	require.NoError(t, err)
	require.Nil(t, legacy.HubItems)
}
