package crowdsec

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const testMirrorTemplate = "https://raw.githubusercontent.com/crowdsecurity/hub/%s/%s"

func writeCSCLIConfig(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

// dirEntryNames lists the file names in dir, sorted by the OS.
func dirEntryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestIsHubNetworkOp(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"hub update":                  true,
		"hub upgrade":                 true,
		"collections install x/y":     true,
		"parsers install x/y":         true,
		"scenarios upgrade x/y":       true,
		"postoverflows install x/y":   true,
		"collections inspect x/y":     false,
		"collections list":            false,
		"hub list":                    false,
		"version":                     false,
		"hub":                         false,
		"install":                     false,
		"":                            false,
		"collections remove x/y":      false,
		"hub install x":               false,
		"collections install-old x/y": false,
	}
	for line, want := range cases {
		args := strings.Fields(line)
		require.Equal(t, want, isHubNetworkOp(args), line)
	}
	require.False(t, isHubNetworkOp(nil))
}

func TestIsRetryableHubFailure(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"failed http request for https://cdn-hub.crowdsec.net: dial tcp 1.2.3.4:443: connection refused": true,
		"lookup cdn-hub.crowdsec.net: no such host":                                                      true,
		"context deadline exceeded":                                                                      true,
		"Get \"https://x\": Client.Timeout exceeded while awaiting headers":                              true,
		"net/http: TLS handshake timeout":                                                                true,
		"item not found":                                                                                 false,
		"item is tainted":                                                                                false,
		"":                                                                                               false,
	}
	for text, want := range cases {
		require.Equal(t, want, isRetryableHubFailure(text), text)
	}
}

func TestMirrorMemoryWindow(t *testing.T) {
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := func() time.Time { return clock }
	m := NewMirrorMemory(10*time.Minute, now)

	require.False(t, m.Active())
	m.Remember()
	require.True(t, m.Active())

	// A later mirror win inside the window does not extend it.
	clock = clock.Add(5 * time.Minute)
	m.Remember()
	require.True(t, m.Active())

	clock = clock.Add(5 * time.Minute)
	require.False(t, m.Active(), "window ends ttl after the first win")

	// A win after the window closed opens a new one.
	m.Remember()
	require.True(t, m.Active())

	// The live config answering again closes the window.
	m.Forget()
	require.False(t, m.Active())
}

func TestMirrorMemoryNilAndZeroValues(t *testing.T) {
	t.Parallel()
	var nilMemory *MirrorMemory
	require.False(t, nilMemory.Active())
	nilMemory.Remember()
	nilMemory.Forget()

	var zero MirrorMemory
	require.False(t, zero.Active())
	zero.Remember()
	zero.Forget()
}

func TestMirrorHubTemplate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		base     string
		want     string
		operator bool
	}{
		{name: "unset uses the built-in mirror", base: "", want: testMirrorTemplate, operator: false},
		{name: "built-in base", base: defaultHubMirrorBaseURL, want: testMirrorTemplate, operator: true},
		{name: "operator master with trailing slash", base: defaultHubMirrorBaseURL + "/", want: testMirrorTemplate, operator: true},
		{
			name:     "operator fork on the allowlisted mirror host",
			base:     "https://raw.githubusercontent.com/example/hub/master",
			want:     "https://raw.githubusercontent.com/example/hub/%s/%s",
			operator: true,
		},
		{name: "operator other branch is refused", base: "https://raw.githubusercontent.com/crowdsecurity/hub/dev", want: testMirrorTemplate},
		{name: "operator plain http is refused", base: "http://raw.githubusercontent.com/crowdsecurity/hub/master", want: testMirrorTemplate},
		{name: "operator host outside the allowlist is refused", base: "https://example.com/hub/master", want: testMirrorTemplate},
		{name: "operator percent escape would add a verb", base: "https://raw.githubusercontent.com/x%41/hub/master", want: testMirrorTemplate},
		{name: "unparseable operator value is refused", base: "https://[::1", want: testMirrorTemplate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, operator := mirrorHubTemplate(tc.base)
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.operator, operator)
		})
	}
}

func TestValidMirrorTemplate(t *testing.T) {
	t.Parallel()
	require.True(t, validMirrorTemplate(testMirrorTemplate))
	for _, bad := range []string{
		"",
		"https://x/%s",
		"https://x/%s/%s/%s",
		"https://x/%s/%s%",
		"https://x/%41/%s/%s",
		"http://x/%s/%s",
		"https://x/%s/%s\n__hub_url_template__: evil",
		"https://x/%s/%s\r",
		"https://x/%s/%s\x00",
	} {
		require.False(t, validMirrorTemplate(bad), "%q", bad)
	}
}

func TestMirrorCSCLIConfigInsertsTemplateAndKeepsOtherKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	live := writeCSCLIConfig(t, dir, "common:\n  log_level: info\ncscli:\n  output: human\n  hub_branch: dev\n")

	derived, cleanup, err := mirrorCSCLIConfig(live, testMirrorTemplate)
	require.NoError(t, err)
	require.Equal(t, dir, filepath.Dir(derived))
	require.True(t, strings.HasPrefix(filepath.Base(derived), mirrorConfigPrefix))
	require.True(t, strings.HasSuffix(derived, ".yaml"))

	info, err := os.Stat(derived)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	raw, err := os.ReadFile(derived)
	require.NoError(t, err)
	var got map[string]map[string]string
	require.NoError(t, yaml.Unmarshal(raw, &got))
	require.Equal(t, testMirrorTemplate, got["cscli"]["__hub_url_template__"])
	require.Equal(t, "dev", got["cscli"]["hub_branch"], "an existing branch is kept")
	require.Equal(t, "human", got["cscli"]["output"])
	require.Equal(t, "info", got["common"]["log_level"])

	// The live file is never touched.
	liveRaw, err := os.ReadFile(live)
	require.NoError(t, err)
	require.Contains(t, string(liveRaw), "hub_branch: dev")
	require.NotContains(t, string(liveRaw), "__hub_url_template__")

	cleanup()
	_, err = os.Stat(derived)
	require.True(t, errors.Is(err, os.ErrNotExist), "cleanup removes the derived file")
}

func TestMirrorCSCLIConfigSetsBranchWhenMissingOrEmpty(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"section missing":        "common:\n  log_level: info\n",
		"section empty":          "cscli:\n",
		"branch empty":           "cscli:\n  hub_branch: \"\"\n",
		"branch missing":         "cscli:\n  output: human\n",
		"empty file":             "",
		"comment only":           "# nothing configured yet\n",
		"section null tilde":     "cscli: ~\n",
		"branch null with value": "cscli:\n  hub_branch:\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			live := writeCSCLIConfig(t, dir, content)
			derived, cleanup, err := mirrorCSCLIConfig(live, testMirrorTemplate)
			require.NoError(t, err)
			defer cleanup()

			raw, err := os.ReadFile(derived)
			require.NoError(t, err)
			var got map[string]map[string]string
			require.NoError(t, yaml.Unmarshal(raw, &got))
			require.Equal(t, "master", got["cscli"]["hub_branch"])
			require.Equal(t, testMirrorTemplate, got["cscli"]["__hub_url_template__"])
		})
	}
}

func TestMirrorCSCLIConfigQuotesTemplateValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	live := writeCSCLIConfig(t, dir, "cscli:\n  hub_branch: master\n")
	derived, cleanup, err := mirrorCSCLIConfig(live, testMirrorTemplate)
	require.NoError(t, err)
	defer cleanup()

	raw, err := os.ReadFile(derived)
	require.NoError(t, err)
	require.Contains(t, string(raw), `__hub_url_template__: "https://raw.githubusercontent.com/crowdsecurity/hub/%s/%s"`)
}

func TestMirrorCSCLIConfigRefusesOperatorTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	live := writeCSCLIConfig(t, dir, "cscli:\n  __hub_url_template__: https://private.example/%s/%s\n")

	derived, cleanup, err := mirrorCSCLIConfig(live, testMirrorTemplate)
	require.ErrorIs(t, err, errMirrorTemplateSet)
	require.Empty(t, derived)
	require.NotNil(t, cleanup)
	cleanup()
	require.Equal(t, []string{"config.yaml"}, dirEntryNames(t, dir), "no derived file is left behind")
}

func TestMirrorCSCLIConfigAllowsEmptyOperatorTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	live := writeCSCLIConfig(t, dir, "cscli:\n  __hub_url_template__:\n")
	derived, cleanup, err := mirrorCSCLIConfig(live, testMirrorTemplate)
	require.NoError(t, err)
	defer cleanup()

	raw, err := os.ReadFile(derived)
	require.NoError(t, err)
	require.Contains(t, string(raw), testMirrorTemplate)
}

func TestMirrorCSCLIConfigRefusesInvalidTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	live := writeCSCLIConfig(t, dir, "cscli:\n  hub_branch: master\n")
	for _, bad := range []string{"https://x/%s", "https://x/%s/%s\nevil: true", "http://x/%s/%s"} {
		derived, cleanup, err := mirrorCSCLIConfig(live, bad)
		require.ErrorIs(t, err, errMirrorTemplateInvalid, "%q", bad)
		require.Empty(t, derived)
		cleanup()
	}
	require.Equal(t, []string{"config.yaml"}, dirEntryNames(t, dir))
}

func TestMirrorCSCLIConfigRejectsUnusableInput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := map[string]string{
		"malformed yaml":      "cscli: [\n",
		"root is a sequence":  "- a\n- b\n",
		"cscli is a scalar":   "cscli: 5\n",
		"cscli is a sequence": "cscli: [1]\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			sub := t.TempDir()
			live := writeCSCLIConfig(t, sub, content)
			derived, cleanup, err := mirrorCSCLIConfig(live, testMirrorTemplate)
			require.Error(t, err)
			require.Empty(t, derived)
			cleanup()
			require.Equal(t, []string{"config.yaml"}, dirEntryNames(t, sub))
		})
	}

	t.Run("missing config", func(t *testing.T) {
		t.Parallel()
		_, cleanup, err := mirrorCSCLIConfig(filepath.Join(dir, "absent.yaml"), testMirrorTemplate)
		require.Error(t, err)
		cleanup()
	})
	t.Run("empty path", func(t *testing.T) {
		t.Parallel()
		_, cleanup, err := mirrorCSCLIConfig("", testMirrorTemplate)
		require.Error(t, err)
		cleanup()
	})
}

func TestMirrorCSCLIConfigCopiesLocalOverride(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	live := writeCSCLIConfig(t, dir, "cscli:\n  hub_branch: master\n")
	const local = "cscli:\n  hub_dir: /custom/hub\n"
	require.NoError(t, os.WriteFile(live+".local", []byte(local), 0o600))

	derived, cleanup, err := mirrorCSCLIConfig(live, testMirrorTemplate)
	require.NoError(t, err)

	copied, err := os.ReadFile(derived + ".local")
	require.NoError(t, err)
	require.Equal(t, local, string(copied))
	info, err := os.Stat(derived + ".local")
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	cleanup()
	for _, p := range []string{derived, derived + ".local"} {
		_, err := os.Stat(p)
		require.True(t, errors.Is(err, os.ErrNotExist), p)
	}
	require.Equal(t, []string{"config.yaml", "config.yaml.local"}, dirEntryNames(t, dir))
}

func TestMirrorCSCLIConfigFailsClosedWhenLocalUnreadable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	live := writeCSCLIConfig(t, dir, "cscli:\n  hub_branch: master\n")
	// A directory where the override file should be cannot be read, so no retry may run without it.
	require.NoError(t, os.Mkdir(live+".local", 0o750))

	derived, cleanup, err := mirrorCSCLIConfig(live, testMirrorTemplate)
	require.Error(t, err)
	require.Empty(t, derived)
	cleanup()
	require.Equal(t, []string{"config.yaml", "config.yaml.local"}, dirEntryNames(t, dir))
}

func TestMirrorCSCLIConfigFailsWhenTempFileCannotBeCreated(t *testing.T) {
	dir := t.TempDir()
	live := writeCSCLIConfig(t, dir, "cscli:\n  hub_branch: master\n")
	original := createMirrorTemp
	createMirrorTemp = func(string, string) (*os.File, error) { return nil, errors.New("disk full") }
	t.Cleanup(func() { createMirrorTemp = original })

	derived, cleanup, err := mirrorCSCLIConfig(live, testMirrorTemplate)
	require.ErrorContains(t, err, "disk full")
	require.Empty(t, derived)
	cleanup()
}

func TestSweepMirrorLeftovers(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	configDir := filepath.Join(dataDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o750))
	leftovers := []string{
		filepath.Join(configDir, mirrorConfigPrefix+"a1.yaml"),
		filepath.Join(configDir, mirrorConfigPrefix+"a1.yaml.local"),
		filepath.Join(dataDir, mirrorConfigPrefix+"b2.yaml"),
		filepath.Join(dataDir, mirrorConfigPrefix+"b2.yaml.local"),
	}
	keep := []string{
		filepath.Join(configDir, "config.yaml"),
		filepath.Join(configDir, "config.yaml.local"),
		filepath.Join(dataDir, ".charon-hub-other.yaml"),
	}
	for _, p := range append(append([]string{}, leftovers...), keep...) {
		require.NoError(t, os.WriteFile(p, []byte("x"), 0o600))
	}

	require.NoError(t, sweepMirrorLeftovers(dataDir))
	for _, p := range leftovers {
		_, err := os.Stat(p)
		require.True(t, errors.Is(err, os.ErrNotExist), "leftover kept: %s", p)
	}
	for _, p := range keep {
		_, err := os.Stat(p)
		require.NoError(t, err, "unrelated file removed: %s", p)
	}
}

func TestSweepMirrorLeftoversEdgeCases(t *testing.T) {
	t.Parallel()
	t.Run("empty data dir is a no-op", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, sweepMirrorLeftovers(""))
	})
	t.Run("missing data dir is a no-op", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, sweepMirrorLeftovers(filepath.Join(t.TempDir(), "absent")))
	})
	t.Run("a leftover that cannot be removed is reported", func(t *testing.T) {
		t.Parallel()
		dataDir := t.TempDir()
		stuck := filepath.Join(dataDir, mirrorConfigPrefix+"c3.yaml")
		require.NoError(t, os.Mkdir(stuck, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(stuck, "inner"), []byte("x"), 0o600))
		require.Error(t, sweepMirrorLeftovers(dataDir))
	})
}

func TestMirrorCSCLIConfigFailsWhenTempFileIsUnusable(t *testing.T) {
	dir := t.TempDir()
	live := writeCSCLIConfig(t, dir, "cscli:\n  hub_branch: master\n")
	original := createMirrorTemp
	createMirrorTemp = func(d, pattern string) (*os.File, error) {
		f, err := os.CreateTemp(d, pattern)
		if err != nil {
			return nil, err
		}
		closeErr := f.Close()
		return f, closeErr
	}
	t.Cleanup(func() { createMirrorTemp = original })

	derived, cleanup, err := mirrorCSCLIConfig(live, testMirrorTemplate)
	require.ErrorContains(t, err, "write hub mirror config")
	require.Empty(t, derived)
	cleanup()
	require.Equal(t, []string{"config.yaml"}, dirEntryNames(t, dir), "the unusable temp file is removed")
}

func TestWriteMirrorFileReportsClosedFile(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp(t.TempDir(), "x*.yaml")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	require.ErrorContains(t, writeMirrorFile(f, []byte("x")), "write hub mirror config")
}

func TestCopyLocalOverrideRefusesExistingTarget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "config.yaml.local")
	dst := filepath.Join(dir, "derived.yaml.local")
	require.NoError(t, os.WriteFile(src, []byte("new"), 0o600))
	require.NoError(t, os.WriteFile(dst, []byte("keep"), 0o600))

	require.ErrorContains(t, copyLocalOverride(src, dst), "create hub mirror local override")
	kept, err := os.ReadFile(dst)
	require.NoError(t, err)
	require.Equal(t, "keep", string(kept), "an existing target is never overwritten")
}

func TestCopyLocalOverrideMissingSourceIsNoop(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dst := filepath.Join(dir, "derived.yaml.local")
	require.NoError(t, copyLocalOverride(filepath.Join(dir, "absent.local"), dst))
	_, err := os.Stat(dst)
	require.True(t, errors.Is(err, os.ErrNotExist))
}

func TestIsMirrorConfigName(t *testing.T) {
	t.Parallel()
	require.True(t, isMirrorConfigName(".charon-hub-mirror-a.yaml"))
	require.True(t, isMirrorConfigName(".charon-hub-mirror-a.yaml.local"))
	require.False(t, isMirrorConfigName(".charon-hub-mirror-a.yml"))
	require.False(t, isMirrorConfigName("config.yaml"))
}

func TestSweepMirrorLeftoversUnusableDataDir(t *testing.T) {
	t.Parallel()
	notADir := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(notADir, []byte("x"), 0o600))
	require.Error(t, sweepMirrorLeftovers(notADir))
}
