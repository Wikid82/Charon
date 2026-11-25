package services

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"plugin"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// recordingOpener stands in for plugin.Open. It records every path it is asked to open and always fails, so the
// tests can check which path was used without loading any code.
type recordingOpener struct {
	paths []string
}

func (r *recordingOpener) open(path string) (*plugin.Plugin, error) {
	r.paths = append(r.paths, path)
	return nil, errors.New("simulated failure")
}

// newCacheTestLoader builds a loader for pluginDir with its own opener and a cache folder inside the test temp dir.
func newCacheTestLoader(t *testing.T, pluginDir, cacheDir string, sigs map[string]string) (*PluginLoaderService, *recordingOpener) {
	t.Helper()
	rec := &recordingOpener{}
	s := NewPluginLoaderService(nil, pluginDir, cacheDir, sigs)
	s.openPluginFile = rec.open
	return s, rec
}

// writePluginFile writes plugin bytes with an exact mode (WriteFile applies the umask).
func writePluginFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write plugin file: %v", err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod plugin file: %v", err)
	}
}

// skipOnWindows skips the POSIX permission and ownership cases.
func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission semantics are not applicable on Windows")
	}
}

func TestHashPluginBytesFormat(t *testing.T) {
	got := hashPluginBytes([]byte("plugin content"))
	if !strings.HasPrefix(got, "sha256:") || len(got) != len("sha256:")+64 {
		t.Fatalf("hashPluginBytes() = %q, want sha256: prefix and 64 hex digits", got)
	}
	if again := hashPluginBytes([]byte("plugin content")); again != got {
		t.Fatalf("hashPluginBytes is not deterministic: %q vs %q", got, again)
	}
}

func TestLoadPluginRefusesFileOutsidePluginDirectory(t *testing.T) {
	pluginDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.so")
	writePluginFile(t, outside, []byte("x"), 0o600)
	nested := filepath.Join(pluginDir, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	nestedFile := filepath.Join(nested, "nested.so")
	writePluginFile(t, nestedFile, []byte("x"), 0o600)

	s, rec := newCacheTestLoader(t, pluginDir, t.TempDir(), nil)
	for _, path := range []string{outside, nestedFile} {
		if err := s.LoadPlugin(path); !errors.Is(err, dnsprovider.ErrPluginOutsideDirectory) {
			t.Fatalf("LoadPlugin(%s) error = %v, want ErrPluginOutsideDirectory", path, err)
		}
	}
	if len(rec.paths) != 0 {
		t.Fatalf("no plugin should be opened, got %v", rec.paths)
	}
}

func TestLoadPluginRefusesGroupOrWorldWritableFile(t *testing.T) {
	skipOnWindows(t)
	for _, mode := range []os.FileMode{0o664, 0o646, 0o666, 0o662} {
		pluginDir := t.TempDir()
		cacheDir := filepath.Join(t.TempDir(), "cache")
		path := filepath.Join(pluginDir, "writable.so")
		writePluginFile(t, path, []byte("bytes"), mode)

		s, rec := newCacheTestLoader(t, pluginDir, cacheDir, nil)
		if err := s.LoadPlugin(path); err == nil {
			t.Fatalf("mode %o: LoadPlugin succeeded, want refusal", mode)
		}
		if len(rec.paths) != 0 {
			t.Fatalf("mode %o: opened %v, want no open", mode, rec.paths)
		}
		if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
			t.Fatalf("mode %o: cache folder was created for a refused file", mode)
		}
	}
}

func TestLoadPluginRefusesSymlinkPlugin(t *testing.T) {
	skipOnWindows(t)
	pluginDir := t.TempDir()
	target := filepath.Join(t.TempDir(), "real.so")
	writePluginFile(t, target, []byte("bytes"), 0o600)
	link := filepath.Join(pluginDir, "link.so")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	s, rec := newCacheTestLoader(t, pluginDir, t.TempDir(), nil)
	if err := s.LoadPlugin(link); err == nil {
		t.Fatal("LoadPlugin followed a symbolic link, want refusal")
	}
	if len(rec.paths) != 0 {
		t.Fatalf("opened %v, want no open", rec.paths)
	}
}

func TestLoadPluginRefusesOversizedFile(t *testing.T) {
	pluginDir := t.TempDir()
	path := filepath.Join(pluginDir, "huge.so")
	writePluginFile(t, path, nil, 0o600)
	if err := os.Truncate(path, maxPluginFileBytes+1); err != nil {
		t.Fatal(err)
	}

	s, rec := newCacheTestLoader(t, pluginDir, t.TempDir(), nil)
	if err := s.LoadPlugin(path); err == nil {
		t.Fatal("LoadPlugin accepted a file over the size limit")
	}
	if len(rec.paths) != 0 {
		t.Fatalf("opened %v, want no open", rec.paths)
	}
}

func TestLoadPluginHashMismatchCachesNothing(t *testing.T) {
	pluginDir := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	path := filepath.Join(pluginDir, "pinned.so")
	writePluginFile(t, path, []byte("actual bytes"), 0o600)
	wrong := "sha256:" + strings.Repeat("0", 64)

	s, rec := newCacheTestLoader(t, pluginDir, cacheDir, map[string]string{"pinned": wrong})
	err := s.LoadPlugin(path)

	if !errors.Is(err, dnsprovider.ErrSignatureMismatch) {
		t.Fatalf("LoadPlugin error = %v, want ErrSignatureMismatch", err)
	}
	if len(rec.paths) != 0 {
		t.Fatalf("opened %v, want no open", rec.paths)
	}
	if entries, _ := os.ReadDir(cacheDir); len(entries) != 0 {
		t.Fatalf("cache folder holds %d entries after a refused file, want none", len(entries))
	}
}

func TestLoadPluginOpensOnlyTheVerifiedCopy(t *testing.T) {
	skipOnWindows(t)
	pluginDir := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	data := []byte("verified plugin bytes")
	path := filepath.Join(pluginDir, "copy.so")
	writePluginFile(t, path, data, 0o600)

	s, rec := newCacheTestLoader(t, pluginDir, cacheDir, nil)
	_ = s.LoadPlugin(path) // the simulated opener fails; the copy is what matters here

	absCache, err := filepath.Abs(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	wantCopy := filepath.Join(absCache, strings.TrimPrefix(hashPluginBytes(data), "sha256:")+".so")
	if len(rec.paths) != 1 || rec.paths[0] != wantCopy {
		t.Fatalf("opened %v, want exactly [%s]; the original path must never be opened", rec.paths, wantCopy)
	}
	info, err := os.Stat(wantCopy)
	if err != nil {
		t.Fatalf("verified copy missing: %v", err)
	}
	if info.Mode().Perm() != 0o500 {
		t.Fatalf("copy mode = %o, want 0500", info.Mode().Perm())
	}
	if got, _ := os.ReadFile(wantCopy); !bytes.Equal(got, data) { // #nosec G304 -- test-controlled path in t.TempDir
		t.Fatal("copy bytes differ from the verified bytes")
	}
}

func TestLoadPluginNoExecFailureNamesCacheSetting(t *testing.T) {
	pluginDir := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	path := filepath.Join(pluginDir, "noexec.so")
	writePluginFile(t, path, []byte("bytes"), 0o600)

	s, _ := newCacheTestLoader(t, pluginDir, cacheDir, nil)
	err := s.LoadPlugin(path)

	if err == nil || !strings.Contains(err.Error(), "CHARON_PLUGIN_CACHE_DIR") {
		t.Fatalf("LoadPlugin error = %v, want text naming CHARON_PLUGIN_CACHE_DIR", err)
	}
}

func TestLoadPluginReusesExistingVerifiedCopy(t *testing.T) {
	pluginDir := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	data := []byte("reuse me")
	path := filepath.Join(pluginDir, "reuse.so")
	writePluginFile(t, path, data, 0o600)

	s, _ := newCacheTestLoader(t, pluginDir, cacheDir, nil)
	_ = s.LoadPlugin(path)
	copyPath := filepath.Join(cacheDir, strings.TrimPrefix(hashPluginBytes(data), "sha256:")+".so")
	past := time.Date(2001, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(copyPath, past, past); err != nil {
		t.Fatal(err)
	}

	_ = s.LoadPlugin(path)

	info, err := os.Stat(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(past) {
		t.Fatalf("existing verified copy was rewritten (mtime %v)", info.ModTime())
	}
}

func TestLoadPluginReplacesInvalidCachedCopy(t *testing.T) {
	skipOnWindows(t)
	pluginDir := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	data := []byte("the real bytes")
	path := filepath.Join(pluginDir, "tampered.so")
	writePluginFile(t, path, data, 0o600)
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(cacheDir, strings.TrimPrefix(hashPluginBytes(data), "sha256:")+".so")
	writePluginFile(t, copyPath, []byte("other bytes"), 0o500)

	s, _ := newCacheTestLoader(t, pluginDir, cacheDir, nil)
	_ = s.LoadPlugin(path)

	if got, _ := os.ReadFile(copyPath); !bytes.Equal(got, data) { // #nosec G304 -- test-controlled path in t.TempDir
		t.Fatal("a cached copy that does not hash to its name was reused")
	}
}

func TestLoadPluginCopiesVerifiedBytesWhenOriginalChangesAfterRead(t *testing.T) {
	skipOnWindows(t)
	pluginDir := t.TempDir()
	cacheDir := filepath.Join(t.TempDir(), "cache")
	verified := []byte("verified before the change")
	path := filepath.Join(pluginDir, "changing.so")
	writePluginFile(t, path, verified, 0o600)

	s, _ := newCacheTestLoader(t, pluginDir, cacheDir, nil)
	s.afterReadHook = func(string) {
		if err := os.WriteFile(path, []byte("replacement content"), 0o600); err != nil {
			t.Fatalf("rewrite original: %v", err)
		}
	}
	_ = s.LoadPlugin(path)

	copyPath := filepath.Join(cacheDir, strings.TrimPrefix(hashPluginBytes(verified), "sha256:")+".so")
	if got, _ := os.ReadFile(copyPath); !bytes.Equal(got, verified) { // #nosec G304 -- test-controlled path in t.TempDir
		t.Fatalf("cached copy = %q, want the verified bytes", got)
	}
}

func TestLoadPluginCacheFolderRules(t *testing.T) {
	skipOnWindows(t)

	t.Run("symlink folder is refused and its target is untouched", func(t *testing.T) {
		pluginDir := t.TempDir()
		realDir := t.TempDir()
		link := filepath.Join(t.TempDir(), "cache-link")
		if err := os.Symlink(realDir, link); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(pluginDir, "linked.so")
		writePluginFile(t, path, []byte("bytes"), 0o600)

		s, rec := newCacheTestLoader(t, pluginDir, link, nil)
		if err := s.LoadPlugin(path); err == nil {
			t.Fatal("LoadPlugin used a symlinked cache folder")
		}
		if entries, _ := os.ReadDir(realDir); len(entries) != 0 {
			t.Fatal("a file was written through the symlinked cache folder")
		}
		if len(rec.paths) != 0 {
			t.Fatalf("opened %v, want no open", rec.paths)
		}
	})

	t.Run("loose mode is fixed to 0700", func(t *testing.T) {
		pluginDir := t.TempDir()
		cacheDir := filepath.Join(t.TempDir(), "cache")
		if err := os.Mkdir(cacheDir, 0o755); err != nil { // #nosec G301 -- test fixture: the loader must change this mode
			t.Fatal(err)
		}
		if err := os.Chmod(cacheDir, 0o755); err != nil { // #nosec G302 -- test fixture: the loader must change this mode
			t.Fatal(err)
		}
		path := filepath.Join(pluginDir, "mode.so")
		writePluginFile(t, path, []byte("bytes"), 0o600)

		s, _ := newCacheTestLoader(t, pluginDir, cacheDir, nil)
		_ = s.LoadPlugin(path)

		info, err := os.Stat(cacheDir)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("cache folder mode = %o, want 0700", info.Mode().Perm())
		}
	})

	t.Run("files that are not verified copies are removed", func(t *testing.T) {
		pluginDir := t.TempDir()
		cacheDir := filepath.Join(t.TempDir(), "cache")
		if err := os.Mkdir(cacheDir, 0o700); err != nil {
			t.Fatal(err)
		}
		stale := filepath.Join(cacheDir, "leftover.txt")
		if err := os.WriteFile(stale, []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(pluginDir, "fresh.so")
		writePluginFile(t, path, []byte("bytes"), 0o600)

		s, _ := newCacheTestLoader(t, pluginDir, cacheDir, nil)
		_ = s.LoadPlugin(path)

		if _, err := os.Stat(stale); !os.IsNotExist(err) {
			t.Fatalf("stale file survived cache preparation: %v", err)
		}
	})

	t.Run("foreign owner is refused", func(t *testing.T) {
		info, err := os.Stat(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if !ownedBy(info, os.Geteuid()) {
			t.Fatal("folder owned by the effective user was not accepted")
		}
		if ownedBy(info, os.Geteuid()+1) {
			t.Fatal("folder owned by another user was accepted")
		}
	})
}

func TestLoadPluginWithoutCacheFolderFailsClosed(t *testing.T) {
	pluginDir := t.TempDir()
	path := filepath.Join(pluginDir, "nocache.so")
	writePluginFile(t, path, []byte("bytes"), 0o600)

	s, rec := newCacheTestLoader(t, pluginDir, "", nil)
	err := s.LoadPlugin(path)

	if err == nil || !strings.Contains(err.Error(), "CHARON_PLUGIN_CACHE_DIR") {
		t.Fatalf("LoadPlugin error = %v, want text naming CHARON_PLUGIN_CACHE_DIR", err)
	}
	if len(rec.paths) != 0 {
		t.Fatalf("opened %v, want no open", rec.paths)
	}
}

// setPluginFolderMode sets a plugin folder to 0755. t.TempDir creates its numbered folder from 0777 under the umask,
// so it can be group-writable, and the loader refuses such a folder.
func setPluginFolderMode(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o755); err != nil { // #nosec G302 -- test fixture: the loader must change this mode
		t.Fatalf("chmod plugin folder: %v", err)
	}
}
