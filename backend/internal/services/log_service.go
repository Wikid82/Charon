package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// maxLogLineBytes caps a single log line at 1MB. Longer lines are counted as
// skipped and discarded chunk-by-chunk without ever being accumulated in full.
const maxLogLineBytes = 1 << 20

type LogService struct {
	LogDir      string
	CaddyLogDir string
}

func NewLogService(cfg *config.Config) *LogService {
	// Assuming logs are in data/logs relative to app root
	logDir := filepath.Join(filepath.Dir(cfg.DatabasePath), "logs")
	return &LogService{LogDir: logDir, CaddyLogDir: cfg.CaddyLogDir}
}

func (s *LogService) logDirs() []string {
	seen := make(map[string]bool)
	var dirs []string

	addDir := func(dir string) {
		clean := filepath.Clean(dir)
		if clean == "." || clean == "" {
			return
		}
		if !seen[clean] {
			seen[clean] = true
			dirs = append(dirs, clean)
		}
	}

	addDir(s.LogDir)
	if s.CaddyLogDir != "" {
		addDir(s.CaddyLogDir)
	}

	if accessLogPath := os.Getenv("CHARON_CADDY_ACCESS_LOG"); accessLogPath != "" {
		addDir(filepath.Dir(accessLogPath))
	}

	return dirs
}

type LogFile struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

func (s *LogService) ListLogs() ([]LogFile, error) {
	var logs []LogFile
	seen := make(map[string]bool)
	for _, dir := range s.logDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, entry := range entries {
			if entry.IsDir() || !isLogName(entry.Name()) {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}
			// Handle symlinks + deduplicate files (e.g., charon.log and cpmp.log (legacy name) pointing to same file)
			entryPath := filepath.Join(dir, entry.Name())
			resolved, err := filepath.EvalSymlinks(entryPath)
			if err == nil {
				if seen[resolved] {
					continue
				}
				seen[resolved] = true
			}
			logs = append(logs, LogFile{
				Name:    entry.Name(),
				Size:    info.Size(),
				ModTime: info.ModTime().Format(time.RFC3339),
			})
		}
	}

	return logs, nil
}

// ErrInvalidFilename is the sentinel for traversal-shaped or
// containment-violating filename rejections. Handlers map it to 400 via
// errors.Is instead of matching error strings.
var ErrInvalidFilename = errors.New("invalid filename: path traversal attempt detected")

// isLogName reports whether a directory entry name follows the servable
// log-file naming rules (".log" suffix or ".log." infix, e.g. rotations).
func isLogName(name string) bool {
	return strings.HasSuffix(name, ".log") || strings.Contains(name, ".log.")
}

// listServableNames returns the allowlist of servable filenames built from
// RAW directory entries of every log dir. It deliberately does NOT reuse
// ListLogs(): that method dedups symlink aliases by resolved path, which
// would nondeterministically drop one name of a legitimate alias pair such
// as charon.log/cpmp.log. Both alias names must remain servable.
func (s *LogService) listServableNames() (map[string]bool, error) {
	names := make(map[string]bool)
	for _, dir := range s.logDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read log dir %q: %w", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !isLogName(entry.Name()) {
				continue
			}
			names[entry.Name()] = true
		}
	}
	return names, nil
}

// GetLogPath validates a client-supplied filename and returns the
// symlink-RESOLVED absolute path to the log file. Callers must open exactly
// the returned path: because it contains no client-influenced symlink
// component, a post-check symlink swap cannot redirect the open (TOCTOU).
//
// Defense-in-depth layers (spec §5.7):
//  1. shape check (filepath.Base equality; empty/"."/".."/percent-encoded
//     separators rejected) -> ErrInvalidFilename;
//  2. allowlist against raw directory entries -> unknown names get
//     os.ErrNotExist without opening any file;
//  3. joined-path prefix containment in a cleaned log dir;
//  4. both-sides EvalSymlinks containment: the resolved file must live inside
//     a resolved log dir. This invariant is never relaxed under any
//     configuration (R6).
func (s *LogService) GetLogPath(filename string) (string, error) {
	invalid := func() (string, error) {
		return "", fmt.Errorf("get log path %q: %w", filename, ErrInvalidFilename)
	}
	if filename == "" || filename == "." || filename == ".." {
		return invalid()
	}
	if filename != filepath.Base(filename) {
		return invalid()
	}
	// Reject literal percent-encoded path separators: they cannot appear in a
	// legitimate log filename and only serve double-decode traversal attempts.
	if lower := strings.ToLower(filename); strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") {
		return invalid()
	}

	servable, err := s.listServableNames()
	if err != nil {
		return "", fmt.Errorf("list servable log names: %w", err)
	}
	if !servable[filename] {
		return "", fmt.Errorf("log file %q not found: %w", filename, os.ErrNotExist)
	}

	var containmentErr error
	for _, dir := range s.logDirs() {
		baseDir := filepath.Clean(dir)
		path := filepath.Join(baseDir, filename)
		if !strings.HasPrefix(path, baseDir+string(os.PathSeparator)) {
			continue
		}

		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			continue // missing in this dir, or dangling symlink
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			continue // never serve directories or special files
		}
		if s.resolvedInLogDirs(resolved) {
			return resolved, nil
		}
		// Found but resolves outside every resolved log dir: refuse (R6).
		containmentErr = fmt.Errorf("log file %q resolves outside the log directories: %w", filename, ErrInvalidFilename)
	}

	if containmentErr != nil {
		return "", containmentErr
	}
	return "", fmt.Errorf("log file %q not found: %w", filename, os.ErrNotExist)
}

// resolvedInLogDirs reports whether the already-resolved file path lies
// inside the EvalSymlinks-resolved form of one of the log dirs. Resolving
// BOTH sides also covers layouts where a log dir itself (or an ancestor)
// is a symlink.
func (s *LogService) resolvedInLogDirs(resolved string) bool {
	parent := filepath.Dir(resolved)
	for _, dir := range s.logDirs() {
		resolvedDir, err := filepath.EvalSymlinks(filepath.Clean(dir))
		if err != nil {
			continue
		}
		if parent == resolvedDir || strings.HasPrefix(parent, resolvedDir+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

// QueryLogs parses, filters, sorts, and paginates logs from a specific file.
// It returns the requested page, the total number of filtered matches, and
// the number of lines skipped as corrupted or oversized (R4).
func (s *LogService) QueryLogs(filename string, filter models.LogFilter) ([]models.CaddyAccessLog, int64, int64, error) {
	path, err := s.GetLogPath(filename)
	if err != nil {
		return nil, 0, 0, err
	}

	// #nosec G304 -- path is the symlink-resolved location returned by
	// GetLogPath, which enforces filepath.Base equality, a raw directory-entry
	// allowlist, and EvalSymlinks containment inside the configured log dirs.
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("open log file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.Log().WithError(err).Warn("failed to close log file after reading")
		}
	}()

	// Read line by line with a hard per-line cap. bufio.Scanner is not used
	// because a single oversized line would abort the whole query; and
	// ReadBytes('\n') is not used because it buffers the entire line before
	// returning. ReadSlice hands back the reader's internal buffer chunk by
	// chunk, so an over-cap line is counted and discarded without ever being
	// accumulated beyond the cap.
	// The full filtered match set is held in memory to sort and count; this is
	// bounded by the rotation cap (~10MB) and acceptable for rotated logs.
	var (
		logs         []models.CaddyAccessLog
		skippedLines int64
		reader       = bufio.NewReader(file)
		scratch      = make([]byte, 0, 64*1024)
		overCap      bool
	)

	processLine := func(line []byte) {
		if len(line) == 0 {
			return
		}
		entry, ok := parseLogLine(line)
		if !ok {
			skippedLines++
			return
		}
		if s.matchesFilter(entry, filter) {
			logs = append(logs, entry)
		}
	}

	for {
		chunk, rerr := reader.ReadSlice('\n')
		if len(chunk) > 0 && !overCap {
			// +1 tolerates the trailing newline still present in the chunk.
			if len(scratch)+len(chunk) > maxLogLineBytes+1 {
				overCap = true
				scratch = scratch[:0] // discard; do not accumulate past the cap
			} else {
				scratch = append(scratch, chunk...)
			}
		}
		if rerr == bufio.ErrBufferFull {
			continue // same line keeps going; loop for the next chunk
		}
		if rerr != nil && rerr != io.EOF {
			return nil, 0, 0, fmt.Errorf("read log file: %w", rerr)
		}

		// End of line (newline consumed) or EOF reached.
		line := bytes.TrimRight(scratch, "\r\n")
		if overCap || len(line) > maxLogLineBytes {
			skippedLines++
		} else {
			processLine(line)
		}
		scratch = scratch[:0]
		overCap = false

		if rerr == io.EOF {
			break
		}
	}

	sortEntries(logs, filter.SortBy, filter.Sort)

	totalMatches := int64(len(logs))

	// Apply pagination
	start := filter.Offset
	end := start + filter.Limit

	if start >= len(logs) {
		return []models.CaddyAccessLog{}, totalMatches, skippedLines, nil
	}
	if end > len(logs) {
		end = len(logs)
	}

	return logs[start:end], totalMatches, skippedLines, nil
}

// parseLogLine parses a single log line. It returns ok=false only for
// corrupted lines per R4: the line failed JSON parsing AND is either invalid
// UTF-8 or contains a NUL byte. Any other non-JSON line takes the plain-text
// fallback (legitimate charon.log lines).
func parseLogLine(line []byte) (models.CaddyAccessLog, bool) {
	var entry models.CaddyAccessLog
	if err := json.Unmarshal(line, &entry); err == nil {
		return entry, true
	}
	if !utf8.Valid(line) || bytes.IndexByte(line, 0) >= 0 {
		return entry, false
	}

	// Handle non-JSON logs (like cpmp.log, legacy name for Charon).
	// Try to parse standard Go log format: "2006/01/02 15:04:05 msg".
	text := string(line)
	parts := strings.SplitN(text, " ", 3)
	entry.Msg = text
	entry.Level = "INFO" // Default level for plain logs
	if len(parts) >= 3 {
		// Try parsing date/time; if parsing fails, keep the original line as the Msg
		if ts, perr := time.Parse("2006/01/02 15:04:05", parts[0]+" "+parts[1]); perr == nil {
			entry.Ts = float64(ts.Unix())
			entry.Msg = parts[2]
		}
	}
	return entry, true
}

// levelRank maps a log level to its severity rank for sorting (R3):
// debug < info < warn < error; unknown levels rank lowest.
func levelRank(level string) int {
	switch strings.ToLower(level) {
	case "debug":
		return 0
	case "info":
		return 1
	case "warn":
		return 2
	case "error":
		return 3
	default:
		return -1
	}
}

// sortEntries stably sorts logs by the given field and direction (R1).
// Entries with equal keys fall through to Ts descending so pages stay
// deterministic. An empty sortBy/dir falls back to ts/desc.
func sortEntries(logs []models.CaddyAccessLog, sortBy, dir string) {
	asc := dir == "asc"

	compare := func(a, b models.CaddyAccessLog) int {
		switch sortBy {
		case "level":
			return levelRank(a.Level) - levelRank(b.Level)
		case "method":
			return strings.Compare(strings.ToLower(a.Request.Method), strings.ToLower(b.Request.Method))
		case "uri":
			return strings.Compare(strings.ToLower(a.Request.URI), strings.ToLower(b.Request.URI))
		case "status":
			return a.Status - b.Status
		default: // "ts"
			switch {
			case a.Ts < b.Ts:
				return -1
			case a.Ts > b.Ts:
				return 1
			default:
				return 0
			}
		}
	}

	sort.SliceStable(logs, func(i, j int) bool {
		c := compare(logs[i], logs[j])
		if c == 0 {
			return logs[i].Ts > logs[j].Ts // ts-descending tiebreaker
		}
		if asc {
			return c < 0
		}
		return c > 0
	})
}

func (s *LogService) matchesFilter(entry models.CaddyAccessLog, filter models.LogFilter) bool {
	// Status Filter
	if filter.Status != "" {
		statusStr := strconv.Itoa(entry.Status)
		if strings.HasSuffix(filter.Status, "xx") {
			// Handle 2xx, 4xx, 5xx
			prefix := filter.Status[:1]
			if !strings.HasPrefix(statusStr, prefix) {
				return false
			}
		} else if statusStr != filter.Status {
			return false
		}
	}

	// Level Filter
	if filter.Level != "" {
		if !strings.EqualFold(entry.Level, filter.Level) {
			return false
		}
	}

	// Host Filter
	if filter.Host != "" {
		if !strings.Contains(strings.ToLower(entry.Request.Host), strings.ToLower(filter.Host)) {
			return false
		}
	}

	// Search Filter (generic text search)
	if filter.Search != "" {
		term := strings.ToLower(filter.Search)
		// Search in common fields
		if !strings.Contains(strings.ToLower(entry.Request.URI), term) &&
			!strings.Contains(strings.ToLower(entry.Request.Method), term) &&
			!strings.Contains(strings.ToLower(entry.Request.RemoteIP), term) &&
			!strings.Contains(strings.ToLower(entry.Msg), term) {
			return false
		}
	}

	return true
}
