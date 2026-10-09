package crowdsec

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBackupID(t *testing.T) {
	t.Parallel()
	require.Equal(t, "crowdsec.backup.20260101-000000.000000", BackupID("/app/data/crowdsec.backup.20260101-000000.000000"))
	require.Equal(t, "crowdsec.filebackup.20260101-000000.000000", BackupID("/data/crowdsec.filebackup.20260101-000000.000000/"))
	require.Equal(t, "acquis.yaml.backup.20260101-000000", BackupID("/etc/crowdsec/acquis.yaml.backup.20260101-000000"))
	require.Empty(t, BackupID(""))
}

func TestRedactPaths(t *testing.T) {
	t.Parallel()
	require.Equal(t, "open <path>: no such file", RedactPaths("open /app/data/x/y.yaml: no such file"))
	require.Equal(t, "mkdir <path>: denied", RedactPaths("mkdir /tmp/a-b/c: denied"))
	require.Equal(t, `fetch "<path>"`, RedactPaths(`fetch "/var/lib/x"`))
	require.Equal(t, "fetch https://hub.crowdsec.net/a/index.json failed", RedactPaths("fetch https://hub.crowdsec.net/a/index.json failed"))
	require.Equal(t, "plain message", RedactPaths("plain message"))
}

func TestExtractTarGzErrorsOmitAbsolutePaths(t *testing.T) {
	t.Parallel()
	// A file entry colliding with a later nested entry forces a mkdir failure.
	target := t.TempDir()
	svc := NewHubService(nil, nil, target)
	require.NoError(t, svc.extractTarGz(t.Context(), makeTarGz(t, map[string]string{"a": "file"}), target))
	err := svc.extractTarGz(t.Context(), makeTarGz(t, map[string]string{"a/b": "nested"}), target)
	require.Error(t, err)
	require.NotContains(t, err.Error(), target)
}
