package services

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"

	"strconv"
	"strings"
	"time"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

type LogService struct {
	LogDir string
}

func NewLogService(cfg *config.Config) *LogService {
	// Assuming logs are in data/logs relative to app root
	logDir := filepath.Join(filepath.Dir(cfg.DatabasePath), "logs")
	return &LogService{LogDir: logDir}
}

type LogFile struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

func (s *LogService) ListLogs() ([]LogFile, error) {
	entries, err := os.ReadDir(s.LogDir)
	if err != nil {
		// If directory doesn't exist, return empty list instead of error
		if os.IsNotExist(err) {
			return []LogFile{}, nil
		}
		return nil, err
	}

	var logs []LogFile
	for _, entry := range entries {
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".log") || strings.Contains(entry.Name(), ".log.")) {
			info, err := entry.Info()
			if err != nil {
				continue
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

// GetLogPath returns the absolute path to a log file if it exists and is valid
func (s *LogService) GetLogPath(filename string) (string, error) {
	clean := filepath.Base(filename)
	path := filepath.Join(s.LogDir, clean)

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		return "", err
	}

	return path, nil
}

// QueryLogs parses and filters logs from a specific file
func (s *LogService) QueryLogs(filename string, filter models.LogFilter) ([]models.CaddyAccessLog, int64, error) {
	path, err := s.GetLogPath(filename)
	if err != nil {
		return nil, 0, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()

	var logs []models.CaddyAccessLog
	var totalMatches int64 = 0

	// Read file line by line
	// TODO: For large files, reading from end or indexing would be better
	// Current implementation reads all lines, filters, then paginates
	// This is acceptable for rotated logs (max 10MB)
	scanner := bufio.NewScanner(file)

	// We'll store all matching logs first, then slice for pagination
	// This is memory intensive for very large matches but ensures correct sorting/filtering
	// Since we want latest first, we'll prepend or reverse later.
	// Actually, appending and then reversing is better.

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry models.CaddyAccessLog
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Handle non-JSON logs (like cpmp.log)
			// Try to parse standard Go log format: "2006/01/02 15:04:05 msg"
			parts := strings.SplitN(line, " ", 3)
			if len(parts) >= 3 {
				// Try parsing date/time
				ts, err := time.Parse("2006/01/02 15:04:05", parts[0]+" "+parts[1])
				if err == nil {
					entry.Ts = float64(ts.Unix())
					entry.Msg = parts[2]
				} else {
					entry.Msg = line
				}
			} else {
				entry.Msg = line
			}
			entry.Level = "INFO" // Default level for plain logs
		}

		if s.matchesFilter(entry, filter) {
			logs = append(logs, entry)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}

	// Reverse logs to show newest first
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}

	totalMatches = int64(len(logs))

	// Apply pagination
	start := filter.Offset
	end := start + filter.Limit

	if start >= len(logs) {
		return []models.CaddyAccessLog{}, totalMatches, nil
	}
	if end > len(logs) {
		end = len(logs)
	}

	return logs[start:end], totalMatches, nil
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
