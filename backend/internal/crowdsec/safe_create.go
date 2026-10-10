package crowdsec

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrSymlinkInPath is returned when an archive entry would be written through a symlink.
var ErrSymlinkInPath = errors.New("refusing to write through a symlink")

// CreateFileNoSymlink creates (or truncates) dest, which must be inside root, creating missing parent
// directories. It refuses to proceed if dest or any existing component between root and dest is a
// symlink, so archive extraction over a live tree can never be redirected outside root.
func CreateFileNoSymlink(root, dest string, parentMode, fileMode os.FileMode) (*os.File, error) {
	root = filepath.Clean(root)
	rel, err := filepath.Rel(root, filepath.Clean(dest))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("path escapes target: %s", rel)
	}
	cur := root
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		cur = filepath.Join(cur, part)
		info, lerr := os.Lstat(cur)
		if errors.Is(lerr, fs.ErrNotExist) {
			break
		}
		if lerr != nil {
			return nil, fmt.Errorf("inspect %s: %w", rel, withoutPath(lerr))
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: %s", ErrSymlinkInPath, rel)
		}
	}
	if err = os.MkdirAll(filepath.Dir(dest), parentMode); err != nil { //nolint:gosec // G703: dest verified contained under root and symlink-free above
		return nil, fmt.Errorf("mkdir parent: %w", withoutPath(err))
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode) //nolint:gosec // G304,G703: dest verified contained under root and symlink-free above
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", rel, withoutPath(err))
	}
	return f, nil
}
