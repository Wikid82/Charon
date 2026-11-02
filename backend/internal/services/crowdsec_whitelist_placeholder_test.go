package services_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/services"
)

// whitelistParser is the subset of the whitelist parser the placeholder tests inspect.
type whitelistParser struct {
	Name      string `yaml:"name"`
	Filter    string `yaml:"filter"`
	Whitelist struct {
		Reason     string   `yaml:"reason"`
		IP         []string `yaml:"ip"`
		CIDR       []string `yaml:"cidr"`
		Expression []string `yaml:"expression"`
	} `yaml:"whitelist"`
}

func TestWhitelistPlaceholderNeverMatches(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	svc := services.NewCrowdSecWhitelistService(openWhitelistSerialDB(t), dataDir)
	require.NoError(t, svc.WriteYAML(context.Background()))

	var parsed whitelistParser
	require.NoError(t, yaml.Unmarshal([]byte(readWhitelistFile(t, dataDir)), &parsed))

	require.Equal(t, "charon-whitelist", parsed.Name)
	require.Equal(t, "evt.Meta.service == 'http'", parsed.Filter, "the placeholder keeps the real file's filter")
	require.Equal(t, []string{"false"}, parsed.Whitelist.Expression, "the only expression is never true, so no event is allowed")
	require.Empty(t, parsed.Whitelist.IP, "an empty ip list is what CrowdSec rejects as an empty node")
	require.Empty(t, parsed.Whitelist.CIDR)
}

// TestWhitelistFileStaysInPlaceAcrossAddDeleteCycles runs a stat loop beside add, delete and empty cycles. The file
// must never be missing (an engine can die when its whitelist file disappears between reloads), and a real
// entry must load again after each empty cycle.
func TestWhitelistFileStaysInPlaceAcrossAddDeleteCycles(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	svc := services.NewCrowdSecWhitelistService(openWhitelistSerialDB(t), dataDir)
	ctx := context.Background()
	target := filepath.Join(whitelistDir(dataDir), "charon-whitelist.yaml")
	require.NoError(t, svc.WriteYAML(ctx))

	var missing, probes atomic.Int64
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := os.Stat(target); errors.Is(err, fs.ErrNotExist) {
				missing.Add(1)
			}
			probes.Add(1)
		}
	}()

	const cycles = 40
	for i := range cycles {
		ip := fmt.Sprintf("10.20.%d.%d", i/250, i%250+1)
		entry, err := svc.Add(ctx, ip, "cycle")
		require.NoError(t, err)
		require.Equal(t, wantWhitelistYAML([]string{ip}, nil), readWhitelistFile(t, dataDir))

		require.NoError(t, svc.Delete(ctx, entry.UUID))
		require.Equal(t, wantWhitelistPlaceholder, readWhitelistFile(t, dataDir))
	}
	close(stop)
	wg.Wait()

	require.Zero(t, missing.Load(), "the whitelist file must never disappear while an engine may reload it")
	require.Positive(t, probes.Load())
}

func TestWhitelistScanIgnoresPlaceholderAndTempSibling(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	svc := services.NewCrowdSecWhitelistService(openWhitelistSerialDB(t), dataDir)
	ctx := context.Background()
	_, err := svc.Add(ctx, "172.20.0.5", "scan")
	require.NoError(t, err)
	require.Empty(t, crowdsec.FindOtherWhitelistFiles(dataDir), "the managed file is not another whitelist")

	require.NoError(t, svc.Delete(ctx, mustListFirstUUID(t, svc, ctx)))
	require.Empty(t, crowdsec.FindOtherWhitelistFiles(dataDir), "the placeholder is not another whitelist")

	// A temp sibling of the managed file carries a whitelist section, but its extension is not .yaml or .yml.
	temp := filepath.Join(whitelistDir(dataDir), "charon-whitelist.yaml.tmp-123456")
	require.NoError(t, os.WriteFile(temp, []byte("whitelist:\n  reason: in flight\n"), 0o600))
	require.Empty(t, crowdsec.FindOtherWhitelistFiles(dataDir), "an in-flight temp file is not reported")
}

func mustListFirstUUID(t *testing.T, svc *services.CrowdSecWhitelistService, ctx context.Context) string {
	t.Helper()
	entries, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	return entries[0].UUID
}
