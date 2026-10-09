package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const (
	// maxCrowdsecFileBytes caps the content of a single configuration file write.
	maxCrowdsecFileBytes = 1 << 20
	// maxCrowdsecWriteBodyBytes caps the whole request body; it leaves room for JSON escaping of a
	// maximum-size file.
	maxCrowdsecWriteBodyBytes = 8 << 20

	// writeTempPrefix names in-flight temp files; such names are never writable or listed targets.
	writeTempPrefix = ".charon-write-"
)

const errFileCannotBeEdited = "file cannot be edited"

var (
	// writableExtensions is the allowlist of file types the editor may save.
	writableExtensions = map[string]struct{}{
		".yaml": {}, ".yml": {}, ".json": {}, ".txt": {}, ".conf": {},
	}

	// hiddenSecretNames are lowercase base names of secret files that are neither listed nor readable nor
	// writable through the editor.
	hiddenSecretNames = map[string]struct{}{
		"bouncer_key":                 {},
		"local_api_credentials.yaml":  {},
		"online_api_credentials.yaml": {},
	}
)

// fileError carries the HTTP status and client-facing message for a rejected file request.
type fileError struct {
	status int
	msg    string
}

func (e *fileError) respond(c *gin.Context) {
	c.JSON(e.status, gin.H{"error": e.msg})
}

func badRequest(msg string) *fileError { return &fileError{status: http.StatusBadRequest, msg: msg} }

// cleanRelPath normalizes a client-supplied relative path and rejects anything that is not a plain
// path below the data directory.
func cleanRelPath(raw string) (string, *fileError) {
	if raw == "" {
		return "", badRequest("path required")
	}
	if strings.ContainsRune(raw, 0) || !filepath.IsLocal(raw) {
		return "", badRequest("invalid path")
	}
	clean := filepath.Clean(raw)
	if clean == "." {
		return "", badRequest("invalid path")
	}
	return clean, nil
}

// isReadable reports whether rel may be listed and read: engine-owned state (databases, the data and
// hub_cache trees) and secrets (see hiddenSecretNames) are hidden, everything else stays visible.
func isReadable(rel string) bool {
	slash := filepath.ToSlash(rel)
	if slash == "" || slash == "." || crowdsec.IsEngineOwnedPath(slash) {
		return false
	}
	base := strings.ToLower(path.Base(slash))
	if _, hidden := hiddenSecretNames[base]; hidden {
		return false
	}
	if matched, _ := path.Match("*.db*", base); matched {
		return false
	}
	return true
}

// isWritable is the stricter predicate for saves. It is always a subset of isReadable so that every
// file the editor can write is also listed.
func isWritable(rel string) bool {
	if !isReadable(rel) {
		return false
	}
	slash := filepath.ToSlash(rel)
	base := path.Base(slash)
	if _, ok := writableExtensions[strings.ToLower(path.Ext(base))]; !ok {
		return false
	}
	if strings.HasPrefix(base, writeTempPrefix) {
		return false
	}
	// The hub tree is managed by cscli, both at the top level and under config/.
	top, _, _ := strings.Cut(slash, "/")
	return top != "hub" && !strings.HasPrefix(slash, "config/hub/")
}

// pathWithin reports whether p is root or lies below it.
func pathWithin(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// resolveReadable validates rel for reading and returns the file path to open. Symlinks are resolved
// and the result must stay inside the data directory and remain readable.
func resolveReadable(dataDir, rel string) (string, *fileError) {
	if !isReadable(rel) {
		return "", badRequest("invalid path")
	}
	root, err := filepath.EvalSymlinks(dataDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", &fileError{status: http.StatusNotFound, msg: "file not found"}
		}
		return "", &fileError{status: http.StatusInternalServerError, msg: "failed to resolve data directory"}
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(dataDir, rel))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", &fileError{status: http.StatusNotFound, msg: "file not found"}
		}
		return "", badRequest("invalid path")
	}
	if !pathWithin(root, resolved) {
		return "", badRequest("invalid path")
	}
	resolvedRel, err := filepath.Rel(root, resolved)
	if err != nil || !isReadable(resolvedRel) {
		return "", badRequest("invalid path")
	}
	info, err := os.Stat(resolved)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", &fileError{status: http.StatusNotFound, msg: "file not found"}
		}
		return "", badRequest("invalid path")
	}
	if !info.Mode().IsRegular() {
		return "", badRequest("invalid path")
	}
	return resolved, nil
}

// resolveWritable validates rel for saving and returns the lexical path to write. Existing symlinks
// anywhere along the path, and non-regular targets, are rejected.
func resolveWritable(dataDir, rel string) (string, *fileError) {
	if !isReadable(rel) {
		return "", badRequest(errFileCannotBeEdited)
	}
	if !isWritable(rel) {
		if _, ok := writableExtensions[strings.ToLower(filepath.Ext(rel))]; !ok {
			return "", badRequest("file type not allowed")
		}
		return "", badRequest(errFileCannotBeEdited)
	}
	cur := dataDir
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i, part := range parts {
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if errors.Is(err, fs.ErrNotExist) {
			break
		}
		if err != nil {
			return "", &fileError{status: http.StatusInternalServerError, msg: "failed to inspect path"}
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", badRequest(errFileCannotBeEdited)
		}
		last := i == len(parts)-1
		if last && !info.Mode().IsRegular() {
			return "", badRequest("invalid path")
		}
		if !last && !info.IsDir() {
			return "", badRequest("invalid path")
		}
	}
	return filepath.Join(dataDir, rel), nil
}

// checkSize enforces the per-file content limit.
func checkSize(content string) *fileError {
	if len(content) > maxCrowdsecFileBytes {
		return &fileError{status: http.StatusRequestEntityTooLarge, msg: "file too large"}
	}
	return nil
}

// checkYAML verifies that YAML files parse; other allowed types are not inspected.
func checkYAML(rel, content string) *fileError {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".yaml", ".yml":
		dec := yaml.NewDecoder(strings.NewReader(content))
		for {
			var doc any
			err := dec.Decode(&doc)
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return badRequest("invalid YAML")
			}
		}
	}
	return nil
}

// atomicWriteFile writes data to a temp file beside dst and renames it into place. The temp file is
// removed on any failure.
func atomicWriteFile(dst string, data []byte, mode os.FileMode) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(dst), writeTempPrefix+"*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()
	if err = tmp.Chmod(mode); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err = tmp.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err = os.Rename(tmpName, dst); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// ListFiles returns a flat list of the readable files under the CrowdSec DataDir.
func (h *CrowdsecHandler) ListFiles(c *gin.Context) {
	files := []string{}
	if _, err := os.Stat(h.DataDir); os.IsNotExist(err) {
		c.JSON(http.StatusOK, gin.H{"files": files})
		return
	}
	err := filepath.WalkDir(h.DataDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Permission errors (e.g. lost+found) should not abort the walk
			if os.IsPermission(err) {
				fileErrorLog(err).WithField("path", sanitizeForLog(p)).Debug("Skipping inaccessible path during list")
				return filepath.SkipDir
			}
			return err
		}
		rel, err := filepath.Rel(h.DataDir, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if rel != "." && crowdsec.IsEngineOwnedPath(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isReadable(rel) {
			return nil
		}
		// Symlinked entries are listed only when they can actually be read.
		if d.Type()&fs.ModeSymlink != 0 {
			if _, ferr := resolveReadable(h.DataDir, rel); ferr != nil {
				return nil
			}
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		fileErrorLog(err).Warn("crowdsec file listing failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list files"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}

var errFileTooLarge = errors.New("file exceeds read limit")

// readCapped reads at most maxCrowdsecFileBytes from p, failing with errFileTooLarge when the file is
// larger so an oversized file is never loaded into memory.
func readCapped(p string) ([]byte, error) {
	f, err := os.Open(p) // #nosec G304 -- p is validated and symlink-resolved to stay inside DataDir by resolveReadable
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxCrowdsecFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if len(data) > maxCrowdsecFileBytes {
		return nil, errFileTooLarge
	}
	return data, nil
}

// ReadFile returns the contents of a specific file under DataDir. Query param 'path' required.
func (h *CrowdsecHandler) ReadFile(c *gin.Context) {
	rel, ferr := cleanRelPath(c.Query("path"))
	if ferr != nil {
		ferr.respond(c)
		return
	}
	p, ferr := resolveReadable(h.DataDir, rel)
	if ferr != nil {
		ferr.respond(c)
		return
	}
	data, err := readCapped(p)
	if err != nil {
		switch {
		case os.IsNotExist(err):
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		case errors.Is(err, errFileTooLarge):
			(&fileError{status: http.StatusRequestEntityTooLarge, msg: "file too large"}).respond(c)
		default:
			fileErrorLog(err).Warn("crowdsec file read failed")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": string(data)})
}

// WriteFile saves content to an allowed file under DataDir, first copying only the replaced file into
// the file-backup namespace. DataDir itself is never renamed or emptied.
// JSON body: { "path": "relative/path.conf", "content": "..." }
func (h *CrowdsecHandler) WriteFile(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCrowdsecWriteBodyBytes)
	var payload struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(c.Request.Body).Decode(&payload); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			(&fileError{status: http.StatusRequestEntityTooLarge, msg: "file too large"}).respond(c)
			return
		}
		badRequest("invalid payload").respond(c)
		return
	}
	rel, ferr := cleanRelPath(payload.Path)
	if ferr != nil {
		ferr.respond(c)
		return
	}
	if ferr = checkSize(payload.Content); ferr != nil {
		ferr.respond(c)
		return
	}

	h.dataMu.Lock()
	defer h.dataMu.Unlock()

	target, ferr := resolveWritable(h.DataDir, rel)
	if ferr != nil {
		ferr.respond(c)
		return
	}
	if ferr = checkYAML(rel, payload.Content); ferr != nil {
		ferr.respond(c)
		return
	}

	mode := os.FileMode(0o600)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
	}
	backupDir, err := crowdsec.BackupFile(h.DataDir, rel)
	if err != nil {
		fileErrorLog(err).Warn("crowdsec file backup failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create backup"})
		return
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare dir"})
		return
	}
	if err := atomicWriteFile(target, []byte(payload.Content), mode); err != nil {
		fileErrorLog(err).Warn("crowdsec file write failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write file"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "written", "backup": backupDir})
}

// fileErrorLog returns a log entry carrying err with control characters
// stripped, since file operation errors embed request-derived paths.
func fileErrorLog(err error) *logrus.Entry {
	return logger.Log().WithField("error", sanitizeForLog(err.Error()))
}
