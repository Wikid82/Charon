//go:build plugin_smoke

package services

import (
	"os"
	"path/filepath"
	"plugin"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// TestPluginSmoke verifies that a freshly built powerdns.so can be opened by a
// host process built with the same toolchain, flags and dependency versions.
// It is driven by `make plugin-powerdns-smoke`, which always rebuilds the
// plugin into a temporary directory and passes it in CHARON_SMOKE_PLUGIN_SO.
func TestPluginSmoke(t *testing.T) {
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

	p, err := plugin.Open(abs)
	if err != nil {
		t.Fatalf("plugin.Open: %v", err)
	}
	sym, err := p.Lookup("Plugin")
	if err != nil {
		t.Fatalf("lookup Plugin symbol: %v", err)
	}

	var provider dnsprovider.ProviderPlugin
	switch v := sym.(type) {
	case dnsprovider.ProviderPlugin:
		provider = v
	case *dnsprovider.ProviderPlugin:
		provider = *v
	default:
		t.Fatalf("Plugin symbol has unexpected type %T", sym)
	}

	if got := provider.Type(); got != "powerdns" {
		t.Fatalf("Type() = %q, want powerdns", got)
	}
}
