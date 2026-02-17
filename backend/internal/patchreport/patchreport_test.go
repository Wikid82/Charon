package patchreport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		envValue     string
		envSet       bool
		defaultValue float64
		wantValue    float64
		wantSource   string
		wantWarning  bool
	}{
		{
			name:         "uses default when env is absent",
			envSet:       false,
			defaultValue: 90,
			wantValue:    90,
			wantSource:   "default",
			wantWarning:  false,
		},
		{
			name:         "uses env value when valid",
			envSet:       true,
			envValue:     "87.5",
			defaultValue: 85,
			wantValue:    87.5,
			wantSource:   "env",
			wantWarning:  false,
		},
		{
			name:         "falls back when env is invalid",
			envSet:       true,
			envValue:     "invalid",
			defaultValue: 85,
			wantValue:    85,
			wantSource:   "default",
			wantWarning:  true,
		},
		{
			name:         "falls back when env is out of range",
			envSet:       true,
			envValue:     "101",
			defaultValue: 85,
			wantValue:    85,
			wantSource:   "default",
			wantWarning:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lookup := func(name string) (string, bool) {
				if name != "TARGET" {
					t.Fatalf("unexpected env lookup key: %s", name)
				}
				if !tt.envSet {
					return "", false
				}
				return tt.envValue, true
			}

			resolved := ResolveThreshold("TARGET", tt.defaultValue, lookup)
			if resolved.Value != tt.wantValue {
				t.Fatalf("value mismatch: got %.1f want %.1f", resolved.Value, tt.wantValue)
			}
			if resolved.Source != tt.wantSource {
				t.Fatalf("source mismatch: got %s want %s", resolved.Source, tt.wantSource)
			}
			hasWarning := resolved.Warning != ""
			if hasWarning != tt.wantWarning {
				t.Fatalf("warning mismatch: got %v want %v (warning=%q)", hasWarning, tt.wantWarning, resolved.Warning)
			}
		})
	}
}

func TestParseUnifiedDiffChangedLines(t *testing.T) {
	t.Parallel()

	diff := `diff --git a/backend/internal/app.go b/backend/internal/app.go
index 1111111..2222222 100644
--- a/backend/internal/app.go
+++ b/backend/internal/app.go
@@ -10,2 +10,3 @@ func example() {
 line10
-line11
+line11 changed
+line12 new
diff --git a/frontend/src/App.tsx b/frontend/src/App.tsx
index 3333333..4444444 100644
--- a/frontend/src/App.tsx
+++ b/frontend/src/App.tsx
@@ -20,0 +21,2 @@ export default function App() {
+new frontend line
+another frontend line
`

	backendChanged, frontendChanged, err := ParseUnifiedDiffChangedLines(diff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiffChangedLines returned error: %v", err)
	}

	assertHasLines(t, backendChanged, "backend/internal/app.go", []int{11, 12})
	assertHasLines(t, frontendChanged, "frontend/src/App.tsx", []int{21, 22})
}

func TestBackendChangedLineCoverageComputation(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	coverageFile := filepath.Join(tempDir, "coverage.txt")
	coverageContent := `mode: atomic
github.com/Wikid82/charon/backend/internal/service.go:10.1,10.20 1 1
github.com/Wikid82/charon/backend/internal/service.go:11.1,11.20 1 0
github.com/Wikid82/charon/backend/internal/service.go:12.1,12.20 1 1
`
	if err := os.WriteFile(coverageFile, []byte(coverageContent), 0o600); err != nil {
		t.Fatalf("failed to write temp coverage file: %v", err)
	}

	coverage, err := ParseGoCoverageProfile(coverageFile)
	if err != nil {
		t.Fatalf("ParseGoCoverageProfile returned error: %v", err)
	}

	changed := FileLineSet{
		"backend/internal/service.go": {10: {}, 11: {}, 15: {}},
	}

	scope := ComputeScopeCoverage(changed, coverage)
	if scope.ChangedLines != 2 {
		t.Fatalf("changed lines mismatch: got %d want 2", scope.ChangedLines)
	}
	if scope.CoveredLines != 1 {
		t.Fatalf("covered lines mismatch: got %d want 1", scope.CoveredLines)
	}
	if scope.PatchCoveragePct != 50.0 {
		t.Fatalf("coverage pct mismatch: got %.1f want 50.0", scope.PatchCoveragePct)
	}
}

func TestFrontendChangedLineCoverageComputationFromLCOV(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	lcovFile := filepath.Join(tempDir, "lcov.info")
	lcovContent := `TN:
SF:frontend/src/App.tsx
DA:10,1
DA:11,0
DA:12,1
end_of_record
`
	if err := os.WriteFile(lcovFile, []byte(lcovContent), 0o600); err != nil {
		t.Fatalf("failed to write temp lcov file: %v", err)
	}

	coverage, err := ParseLCOVProfile(lcovFile)
	if err != nil {
		t.Fatalf("ParseLCOVProfile returned error: %v", err)
	}

	changed := FileLineSet{
		"frontend/src/App.tsx": {10: {}, 11: {}, 13: {}},
	}

	scope := ComputeScopeCoverage(changed, coverage)
	if scope.ChangedLines != 2 {
		t.Fatalf("changed lines mismatch: got %d want 2", scope.ChangedLines)
	}
	if scope.CoveredLines != 1 {
		t.Fatalf("covered lines mismatch: got %d want 1", scope.CoveredLines)
	}
	if scope.PatchCoveragePct != 50.0 {
		t.Fatalf("coverage pct mismatch: got %.1f want 50.0", scope.PatchCoveragePct)
	}

	status := ApplyStatus(scope, 85)
	if status.Status != "warn" {
		t.Fatalf("status mismatch: got %s want warn", status.Status)
	}
}

func TestParseUnifiedDiffChangedLines_AllowsLongLines(t *testing.T) {
	t.Parallel()

	longLine := strings.Repeat("x", 128*1024)
	diff := strings.Join([]string{
		"diff --git a/backend/internal/app.go b/backend/internal/app.go",
		"index 1111111..2222222 100644",
		"--- a/backend/internal/app.go",
		"+++ b/backend/internal/app.go",
		"@@ -1,1 +1,2 @@",
		" line1",
		"+" + longLine,
	}, "\n")

	backendChanged, _, err := ParseUnifiedDiffChangedLines(diff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiffChangedLines returned error for long line: %v", err)
	}

	assertHasLines(t, backendChanged, "backend/internal/app.go", []int{2})
}

func TestParseGoCoverageProfile_AllowsLongLines(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	coverageFile := filepath.Join(tempDir, "coverage.txt")
	longSegment := strings.Repeat("a", 128*1024)
	coverageContent := "mode: atomic\n" +
		"github.com/Wikid82/charon/backend/internal/" + longSegment + ".go:10.1,10.20 1 1\n"
	if err := os.WriteFile(coverageFile, []byte(coverageContent), 0o600); err != nil {
		t.Fatalf("failed to write temp coverage file: %v", err)
	}

	_, err := ParseGoCoverageProfile(coverageFile)
	if err != nil {
		t.Fatalf("ParseGoCoverageProfile returned error for long line: %v", err)
	}
}

func TestParseLCOVProfile_AllowsLongLines(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	lcovFile := filepath.Join(tempDir, "lcov.info")
	longPath := strings.Repeat("a", 128*1024)
	lcovContent := strings.Join([]string{
		"TN:",
		"SF:frontend/src/" + longPath + ".tsx",
		"DA:10,1",
		"end_of_record",
	}, "\n")
	if err := os.WriteFile(lcovFile, []byte(lcovContent), 0o600); err != nil {
		t.Fatalf("failed to write temp lcov file: %v", err)
	}

	_, err := ParseLCOVProfile(lcovFile)
	if err != nil {
		t.Fatalf("ParseLCOVProfile returned error for long line: %v", err)
	}
}

func assertHasLines(t *testing.T, changed FileLineSet, file string, expected []int) {
	t.Helper()

	lines, ok := changed[file]
	if !ok {
		t.Fatalf("file %s not found in changed lines", file)
	}
	for _, line := range expected {
		if _, hasLine := lines[line]; !hasLine {
			t.Fatalf("expected line %d in file %s", line, file)
		}
	}
}

func TestValidateReadablePath(t *testing.T) {
	t.Parallel()

	t.Run("returns error for empty path", func(t *testing.T) {
		t.Parallel()

		_, err := validateReadablePath("   ")
		if err == nil {
			t.Fatal("expected error for empty path")
		}
	})

	t.Run("returns absolute cleaned path", func(t *testing.T) {
		t.Parallel()

		path, err := validateReadablePath("./backend/../backend/internal")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !filepath.IsAbs(path) {
			t.Fatalf("expected absolute path, got %q", path)
		}
	})
}
