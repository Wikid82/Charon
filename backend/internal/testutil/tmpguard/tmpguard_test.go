package tmpguard

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// useRoot points the temp directory at a fresh directory and restores it
// afterwards, so a test controls where run() creates and sweeps.
func useRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	return root
}

func captureReport(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := report
	report = buf
	t.Cleanup(func() { report = prev })
	return buf
}

func guardDirs(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read root: %v", err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			names = append(names, e.Name())
		}
	}
	return names
}

func TestRun_CleanRunReturnsExitCodeAndRemovesBase(t *testing.T) {
	root := useRoot(t)
	out := captureReport(t)

	var seenTmp string
	code := run(func() int {
		seenTmp = os.TempDir()
		f, err := os.CreateTemp("", "x-*")
		if err != nil {
			t.Fatalf("create temp: %v", err)
		}
		_ = f.Close()
		_ = os.Remove(f.Name())
		return 0
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (output: %s)", code, out)
	}
	if filepath.Dir(seenTmp) != root || !strings.HasPrefix(filepath.Base(seenTmp), prefix) {
		t.Fatalf("TMPDIR during run = %q, want a %s* dir under %q", seenTmp, prefix, root)
	}
	if got := guardDirs(t, root); len(got) != 0 {
		t.Fatalf("base not removed: %v", got)
	}
	if os.TempDir() != root {
		t.Fatalf("TMPDIR not restored: %q", os.TempDir())
	}
}

func TestRun_PassesThroughFailingExitCode(t *testing.T) {
	useRoot(t)
	captureReport(t)
	if code := run(func() int { return 3 }); code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}
}

func TestRun_LeakFailsPackageNamesFileAndStillRemovesBase(t *testing.T) {
	root := useRoot(t)
	out := captureReport(t)

	code := run(func() int {
		if err := os.WriteFile(filepath.Join(os.TempDir(), "leaked-file.sqlite"), []byte("12345"), 0o600); err != nil { //nolint:gosec // G303: deliberately leaks into the guarded temp root
			t.Fatalf("write: %v", err)
		}
		dir := filepath.Join(os.TempDir(), "leaked-dir")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		return 0
	})

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	for _, want := range []string{"leaked-file.sqlite", "leaked-dir"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report does not name %q: %s", want, out)
		}
	}
	if got := guardDirs(t, root); len(got) != 0 {
		t.Fatalf("base not removed after leak: %v", got)
	}
}

func TestRun_LeakDoesNotMaskFailingExitCode(t *testing.T) {
	useRoot(t)
	captureReport(t)
	code := run(func() int {
		_ = os.WriteFile(filepath.Join(os.TempDir(), "leak"), nil, 0o600) //nolint:gosec // G303: deliberately leaks into the guarded temp root
		return 2
	})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestRun_RemovesReadOnlyDirectoriesLeftByTests(t *testing.T) {
	root := useRoot(t)
	captureReport(t)

	code := run(func() int {
		dir := filepath.Join(os.TempDir(), "ro")
		if err := os.MkdirAll(filepath.Join(dir, "inner"), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "inner", "f"), []byte("x"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := os.Chmod(filepath.Join(dir, "inner"), 0o500); err != nil { //nolint:gosec // G302: read-only directory is the scenario under test
			t.Fatalf("chmod: %v", err)
		}
		return 0
	})

	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (leak reported)", code)
	}
	if got := guardDirs(t, root); len(got) != 0 {
		t.Fatalf("read-only leftovers prevented removal: %v", got)
	}
}

func TestRun_SweepsOnlyStaleOwnDirectories(t *testing.T) {
	root := useRoot(t)
	captureReport(t)

	old := time.Now().Add(-2 * staleAfter)
	stale := filepath.Join(root, prefix+"stale")
	fresh := filepath.Join(root, prefix+"fresh")
	unrelated := filepath.Join(root, "unrelated-old")
	staleFile := filepath.Join(root, prefix+"file-not-dir")
	for _, d := range []string{stale, fresh, unrelated} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(staleFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, p := range []string{stale, unrelated, staleFile} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}

	if code := run(func() int { return 0 }); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	assertGone := func(p string) {
		t.Helper()
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Errorf("%s should have been swept (err=%v)", p, err)
		}
	}
	assertKept := func(p string) {
		t.Helper()
		if _, err := os.Lstat(p); err != nil {
			t.Errorf("%s must be kept: %v", p, err)
		}
	}
	assertGone(stale)
	assertKept(fresh)
	assertKept(unrelated)
	assertKept(staleFile)
}

func TestSweepStale_DoesNotFollowSymlinks(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	keep := filepath.Join(target, "keep")
	if err := os.WriteFile(keep, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	link := filepath.Join(root, prefix+"link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	old := time.Now().Add(-2 * staleAfter)
	if err := os.Chtimes(link, old, old); err != nil {
		t.Logf("chtimes on symlink: %v", err)
	}

	sweepStale(root, time.Now())

	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("symlink target was touched: %v", err)
	}
}

func TestSweepStale_MissingRootIsNoop(t *testing.T) {
	sweepStale(filepath.Join(t.TempDir(), "missing"), time.Now())
}

type fakeMain func() int

func (f fakeMain) Run() int { return f() }

// run adapts a plain function to the testMain interface.
func run(f func() int) int { return Run(fakeMain(f)) }

func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
}

func TestRun_RunsUnguardedWhenPrivateRootCannotBeCreated(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	out := captureReport(t)

	ran := false
	code := run(func() int { ran = true; return 0 })

	if !ran || code != 0 {
		t.Fatalf("ran=%v code=%d, want tests to run unguarded and pass", ran, code)
	}
	if !strings.Contains(out.String(), "running unguarded") {
		t.Errorf("missing warning: %s", out)
	}
}

func TestRun_FailsPackageWhenPrivateRootCannotBeRemoved(t *testing.T) {
	skipIfRoot(t)
	root := useRoot(t)
	out := captureReport(t)
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) }) //nolint:gosec // restore so t.TempDir cleanup works

	code := run(func() int {
		// A read-only parent stops the guard from unlinking its base.
		if err := os.Chmod(root, 0o500); err != nil { //nolint:gosec // G302: read-only directory is the scenario under test
			t.Fatalf("chmod: %v", err)
		}
		return 0
	})

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "could not remove private temp root") {
		t.Errorf("missing removal failure report: %s", out)
	}
}

func TestRun_TestRemovedBaseIsNotALeak(t *testing.T) {
	useRoot(t)
	captureReport(t)
	code := run(func() int {
		guardRoot := os.Getenv("TMPDIR")                // the guard's private root, not the shared one
		if err := os.RemoveAll(guardRoot); err != nil { //nolint:gosec // G703: guardRoot is the guard's own private temp root
			t.Fatalf("remove: %v", err)
		}
		return 0
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestRun_ReportsLeakInUnreadableDirectory(t *testing.T) {
	skipIfRoot(t)
	useRoot(t)
	out := captureReport(t)

	code := run(func() int {
		dir := filepath.Join(os.TempDir(), "locked")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.Chmod(dir, 0); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		return 0
	})

	if code != 1 || !strings.Contains(out.String(), "locked") {
		t.Fatalf("code=%d report=%s, want leak named and exit 1", code, out)
	}
}
