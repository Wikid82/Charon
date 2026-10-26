package handlers

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// stubReadAcquisitionFile replaces the GET read seam for the duration of the test.
func stubReadAcquisitionFile(t *testing.T, fn func(string) ([]byte, error)) {
	t.Helper()
	orig := readAcquisitionFile
	t.Cleanup(func() { readAcquisitionFile = orig })
	readAcquisitionFile = fn
}

// stubStatAcquisitionConfig replaces the stat seam of replaceAcquisitionConfig for the duration of the test.
func stubStatAcquisitionConfig(t *testing.T, fn func(string) (fs.FileInfo, error)) {
	t.Helper()
	orig := statAcquisitionConfig
	t.Cleanup(func() { statAcquisitionConfig = orig })
	statAcquisitionConfig = fn
}

// The seam variants below always run, so the coded 500 is pinned on every platform, including as root.

func TestAcquisitionGetPermissionDeniedIsCoded500Seam(t *testing.T) {
	acquis := filepath.Join(t.TempDir(), "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("x\n"), 0o600))
	stubReadAcquisitionFile(t, func(string) ([]byte, error) {
		return nil, &fs.PathError{Op: "open", Path: acquis, Err: syscall.EACCES}
	})
	r := newAcquisitionCharacterisationRouter(t, acquis)

	requireCodedAcquisitionError(t, getAcquisitionCharacterisation(r), codeAcquisitionUnreadable, msgAcquisitionUnreadable)
}

func TestAcquisitionGetNonPermissionReadErrorStaysGeneric500(t *testing.T) {
	acquis := filepath.Join(t.TempDir(), "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("x\n"), 0o600))
	stubReadAcquisitionFile(t, func(string) ([]byte, error) {
		return nil, fmt.Errorf("read acquisition config: %w", syscall.EIO)
	})
	r := newAcquisitionCharacterisationRouter(t, acquis)

	requireGenericAcquisitionError(t, getAcquisitionCharacterisation(r), "failed to read acquisition config")
}

func TestAcquisitionPutStatPermissionDeniedIsCoded500Seam(t *testing.T) {
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))
	stubStatAcquisitionConfig(t, func(string) (fs.FileInfo, error) {
		return nil, &fs.PathError{Op: "stat", Path: acquis, Err: syscall.EACCES}
	})
	r := newAcquisitionCharacterisationRouter(t, acquis)

	requireCodedAcquisitionError(t, putAcquisition(r, "new\n"), codeAcquisitionUnreadable, msgAcquisitionUnreadable)
	got, err := os.ReadFile(acquis) //nolint:gosec // G304: t.TempDir-derived path
	require.NoError(t, err)
	require.Equal(t, "original\n", string(got))
	requireOnlyEntries(t, dir, "acquis.yaml")
}

// TestAcquisitionPutStatNotPermissionStaysGeneric500 pins that a stat failure that is not a permission problem keeps
// the generic 500: the ENOTDIR case from the write tests, here through the stat seam.
func TestAcquisitionPutStatNotPermissionStaysGeneric500(t *testing.T) {
	acquis := filepath.Join(t.TempDir(), "acquis.yaml")
	stubStatAcquisitionConfig(t, func(string) (fs.FileInfo, error) {
		return nil, &fs.PathError{Op: "stat", Path: acquis, Err: syscall.ENOTDIR}
	})
	r := newAcquisitionCharacterisationRouter(t, acquis)

	requireGenericAcquisitionError(t, putAcquisition(r, "new\n"), "failed to write acquisition config")
}

// TestAcquisitionPutDirectoryStaysGeneric500 pins that a non-permission read failure on the live file (a directory
// reads as EISDIR) keeps the generic 500 with no code.
func TestAcquisitionPutDirectoryStaysGeneric500(t *testing.T) {
	acquis := filepath.Join(t.TempDir(), "acquis.yaml")
	require.NoError(t, os.Mkdir(acquis, 0o700))
	r := newAcquisitionCharacterisationRouter(t, acquis)

	requireGenericAcquisitionError(t, putAcquisition(r, "new\n"), "failed to write acquisition config")
}

// Root-dependent variants: these bind only for a non-root user, so they skip as root and the seam variants above
// carry the coverage there.

func TestAcquisitionPutUnreadableOldFileIsCoded500(t *testing.T) {
	requireNotRoot(t)
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("secret\n"), 0o000)) //nolint:gosec // G306: deliberately unreadable file under test
	t.Cleanup(func() { _ = os.Chmod(acquis, 0o600) })                   //nolint:gosec // G302: restore file so TempDir cleanup works
	r := newAcquisitionCharacterisationRouter(t, acquis)

	requireCodedAcquisitionError(t, putAcquisition(r, "new\n"), codeAcquisitionUnreadable, msgAcquisitionUnreadable)
	requireOnlyEntries(t, dir, "acquis.yaml")
}

// TestAcquisitionPutFolderWithoutSearchIsCoded500 uses a real folder the Charon user cannot search: the stat of the
// file inside it fails with EACCES.
func TestAcquisitionPutFolderWithoutSearchIsCoded500(t *testing.T) {
	requireNotRoot(t)
	dir := t.TempDir()
	acquis := filepath.Join(dir, "acquis.yaml")
	require.NoError(t, os.WriteFile(acquis, []byte("original\n"), 0o600))
	require.NoError(t, os.Chmod(dir, 0o600))       //nolint:gosec // G302: a folder without search permission is the failure under test
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restore directory so TempDir cleanup works
	r := newAcquisitionCharacterisationRouter(t, acquis)

	requireCodedAcquisitionError(t, putAcquisition(r, "new\n"), codeAcquisitionUnreadable, msgAcquisitionUnreadable)
}

// TestAcquisitionWriteFailureClassification pins the classification order: a partial in-place write is never a
// permission problem, even when it wraps one; a read sentinel wins over a plain permission error.
func TestAcquisitionWriteFailureClassification(t *testing.T) {
	cases := map[string]struct {
		err  error
		code string
		msg  string
	}{
		"partial in-place write wrapping EACCES": {
			err: fmt.Errorf("write: %w: %w", crowdsec.ErrPartialInPlaceWrite, fs.ErrPermission),
			msg: msgAcquisitionWriteFailed,
		},
		"stat or read failure wrapping EACCES": {
			err:  fmt.Errorf("%w: stat acquisition config: %w", errAcquisitionRead, fs.ErrPermission),
			code: codeAcquisitionUnreadable,
			msg:  msgAcquisitionUnreadable,
		},
		"permission error on write": {
			err:  fmt.Errorf("create backup: %w", fs.ErrPermission),
			code: codeAcquisitionNotWritable,
			msg:  msgAcquisitionNotWritable,
		},
		"read-only mount on write": {
			err:  fmt.Errorf("replace acquisition config: %w", syscall.EROFS),
			code: codeAcquisitionNotWritable,
			msg:  msgAcquisitionNotWritable,
		},
		"other failure": {
			err: fmt.Errorf("replace acquisition config: %w", syscall.EXDEV),
			msg: msgAcquisitionWriteFailed,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			code, msg := acquisitionWriteFailure(tc.err)
			require.Equal(t, tc.code, code)
			require.Equal(t, tc.msg, msg)
		})
	}
}

// TestAcquisitionAccessErrorIsPermissionOnly pins that only permission and read-only errors are coded; a wrapped
// sentinel alone is not a permission problem.
func TestAcquisitionAccessErrorIsPermissionOnly(t *testing.T) {
	require.True(t, acquisitionAccessError(fmt.Errorf("x: %w", fs.ErrPermission)))
	require.True(t, acquisitionAccessError(fmt.Errorf("x: %w", syscall.EROFS)))
	require.False(t, acquisitionAccessError(fmt.Errorf("x: %w", errAcquisitionRead)))
	require.False(t, acquisitionAccessError(fmt.Errorf("x: %w", syscall.EIO)))
}
