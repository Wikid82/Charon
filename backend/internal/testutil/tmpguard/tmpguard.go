// Package tmpguard keeps a test package from leaking files into the shared
// temp directory. A package's TestMain wraps m.Run() in Run: the tests then
// work inside a private temp root that is always removed afterwards, and the
// package fails if anything was left behind in it.
package tmpguard

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// prefix is the only name prefix this package ever creates or sweeps.
	prefix = "charon-gotest-"
	// staleAfter is the age after which a prefix-named directory is assumed
	// to belong to an aborted run (SIGKILL, timeout panic, OOM) and is removed.
	staleAfter = 24 * time.Hour
)

// report receives the leak report; replaced in tests.
var report io.Writer = os.Stderr

// testMain is satisfied by *testing.M; a narrow interface keeps Run testable.
type testMain interface{ Run() int }

// Run executes the package's tests (m.Run) inside a private temp root and
// returns the exit code for os.Exit. It returns 1 when the tests passed but
// left files behind, and removes the private root in every case.
func Run(m testMain) int {
	origTmp := os.TempDir()
	sweepStale(origTmp, time.Now())

	base, err := os.MkdirTemp(origTmp, prefix+"*")
	if err != nil {
		logf("tmpguard: cannot create private temp root, running unguarded: %v\n", err)
		return m.Run()
	}

	// base comes from MkdirTemp and cannot hold a NUL byte, the only way Setenv fails.
	_ = os.Setenv("TMPDIR", base)
	code := m.Run()
	_ = os.Setenv("TMPDIR", origTmp)

	leaked := listLeaks(base)
	removeErr := removeTree(base)

	if len(leaked) > 0 {
		logf("tmpguard: tests leaked %d entr(y/ies) into the temp directory (removed):\n", len(leaked))
		for _, l := range leaked {
			logf("  %s\n", l)
		}
	}
	if removeErr != nil {
		logf("tmpguard: could not remove private temp root %s: %v\n", base, removeErr)
	}
	if code == 0 && (len(leaked) > 0 || removeErr != nil) {
		return 1
	}
	return code
}

// listLeaks describes every top-level entry left in base with its size.
func listLeaks(base string) []string {
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	leaks := make([]string, 0, len(entries))
	for _, e := range entries {
		leaks = append(leaks, fmt.Sprintf("%s (%d bytes)", e.Name(), treeSize(filepath.Join(base, e.Name()))))
	}
	return leaks
}

func treeSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, infoErr := d.Info(); infoErr == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// removeTree deletes root after making every directory in it writable, since
// a test may have left a read-only directory that would block RemoveAll.
func removeTree(root string) error {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if d != nil && d.IsDir() {
			_ = os.Chmod(path, 0o700) //nolint:gosec // private test scratch dir, needs rwx to be deletable
		}
		return nil
	})
	return os.RemoveAll(root)
}

// sweepStale removes directories in root that carry this package's prefix and
// are older than staleAfter. Symlinks and plain files are never touched, and
// nothing outside root is reachable because only entry names are used.
func sweepStale(root string, now time.Time) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), prefix) || !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) < staleAfter {
			continue
		}
		_ = removeTree(filepath.Join(root, e.Name()))
	}
}

// logf writes to report; a failing stderr is not actionable here.
func logf(format string, args ...any) {
	_, _ = fmt.Fprintf(report, format, args...)
}
