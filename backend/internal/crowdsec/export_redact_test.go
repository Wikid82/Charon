package crowdsec

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/logger"
)

const planted = "SECRETTOKEN-"

// exportEntries exports dir and returns the archive bytes, the entries by name and the reported total.
func exportEntries(t *testing.T, dir string) (archive []byte, entries map[string]string, total int64) {
	t.Helper()
	var buf bytes.Buffer
	total, err := WriteExportArchive(context.Background(), dir, &buf)
	require.NoError(t, err)
	return buf.Bytes(), readEntries(t, buf.Bytes()), total
}

func readEntries(t *testing.T, archive []byte) map[string]string {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(archive))
	require.NoError(t, err)
	tr := tar.NewReader(gr)
	entries := map[string]string{}
	for {
		hdr, nextErr := tr.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		require.NoError(t, nextErr)
		body, readErr := io.ReadAll(tr)
		require.NoError(t, readErr)
		require.EqualValues(t, len(body), hdr.Size)
		entries[hdr.Name] = string(body)
	}
	return entries
}

// requireNoToken scans the whole decompressed archive (headers, names and bodies) for the planted prefix.
func requireNoToken(t *testing.T, archive []byte) {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(archive))
	require.NoError(t, err)
	raw, err := io.ReadAll(gr)
	require.NoError(t, err)
	require.NotContains(t, string(raw), planted)
}

const plantedHTTP = `# comment ` + planted + `head
type: http # ` + planted + `line
name: http_default
url: https://user:` + planted + `userinfo@hooks.example.invalid/` + planted + `path
headers: &` + planted + `anchor
  Authorization: Bearer ` + planted + `bearer
  X-Api-Token: ` + planted + `apitoken
format: '{{.|toJson}}'
# ` + planted + `foot
`

var plantedPlugins = map[string]string{
	"slack.yaml":    "type: slack\nname: s\nwebhook: https://hooks.example.invalid/" + planted + "hook\nchannel: ops\n",
	"splunk.yaml":   "type: splunk\nname: sp\nurl: https://splunk.example.invalid\ntoken: " + planted + "splunk\n",
	"email.yaml":    "type: email\nname: e\nsmtp_host: smtp.example.invalid\nsmtp_username: " + planted + "user\nsmtp_password: " + planted + "pass\nreceiver_emails: [a@example.invalid]\n",
	"sentinel.yaml": "type: sentinel\nname: se\ncustomer_id: " + planted + "cust\nshared_key: " + planted + "key\nlog_type: Charon\n",
	"unknown.yaml":  "type: mystery\nname: m\nsecret: " + planted + "unknown\nnested:\n  " + planted + "key: " + planted + "value\n",
	"yes.yaml":      "type: splunk\nname: y\ntoken: yes\nurl: on\n",
	"http.yaml":     plantedHTTP,
}

func TestExportArchiveHoldsNoPlantedValueAnywhere(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := map[string]string{
		"config/config.yaml":                         "config_paths:\n  notification_dir: " + filepath.Join(dir, "config", "custom_notify") + "\n",
		"config/notifications.bak/copy.yaml":         plantedHTTP,
		"config/notifications/sub/deeper/http.yaml":  plantedHTTP,
		"config/Notifications/UPPER.YML":             plantedHTTP,
		"config/custom_notify/http.yaml":             plantedHTTP,
		"config/notifications/http.yaml.bak":         plantedHTTP,
		"config/notifications/notes.txt":             planted + "notes",
		"config/notifications/sub/old.bak":           planted + "old",
		"config/notifications/binary.yaml":           "\x00\x01" + planted + "\xff",
		"config/notifications/duplicate_keys.yaml":   "type: http\ntype: http\nurl: " + planted + "dup\n",
		"config/notifications/oversize.yaml":         "type: http\nurl: " + planted + strings.Repeat("a", int(maxInspectEntryBytes)) + "\n",
		"config/profiles.yaml":                       "name: p\nnotifications:\n  - http_default\n",
		"config/parsers/s01-parse/custom.yaml":       "name: custom\n",
		"config/notifications/not_a_notification.md": planted + "readme",
	}
	for name, content := range plantedPlugins {
		files["config/notifications/"+name] = content
	}
	writeExportTree(t, dir, files)
	outside := filepath.Join(t.TempDir(), "outside.yaml")
	require.NoError(t, os.WriteFile(outside, []byte("type: http\nurl: "+planted+"outside\n"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "config", "notifications", "link.yaml")))
	linkedDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(linkedDir, "x.yaml"), []byte("type: http\nurl: "+planted+"dirlink\n"), 0o600))
	require.NoError(t, os.Symlink(linkedDir, filepath.Join(dir, "config", "notifications", "linked_dir")))

	archive, entries, _ := exportEntries(t, dir)
	requireNoToken(t, archive)

	for name := range entries {
		require.NotContains(t, name, planted)
	}
	require.NotContains(t, entries, "config/notifications/http.yaml.bak")
	require.NotContains(t, entries, "config/notifications/notes.txt")
	require.NotContains(t, entries, "config/notifications/sub/old.bak")
	require.NotContains(t, entries, "config/notifications/not_a_notification.md")
	require.NotContains(t, entries, "config/notifications/link.yaml")
	require.NotContains(t, entries, "config/notifications/linked_dir/x.yaml")

	for _, name := range []string{
		"config/notifications/http.yaml",
		"config/notifications/sub/deeper/http.yaml",
		"config/Notifications/UPPER.YML",
		"config/custom_notify/http.yaml",
		"config/notifications.bak/copy.yaml",
	} {
		require.Contains(t, entries, name)
		require.Contains(t, entries[name], "name: http_default")
		require.Contains(t, entries[name], RedactedPlaceholder)
	}
	for _, name := range []string{"binary.yaml", "duplicate_keys.yaml", "oversize.yaml"} {
		require.Equal(t, RedactStubContent, entries["config/notifications/"+name], name)
	}
	require.Equal(t, "name: p\nnotifications:\n  - http_default\n", entries["config/profiles.yaml"])
	require.Equal(t, "name: custom\n", entries["config/parsers/s01-parse/custom.yaml"])
}

func TestExportArchiveBlanksFlatLayoutNotifications(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{
		"config.yaml":             "common:\n  log_level: info\n",
		"notifications/http.yaml": plantedHTTP,
		"notifications/http.bak":  plantedHTTP,
	})
	archive, entries, _ := exportEntries(t, dir)
	requireNoToken(t, archive)
	require.Contains(t, entries, "notifications/http.yaml")
	require.NotContains(t, entries, "notifications/http.bak")
}

func TestExportArchiveLeavesOtherYAMLUnchanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	other := map[string]string{
		"config/config.yaml":           "db_config:\n  type: sqlite\n",
		"config/typeless.yaml":         "name: x\nurl: https://example.invalid\n",
		"config/unparsable.yaml":       "a: [unclosed\n",
		"config/type_unknown.yaml":     "type: parser\nname: x\n",
		"config/type_not_a_str.yaml":   "type: !!binary aHR0cA==\n",
		"config/parsers/dup_keys.yaml": "a: 1\na: 2\n",
		"config/plain.txt":             "type: http\nurl: x\n",
	}
	writeExportTree(t, dir, other)
	_, entries, _ := exportEntries(t, dir)
	for name, want := range other {
		require.Equal(t, want, entries[name], name)
	}
}

func TestExportArchiveLargeLookAlikeIsStreamedUnchanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	big := "type: http\nurl: x\n# " + strings.Repeat("a", int(maxInspectEntryBytes)) + "\n"
	writeExportTree(t, dir, map[string]string{"config/big.yaml": big})
	_, entries, _ := exportEntries(t, dir)
	require.Equal(t, big, entries["config/big.yaml"])
}

func TestExportArchiveSizeAccountingUsesWrittenBytes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{
		"config/config.yaml":                "a: 1\n",
		"config/notifications/http.yaml":    plantedHTTP,
		"config/notifications/stub.yaml":    "a: [unclosed\n",
		"config/notifications/ignored.back": "left out",
	})
	_, entries, total := exportEntries(t, dir)
	var sum int64
	for _, body := range entries {
		sum += int64(len(body))
	}
	require.Equal(t, sum, total)
	require.Equal(t, RedactStubContent, entries["config/notifications/stub.yaml"])
	require.NotEqual(t, int64(len(plantedHTTP)), int64(len(entries["config/notifications/http.yaml"])))
}

func TestExportArchiveSizeCapCountsRedactedLength(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	original := "type: splunk\nname: n\ntoken: " + strings.Repeat("x", 600) + "\n"
	writeExportTree(t, dir, map[string]string{"notifications/a.yaml": original})
	var buf bytes.Buffer
	redacted, err := RedactNotificationFile([]byte(original))
	require.NoError(t, err)
	require.Less(t, len(redacted.Data), len(original))

	total, err := writeExportArchive(context.Background(), dir, &buf, int64(len(redacted.Data)), exportHooks{})
	require.NoError(t, err, "the cap applies to the redacted length")
	require.EqualValues(t, len(redacted.Data), total)

	_, err = writeExportArchive(context.Background(), dir, io.Discard, int64(len(redacted.Data))-1, exportHooks{})
	require.ErrorIs(t, err, ErrExportTooLarge)
}

func TestExportArchiveNotificationFileChangedWhileReading(t *testing.T) {
	t.Parallel()
	cases := map[string]func(path string){
		"grows":   func(path string) { appendTo(path, "# more\n") },
		"shrinks": func(path string) { _ = os.Truncate(path, 4) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeExportTree(t, dir, map[string]string{"notifications/http.yaml": plantedHTTP})
			_, err := writeExportArchive(context.Background(), dir, io.Discard, MaxImportUncompressedBytes,
				exportHooks{beforeOpen: mutate})
			require.ErrorIs(t, err, ErrExportFileUnreadable)
		})
	}
}

func appendTo(path, text string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // test fixture
	if err == nil {
		_, _ = f.WriteString(text)
		_ = f.Close()
	}
}

func TestExportArchiveNotificationFileSwappedForSymlinkIsNotFollowed(t *testing.T) {
	t.Parallel()
	secret := filepath.Join(t.TempDir(), "secret.yaml")
	require.NoError(t, os.WriteFile(secret, []byte("type: http\nurl: "+planted+"swap\n"), 0o600))
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"notifications/http.yaml": plantedHTTP, "notifications/z.yaml": "type: dummy\nname: z\n"})
	var buf bytes.Buffer
	_, err := writeExportArchive(context.Background(), dir, &buf, MaxImportUncompressedBytes, exportHooks{
		beforeOpen: func(path string) {
			if filepath.Base(path) == "http.yaml" {
				require.NoError(t, os.Remove(path))
				require.NoError(t, os.Symlink(secret, path))
			}
		},
	})
	require.NoError(t, err)
	requireNoToken(t, buf.Bytes())
	require.Equal(t, []string{"notifications/z.yaml"}, exportNames(t, buf.Bytes()))
}

func TestExportArchiveNotificationFileSwappedForAnotherFileFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{"notifications/http.yaml": plantedHTTP})
	_, err := writeExportArchive(context.Background(), dir, io.Discard, MaxImportUncompressedBytes, exportHooks{
		beforeOpen: func(path string) {
			replacement := path + ".new"
			require.NoError(t, os.WriteFile(replacement, []byte(plantedHTTP), 0o600))
			require.NoError(t, os.Rename(replacement, path))
		},
	})
	require.ErrorIs(t, err, ErrExportFileUnreadable)
}

func TestLiveNotificationDirsFallsBackToDefaults(t *testing.T) {
	t.Parallel()
	defaults := NotificationDirs(t.TempDir(), nil)

	missing := t.TempDir()
	require.Equal(t, defaults, liveNotificationDirs(missing))

	oversize := t.TempDir()
	writeExportTree(t, oversize, map[string]string{"config/config.yaml": strings.Repeat("#", int(maxInspectEntryBytes)+1)})
	require.Equal(t, defaults, liveNotificationDirs(oversize))

	flat := t.TempDir()
	writeExportTree(t, flat, map[string]string{"config.yaml": "config_paths:\n  notification_dir: " + filepath.Join(flat, "alerts") + "\n"})
	require.Equal(t, append(append([]string(nil), defaults...), "alerts"), liveNotificationDirs(flat))
}

// recordingHook keeps the messages of every log entry while enabled.
type recordingHook struct {
	mu       sync.Mutex
	messages []string
}

func (h *recordingHook) Levels() []logrus.Level { return logrus.AllLevels }

func (h *recordingHook) Fire(e *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	line := e.Message
	for _, v := range e.Data {
		if s, ok := v.(error); ok {
			line += " " + s.Error()
		}
	}
	h.messages = append(h.messages, line)
	return nil
}

// Not parallel: it switches the shared logger to debug level to see every message.
func TestExportArchiveLogsHoldNoNotificationContent(t *testing.T) {
	hook := &recordingHook{}
	logger.Log().Logger.AddHook(hook)
	previous := logger.Log().Logger.GetLevel()
	logger.Log().Logger.SetLevel(logrus.DebugLevel)
	t.Cleanup(func() { logger.Log().Logger.SetLevel(previous) })

	dir := t.TempDir()
	writeExportTree(t, dir, map[string]string{
		"notifications/http.yaml": plantedHTTP,
		"notifications/dup.yaml":  "type: http\ntype: http\nurl: " + planted + "dup\n",
		"notifications/stray.bak": planted + "stray",
	})
	_, _, _ = exportEntries(t, dir)

	hook.mu.Lock()
	defer hook.mu.Unlock()
	var sawStub bool
	for _, m := range hook.messages {
		if strings.Contains(m, "dup.yaml") {
			sawStub = true
		}
		require.NotContains(t, m, planted)
	}
	require.True(t, sawStub)
}
