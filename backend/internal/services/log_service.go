package services

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
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

// ReadLog reads the last N lines of a log file
func (s *LogService) ReadLog(filename string, lines int) ([]string, error) {
	// Basic sanitization
	clean := filepath.Base(filename)
	path := filepath.Join(s.LogDir, clean)

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// This is a simple implementation. For huge files, we should seek to the end and read backwards.
	// For now, reading the whole file and taking the last N lines is "okay" for rotated logs (10MB max).
	var result []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		result = append(result, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(result) > lines {
		return result[len(result)-lines:], nil
	}
	return result, nil
}
