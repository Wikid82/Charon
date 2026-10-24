package crowdsec

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

// ErrPartialInPlaceWrite is wrapped when a write into the live file itself failed, so the live file may be half
// written. Callers that keep a backup must keep it when they see this error; it is the only intact copy then.
var ErrPartialInPlaceWrite = errors.New("crowdsec: in-place write failed; the live file may be partly written")

// AtomicWriteOptions selects how WriteFileAtomic replaces a file. The zero value replaces the file by rename and
// keeps no owner, sweeps nothing and uses the current time.
type AtomicWriteOptions struct {
	// TempPattern is the os.CreateTemp pattern of the temporary sibling; it must end in "*". Empty means
	// "<base>.tmp-*". Stale files matching its prefix are swept (see SweepTempsOlderThan).
	TempPattern string
	// KeepOwner gives the temporary file the owner of the live file before the rename, so the rename does not
	// change who owns a file another process reads.
	KeepOwner bool
	// InPlaceFallback writes the live file itself when the owner cannot be kept with EPERM, or when the rename is
	// refused with EBUSY, EXDEV or EROFS (bind-mounted single file, cross-device, read-only mount).
	InPlaceFallback bool
	// ForceMode sets the requested mode on the live file when it is written in place, so a file that was
	// left with another mode ends up with the mode the writer wants. A chmod failure is logged and the write goes
	// on. The zero value keeps the live file's mode.
	ForceMode bool
	// NoFollow makes the in-place write refuse a live file that is a symlink (O_NOFOLLOW), so a link planted at the
	// target is never written through. The refusal happens before the file is opened, so nothing is touched.
	NoFollow bool
	// SweepTempsOlderThan removes regular files matching the temp pattern prefix that are older than this age.
	// Zero disables the sweep. Such names are only created by Charon's own writers.
	SweepTempsOlderThan time.Duration
	// Now is the reference time for the sweep age. The zero value means time.Now().
	Now time.Time
}

// AtomicWriteResult describes how the file was replaced.
type AtomicWriteResult struct {
	// InPlace reports that the live file was written itself, so its inode did not change.
	InPlace bool
	// OwnerKept reports that the new content carries the owner of the live file.
	OwnerKept bool
}

// WriteSyncCloser is the part of *os.File the writers use for a whole-file write.
type WriteSyncCloser interface {
	Write(p []byte) (int, error)
	Sync() error
	Close() error
}

// tempFile is the part of *os.File the atomic writer uses. It exists so tests can make any single step fail.
type tempFile interface {
	WriteSyncCloser
	Name() string
	Chmod(mode fs.FileMode) error
	Chown(uid, gid int) error
}

// The seams of WriteFileAtomic. The defaults return a literal nil interface on error: a typed nil *os.File
// inside the interface would look like a file.
var (
	// createTempFile creates the temporary sibling the new content is written to.
	createTempFile = func(dir, pattern string) (tempFile, error) {
		f, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	// openInPlaceFile opens the live file itself for an in-place write; noFollow adds O_NOFOLLOW.
	openInPlaceFile = func(name string, mode fs.FileMode, noFollow bool) (tempFile, error) {
		flags := os.O_WRONLY | os.O_TRUNC | os.O_CREATE
		if noFollow {
			flags |= syscall.O_NOFOLLOW
		}
		f, err := os.OpenFile(name, flags, mode) //nolint:gosec // G304: name is a validated Charon config path
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	// renameFile is the rename that replaces the live file.
	renameFile = os.Rename
	// syncDir flushes a directory after a rename into it (see SyncDir).
	syncDir = SyncDir
	// fileOwner reads the owner of a file and processOwner the owner of the running process (see keepOwner).
	fileOwner = func(info fs.FileInfo) (uid, gid int, ok bool) {
		stat, isStat := info.Sys().(*syscall.Stat_t)
		if !isStat {
			return 0, 0, false
		}
		return int(stat.Uid), int(stat.Gid), true
	}
	processOwner = func() (uid, gid int) { return os.Geteuid(), os.Getegid() }
)

// WriteFileAtomic replaces the file at path with data and mode. The data goes to a temporary sibling that is
// renamed over the file, so a failure at any step leaves the previous file untouched. The options decide the
// owner handling and the in-place fallbacks (see AtomicWriteOptions). A failed in-place write wraps
// ErrPartialInPlaceWrite. Directory sync failures and failed sweeps are logged, never returned.
func WriteFileAtomic(path string, data []byte, mode fs.FileMode, opts AtomicWriteOptions) (AtomicWriteResult, error) {
	path = filepath.Clean(path)
	dir, base := filepath.Dir(path), filepath.Base(path)
	pattern := opts.TempPattern
	if pattern == "" {
		pattern = base + ".tmp-*"
	}
	if opts.SweepTempsOlderThan > 0 {
		now := opts.Now
		if now.IsZero() {
			now = time.Now()
		}
		sweepStaleTemps(dir, strings.TrimSuffix(pattern, "*"), opts.SweepTempsOlderThan, now)
	}

	live, err := liveFileInfo(path)
	if err != nil {
		return AtomicWriteResult{}, err
	}
	tmp, err := createTempFile(dir, pattern)
	if err != nil {
		return AtomicWriteResult{}, fmt.Errorf("create temporary crowdsec file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName) // already gone after a successful rename
	}()

	ownerKept, ownerErr, err := writeTempFile(tmp, data, mode, live, opts.KeepOwner)
	if err != nil {
		return AtomicWriteResult{}, err
	}
	if ownerErr != nil {
		if opts.InPlaceFallback && errors.Is(ownerErr, syscall.EPERM) {
			// A permission refusal means the owner must be kept, so the live file itself is written.
			opened, ipErr := writeInPlace(path, data, mode, opts)
			switch {
			case ipErr == nil:
				return AtomicWriteResult{InPlace: true, OwnerKept: true}, nil
			case opened:
				return AtomicWriteResult{}, fmt.Errorf("write crowdsec file in place: %w: %w", ErrPartialInPlaceWrite, ipErr)
			default:
				// Nothing was touched, so the rename below is the last resort.
				warnOwnerNotKept(path, live, errors.Join(ownerErr, ipErr))
			}
		} else {
			// Any other chown failure keeps the atomic rename, so owner handling never makes the write less atomic.
			warnOwnerNotKept(path, live, ownerErr)
		}
	}
	if err = renameFile(tmpName, path); err == nil {
		syncDirAfterRename(dir)
		return AtomicWriteResult{OwnerKept: ownerKept}, nil
	}
	if !opts.InPlaceFallback || !isRenameUnsupported(err) {
		return AtomicWriteResult{}, fmt.Errorf("replace crowdsec file: %w", err)
	}
	// The rename is refused, so the file is written in place. Any failure here counts as partial: the live file may
	// already have been truncated, and callers keep their backup. The one exception is a symlink refused by NoFollow:
	// that open is refused before the file is touched, so nothing is partial.
	if _, err = writeInPlace(path, data, mode, opts); err != nil {
		if errors.Is(err, syscall.ELOOP) {
			return AtomicWriteResult{}, fmt.Errorf("write crowdsec file in place: %w", err)
		}
		return AtomicWriteResult{}, fmt.Errorf("write crowdsec file in place: %w: %w", ErrPartialInPlaceWrite, err)
	}
	return AtomicWriteResult{InPlace: true, OwnerKept: true}, nil
}

// liveFileInfo returns the info of the file at path, or nil when it does not exist yet.
func liveFileInfo(path string) (fs.FileInfo, error) {
	info, err := os.Stat(path)
	switch {
	case err == nil:
		return info, nil
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	default:
		return nil, fmt.Errorf("stat crowdsec file: %w", err)
	}
}

// writeTempFile fills the temporary file, chmods it, and when keep is set tries to give it the live file's owner
// (see keepOwner), then syncs and closes it. ownerErr is non-nil when the owner could not be kept, so the caller
// decides between the in-place write and the rename; err is a failure that stops the write.
func writeTempFile(tmp tempFile, data []byte, mode fs.FileMode, live fs.FileInfo, keep bool) (ownerKept bool, ownerErr, err error) {
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Chmod(mode)
	}
	if err == nil && keep {
		ownerKept, ownerErr = keepOwner(tmp, live)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return false, nil, fmt.Errorf("write temporary crowdsec file: %w", err)
	}
	return ownerKept, ownerErr, nil
}

// keepOwner gives the temporary file the owner of the live file. It reports whether the temporary file now has
// that owner. A new file (live nil), an unknown owner and an owner that already is the process owner need
// nothing: the first two cannot be matched, and the third is already true. A chown failure is returned rather
// than fatal.
func keepOwner(tmp tempFile, live fs.FileInfo) (bool, error) {
	if live == nil {
		return false, nil
	}
	uid, gid, ok := fileOwner(live)
	if !ok {
		return false, nil
	}
	if processUID, processGID := processOwner(); uid == processUID && gid == processGID {
		return true, nil
	}
	if err := tmp.Chown(uid, gid); err != nil {
		return false, fmt.Errorf("keep crowdsec file owner: %w", err)
	}
	return true, nil
}

// warnOwnerNotKept logs that the live file's owner could not be kept and that the rename replaces it instead.
// live is never nil here: an owner error only arises for an existing file (see keepOwner).
func warnOwnerNotKept(path string, live fs.FileInfo, cause error) {
	uid, gid, _ := fileOwner(live)
	logger.Log().WithError(cause).WithField("path", util.SanitizeForLog(path)).WithField("uid", uid).WithField("gid", gid).
		Warn("crowdsec file owner could not be kept; replacing the file instead")
}

// writeInPlace replaces the content of the live file at path without renaming it, so the inode and owner stay.
// The mode stays too unless opts.ForceMode is set. opened reports whether the file was opened; when it was not,
// nothing was touched.
func writeInPlace(path string, data []byte, mode fs.FileMode, opts AtomicWriteOptions) (opened bool, err error) {
	f, err := openInPlaceFile(path, mode, opts.NoFollow)
	if err != nil {
		return false, fmt.Errorf("open crowdsec file: %w", err)
	}
	if opts.ForceMode {
		if chmodErr := f.Chmod(mode); chmodErr != nil {
			logger.Log().WithError(chmodErr).WithField("path", util.SanitizeForLog(path)).WithField("mode", fmt.Sprintf("%o", mode)).
				Warn("could not set the crowdsec file mode; writing the content anyway")
		}
	}
	return true, WriteSyncClose(f, data)
}

// WriteSyncClose writes data, syncs and closes f. The close error is reported only when nothing else failed.
func WriteSyncClose(f WriteSyncCloser, data []byte) error {
	_, err := f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// isRenameUnsupported reports whether err means the target cannot be replaced by rename (bind-mounted file,
// cross-device, read-only mount) but may still be writable in place.
func isRenameUnsupported(err error) bool {
	var linkErr *os.LinkError
	if !errors.As(err, &linkErr) {
		return false
	}
	return errors.Is(linkErr.Err, syscall.EBUSY) || errors.Is(linkErr.Err, syscall.EXDEV) || errors.Is(linkErr.Err, syscall.EROFS)
}

// syncDirAfterRename makes a completed rename durable. The new content is already in place, so a failure is
// only logged and never fails the write.
func syncDirAfterRename(dir string) {
	if err := syncDir(dir); err != nil {
		logger.Log().WithError(err).WithField("path", util.SanitizeForLog(dir)).Warn("could not sync the directory after replacing a crowdsec file")
	}
}

// sweepStaleTemps removes the temporary files that earlier writes left behind when the process died mid-write.
// Only regular files whose names start with prefix and that are older than age are removed; symlinks and
// directories are never touched. The age guard matters when another writer uses the same directory, where a fresh
// temporary file may still be in use. Failures are logged at debug level and never fail the write.
func sweepStaleTemps(dir, prefix string, age time.Duration, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		logger.Log().WithError(err).WithField("path", util.SanitizeForLog(dir)).Debug("could not list the crowdsec directory to sweep temporary files")
		return
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		tmpPath := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(tmpPath)
		if err != nil {
			logger.Log().WithError(err).WithField("path", util.SanitizeForLog(tmpPath)).Debug("could not inspect a stale crowdsec temporary file")
			continue
		}
		if !info.Mode().IsRegular() || now.Sub(info.ModTime()) < age {
			continue
		}
		if err := os.Remove(tmpPath); err != nil {
			logger.Log().WithError(err).WithField("path", util.SanitizeForLog(tmpPath)).Debug("could not remove a stale crowdsec temporary file")
		}
	}
}
