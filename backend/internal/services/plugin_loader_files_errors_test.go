package services

import (
	"os"
	"path/filepath"
	"plugin"
	"strings"
	"testing"
)

// skipAsRoot skips permission-denial cases, which root bypasses.
func skipAsRoot(t *testing.T) {
	t.Helper()
	skipOnWindows(t)
	if os.Geteuid() == 0 {
		t.Skip("permission denial does not apply to root")
	}
}

// makeReadOnly sets dir to mode and restores 0700 at cleanup so the temp folder can be removed.
func makeReadOnly(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatalf("chmod %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) // #nosec G302 -- test cleanup restores the owner-only mode of a temp folder
}

// removeCwdForTest changes into a folder and deletes it, so relative paths can no longer be made absolute.
func removeCwdForTest(t *testing.T) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	gone := t.TempDir()
	if err := os.Chdir(gone); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(gone); err != nil {
		_ = os.Chdir(orig)
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func TestCheckInPluginDirectory_AbsFailuresAreRefused(t *testing.T) {
	t.Run("plugin path cannot be made absolute", func(t *testing.T) {
		removeCwdForTest(t)
		s := NewPluginLoaderService(nil, t.TempDir(), "", nil)
		if err := s.checkInPluginDirectory("relative/plugin.so"); err == nil {
			t.Fatal("expected refusal when the plugin path cannot be made absolute")
		}
	})

	t.Run("plugin folder cannot be made absolute", func(t *testing.T) {
		removeCwdForTest(t)
		s := NewPluginLoaderService(nil, "relative-plugins", "", nil)
		if err := s.checkInPluginDirectory(filepath.Join(t.TempDir(), "plugin.so")); err == nil {
			t.Fatal("expected refusal when the plugin folder cannot be made absolute")
		}
	})
}

func TestReadPluginFile_NotARegularFileIsRefused(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := readPluginFile(dir); err == nil {
		t.Fatal("readPluginFile accepted a folder")
	}
}

func TestPrepareCacheDir_UncreatableFolderFails(t *testing.T) {
	skipAsRoot(t)
	parent := t.TempDir()
	makeReadOnly(t, parent, 0o500)

	s := NewPluginLoaderService(nil, t.TempDir(), filepath.Join(parent, "cache"), nil)
	if err := s.prepareCacheDir(); err == nil {
		t.Fatal("prepareCacheDir created a folder inside a read-only parent")
	}
}

func TestPrepareCacheDir_UnremovableEntryFails(t *testing.T) {
	skipAsRoot(t)
	cacheDir := filepath.Join(t.TempDir(), "cache")
	if err := os.Mkdir(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// A folder holding a file, made read-only, cannot be emptied by its owner.
	stuck := filepath.Join(cacheDir, "stuck")
	if err := os.Mkdir(stuck, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stuck, "inside.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	makeReadOnly(t, stuck, 0o500)

	s := NewPluginLoaderService(nil, t.TempDir(), cacheDir, nil)
	if err := s.prepareCacheDir(); err == nil {
		t.Fatal("prepareCacheDir reported success with an entry it could not remove")
	}
}

func TestCacheVerifiedCopy_CacheFolderRemovedAfterPreparation(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	s := NewPluginLoaderService(nil, t.TempDir(), cacheDir, nil)
	data := []byte("verified bytes")
	sig := hashPluginBytes(data)
	if _, err := s.cacheVerifiedCopy(sig, data); err != nil {
		t.Fatalf("first copy failed: %v", err)
	}
	if err := os.RemoveAll(cacheDir); err != nil {
		t.Fatal(err)
	}
	if _, err := s.cacheVerifiedCopy(sig, data); err == nil {
		t.Fatal("cacheVerifiedCopy succeeded with the cache folder gone")
	}
}

func TestCacheVerifiedCopy_ReadOnlyFolderCannotHoldCopy(t *testing.T) {
	skipAsRoot(t)
	cacheDir := filepath.Join(t.TempDir(), "cache")
	s := NewPluginLoaderService(nil, t.TempDir(), cacheDir, nil)
	data := []byte("first")
	if _, err := s.cacheVerifiedCopy(hashPluginBytes(data), data); err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	makeReadOnly(t, cacheDir, 0o500)

	other := []byte("second")
	if _, err := s.cacheVerifiedCopy(hashPluginBytes(other), other); err == nil {
		t.Fatal("cacheVerifiedCopy wrote into a read-only folder")
	}
}

func TestCacheVerifiedCopy_DirectoryInPlaceOfCopyIsReplaced(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	s := NewPluginLoaderService(nil, t.TempDir(), cacheDir, nil)
	data := []byte("replace me")
	sig := hashPluginBytes(data)
	if _, err := s.cacheVerifiedCopy(sig, data); err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	name := filepath.Join(cacheDir, sig[len("sha256:"):]+".so")
	if err := os.Remove(name); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(name, 0o700); err != nil {
		t.Fatal(err)
	}

	path, err := s.cacheVerifiedCopy(sig, data)
	if err != nil {
		t.Fatalf("cacheVerifiedCopy failed: %v", err)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("copy at %s is not a regular file: %v", path, err)
	}
}

func TestWriteAndSeal_ClosedFileReportsError(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "sealed-*")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writeAndSeal(f, []byte("data")); err == nil {
		t.Fatal("writeAndSeal reported success on a closed file")
	}
}

func TestOpenCachedCopy_ReturnsPluginWhenOpenSucceeds(t *testing.T) {
	s := NewPluginLoaderService(nil, t.TempDir(), t.TempDir(), nil)
	s.openPluginFile = func(string) (*plugin.Plugin, error) { return nil, nil }
	if _, err := s.openCachedCopy("cached.so"); err != nil {
		t.Fatalf("openCachedCopy returned %v, want nil", err)
	}
}

func TestPrepareCacheDir_ForeignOwnerIsRefused(t *testing.T) {
	skipOnWindows(t)
	original := effectiveUID
	effectiveUID = func() int { return os.Geteuid() + 1 }
	t.Cleanup(func() { effectiveUID = original })

	s := NewPluginLoaderService(nil, t.TempDir(), filepath.Join(t.TempDir(), "cache"), nil)
	err := s.prepareCacheDir()

	if err == nil || !strings.Contains(err.Error(), "not owned by the Charon user") {
		t.Fatalf("prepareCacheDir error = %v, want an ownership refusal", err)
	}
}
