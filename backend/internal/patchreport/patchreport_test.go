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

func TestResolveThreshold_WithNilLookupUsesOSLookupEnv(t *testing.T) {
	t.Setenv("PATCH_THRESHOLD_TEST", "91.2")

	resolved := ResolveThreshold("PATCH_THRESHOLD_TEST", 85.0, nil)
	if resolved.Value != 91.2 {
		t.Fatalf("expected env value 91.2, got %.1f", resolved.Value)
	}
	if resolved.Source != "env" {
		t.Fatalf("expected source env, got %s", resolved.Source)
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

func TestParseUnifiedDiffChangedLines_InvalidHunkStartReturnsError(t *testing.T) {
	t.Parallel()

	diff := `diff --git a/backend/internal/app.go b/backend/internal/app.go
index 1111111..2222222 100644
--- a/backend/internal/app.go
+++ b/backend/internal/app.go
@@ -1,1 +abc,2 @@
+line
`

	backendChanged, frontendChanged, err := ParseUnifiedDiffChangedLines(diff)
	if err != nil {
		t.Fatalf("expected graceful handling for invalid hunk, got error: %v", err)
	}
	if len(backendChanged) != 0 || len(frontendChanged) != 0 {
		t.Fatalf("expected no changed lines for invalid hunk, got backend=%v frontend=%v", backendChanged, frontendChanged)
	}
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

func TestComputeFilesNeedingCoverage_IncludesUncoveredAndSortsDeterministically(t *testing.T) {
	t.Parallel()

	changed := FileLineSet{
		"backend/internal/b.go": {1: {}, 2: {}},
		"backend/internal/a.go": {1: {}, 2: {}},
		"backend/internal/c.go": {1: {}, 2: {}},
	}

	coverage := CoverageData{
		Executable: FileLineSet{
			"backend/internal/a.go": {1: {}, 2: {}},
			"backend/internal/b.go": {1: {}, 2: {}},
			"backend/internal/c.go": {1: {}, 2: {}},
		},
		Covered: FileLineSet{
			"backend/internal/a.go": {1: {}},
			"backend/internal/c.go": {1: {}, 2: {}},
		},
	}

	details := ComputeFilesNeedingCoverage(changed, coverage, 40)
	if len(details) != 2 {
		t.Fatalf("expected 2 files needing coverage, got %d", len(details))
	}

	if details[0].Path != "backend/internal/b.go" {
		t.Fatalf("expected first file to be backend/internal/b.go, got %s", details[0].Path)
	}
	if details[0].PatchCoveragePct != 0.0 {
		t.Fatalf("expected first file coverage 0.0, got %.1f", details[0].PatchCoveragePct)
	}
	if details[0].UncoveredChangedLines != 2 {
		t.Fatalf("expected first file uncovered lines 2, got %d", details[0].UncoveredChangedLines)
	}
	if strings.Join(details[0].UncoveredChangedLineRange, ",") != "1-2" {
		t.Fatalf("expected first file uncovered ranges 1-2, got %v", details[0].UncoveredChangedLineRange)
	}

	if details[1].Path != "backend/internal/a.go" {
		t.Fatalf("expected second file to be backend/internal/a.go, got %s", details[1].Path)
	}
	if details[1].PatchCoveragePct != 50.0 {
		t.Fatalf("expected second file coverage 50.0, got %.1f", details[1].PatchCoveragePct)
	}
	if details[1].UncoveredChangedLines != 1 {
		t.Fatalf("expected second file uncovered lines 1, got %d", details[1].UncoveredChangedLines)
	}
	if strings.Join(details[1].UncoveredChangedLineRange, ",") != "2" {
		t.Fatalf("expected second file uncovered range 2, got %v", details[1].UncoveredChangedLineRange)
	}
}

func TestComputeFilesNeedingCoverage_IncludesFullyCoveredWhenThresholdAbove100(t *testing.T) {
	t.Parallel()

	changed := FileLineSet{
		"backend/internal/fully.go": {10: {}, 11: {}},
	}
	coverage := CoverageData{
		Executable: FileLineSet{
			"backend/internal/fully.go": {10: {}, 11: {}},
		},
		Covered: FileLineSet{
			"backend/internal/fully.go": {10: {}, 11: {}},
		},
	}

	details := ComputeFilesNeedingCoverage(changed, coverage, 101)
	if len(details) != 1 {
		t.Fatalf("expected 1 file detail when threshold is 101, got %d", len(details))
	}
	if details[0].PatchCoveragePct != 100.0 {
		t.Fatalf("expected 100%% patch coverage detail, got %.1f", details[0].PatchCoveragePct)
	}
}

func TestMergeFileCoverageDetails_SortsWorstCoverageThenPath(t *testing.T) {
	t.Parallel()

	merged := MergeFileCoverageDetails(
		[]FileCoverageDetail{
			{Path: "frontend/src/z.ts", PatchCoveragePct: 50.0},
			{Path: "frontend/src/a.ts", PatchCoveragePct: 50.0},
		},
		[]FileCoverageDetail{
			{Path: "backend/internal/w.go", PatchCoveragePct: 0.0},
		},
	)

	if len(merged) != 3 {
		t.Fatalf("expected 3 merged items, got %d", len(merged))
	}

	orderedPaths := []string{merged[0].Path, merged[1].Path, merged[2].Path}
	got := strings.Join(orderedPaths, ",")
	want := "backend/internal/w.go,frontend/src/a.ts,frontend/src/z.ts"
	if got != want {
		t.Fatalf("unexpected merged order: got %s want %s", got, want)
	}
}

func TestParseCoverageRange_ErrorBranches(t *testing.T) {
	t.Parallel()

	_, _, _, err := parseCoverageRange("missing-colon")
	if err == nil {
		t.Fatal("expected error for missing colon")
	}

	_, _, _, err = parseCoverageRange("file.go:10.1")
	if err == nil {
		t.Fatal("expected error for missing end coordinate")
	}

	_, _, _, err = parseCoverageRange("file.go:bad.1,10.1")
	if err == nil {
		t.Fatal("expected error for bad start line")
	}

	_, _, _, err = parseCoverageRange("file.go:10.1,9.1")
	if err == nil {
		t.Fatal("expected error for reversed range")
	}
}

func TestSortedWarnings_FiltersBlanksAndSorts(t *testing.T) {
	t.Parallel()

	sorted := SortedWarnings([]string{"z warning", "", "  ", "a warning"})
	got := strings.Join(sorted, ",")
	want := "a warning,z warning"
	if got != want {
		t.Fatalf("unexpected warnings ordering: got %q want %q", got, want)
	}
}

func TestNormalizePathsAndRanges(t *testing.T) {
	t.Parallel()

	if got := normalizeGoCoveragePath("internal/service.go"); got != "backend/internal/service.go" {
		t.Fatalf("unexpected normalized go path: %s", got)
	}

	if got := normalizeGoCoveragePath("/tmp/work/backend/internal/service.go"); got != "backend/internal/service.go" {
		t.Fatalf("unexpected backend extraction path: %s", got)
	}

	frontend := normalizeFrontendCoveragePaths("/tmp/work/frontend/src/App.tsx")
	if len(frontend) == 0 {
		t.Fatal("expected frontend normalized paths")
	}

	ranges := formatLineRanges([]int{1, 2, 3, 7, 9, 10})
	gotRanges := strings.Join(ranges, ",")
	wantRanges := "1-3,7,9-10"
	if gotRanges != wantRanges {
		t.Fatalf("unexpected ranges: got %q want %q", gotRanges, wantRanges)
	}
}

func TestScopeCoverageMergeAndStatus(t *testing.T) {
	t.Parallel()

	merged := MergeScopeCoverage(
		ScopeCoverage{ChangedLines: 4, CoveredLines: 3},
		ScopeCoverage{ChangedLines: 0, CoveredLines: 0},
	)

	if merged.ChangedLines != 4 || merged.CoveredLines != 3 || merged.PatchCoveragePct != 75.0 {
		t.Fatalf("unexpected merged scope: %+v", merged)
	}

	if status := ApplyStatus(merged, 70); status.Status != "pass" {
		t.Fatalf("expected pass status, got %s", status.Status)
	}
}

func TestParseCoverageProfiles_InvalidPath(t *testing.T) {
	t.Parallel()

	_, err := ParseGoCoverageProfile("   ")
	if err == nil {
		t.Fatal("expected go profile path validation error")
	}

	_, err = ParseLCOVProfile("\t")
	if err == nil {
		t.Fatal("expected lcov profile path validation error")
	}
}

func TestNormalizeFrontendCoveragePaths_EmptyInput(t *testing.T) {
	t.Parallel()

	paths := normalizeFrontendCoveragePaths("   ")
	if len(paths) == 0 {
		t.Fatalf("expected normalized fallback paths, got %#v", paths)
	}
}

func TestAddLine_IgnoresInvalidInputs(t *testing.T) {
	t.Parallel()

	set := make(FileLineSet)
	addLine(set, "", 10)
	addLine(set, "backend/internal/x.go", 0)
	if len(set) != 0 {
		t.Fatalf("expected no entries for invalid addLine input, got %#v", set)
	}
}
