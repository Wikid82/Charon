package services_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/stretchr/testify/require"
)

// These characterisation tests pin how the whitelist YAML file is written (content, mode, order, the empty list and
// the legacy temp file). They were written before the whitelist writer rewrite (#1718) and must stay green across it,
// so the diff of that rewrite shows only what changes. They deliberately avoid the error text of the write steps.

// whitelistDir is the directory CrowdSec loads the Charon whitelist parser from.
func whitelistDir(dataDir string) string {
	return filepath.Join(dataDir, "config", "parsers", "s02-enrich")
}

// wantWhitelistYAML is the exact file content for the given IP and CIDR lists, in the order they are passed.
func wantWhitelistYAML(ips, cidrs []string) string {
	s := "name: charon-whitelist\n" +
		"description: \"Charon-managed IP/CIDR whitelist\"\n" +
		"filter: \"evt.Meta.service == 'http'\"\n" +
		"whitelist:\n" +
		"  reason: \"Charon managed whitelist\"\n"
	s += "  ip:"
	if len(ips) == 0 {
		s += " []\n"
	} else {
		s += "\n"
		for _, ip := range ips {
			s += "    - \"" + ip + "\"\n"
		}
	}
	s += "  cidr:"
	if len(cidrs) == 0 {
		s += " []\n"
	} else {
		s += "\n"
		for _, c := range cidrs {
			s += "    - \"" + c + "\"\n"
		}
	}
	return s
}

func readWhitelistFile(t *testing.T, dataDir string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(whitelistDir(dataDir), "charon-whitelist.yaml")) //nolint:gosec // G304: path built from t.TempDir
	require.NoError(t, err)
	return string(content)
}

func TestWhitelistCharacterisationContentAfterAddAndDelete(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), dataDir)
	ctx := context.Background()

	first, err := svc.Add(ctx, "1.1.1.1", "dns")
	require.NoError(t, err)
	_, err = svc.Add(ctx, "10.0.0.0/8", "internal")
	require.NoError(t, err)
	_, err = svc.Add(ctx, "172.16.9.9/12", "normalised to the network address")
	require.NoError(t, err)
	_, err = svc.Add(ctx, "2.2.2.2", "second ip")
	require.NoError(t, err)

	// IPs and CIDRs are split into their own lists; each list keeps creation order.
	require.Equal(t,
		wantWhitelistYAML([]string{"1.1.1.1", "2.2.2.2"}, []string{"10.0.0.0/8", "172.16.0.0/12"}),
		readWhitelistFile(t, dataDir))

	require.NoError(t, svc.Delete(ctx, first.UUID))
	require.Equal(t,
		wantWhitelistYAML([]string{"2.2.2.2"}, []string{"10.0.0.0/8", "172.16.0.0/12"}),
		readWhitelistFile(t, dataDir))
}

func TestWhitelistCharacterisationDuplicateAddLeavesFileUnchanged(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), dataDir)
	ctx := context.Background()
	_, err := svc.Add(ctx, "3.3.3.3", "")
	require.NoError(t, err)
	before := readWhitelistFile(t, dataDir)

	_, err = svc.Add(ctx, "3.3.3.3", "again")
	require.ErrorIs(t, err, services.ErrDuplicateEntry)

	require.Equal(t, before, readWhitelistFile(t, dataDir))
}

// wantWhitelistPlaceholder is the exact file content for an empty allow list. It matches no event (expression false)
// and carries no ip or cidr list, which CrowdSec would reject as an empty node.
const wantWhitelistPlaceholder = "name: charon-whitelist\n" +
	"description: \"Charon-managed IP/CIDR whitelist\"\n" +
	"filter: \"evt.Meta.service == 'http'\"\n" +
	"whitelist:\n" +
	"  reason: \"Charon managed whitelist (no entries)\"\n" +
	"  expression:\n" +
	"    - \"false\"\n"

func TestWhitelistCharacterisationEmptyListWritesPlaceholderAndKeepsDirectory(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), dataDir)
	ctx := context.Background()
	only, err := svc.Add(ctx, "4.4.4.4", "")
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, only.UUID))

	require.Equal(t, wantWhitelistPlaceholder, readWhitelistFile(t, dataDir), "an empty list keeps the file as the placeholder")
	require.DirExists(t, whitelistDir(dataDir))

	// Adding again writes the file back with the single entry.
	_, err = svc.Add(ctx, "4.4.4.4", "")
	require.NoError(t, err)
	require.Equal(t, wantWhitelistYAML([]string{"4.4.4.4"}, nil), readWhitelistFile(t, dataDir))
}

func TestWhitelistCharacterisationFileModeIs0640(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), dataDir)
	_, err := svc.Add(context.Background(), "5.5.5.5", "")
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(whitelistDir(dataDir), "charon-whitelist.yaml"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o640), info.Mode().Perm())
}

func TestWhitelistCharacterisationWriteLeavesOnlyTheYAMLFile(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), dataDir)
	_, err := svc.Add(context.Background(), "6.6.6.6", "")
	require.NoError(t, err)

	entries, err := os.ReadDir(whitelistDir(dataDir))
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	require.Equal(t, []string{"charon-whitelist.yaml"}, names, "a successful write leaves no temp file behind")
}

// TestWhitelistCharacterisationStaleLegacyTempIsRemoved pins that a leftover regular file at the fixed legacy temp
// name (from an older Charon) does not block the next write, and is removed by it. Before #1718 the write overwrote
// that file and renamed it; the new writer uses its own temp name, so it removes the legacy file instead.
func TestWhitelistCharacterisationStaleLegacyTempIsRemoved(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	dir := whitelistDir(dataDir)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "charon-whitelist.yaml.tmp"), []byte("stale\n"), 0o600))
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), dataDir)

	_, err := svc.Add(context.Background(), "7.7.7.7", "")
	require.NoError(t, err)

	require.Equal(t, wantWhitelistYAML([]string{"7.7.7.7"}, nil), readWhitelistFile(t, dataDir))
	require.NoFileExists(t, filepath.Join(dir, "charon-whitelist.yaml.tmp"))
}
