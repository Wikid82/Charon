package services

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/robfig/cron/v3"
)

type BackupService struct {
	DataDir   string
	BackupDir string
	Cron      *cron.Cron
}

type BackupFile struct {
	Filename string    `json:"filename"`
	Size     int64     `json:"size"`
	Time     time.Time `json:"time"`
}

func NewBackupService(cfg *config.Config) *BackupService {
	// Ensure backup directory exists
	backupDir := filepath.Join(filepath.Dir(cfg.DatabasePath), "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		fmt.Printf("Failed to create backup directory: %v\n", err)
	}

	s := &BackupService{
		DataDir:   filepath.Dir(cfg.DatabasePath), // e.g. /app/data
		BackupDir: backupDir,
		Cron:      cron.New(),
	}

	// Schedule daily backup at 3 AM
	_, err := s.Cron.AddFunc("0 3 * * *", func() {
		fmt.Println("Starting scheduled backup...")
		if name, err := s.CreateBackup(); err != nil {
			fmt.Printf("Scheduled backup failed: %v\n", err)
		} else {
			fmt.Printf("Scheduled backup created: %s\n", name)
		}
	})
	if err != nil {
		fmt.Printf("Failed to schedule backup: %v\n", err)
	}
	s.Cron.Start()

	return s
}

// ListBackups returns all backup files sorted by time (newest first)
func (s *BackupService) ListBackups() ([]BackupFile, error) {
	entries, err := os.ReadDir(s.BackupDir)
	if err != nil {
		return nil, err
	}

	var backups []BackupFile
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".zip") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			backups = append(backups, BackupFile{
				Filename: entry.Name(),
				Size:     info.Size(),
				Time:     info.ModTime(),
			})
		}
	}

	// Sort newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Time.After(backups[j].Time)
	})

	return backups, nil
}

// CreateBackup creates a zip archive of the database and caddy data
func (s *BackupService) CreateBackup() (string, error) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("backup_%s.zip", timestamp)
	zipPath := filepath.Join(s.BackupDir, filename)

	outFile, err := os.Create(zipPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	w := zip.NewWriter(outFile)
	defer w.Close()

	// Files/Dirs to backup
	// 1. Database
	dbPath := filepath.Join(s.DataDir, "cpm.db")
	if err := s.addToZip(w, dbPath, "cpm.db"); err != nil {
		return "", fmt.Errorf("backup db: %w", err)
	}

	// 2. Caddy Data (Certificates, etc)
	// We walk the 'caddy' subdirectory
	caddyDir := filepath.Join(s.DataDir, "caddy")
	if err := s.addDirToZip(w, caddyDir, "caddy"); err != nil {
		// It's possible caddy dir doesn't exist yet, which is fine
		fmt.Printf("Warning: could not backup caddy dir: %v\n", err)
	}

	return filename, nil
}

func (s *BackupService) addToZip(w *zip.Writer, srcPath, zipPath string) error {
	file, err := os.Open(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	f, err := w.Create(zipPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, file)
	return err
}

func (s *BackupService) addDirToZip(w *zip.Writer, srcDir, zipBase string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		zipPath := filepath.Join(zipBase, relPath)
		return s.addToZip(w, path, zipPath)
	})
}

// DeleteBackup removes a backup file
func (s *BackupService) DeleteBackup(filename string) error {
	// Basic sanitization to prevent directory traversal
	clean := filepath.Base(filename)
	return os.Remove(filepath.Join(s.BackupDir, clean))
}

// GetBackupPath returns the full path to a backup file (for downloading)
func (s *BackupService) GetBackupPath(filename string) string {
	clean := filepath.Base(filename)
	return filepath.Join(s.BackupDir, clean)
}

// RestoreBackup restores the database and caddy data from a zip archive
func (s *BackupService) RestoreBackup(filename string) error {
	// 1. Verify backup exists
	srcPath := filepath.Join(s.BackupDir, filename)
	if _, err := os.Stat(srcPath); err != nil {
		return err
	}

	// 2. Unzip to DataDir (overwriting)
	return s.unzip(srcPath, s.DataDir)
}

func (s *BackupService) unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		// Check for ZipSlip
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}
