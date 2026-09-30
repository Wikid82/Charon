package dbmaint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAvailableBytes(t *testing.T) {
	n, err := AvailableBytes(t.TempDir())
	require.NoError(t, err)
	assert.Greater(t, n, int64(0))
}

func TestAvailableBytes_MissingPath(t *testing.T) {
	_, err := AvailableBytes("/definitely/not/there")
	require.Error(t, err)
}

func TestSameFilesystem(t *testing.T) {
	dir := t.TempDir()

	same, err := SameFilesystem(dir, dir)
	require.NoError(t, err)
	assert.True(t, same)

	_, err = SameFilesystem(dir, "/definitely/not/there")
	require.Error(t, err)
	_, err = SameFilesystem("/definitely/not/there", dir)
	require.Error(t, err)
}

func TestSameFilesystem_ProcIsDifferentDevice(t *testing.T) {
	same, err := SameFilesystem(t.TempDir(), "/proc")
	require.NoError(t, err)
	assert.False(t, same)
}
