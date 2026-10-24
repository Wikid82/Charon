package crowdsec

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

const (
	// bouncerKeyFileMode is the mode of the saved key: only the owner may read it.
	bouncerKeyFileMode fs.FileMode = 0o600
	// bouncerKeyDirMode is the mode of a key directory created by SaveBouncerKey.
	bouncerKeyDirMode fs.FileMode = 0o750
	// bouncerKeyTempPattern is the temporary sibling of the key file. Its "bouncer_key.tmp-" prefix is one of the
	// secret names (see IsSecretPath), so a crash leftover is never exported, listed or imported.
	bouncerKeyTempPattern = "bouncer_key.tmp-*"
	// bouncerKeyStaleTempAge is how old a leftover temporary file must be before a later save removes it.
	bouncerKeyStaleTempAge = 10 * time.Minute
)

// SaveBouncerKey saves the bouncer key to path as one line with mode 0600, replacing any previous file. The
// key is written to a temporary sibling that is renamed over path, so a failed save leaves the old key intact.
// The rename is the only replacement: the live file is written in place only when the rename is refused with
// EBUSY, EXDEV or EROFS (a bind-mounted single file, a cross-device path or a read-only mount), and then its mode
// is forced to 0600 as well. A symlink at path is replaced by a regular file; its target is not written. A blank
// key is refused. The key is never logged.
func SaveBouncerKey(path, key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("cannot save empty key")
	}
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), bouncerKeyDirMode); err != nil {
		return fmt.Errorf("create bouncer key directory: %w", err)
	}
	removeLegacyKeyTemp(path)

	_, err := WriteFileAtomic(path, []byte(key+"\n"), bouncerKeyFileMode, AtomicWriteOptions{
		TempPattern:         bouncerKeyTempPattern,
		KeepOwner:           false, // the key is Charon's own file, so the owner is not kept
		InPlaceFallback:     true,
		ForceMode:           true,
		SweepTempsOlderThan: bouncerKeyStaleTempAge,
	})
	if err != nil {
		return fmt.Errorf("save bouncer key: %w", err)
	}
	return nil
}

// removeLegacyKeyTemp removes the fixed-name temporary file that earlier versions wrote next to the key
// ("<path>.tmp"), when it is a regular file. A directory or any other entry with that name is left alone.
func removeLegacyKeyTemp(path string) {
	legacy := path + ".tmp"
	info, err := os.Lstat(legacy)
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	if err := os.Remove(legacy); err != nil {
		logger.Log().WithError(err).WithField("path", util.SanitizeForLog(legacy)).Debug("could not remove a legacy bouncer key temporary file")
	}
}
