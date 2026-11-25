package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"plugin"
	"regexp"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

// effectiveUID returns the user id this process expects to own the cache folder. It is a variable so tests can
// simulate another owner.
var effectiveUID = os.Geteuid

// maxPluginFileBytes caps the size of a plugin file that is read, hashed and copied.
const maxPluginFileBytes = 256 << 20

// cacheFileName matches a verified copy's name: the SHA-256 of its bytes plus ".so".
var cacheFileName = regexp.MustCompile(`^[0-9a-f]{64}\.so$`)

// cacheCopyMode is the mode of a verified copy: readable and executable by its owner only.
const cacheCopyMode fs.FileMode = 0o500

// hashPluginBytes returns the allowlist form of a plugin's digest: "sha256:" followed by hex.
func hashPluginBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// checkInPluginDirectory accepts only a .so file directly inside the plugin folder.
func (s *PluginLoaderService) checkInPluginDirectory(path string) error {
	dirAbs, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("%w: %v", dnsprovider.ErrPluginOutsideDirectory, err)
	}
	pluginDirAbs, err := filepath.Abs(s.pluginDir)
	if err != nil {
		return fmt.Errorf("%w: %v", dnsprovider.ErrPluginOutsideDirectory, err)
	}
	if dirAbs != pluginDirAbs || filepath.Ext(path) != ".so" {
		return fmt.Errorf("%w: %s", dnsprovider.ErrPluginOutsideDirectory, path)
	}
	return nil
}

// readPluginFile opens a plugin without following links, checks that it is a regular file that no other user can
// write, reads it once and returns its bytes with their allowlist digest.
func readPluginFile(path string) (data []byte, sig string, err error) {
	// #nosec G304 -- path is checked by checkInPluginDirectory; the open refuses links
	f, err := os.OpenFile(path, os.O_RDONLY|openNoFollowFlags, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open plugin file: %w", err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("stat plugin file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, "", fmt.Errorf("plugin file is not a regular file: %s", path)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return nil, "", fmt.Errorf("plugin file is group- or world-writable (mode %o): set it to mode 644 or stricter", info.Mode().Perm())
	}
	if info.Size() > maxPluginFileBytes {
		return nil, "", fmt.Errorf("plugin file is larger than %d bytes", maxPluginFileBytes)
	}

	data, err = io.ReadAll(io.LimitReader(f, maxPluginFileBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read plugin file: %w", err)
	}
	if len(data) > maxPluginFileBytes {
		return nil, "", fmt.Errorf("plugin file is larger than %d bytes", maxPluginFileBytes)
	}
	return data, hashPluginBytes(data), nil
}

// prepareCacheDir makes the cache folder private to this user. It refuses a folder that is a link, that another
// user owns, or that cannot be made real, and it removes anything that is not a verified copy. It runs once per loader.
func (s *PluginLoaderService) prepareCacheDir() error {
	if s.cacheDir == "" {
		return errors.New("the plugin cache folder is not configured: set CHARON_PLUGIN_CACHE_DIR")
	}
	abs, err := filepath.Abs(s.cacheDir)
	if err != nil {
		return fmt.Errorf("resolve plugin cache folder: %w", err)
	}
	if err = os.MkdirAll(abs, 0o700); err != nil {
		return fmt.Errorf("create plugin cache folder %s: %w", abs, err)
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return fmt.Errorf("inspect plugin cache folder %s: %w", abs, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("the plugin cache folder %s is not a real folder: set CHARON_PLUGIN_CACHE_DIR to a real folder", abs)
	}
	if !ownedBy(info, effectiveUID()) {
		return fmt.Errorf("the plugin cache folder %s is not owned by the Charon user: set CHARON_PLUGIN_CACHE_DIR to a folder that user owns", abs)
	}
	if err = os.Chmod(abs, 0o700); err != nil { // #nosec G302 -- directory mode 0700 is required; execute bit is needed to traverse the plugin cache folder
		return fmt.Errorf("set plugin cache folder mode: %w", err)
	}

	root, err := os.OpenRoot(abs)
	if err != nil {
		return fmt.Errorf("open plugin cache folder: %w", err)
	}
	defer func() { _ = root.Close() }()

	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return fmt.Errorf("list plugin cache folder: %w", err)
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && cacheFileName.MatchString(entry.Name()) {
			continue
		}
		if err := root.RemoveAll(entry.Name()); err != nil {
			return fmt.Errorf("remove unexpected entry from plugin cache folder: %w", err)
		}
	}
	s.cacheDir = abs
	return nil
}

// cacheVerifiedCopy stores the verified bytes as <cacheDir>/<digest>.so and returns that path. An existing copy is
// reused only when it is a private regular file owned by this user that still hashes to its name.
func (s *PluginLoaderService) cacheVerifiedCopy(sig string, data []byte) (string, error) {
	s.cacheOnce.Do(func() { s.cacheErr = s.prepareCacheDir() })
	if s.cacheErr != nil {
		return "", s.cacheErr
	}

	root, err := os.OpenRoot(s.cacheDir)
	if err != nil {
		return "", fmt.Errorf("open plugin cache folder: %w", err)
	}
	defer func() { _ = root.Close() }()

	name := strings.TrimPrefix(sig, "sha256:") + ".so"
	if !reusableCopy(root, name, sig) {
		// Replace whatever is there with the verified bytes. A missing file is not an error here.
		_ = root.RemoveAll(name)
		if err := writeCopy(root, name, data); err != nil {
			return "", err
		}
	}
	return filepath.Join(s.cacheDir, name), nil
}

// reusableCopy reports whether name is an existing verified copy that can be loaded as it is.
func reusableCopy(root *os.Root, name, sig string) bool {
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != cacheCopyMode || !ownedBy(info, os.Geteuid()) {
		return false
	}
	f, err := root.Open(name)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxPluginFileBytes+1))
	return err == nil && len(data) <= maxPluginFileBytes && hashPluginBytes(data) == sig
}

// writeCopy writes data to a temporary file in the cache folder, makes it read-only, and renames it to name.
// The rename is the last step, so a copy is never seen half written.
func writeCopy(root *os.Root, name string, data []byte) error {
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return fmt.Errorf("generate plugin copy name: %w", err)
	}
	tmp := ".plugin-" + hex.EncodeToString(suffix[:])

	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create plugin copy: %w", err)
	}
	writeErr := writeAndSeal(f, data)
	if writeErr == nil {
		writeErr = root.Rename(tmp, name)
	}
	if writeErr != nil {
		_ = root.Remove(tmp)
		return fmt.Errorf("write plugin copy: %w", writeErr)
	}
	return nil
}

// writeAndSeal writes and syncs the bytes, sets the copy mode and closes the file.
func writeAndSeal(f *os.File, data []byte) error {
	_, werr := f.Write(data)
	serr := f.Sync()
	cerr := f.Chmod(cacheCopyMode)
	if err := f.Close(); err != nil {
		return err
	}
	return errors.Join(werr, serr, cerr)
}

// openCachedCopy opens the verified copy as a plugin. The original path is never opened. A failure here is usually a
// data folder that does not allow running program files, so the status names the setting that fixes it.
func (s *PluginLoaderService) openCachedCopy(cachedPath string) (*plugin.Plugin, error) {
	p, err := s.openPluginFile(cachedPath)
	if err == nil {
		return p, nil
	}
	msg := fmt.Sprintf("Could not load the plugin from %s. If that folder does not allow running program files (for example a noexec mount), set CHARON_PLUGIN_CACHE_DIR to a folder that does.", s.cacheDir)
	logger.Log().WithError(err).Warn(msg)
	return nil, errors.New(msg)
}
