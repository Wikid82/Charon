//go:build plugin_smoke

package services

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// TestPluginSmoke verifies that a freshly built powerdns.so loads through the real loader in a host process built
// with the same toolchain, flags and dependency versions. It is driven by `make plugin-powerdns-smoke`, which always
// rebuilds the plugin into a temporary directory and passes it in CHARON_SMOKE_PLUGIN_SO.
//
// The loader opens the verified copy in its cache folder, never the original path. A second load of the same bytes
// must also succeed on that copy. Both loads run in this one test because the Go plugin runtime refuses to open the
// same plugin package twice from different paths in one process.
func TestPluginSmoke(t *testing.T) {
	src := smokePluginPath(t)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read smoke plugin: %v", err)
	}

	pluginDir := t.TempDir()
	setPluginFolderMode(t, pluginDir)
	pluginPath := filepath.Join(pluginDir, "powerdns.so")
	if err := os.WriteFile(pluginPath, data, 0o600); err != nil {
		t.Fatalf("copy smoke plugin into plugin folder: %v", err)
	}
	cacheDir := t.TempDir()
	loader := NewPluginLoaderService(nil, pluginDir, cacheDir, nil)

	t.Cleanup(func() { dnsprovider.Global().Unregister("powerdns") })
	if err := loader.LoadPlugin(pluginPath); err != nil {
		t.Fatalf("LoadPlugin: %v", err)
	}
	provider, ok := dnsprovider.Global().Get("powerdns")
	if !ok {
		t.Fatal("powerdns provider not registered after load")
	}
	if got := provider.Type(); got != "powerdns" {
		t.Fatalf("Type() = %q, want powerdns", got)
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), ".so") {
		t.Fatalf("cache folder holds %d entries, want one verified copy", len(entries))
	}
	if got, _ := os.ReadFile(filepath.Join(cacheDir, entries[0].Name())); !bytes.Equal(got, data) {
		t.Fatal("verified copy bytes differ from the plugin")
	}

	// The provider stays registered after a load, so it is removed before the second load.
	dnsprovider.Global().Unregister("powerdns")
	if err := loader.LoadPlugin(pluginPath); err != nil {
		t.Fatalf("second LoadPlugin of the same bytes: %v", err)
	}
}

// smokePluginPath returns the freshly built plugin that the make target passes in CHARON_SMOKE_PLUGIN_SO.
func smokePluginPath(t *testing.T) string {
	t.Helper()
	soPath := os.Getenv("CHARON_SMOKE_PLUGIN_SO")
	if soPath == "" {
		t.Fatal("CHARON_SMOKE_PLUGIN_SO must point at a freshly built plugin (use make plugin-powerdns-smoke)")
	}

	abs, err := filepath.Abs(filepath.Clean(soPath))
	if err != nil {
		t.Fatalf("resolve plugin path: %v", err)
	}
	if strings.Contains(filepath.ToSlash(abs), "/plugins/powerdns/") {
		t.Fatalf("refusing to load a plugin from the source directory (stale build risk): %s", abs)
	}
	return abs
}
