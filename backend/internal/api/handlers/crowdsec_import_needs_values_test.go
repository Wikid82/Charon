package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/logger"
)

const (
	nvToken = "SECRETTOKEN-live"
	nvPlain = "type: http\nname: plain\nurl: https://hooks.example.invalid/" + nvToken + "\nmethod: POST\nheaders:\n  Authorization: Bearer " + nvToken + "\n"
	nvEmail = "type: email\nname: mail\nsmtp_host: smtp.example.invalid\nsmtp_username: " + nvToken + "\nsmtp_password: " + nvToken + "\nreceiver_emails:\n  - ops@example.invalid\n"
	// nvAliased reuses a kept value through an anchor, so the exported file still holds an alias.
	nvAliased = "type: slack\nname: shared\nwebhook: https://hooks.example.invalid/" + nvToken + "\nchannel: &c ops\nusername: *c\n"
)

func nvProfiles(names ...string) string {
	out := "name: p\nfilters:\n - Alert.Remediation == true\ndecisions:\n - type: ban\n   duration: 4h\nnotifications:\n"
	for _, n := range names {
		out += " - " + n + "\n"
	}
	return out
}

func archiveBodies(t *testing.T, data []byte) map[string]string {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	tr := tar.NewReader(gr)
	bodies := map[string]string{}
	for {
		hdr, nextErr := tr.Next()
		if errors.Is(nextErr, io.EOF) {
			return bodies
		}
		require.NoError(t, nextErr)
		body, readErr := io.ReadAll(tr)
		require.NoError(t, readErr)
		bodies[filepath.ToSlash(hdr.Name)] = string(body)
	}
}

func needsValuesOf(t *testing.T, w *httptest.ResponseRecorder) (names []string, present bool) {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	raw, present := body["needs_values"]
	if present {
		require.NoError(t, json.Unmarshal(raw, &names))
	}
	return names, present
}

func parsedYAML(t *testing.T, text string) []any {
	t.Helper()
	dec := yaml.NewDecoder(strings.NewReader(text))
	var docs []any
	for {
		var v any
		if err := dec.Decode(&v); err != nil {
			require.ErrorIs(t, err, io.EOF)
			return docs
		}
		docs = append(docs, v)
	}
}

func TestImportConfigRoundTripKeepsCurrentNotificationValues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	writeTree(t, dir, map[string]string{
		"config/profiles.yaml":             nvProfiles("plain", "mail", "shared"),
		"config/notifications/plain.yaml":  nvPlain,
		"config/notifications/mail.yaml":   nvEmail,
		"config/notifications/shared.yaml": nvAliased,
	})
	_, r := newImportRouter(t, dir)
	exported := exportBytes(t, r)
	entries := archiveBodies(t, exported)
	require.NotContains(t, entries["config/notifications/plain.yaml"], nvToken)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, importBytesRequest(t, exported))

	names, present := needsValuesOf(t, w)
	require.True(t, present)
	require.Equal(t, []string{"notifications/shared.yaml"}, names)
	require.Equal(t, parsedYAML(t, nvPlain), parsedYAML(t, readTreeFile(t, filepath.Join(dir, "config", "notifications", "plain.yaml"))))
	require.Equal(t, parsedYAML(t, nvEmail), parsedYAML(t, readTreeFile(t, filepath.Join(dir, "config", "notifications", "mail.yaml"))))
	require.Equal(t, entries["config/notifications/shared.yaml"], readTreeFile(t, filepath.Join(dir, "config", "notifications", "shared.yaml")),
		"a file with shared structure is left exactly as extracted")
}

func TestImportConfigOmitsNeedsValuesWhenNothingIsBlanked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	writeTree(t, dir, map[string]string{"config/notifications/plain.yaml": nvPlain})
	_, r := newImportRouter(t, dir)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, importBytesRequest(t, exportBytes(t, r)))

	_, present := needsValuesOf(t, w)
	require.False(t, present)
}

func TestImportConfigIntoAHostWithoutTheFileReportsIt(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	seedRealLayout(t, source)
	writeTree(t, source, map[string]string{
		"config/profiles.yaml":            nvProfiles("plain"),
		"config/notifications/plain.yaml": nvPlain,
	})
	_, sourceRouter := newImportRouter(t, source)
	exported := exportBytes(t, sourceRouter)

	target := t.TempDir()
	seedRealLayout(t, target)
	_, targetRouter := newImportRouter(t, target)
	w := httptest.NewRecorder()
	targetRouter.ServeHTTP(w, importBytesRequest(t, exported))

	names, _ := needsValuesOf(t, w)
	require.Equal(t, []string{"notifications/plain.yaml"}, names)
	got := readTreeFile(t, filepath.Join(target, "config", "notifications", "plain.yaml"))
	require.Contains(t, got, crowdsec.RedactedPlaceholder)
	require.NotContains(t, got, nvToken)
}

func TestImportConfigReportsRestoredFilesThatAreStillBlanked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	blanked := "type: http\nname: alpha\nurl: " + crowdsec.RedactedPlaceholder + "\n"
	writeTree(t, dir, map[string]string{
		"config/profiles.yaml":            nvProfiles("alpha"),
		"config/notifications/alpha.yaml": blanked,
	})
	_, r := newImportRouter(t, dir)

	w := importStatus(t, r, map[string]string{"config/config.yaml": importableConfig})

	names, _ := needsValuesOf(t, w)
	require.Equal(t, []string{"notifications/alpha.yaml"}, names)
	require.Contains(t, keptFilesOf(t, w.Body.Bytes()), "notifications/alpha.yaml")
	require.Equal(t, blanked, readTreeFile(t, filepath.Join(dir, "config", "notifications", "alpha.yaml")))
}

func TestImportConfigKeepsCurrentValuesWhenAProfileNamesTheBlankedNotification(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	writeTree(t, dir, map[string]string{
		"config/profiles.yaml":            nvProfiles("alpha"),
		"config/notifications/alpha.yaml": strings.Replace(importNotification, "%s", "alpha", 1),
	})
	_, r := newImportRouter(t, dir)
	blanked := "type: http\nname: alpha\nurl: https://example.invalid/\nmethod: POST\nheaders:\n  X-Key: " + crowdsec.RedactedPlaceholder + "\n"

	w := importStatus(t, r, map[string]string{
		"config/config.yaml":              importableConfig,
		"config/profiles.yaml":            nvProfiles("alpha"),
		"config/notifications/alpha.yaml": blanked,
	})

	_, present := needsValuesOf(t, w)
	require.True(t, present, "the live file has no X-Key header to fill from")
	require.Equal(t, blanked, readTreeFile(t, filepath.Join(dir, "config", "notifications", "alpha.yaml")))
}

func TestImportConfigRollsBackWhenKeepingCurrentValuesFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	writeTree(t, dir, map[string]string{"config/notifications/plain.yaml": nvPlain})
	h, r := newImportRouter(t, dir)
	h.mergeNotifications = func(string, string) error { return errors.New("disk problem with " + nvToken) }
	before := treeState(t, dir)

	w := importStatus(t, r, map[string]string{
		"config/config.yaml":              importableConfig,
		"config/notifications/plain.yaml": "type: http\nname: plain\nurl: " + crowdsec.RedactedPlaceholder + "\n",
	})

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, msgKeepCurrentFiles, importNotificationBody(t, w))
	requireSameTree(t, before, dir)
}

// Not parallel: it adds a hook to the shared logger to see every message.
func TestImportConfigKeepsTheImportWhenTheReportFails(t *testing.T) {
	hook := &messageHook{}
	logger.Log().Logger.AddHook(hook)

	dir := t.TempDir()
	seedRealLayout(t, dir)
	writeTree(t, dir, map[string]string{"config/notifications/plain.yaml": nvPlain})
	h, r := newImportRouter(t, dir)
	h.scanPlaceholders = func(string) ([]string, error) {
		return []string{"notifications/" + nvToken + ".yaml"}, errors.New("walk failed at " + nvToken)
	}
	archive := "type: http\nname: plain\nurl: " + crowdsec.RedactedPlaceholder + "\n"

	w := importStatus(t, r, map[string]string{
		"config/config.yaml":              importableConfig,
		"config/notifications/plain.yaml": archive,
	})

	_, present := needsValuesOf(t, w)
	require.False(t, present)
	require.Contains(t, readTreeFile(t, filepath.Join(dir, "config", "notifications", "plain.yaml")), nvToken, "the import itself stands")
	hook.mu.Lock()
	defer hook.mu.Unlock()
	var sawReport bool
	for _, m := range hook.messages {
		require.NotContains(t, m, nvToken)
		sawReport = sawReport || strings.Contains(m, "still need values")
	}
	require.True(t, sawReport)
}

type messageHook struct {
	mu       sync.Mutex
	messages []string
}

func (h *messageHook) Levels() []logrus.Level { return logrus.AllLevels }

func (h *messageHook) Fire(e *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	line := e.Message
	for k, v := range e.Data {
		line += " " + k + "=" + toString(v)
	}
	h.messages = append(h.messages, line)
	return nil
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case error:
		return x.Error()
	}
	b, _ := json.Marshal(v)
	return string(b)
}
