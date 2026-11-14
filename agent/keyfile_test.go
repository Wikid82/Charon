package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sentinelContent = "sentinel-never-logged-9d2c"

func writeKeyFile(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auth_key")
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}

func TestReadAuthKeyFileReturnsTrimmedKey(t *testing.T) {
	path := writeKeyFile(t, []byte("  \t"+sentinelContent+"\r\n\n"))

	key, err := readAuthKeyFile(path)

	require.NoError(t, err)
	assert.Equal(t, sentinelContent, key)
}

func TestReadAuthKeyFileMissing(t *testing.T) {
	_, err := readAuthKeyFile(filepath.Join(t.TempDir(), "missing"))

	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestReadAuthKeyFileRelativePathRejected(t *testing.T) {
	_, err := readAuthKeyFile("auth_key")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be an absolute path")
}

func TestReadAuthKeyFileDirectoryRejected(t *testing.T) {
	_, err := readAuthKeyFile(t.TempDir())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a regular file")
}

func TestReadAuthKeyFileOversizeRejected(t *testing.T) {
	path := writeKeyFile(t, []byte(strings.Repeat("A", maxAuthKeyFileBytes+1)))

	_, err := readAuthKeyFile(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "larger than")
}

func TestReadAuthKeyFileAtSizeLimitAccepted(t *testing.T) {
	path := writeKeyFile(t, []byte(strings.Repeat("A", maxAuthKeyFileBytes)))

	key, err := readAuthKeyFile(path)

	require.NoError(t, err)
	assert.Len(t, key, maxAuthKeyFileBytes)
}

func TestReadAuthKeyFileEmptyReportsNotSetYet(t *testing.T) {
	path := writeKeyFile(t, []byte(" \n\t\n"))

	_, err := readAuthKeyFile(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "is empty: the agent key is not set yet")
}

func TestReadAuthKeyFileErrorsNeverContainKeyContent(t *testing.T) {
	cases := map[string][]byte{
		"oversize": append([]byte(sentinelContent), []byte(strings.Repeat("B", maxAuthKeyFileBytes))...),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeKeyFile(t, content)

			_, err := readAuthKeyFile(path)

			require.Error(t, err)
			assert.NotContains(t, err.Error(), sentinelContent)
		})
	}

	dirPath := t.TempDir()
	_, err := readAuthKeyFile(dirPath)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), sentinelContent)
}
