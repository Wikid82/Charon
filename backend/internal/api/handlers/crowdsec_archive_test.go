package handlers

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// TestRequireTarGz tests the requireTarGz helper function.
func TestRequireTarGz(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"tar.gz extension", "/path/to/archive.tar.gz", false},
		{"TAR.GZ uppercase", "/path/to/ARCHIVE.TAR.GZ", false},
		{"zip extension", "/path/to/archive.zip", true},
		{"ZIP uppercase", "/path/to/ARCHIVE.ZIP", true},
		{"tgz extension", "/path/to/archive.tgz", true},
		{"rar extension", "/path/to/archive.rar", true},
		{"no extension", "/path/to/archive", true},
		{"txt extension", "/path/to/archive.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := requireTarGz(tt.path)
			if !tt.wantErr {
				if err != nil {
					t.Errorf("requireTarGz() unexpected error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != "only .tar.gz archives are supported" {
				t.Errorf("requireTarGz() error = %v, want the fixed archive format message", err)
			}
		})
	}
}

// TestCalculateUncompressedSize tests the calculateUncompressedSize helper function.
func TestCalculateUncompressedSize(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create a valid tar.gz archive with known content
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	testContent := "This is test content for the archive with some additional text to give it size."

	// Create tar.gz file
	// #nosec G304 -- Test file path is controlled in test scope
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive file: %v", err)
	}

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Add a file to the archive
	hdr := &tar.Header{
		Name:     "test.txt",
		Mode:     0o644,
		Size:     int64(len(testContent)),
		Typeflag: tar.TypeReg,
	}
	if writeHeaderErr := tw.WriteHeader(hdr); writeHeaderErr != nil {
		t.Fatalf("Failed to write tar header: %v", writeHeaderErr)
	}
	if _, writeErr := tw.Write([]byte(testContent)); writeErr != nil {
		t.Fatalf("Failed to write tar content: %v", writeErr)
	}

	// Add a second file
	content2 := "Second file content."
	hdr2 := &tar.Header{
		Name:     "test2.txt",
		Mode:     0o644,
		Size:     int64(len(content2)),
		Typeflag: tar.TypeReg,
	}
	if writeHeaderErr := tw.WriteHeader(hdr2); writeHeaderErr != nil {
		t.Fatalf("Failed to write tar header 2: %v", writeHeaderErr)
	}
	if _, writeErr := tw.Write([]byte(content2)); writeErr != nil {
		t.Fatalf("Failed to write tar content 2: %v", writeErr)
	}

	if closeErr := tw.Close(); closeErr != nil {
		t.Fatalf("Failed to close tar writer: %v", closeErr)
	}
	if closeErr := gw.Close(); closeErr != nil {
		t.Fatalf("Failed to close gzip writer: %v", closeErr)
	}
	if closeErr := f.Close(); closeErr != nil {
		t.Fatalf("Failed to close file: %v", closeErr)
	}

	// Test calculateUncompressedSize
	expectedSize := int64(len(testContent) + len(content2))
	size, err := calculateUncompressedSize(archivePath)
	if err != nil {
		t.Errorf("calculateUncompressedSize() unexpected error = %v", err)
		return
	}
	if size != expectedSize {
		t.Errorf("calculateUncompressedSize() = %d, want %d", size, expectedSize)
	}

	// Test with non-existent file
	_, err = calculateUncompressedSize("/nonexistent/path.tar.gz")
	if err == nil {
		t.Error("calculateUncompressedSize() expected error for non-existent file")
	}
}

// TestListArchiveContents tests the listArchiveContents helper function.
func TestListArchiveContents(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create a valid tar.gz archive with known files
	archivePath := filepath.Join(tmpDir, "test.tar.gz")

	// Create tar.gz file
	// #nosec G304 -- Test file path is controlled in test scope
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive file: %v", err)
	}

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Add files to the archive
	files := []struct {
		name    string
		content string
	}{
		{"config.yaml", "api:\n  enabled: true"},
		{"parsers/test.yaml", "parser content"},
		{"scenarios/brute.yaml", "scenario content"},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.name,
			Mode:     0o644,
			Size:     int64(len(file.content)),
			Typeflag: tar.TypeReg,
		}
		if writeHeaderErr := tw.WriteHeader(hdr); writeHeaderErr != nil {
			t.Fatalf("Failed to write tar header for %s: %v", file.name, writeHeaderErr)
		}
		if _, writeErr := tw.Write([]byte(file.content)); writeErr != nil {
			t.Fatalf("Failed to write tar content for %s: %v", file.name, writeErr)
		}
	}

	if closeErr := tw.Close(); closeErr != nil {
		t.Fatalf("Failed to close tar writer: %v", closeErr)
	}
	if closeErr := gw.Close(); closeErr != nil {
		t.Fatalf("Failed to close gzip writer: %v", closeErr)
	}
	if closeErr := f.Close(); closeErr != nil {
		t.Fatalf("Failed to close file: %v", closeErr)
	}

	// Test listArchiveContents
	contents, err := listArchiveContents(archivePath)
	if err != nil {
		t.Errorf("listArchiveContents() unexpected error = %v", err)
		return
	}

	expectedFiles := map[string]bool{
		"config.yaml":          false,
		"parsers/test.yaml":    false,
		"scenarios/brute.yaml": false,
	}

	for _, file := range contents {
		if _, ok := expectedFiles[file]; ok {
			expectedFiles[file] = true
		}
	}

	for file, found := range expectedFiles {
		if !found {
			t.Errorf("listArchiveContents() missing expected file: %s", file)
		}
	}

	if len(contents) != len(expectedFiles) {
		t.Errorf("listArchiveContents() returned %d files, want %d", len(contents), len(expectedFiles))
	}

	// Test with non-existent file
	_, err = listArchiveContents("/nonexistent/path.tar.gz")
	if err == nil {
		t.Error("listArchiveContents() expected error for non-existent file")
	}
}

// TestConfigArchiveValidator_Validate tests the ConfigArchiveValidator.Validate method.
func TestConfigArchiveValidator_Validate(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create a valid tar.gz archive with config.yaml
	validArchivePath := filepath.Join(tmpDir, "valid.tar.gz")
	createTestTarGz(t, validArchivePath, []struct {
		name    string
		content string
	}{
		{"config.yaml", "api:\n  enabled: true"},
	})

	validator := &ConfigArchiveValidator{
		MaxSize:             50 * 1024 * 1024,
		MaxUncompressed:     500 * 1024 * 1024,
		MaxCompressionRatio: 100,
		RequiredFiles:       []string{"config.yaml"},
	}

	// Test valid archive
	err := validator.Validate(validArchivePath)
	if err != nil {
		t.Errorf("Validate() unexpected error for valid archive: %v", err)
	}

	// Test missing required file
	missingArchivePath := filepath.Join(tmpDir, "missing.tar.gz")
	createTestTarGz(t, missingArchivePath, []struct {
		name    string
		content string
	}{
		{"other.yaml", "other content"},
	})

	err = validator.Validate(missingArchivePath)
	if err == nil {
		t.Error("Validate() expected error for missing required file")
	}

	// Test non-existent file
	err = validator.Validate("/nonexistent/path.tar.gz")
	if err == nil {
		t.Error("Validate() expected error for non-existent file")
	}

	// Test unsupported format
	unsupportedPath := filepath.Join(tmpDir, "test.rar")
	// #nosec G306 -- Test file permissions, not security-critical
	if writeErr := os.WriteFile(unsupportedPath, []byte("dummy"), 0o644); writeErr != nil {
		t.Fatalf("Failed to create dummy file: %v", writeErr)
	}
	err = validator.Validate(unsupportedPath)
	if err == nil || err.Error() != "only .tar.gz archives are supported" {
		t.Errorf("Validate() error = %v, want the fixed archive format message", err)
	}
}

// createTestTarGz creates a test tar.gz archive with the given files.
func createTestTarGz(t *testing.T, path string, files []struct {
	name    string
	content string
}) {
	t.Helper()

	// #nosec G304 -- Test helper function with controlled file path
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create archive file: %v", err)
	}

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.name,
			Mode:     0o644,
			Size:     int64(len(file.content)),
			Typeflag: tar.TypeReg,
		}
		if writeHeaderErr := tw.WriteHeader(hdr); writeHeaderErr != nil {
			t.Fatalf("Failed to write tar header for %s: %v", file.name, writeHeaderErr)
		}
		if _, writeErr := tw.Write([]byte(file.content)); writeErr != nil {
			t.Fatalf("Failed to write tar content for %s: %v", file.name, writeErr)
		}
	}

	if closeErr := tw.Close(); closeErr != nil {
		t.Fatalf("Failed to close tar writer: %v", closeErr)
	}
	if closeErr := gw.Close(); closeErr != nil {
		t.Fatalf("Failed to close gzip writer: %v", closeErr)
	}
	if closeErr := f.Close(); closeErr != nil {
		t.Fatalf("Failed to close file: %v", closeErr)
	}
}
