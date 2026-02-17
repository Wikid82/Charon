//nolint:gosec
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/patchreport"
)

func TestMainProcessHelper(t *testing.T) {
	t.Helper()
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	separatorIndex := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separatorIndex = index
			break
		}
	}
	if separatorIndex == -1 {
		os.Exit(2)
	}

	os.Args = append([]string{os.Args[0]}, os.Args[separatorIndex+1:]...)
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	main()
	os.Exit(0)
}

func TestMain_SuccessWritesReports(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	jsonOut := filepath.Join(repoRoot, "reports", "local-patch.json")
	mdOut := filepath.Join(repoRoot, "reports", "local-patch.md")

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-backend-coverage", "backend/coverage.txt",
		"-frontend-coverage", "frontend/coverage/lcov.info",
		"-json-out", jsonOut,
		"-md-out", mdOut,
	)

	if result.exitCode != 0 {
		t.Fatalf("expected success exit code 0, got %d, stderr=%s", result.exitCode, result.stderr)
	}

	if _, err := os.Stat(jsonOut); err != nil {
		t.Fatalf("expected json report to exist: %v", err)
	}
	if _, err := os.Stat(mdOut); err != nil {
		t.Fatalf("expected markdown report to exist: %v", err)
	}

	// #nosec G304 -- Test reads artifact path created by this test.
	reportBytes, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("read json report: %v", err)
	}

	var report reportJSON
	if err := json.Unmarshal(reportBytes, &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if report.Mode != "warn" {
		t.Fatalf("unexpected mode: %s", report.Mode)
	}
	if report.Artifacts.JSON == "" || report.Artifacts.Markdown == "" {
		t.Fatalf("expected artifacts to be populated: %+v", report.Artifacts)
	}
	if !strings.Contains(result.stdout, "Local patch report generated") {
		t.Fatalf("expected success output, got: %s", result.stdout)
	}
}

func TestMain_FailsWhenBackendCoverageIsMissing(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	if err := os.Remove(filepath.Join(repoRoot, "backend", "coverage.txt")); err != nil {
		t.Fatalf("remove backend coverage: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
	)

	if result.exitCode == 0 {
		t.Fatalf("expected non-zero exit code for missing backend coverage")
	}
	if !strings.Contains(result.stderr, "missing backend coverage file") {
		t.Fatalf("expected missing backend coverage error, stderr=%s", result.stderr)
	}
}

func TestMain_FailsWhenGitBaselineIsInvalid(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "this-is-not-a-valid-revision",
	)

	if result.exitCode == 0 {
		t.Fatalf("expected non-zero exit code for invalid baseline")
	}
	if !strings.Contains(result.stderr, "error generating git diff") {
		t.Fatalf("expected git diff error, stderr=%s", result.stderr)
	}
}

func TestMain_FailsWhenBackendCoverageParseErrors(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	backendCoverage := filepath.Join(repoRoot, "backend", "coverage.txt")

	tooLongLine := strings.Repeat("a", 3*1024*1024)
	if err := os.WriteFile(backendCoverage, []byte("mode: atomic\n"+tooLongLine+"\n"), 0o600); err != nil {
		t.Fatalf("write backend coverage: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
	)

	if result.exitCode == 0 {
		t.Fatalf("expected non-zero exit code for backend parse error")
	}
	if !strings.Contains(result.stderr, "error parsing backend coverage") {
		t.Fatalf("expected backend parse error, stderr=%s", result.stderr)
	}
}

func TestMain_FailsWhenFrontendCoverageParseErrors(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	frontendCoverage := filepath.Join(repoRoot, "frontend", "coverage", "lcov.info")

	tooLongLine := strings.Repeat("b", 3*1024*1024)
	if err := os.WriteFile(frontendCoverage, []byte("TN:\nSF:frontend/src/file.ts\nDA:1,1\n"+tooLongLine+"\n"), 0o600); err != nil {
		t.Fatalf("write frontend coverage: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
	)

	if result.exitCode == 0 {
		t.Fatalf("expected non-zero exit code for frontend parse error")
	}
	if !strings.Contains(result.stderr, "error parsing frontend coverage") {
		t.Fatalf("expected frontend parse error, stderr=%s", result.stderr)
	}
}

func TestMain_FailsWhenJSONOutputCannotBeWritten(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	jsonDir := filepath.Join(repoRoot, "locked-json-dir")
	if err := os.MkdirAll(jsonDir, 0o750); err != nil {
		t.Fatalf("create json dir: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-json-out", jsonDir,
	)

	if result.exitCode == 0 {
		t.Fatalf("expected non-zero exit code when json output path is a directory")
	}
	if !strings.Contains(result.stderr, "error writing json report") {
		t.Fatalf("expected json write error, stderr=%s", result.stderr)
	}
}

func TestResolvePathAndRelOrAbs(t *testing.T) {
	repoRoot := t.TempDir()
	absolute := filepath.Join(repoRoot, "absolute.txt")
	if got := resolvePath(repoRoot, absolute); got != absolute {
		t.Fatalf("expected absolute path unchanged, got %s", got)
	}

	relative := "nested/file.txt"
	expected := filepath.Join(repoRoot, relative)
	if got := resolvePath(repoRoot, relative); got != expected {
		t.Fatalf("expected joined path %s, got %s", expected, got)
	}

	if got := relOrAbs(repoRoot, expected); got != "nested/file.txt" {
		t.Fatalf("expected repo-relative path, got %s", got)
	}
}

func TestAssertFileExists(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "ok.txt")
	if err := os.WriteFile(filePath, []byte("ok"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := assertFileExists(filePath, "test file"); err != nil {
		t.Fatalf("expected existing file to pass: %v", err)
	}

	err := assertFileExists(filepath.Join(tempDir, "missing.txt"), "missing file")
	if err == nil || !strings.Contains(err.Error(), "missing missing file") {
		t.Fatalf("expected missing file error, got: %v", err)
	}

	err = assertFileExists(tempDir, "directory input")
	if err == nil || !strings.Contains(err.Error(), "found directory") {
		t.Fatalf("expected directory error, got: %v", err)
	}
}

func TestGitDiffAndWriters(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)

	diffContent, err := gitDiff(repoRoot, "HEAD...HEAD")
	if err != nil {
		t.Fatalf("gitDiff should succeed for HEAD...HEAD: %v", err)
	}
	if diffContent != "" {
		t.Fatalf("expected empty diff for HEAD...HEAD, got: %q", diffContent)
	}

	if _, err := gitDiff(repoRoot, "bad-baseline"); err == nil {
		t.Fatal("expected gitDiff failure for invalid baseline")
	}

	report := reportJSON{
		Baseline:    "origin/main...HEAD",
		GeneratedAt: "2026-02-17T00:00:00Z",
		Mode:        "warn",
		Thresholds: thresholdJSON{Overall: 90, Backend: 85, Frontend: 85},
		ThresholdSources: thresholdSourcesJSON{
			Overall:  "default",
			Backend:  "default",
			Frontend: "default",
		},
		Overall:  patchreport.ScopeCoverage{ChangedLines: 10, CoveredLines: 5, PatchCoveragePct: 50, Status: "warn"},
		Backend:  patchreport.ScopeCoverage{ChangedLines: 6, CoveredLines: 2, PatchCoveragePct: 33.3, Status: "warn"},
		Frontend: patchreport.ScopeCoverage{ChangedLines: 4, CoveredLines: 3, PatchCoveragePct: 75, Status: "warn"},
		FilesNeedingCoverage: []patchreport.FileCoverageDetail{{
			Path:                      "backend/cmd/localpatchreport/main.go",
			PatchCoveragePct:          0,
			UncoveredChangedLines:     2,
			UncoveredChangedLineRange: []string{"10-11"},
		}},
		Warnings: []string{"warning one"},
		Artifacts: artifactsJSON{Markdown: "test-results/report.md", JSON: "test-results/report.json"},
	}

	jsonPath := filepath.Join(t.TempDir(), "report.json")
	if err := writeJSON(jsonPath, report); err != nil {
		t.Fatalf("writeJSON should succeed: %v", err)
	}
	// #nosec G304 -- Test reads artifact path created by this test.
	jsonBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json file: %v", err)
	}
	if !strings.Contains(string(jsonBytes), "\"baseline\": \"origin/main...HEAD\"") {
		t.Fatalf("unexpected json content: %s", string(jsonBytes))
	}

	markdownPath := filepath.Join(t.TempDir(), "report.md")
	if err := writeMarkdown(markdownPath, report, "backend/coverage.txt", "frontend/coverage/lcov.info"); err != nil {
		t.Fatalf("writeMarkdown should succeed: %v", err)
	}
	// #nosec G304 -- Test reads artifact path created by this test.
	markdownBytes, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read markdown file: %v", err)
	}
	markdown := string(markdownBytes)
	if !strings.Contains(markdown, "## Files Needing Coverage") {
		t.Fatalf("expected files section in markdown: %s", markdown)
	}
	if !strings.Contains(markdown, "## Warnings") {
		t.Fatalf("expected warnings section in markdown: %s", markdown)
	}

	scope := patchreport.ScopeCoverage{ChangedLines: 3, CoveredLines: 2, PatchCoveragePct: 66.7, Status: "warn"}
	row := scopeRow("Backend", scope)
	if !strings.Contains(row, "| Backend | 3 | 2 | 66.7 | warn |") {
		t.Fatalf("unexpected scope row: %s", row)
	}
}

func runMainSubprocess(t *testing.T, args ...string) subprocessResult {
	t.Helper()

	commandArgs := append([]string{"-test.run=TestMainProcessHelper", "--"}, args...)
	// #nosec G204 -- Test helper subprocess invocation with controlled arguments.
	cmd := exec.Command(os.Args[0], commandArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")

	stdout, err := cmd.Output()
	if err == nil {
		return subprocessResult{exitCode: 0, stdout: string(stdout), stderr: ""}
	}

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return subprocessResult{exitCode: exitError.ExitCode(), stdout: string(stdout), stderr: string(exitError.Stderr)}
	}

	t.Fatalf("unexpected subprocess failure: %v", err)
	return subprocessResult{}
}

type subprocessResult struct {
	exitCode int
	stdout   string
	stderr   string
}

func createGitRepoWithCoverageInputs(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	mustRunCommand(t, repoRoot, "git", "init")
	mustRunCommand(t, repoRoot, "git", "config", "user.email", "test@example.com")
	mustRunCommand(t, repoRoot, "git", "config", "user.name", "Test User")

	paths := []string{
		filepath.Join(repoRoot, "backend", "internal"),
		filepath.Join(repoRoot, "frontend", "src"),
		filepath.Join(repoRoot, "frontend", "coverage"),
		filepath.Join(repoRoot, "backend"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(path, 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}

	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "internal", "sample.go"), []byte("package internal\nvar Sample = 1\n"), 0o600); err != nil {
		t.Fatalf("write backend sample: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "frontend", "src", "sample.ts"), []byte("export const sample = 1;\n"), 0o600); err != nil {
		t.Fatalf("write frontend sample: %v", err)
	}

	backendCoverage := "mode: atomic\nbackend/internal/sample.go:1.1,2.20 1 1\n"
	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "coverage.txt"), []byte(backendCoverage), 0o600); err != nil {
		t.Fatalf("write backend coverage: %v", err)
	}

	frontendCoverage := "TN:\nSF:frontend/src/sample.ts\nDA:1,1\nend_of_record\n"
	if err := os.WriteFile(filepath.Join(repoRoot, "frontend", "coverage", "lcov.info"), []byte(frontendCoverage), 0o600); err != nil {
		t.Fatalf("write frontend coverage: %v", err)
	}

	mustRunCommand(t, repoRoot, "git", "add", ".")
	mustRunCommand(t, repoRoot, "git", "commit", "-m", "initial commit")

	return repoRoot
}

func mustRunCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	// #nosec G204 -- Test helper executes deterministic local commands.
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %s %s failed: %v\n%s", name, strings.Join(args, " "), err, string(output))
	}
}

func TestWriteJSONReturnsErrorWhenPathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	report := reportJSON{Baseline: "x", GeneratedAt: "y", Mode: "warn"}
	if err := writeJSON(dir, report); err == nil {
		t.Fatal("expected writeJSON to fail when target is a directory")
	}
}

func TestWriteMarkdownReturnsErrorWhenPathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	report := reportJSON{
		Baseline:             "origin/main...HEAD",
		GeneratedAt:          "2026-02-17T00:00:00Z",
		Mode:                 "warn",
		Thresholds:           thresholdJSON{Overall: 90, Backend: 85, Frontend: 85},
		ThresholdSources:     thresholdSourcesJSON{Overall: "default", Backend: "default", Frontend: "default"},
		Overall:              patchreport.ScopeCoverage{Status: "pass"},
		Backend:              patchreport.ScopeCoverage{Status: "pass"},
		Frontend:             patchreport.ScopeCoverage{Status: "pass"},
		FilesNeedingCoverage: nil,
		Warnings:             nil,
		Artifacts:            artifactsJSON{Markdown: "a", JSON: "b"},
	}
	if err := writeMarkdown(dir, report, "backend/coverage.txt", "frontend/coverage/lcov.info"); err == nil {
		t.Fatal("expected writeMarkdown to fail when target is a directory")
	}
}

func TestMain_FailsWhenMarkdownDirectoryCreationFails(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)

	lockedParent := filepath.Join(repoRoot, "md-root")
	if err := os.WriteFile(lockedParent, []byte("file-not-dir"), 0o600); err != nil {
		t.Fatalf("write locked parent file: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-md-out", filepath.Join(lockedParent, "report.md"),
	)

	if result.exitCode == 0 {
		t.Fatalf("expected markdown directory creation failure")
	}
	if !strings.Contains(result.stderr, "error creating markdown output directory") {
		t.Fatalf("expected markdown mkdir error, stderr=%s", result.stderr)
	}
}

func TestMain_FailsWhenJSONDirectoryCreationFails(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)

	lockedParent := filepath.Join(repoRoot, "json-root")
	if err := os.WriteFile(lockedParent, []byte("file-not-dir"), 0o600); err != nil {
		t.Fatalf("write locked parent file: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-json-out", filepath.Join(lockedParent, "report.json"),
	)

	if result.exitCode == 0 {
		t.Fatalf("expected json directory creation failure")
	}
	if !strings.Contains(result.stderr, "error creating json output directory") {
		t.Fatalf("expected json mkdir error, stderr=%s", result.stderr)
	}
}

func TestMain_PrintsWarningsWhenThresholdsNotMet(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)

	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "internal", "sample.go"), []byte("package internal\nvar Sample = 2\n"), 0o600); err != nil {
		t.Fatalf("update backend sample: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "frontend", "src", "sample.ts"), []byte("export const sample = 2;\n"), 0o600); err != nil {
		t.Fatalf("update frontend sample: %v", err)
	}

	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "coverage.txt"), []byte("mode: atomic\nbackend/internal/sample.go:1.1,2.20 1 0\n"), 0o600); err != nil {
		t.Fatalf("write backend uncovered coverage: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "frontend", "coverage", "lcov.info"), []byte("TN:\nSF:frontend/src/sample.ts\nDA:1,0\nend_of_record\n"), 0o600); err != nil {
		t.Fatalf("write frontend uncovered coverage: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD",
	)

	if result.exitCode != 0 {
		t.Fatalf("expected success with warnings, got exit=%d stderr=%s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "WARN: Overall patch coverage") {
		t.Fatalf("expected WARN output, stdout=%s", result.stdout)
	}
}

func TestRelOrAbsConvertsSlashes(t *testing.T) {
	repoRoot := t.TempDir()
	targetPath := filepath.Join(repoRoot, "reports", "file.json")

	got := relOrAbs(repoRoot, targetPath)
	if got != "reports/file.json" {
		t.Fatalf("expected slash-normalized relative path, got %s", got)
	}
}

func TestHelperCommandFailureHasContext(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	_, err := gitDiff(repoRoot, "definitely-invalid")
	if err == nil {
		t.Fatal("expected gitDiff error")
	}
	if !strings.Contains(err.Error(), "git diff definitely-invalid failed") {
		t.Fatalf("expected contextual error message, got %v", err)
	}
}

func TestMain_FailsWhenMarkdownWriteFails(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	mdDir := filepath.Join(repoRoot, "md-as-dir")
	if err := os.MkdirAll(mdDir, 0o750); err != nil {
		t.Fatalf("create markdown dir: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-md-out", mdDir,
	)

	if result.exitCode == 0 {
		t.Fatalf("expected markdown write failure")
	}
	if !strings.Contains(result.stderr, "error writing markdown report") {
		t.Fatalf("expected markdown write error, stderr=%s", result.stderr)
	}
}

func TestMain_FailsWhenFrontendCoverageIsMissing(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	if err := os.Remove(filepath.Join(repoRoot, "frontend", "coverage", "lcov.info")); err != nil {
		t.Fatalf("remove frontend coverage: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
	)

	if result.exitCode == 0 {
		t.Fatalf("expected non-zero exit code for missing frontend coverage")
	}
	if !strings.Contains(result.stderr, "missing frontend coverage file") {
		t.Fatalf("expected missing frontend coverage error, stderr=%s", result.stderr)
	}
}

func TestMain_FailsWhenRepoRootInvalid(t *testing.T) {
	nonexistentPath := filepath.Join(t.TempDir(), "missing", "repo")

	result := runMainSubprocess(t,
		"-repo-root", nonexistentPath,
		"-baseline", "HEAD...HEAD",
		"-backend-coverage", "backend/coverage.txt",
		"-frontend-coverage", "frontend/coverage/lcov.info",
	)

	if result.exitCode == 0 {
		t.Fatalf("expected non-zero exit code for invalid repo root")
	}
	if !strings.Contains(result.stderr, "missing backend coverage file") {
		t.Fatalf("expected backend missing error for invalid repo root, stderr=%s", result.stderr)
	}
}

func TestMain_WarnsForInvalidThresholdEnv(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)

	commandArgs := []string{"-test.run=TestMainProcessHelper", "--", "-repo-root", repoRoot, "-baseline", "HEAD...HEAD"}
	// #nosec G204 -- Test helper subprocess invocation with controlled arguments.
	cmd := exec.Command(os.Args[0], commandArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "CHARON_OVERALL_PATCH_COVERAGE_MIN=invalid")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected success with warning env, got err=%v output=%s", err, string(output))
	}

	if !strings.Contains(string(output), "WARN: Ignoring invalid CHARON_OVERALL_PATCH_COVERAGE_MIN") {
		t.Fatalf("expected invalid-threshold warning, output=%s", string(output))
	}
}

func TestWriteMarkdownIncludesArtifactsSection(t *testing.T) {
	report := reportJSON{
		Baseline:         "origin/main...HEAD",
		GeneratedAt:      "2026-02-17T00:00:00Z",
		Mode:             "warn",
		Thresholds:       thresholdJSON{Overall: 90, Backend: 85, Frontend: 85},
		ThresholdSources: thresholdSourcesJSON{Overall: "default", Backend: "default", Frontend: "default"},
		Overall:          patchreport.ScopeCoverage{ChangedLines: 1, CoveredLines: 1, PatchCoveragePct: 100, Status: "pass"},
		Backend:          patchreport.ScopeCoverage{ChangedLines: 1, CoveredLines: 1, PatchCoveragePct: 100, Status: "pass"},
		Frontend:         patchreport.ScopeCoverage{ChangedLines: 0, CoveredLines: 0, PatchCoveragePct: 100, Status: "pass"},
		Artifacts:        artifactsJSON{Markdown: "test-results/local-patch-report.md", JSON: "test-results/local-patch-report.json"},
	}

	path := filepath.Join(t.TempDir(), "report.md")
	if err := writeMarkdown(path, report, "backend/coverage.txt", "frontend/coverage/lcov.info"); err != nil {
		t.Fatalf("writeMarkdown: %v", err)
	}

	// #nosec G304 -- Test reads artifact path created by this test.
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	if !strings.Contains(string(body), "## Artifacts") {
		t.Fatalf("expected artifacts section, got: %s", string(body))
	}
}

func TestRunMainSubprocessReturnsExitCode(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "not-a-revision",
	)

	if result.exitCode == 0 {
		t.Fatalf("expected non-zero exit for invalid baseline")
	}
	if result.stderr == "" {
		t.Fatal("expected stderr to be captured")
	}
}

func TestMustRunCommandHelper(t *testing.T) {
	temp := t.TempDir()
	mustRunCommand(t, temp, "git", "init")

	// #nosec G204 -- Test setup command with fixed arguments.
	configEmail := exec.Command("git", "-C", temp, "config", "user.email", "test@example.com")
	if output, err := configEmail.CombinedOutput(); err != nil {
		t.Fatalf("configure email failed: %v output=%s", err, string(output))
	}
	// #nosec G204 -- Test setup command with fixed arguments.
	configName := exec.Command("git", "-C", temp, "config", "user.name", "Test User")
	if output, err := configName.CombinedOutput(); err != nil {
		t.Fatalf("configure name failed: %v output=%s", err, string(output))
	}

	if err := os.WriteFile(filepath.Join(temp, "README.md"), []byte("content\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	mustRunCommand(t, temp, "git", "add", ".")
	mustRunCommand(t, temp, "git", "commit", "-m", "test")
}

func TestSubprocessHelperFailsWithoutSeparator(t *testing.T) {
	// #nosec G204 -- Test helper subprocess invocation with fixed arguments.
	cmd := exec.Command(os.Args[0], "-test.run=TestMainProcessHelper")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	_, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected helper process to fail without separator")
	}
}

func TestScopeRowFormatting(t *testing.T) {
	row := scopeRow("Overall", patchreport.ScopeCoverage{ChangedLines: 10, CoveredLines: 8, PatchCoveragePct: 80.0, Status: "warn"})
	expected := "| Overall | 10 | 8 | 80.0 | warn |\n"
	if row != expected {
		t.Fatalf("unexpected row\nwant: %q\ngot:  %q", expected, row)
	}
}

func TestMainProcessHelperNoopWhenEnvUnset(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "" {
		t.Skip("helper env is set by parent process")
	}
}

func TestRelOrAbsWithNestedPath(t *testing.T) {
	repoRoot := t.TempDir()
	nested := filepath.Join(repoRoot, "a", "b", "c", "report.json")
	if got := relOrAbs(repoRoot, nested); got != "a/b/c/report.json" {
		t.Fatalf("unexpected relative path: %s", got)
	}
}

func TestResolvePathWithAbsoluteInput(t *testing.T) {
	repoRoot := t.TempDir()
	abs := filepath.Join(repoRoot, "direct.txt")
	if resolvePath(repoRoot, abs) != abs {
		t.Fatal("resolvePath should return absolute input unchanged")
	}
}

func TestResolvePathWithRelativeInput(t *testing.T) {
	repoRoot := t.TempDir()
	got := resolvePath(repoRoot, "test-results/out.json")
	expected := filepath.Join(repoRoot, "test-results", "out.json")
	if got != expected {
		t.Fatalf("unexpected resolved path: %s", got)
	}
}

func TestAssertFileExistsErrorMessageIncludesLabel(t *testing.T) {
	err := assertFileExists(filepath.Join(t.TempDir(), "missing"), "backend coverage file")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "backend coverage file") {
		t.Fatalf("expected label in error, got: %v", err)
	}
}

func TestWriteJSONContentIncludesTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	report := reportJSON{Baseline: "origin/main...HEAD", GeneratedAt: "2026-02-17T00:00:00Z", Mode: "warn"}
	if err := writeJSON(path, report); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	// #nosec G304 -- Test reads artifact path created by this test.
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	if len(body) == 0 || body[len(body)-1] != '\n' {
		t.Fatalf("expected trailing newline, got: %q", string(body))
	}
}

func TestMainProducesRelArtifactPaths(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	jsonOut := "test-results/custom/report.json"
	mdOut := "test-results/custom/report.md"

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-json-out", jsonOut,
		"-md-out", mdOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: stderr=%s", result.stderr)
	}

	// #nosec G304 -- Test reads artifact path created by this test.
	content, err := os.ReadFile(filepath.Join(repoRoot, jsonOut))
	if err != nil {
		t.Fatalf("read json report: %v", err)
	}

	var report reportJSON
	if err := json.Unmarshal(content, &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if report.Artifacts.JSON != "test-results/custom/report.json" {
		t.Fatalf("unexpected json artifact path: %s", report.Artifacts.JSON)
	}
	if report.Artifacts.Markdown != "test-results/custom/report.md" {
		t.Fatalf("unexpected markdown artifact path: %s", report.Artifacts.Markdown)
	}
}

func TestMainWithExplicitInputPaths(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-backend-coverage", filepath.Join(repoRoot, "backend", "coverage.txt"),
		"-frontend-coverage", filepath.Join(repoRoot, "frontend", "coverage", "lcov.info"),
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success with explicit paths: stderr=%s", result.stderr)
	}
}

func TestMainOutputIncludesArtifactPaths(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	jsonOut := "test-results/a.json"
	mdOut := "test-results/a.md"

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-json-out", jsonOut,
		"-md-out", mdOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: stderr=%s", result.stderr)
	}
	if !strings.Contains(result.stdout, "JSON: test-results/a.json") {
		t.Fatalf("expected JSON output path in stdout: %s", result.stdout)
	}
	if !strings.Contains(result.stdout, "Markdown: test-results/a.md") {
		t.Fatalf("expected markdown output path in stdout: %s", result.stdout)
	}
}

func TestMainWithFileNeedingCoverageIncludesMarkdownTable(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)

	backendSource := filepath.Join(repoRoot, "backend", "internal", "sample.go")
	if err := os.WriteFile(backendSource, []byte("package internal\nvar Sample = 3\n"), 0o600); err != nil {
		t.Fatalf("update backend source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "coverage.txt"), []byte("mode: atomic\nbackend/internal/sample.go:1.1,2.20 1 0\n"), 0o600); err != nil {
		t.Fatalf("write backend coverage: %v", err)
	}

	mdOut := filepath.Join(repoRoot, "test-results", "patch.md")
	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD",
		"-md-out", mdOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: stderr=%s", result.stderr)
	}

	// #nosec G304 -- Test reads artifact path created by this test.
	body, err := os.ReadFile(mdOut)
	if err != nil {
		t.Fatalf("read markdown report: %v", err)
	}
	if !strings.Contains(string(body), "| Path | Patch Coverage (%) | Uncovered Changed Lines | Uncovered Changed Line Ranges |") {
		t.Fatalf("expected files table in markdown, got: %s", string(body))
	}
}

func TestMainStderrForMissingFrontendCoverage(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	if err := os.Remove(filepath.Join(repoRoot, "frontend", "coverage", "lcov.info")); err != nil {
		t.Fatalf("remove lcov: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
	)
	if result.exitCode == 0 {
		t.Fatalf("expected failure for missing lcov")
	}
	if !strings.Contains(result.stderr, "missing frontend coverage file") {
		t.Fatalf("unexpected stderr: %s", result.stderr)
	}
}

func TestWriteMarkdownWithoutWarningsOrFiles(t *testing.T) {
	report := reportJSON{
		Baseline:         "origin/main...HEAD",
		GeneratedAt:      "2026-02-17T00:00:00Z",
		Mode:             "warn",
		Thresholds:       thresholdJSON{Overall: 90, Backend: 85, Frontend: 85},
		ThresholdSources: thresholdSourcesJSON{Overall: "default", Backend: "default", Frontend: "default"},
		Overall:          patchreport.ScopeCoverage{ChangedLines: 0, CoveredLines: 0, PatchCoveragePct: 100, Status: "pass"},
		Backend:          patchreport.ScopeCoverage{ChangedLines: 0, CoveredLines: 0, PatchCoveragePct: 100, Status: "pass"},
		Frontend:         patchreport.ScopeCoverage{ChangedLines: 0, CoveredLines: 0, PatchCoveragePct: 100, Status: "pass"},
		Artifacts:        artifactsJSON{Markdown: "test-results/out.md", JSON: "test-results/out.json"},
	}

	path := filepath.Join(t.TempDir(), "report.md")
	if err := writeMarkdown(path, report, "backend/coverage.txt", "frontend/coverage/lcov.info"); err != nil {
		t.Fatalf("writeMarkdown failed: %v", err)
	}

	// #nosec G304 -- Test reads artifact path created by this test.
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	text := string(body)
	if strings.Contains(text, "## Warnings") {
		t.Fatalf("did not expect warnings section: %s", text)
	}
	if strings.Contains(text, "## Files Needing Coverage") {
		t.Fatalf("did not expect files section: %s", text)
	}
}

func TestMainProducesExpectedJSONSchemaFields(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	jsonOut := filepath.Join(repoRoot, "test-results", "schema.json")

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-json-out", jsonOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: stderr=%s", result.stderr)
	}

	// #nosec G304 -- Test reads artifact path created by this test.
	body, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal raw json: %v", err)
	}
	required := []string{"baseline", "generated_at", "mode", "thresholds", "threshold_sources", "overall", "backend", "frontend", "artifacts"}
	for _, key := range required {
		if _, ok := raw[key]; !ok {
			t.Fatalf("missing required key %q in report json", key)
		}
	}
}

func TestMainReturnsNonZeroWhenBackendCoveragePathIsDirectory(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	if err := os.Remove(filepath.Join(repoRoot, "backend", "coverage.txt")); err != nil {
		t.Fatalf("remove backend coverage: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "backend", "coverage.txt"), 0o750); err != nil {
		t.Fatalf("create backend coverage dir: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
	)
	if result.exitCode == 0 {
		t.Fatalf("expected failure when backend coverage path is dir")
	}
	if !strings.Contains(result.stderr, "expected backend coverage file to be a file") {
		t.Fatalf("unexpected stderr: %s", result.stderr)
	}
}

func TestMainReturnsNonZeroWhenFrontendCoveragePathIsDirectory(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	lcovPath := filepath.Join(repoRoot, "frontend", "coverage", "lcov.info")
	if err := os.Remove(lcovPath); err != nil {
		t.Fatalf("remove lcov path: %v", err)
	}
	if err := os.MkdirAll(lcovPath, 0o750); err != nil {
		t.Fatalf("create lcov dir: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
	)
	if result.exitCode == 0 {
		t.Fatalf("expected failure when frontend coverage path is dir")
	}
	if !strings.Contains(result.stderr, "expected frontend coverage file to be a file") {
		t.Fatalf("unexpected stderr: %s", result.stderr)
	}
}

func TestMainHandlesAbsoluteOutputPaths(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	jsonOut := filepath.Join(t.TempDir(), "absolute", "report.json")
	mdOut := filepath.Join(t.TempDir(), "absolute", "report.md")

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-json-out", jsonOut,
		"-md-out", mdOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success with absolute outputs: stderr=%s", result.stderr)
	}
	if _, err := os.Stat(jsonOut); err != nil {
		t.Fatalf("expected absolute json file to exist: %v", err)
	}
	if _, err := os.Stat(mdOut); err != nil {
		t.Fatalf("expected absolute markdown file to exist: %v", err)
	}
}

func TestMainWithNoChangedLinesStillPasses(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success when no lines changed, stderr=%s", result.stderr)
	}
}

func TestMain_UsageOfBaselineFlagAffectsGitDiff(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "internal", "sample.go"), []byte("package internal\nvar Sample = 5\n"), 0o600); err != nil {
		t.Fatalf("update backend source: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD",
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success for baseline HEAD, stderr=%s", result.stderr)
	}
}

func TestMainOutputsWarnLinesWhenAnyScopeWarns(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "internal", "sample.go"), []byte("package internal\nvar Sample = 7\n"), 0o600); err != nil {
		t.Fatalf("update backend file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "coverage.txt"), []byte("mode: atomic\nbackend/internal/sample.go:1.1,2.20 1 0\n"), 0o600); err != nil {
		t.Fatalf("write backend coverage: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD",
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success with warnings: stderr=%s", result.stderr)
	}
	if !strings.Contains(result.stdout, "WARN:") {
		t.Fatalf("expected warning lines in stdout: %s", result.stdout)
	}
}

func TestMainProcessHelperWithMalformedArgsExitsNonZero(t *testing.T) {
	// #nosec G204 -- Test helper subprocess invocation with fixed arguments.
	cmd := exec.Command(os.Args[0], "-test.run=TestMainProcessHelper", "--", "-repo-root")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	_, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected helper process to fail for malformed args")
	}
}

func TestWriteMarkdownContainsSummaryTable(t *testing.T) {
	report := reportJSON{
		Baseline:         "origin/main...HEAD",
		GeneratedAt:      "2026-02-17T00:00:00Z",
		Mode:             "warn",
		Thresholds:       thresholdJSON{Overall: 90, Backend: 85, Frontend: 85},
		ThresholdSources: thresholdSourcesJSON{Overall: "default", Backend: "default", Frontend: "default"},
		Overall:          patchreport.ScopeCoverage{ChangedLines: 5, CoveredLines: 2, PatchCoveragePct: 40.0, Status: "warn"},
		Backend:          patchreport.ScopeCoverage{ChangedLines: 3, CoveredLines: 1, PatchCoveragePct: 33.3, Status: "warn"},
		Frontend:         patchreport.ScopeCoverage{ChangedLines: 2, CoveredLines: 1, PatchCoveragePct: 50.0, Status: "warn"},
		Artifacts:        artifactsJSON{Markdown: "test-results/report.md", JSON: "test-results/report.json"},
	}

	path := filepath.Join(t.TempDir(), "summary.md")
	if err := writeMarkdown(path, report, "backend/coverage.txt", "frontend/coverage/lcov.info"); err != nil {
		t.Fatalf("write markdown: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	if !strings.Contains(string(body), "| Scope | Changed Lines | Covered Lines | Patch Coverage (%) | Status |") {
		t.Fatalf("expected summary table in markdown: %s", string(body))
	}
}

func TestMainWithRepoRootDotFromSubprocess(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	commandArgs := []string{"-test.run=TestMainProcessHelper", "--", "-repo-root", ".", "-baseline", "HEAD...HEAD"}
	// #nosec G204 -- Test helper subprocess invocation with controlled arguments.
	cmd := exec.Command(os.Args[0], commandArgs...)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected success with repo-root dot: %v\n%s", err, string(output))
	}
}

func TestMain_InvalidBackendCoverageFlagPath(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-backend-coverage", "backend/does-not-exist.txt",
	)
	if result.exitCode == 0 {
		t.Fatalf("expected failure for invalid backend coverage flag path")
	}
}

func TestMain_InvalidFrontendCoverageFlagPath(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-frontend-coverage", "frontend/coverage/missing.info",
	)
	if result.exitCode == 0 {
		t.Fatalf("expected failure for invalid frontend coverage flag path")
	}
}

func TestGitDiffReturnsContextualErrorOutput(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	_, err := gitDiff(repoRoot, "refs/heads/does-not-exist")
	if err == nil {
		t.Fatal("expected gitDiff to fail")
	}
	if !strings.Contains(err.Error(), "refs/heads/does-not-exist") {
		t.Fatalf("expected baseline in error: %v", err)
	}
}

func TestMain_EmitsWarningsInSortedOrderWithEnvWarning(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	// #nosec G204 -- Test helper subprocess invocation with controlled arguments.
	// #nosec G204 -- Test helper subprocess invocation with controlled arguments.
	cmd := exec.Command(os.Args[0], "-test.run=TestMainProcessHelper", "--", "-repo-root", repoRoot, "-baseline", "HEAD...HEAD")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "CHARON_FRONTEND_PATCH_COVERAGE_MIN=bad")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected success with env warning: %v\n%s", err, string(output))
	}
	if !strings.Contains(string(output), "WARN: Ignoring invalid CHARON_FRONTEND_PATCH_COVERAGE_MIN") {
		t.Fatalf("expected frontend env warning: %s", string(output))
	}
}

func TestMain_FrontendParseErrorWithMissingSFDataStillSucceeds(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	if err := os.WriteFile(filepath.Join(repoRoot, "frontend", "coverage", "lcov.info"), []byte("TN:\nDA:1,1\nend_of_record\n"), 0o600); err != nil {
		t.Fatalf("write lcov: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success with lcov missing SF sections, stderr=%s", result.stderr)
	}
}

func TestMain_BackendCoverageWithInvalidRowsStillSucceeds(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "coverage.txt"), []byte("mode: atomic\nthis is not valid coverage row\nbackend/internal/sample.go:1.1,2.20 1 1\n"), 0o600); err != nil {
		t.Fatalf("write coverage: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success with ignored invalid rows, stderr=%s", result.stderr)
	}
}

func TestMainOutputMentionsModeWarn(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}
	if !strings.Contains(result.stdout, "mode=warn") {
		t.Fatalf("expected mode in stdout: %s", result.stdout)
	}
}

func TestMain_GeneratesMarkdownAtConfiguredRelativePath(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	mdOut := "custom/out/report.md"

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-md-out", mdOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, mdOut)); err != nil {
		t.Fatalf("expected markdown output to exist: %v", err)
	}
}

func TestMain_GeneratesJSONAtConfiguredRelativePath(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	jsonOut := "custom/out/report.json"

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-json-out", jsonOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, jsonOut)); err != nil {
		t.Fatalf("expected json output to exist: %v", err)
	}
}

func TestMainWarningsAppearWhenThresholdRaised(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	cmd := exec.Command(os.Args[0], "-test.run=TestMainProcessHelper", "--", "-repo-root", repoRoot, "-baseline", "HEAD...HEAD")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "CHARON_OVERALL_PATCH_COVERAGE_MIN=101")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected success with invalid threshold env: %v\n%s", err, string(output))
	}
	if !strings.Contains(string(output), "WARN: Ignoring invalid CHARON_OVERALL_PATCH_COVERAGE_MIN") {
		t.Fatalf("expected invalid threshold warning in output: %s", string(output))
	}
}

func TestMain_BaselineFlagRoundTripIntoJSON(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	jsonOut := filepath.Join(repoRoot, "test-results", "baseline.json")

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-json-out", jsonOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}
	body, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}

	var report reportJSON
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if report.Baseline != "HEAD...HEAD" {
		t.Fatalf("expected baseline to match flag, got %s", report.Baseline)
	}
}

func TestMain_WithChangedFilesProducesFilesNeedingCoverageInJSON(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "internal", "sample.go"), []byte("package internal\nvar Sample = 42\n"), 0o600); err != nil {
		t.Fatalf("update backend file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "coverage.txt"), []byte("mode: atomic\nbackend/internal/sample.go:1.1,2.20 1 0\n"), 0o600); err != nil {
		t.Fatalf("write backend coverage: %v", err)
	}

	jsonOut := filepath.Join(repoRoot, "test-results", "coverage-gaps.json")
	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD",
		"-json-out", jsonOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}

	body, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("read json output: %v", err)
	}
	var report reportJSON
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if len(report.FilesNeedingCoverage) == 0 {
		t.Fatalf("expected files_needing_coverage to be non-empty")
	}
}

func TestMain_FailsWhenMarkdownPathParentIsDirectoryFileConflict(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	conflict := filepath.Join(repoRoot, "conflict")
	if err := os.WriteFile(conflict, []byte("x"), 0o600); err != nil {
		t.Fatalf("write conflict file: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-md-out", filepath.Join(conflict, "nested", "report.md"),
	)
	if result.exitCode == 0 {
		t.Fatalf("expected failure due to markdown path parent conflict")
	}
}

func TestMain_FailsWhenJSONPathParentIsDirectoryFileConflict(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	conflict := filepath.Join(repoRoot, "json-conflict")
	if err := os.WriteFile(conflict, []byte("x"), 0o600); err != nil {
		t.Fatalf("write conflict file: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-json-out", filepath.Join(conflict, "nested", "report.json"),
	)
	if result.exitCode == 0 {
		t.Fatalf("expected failure due to json path parent conflict")
	}
}

func TestMain_ReportContainsThresholdSources(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	jsonOut := filepath.Join(repoRoot, "test-results", "threshold-sources.json")

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-json-out", jsonOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}
	body, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	if !strings.Contains(string(body), "\"threshold_sources\"") {
		t.Fatalf("expected threshold_sources in json: %s", string(body))
	}
}

func TestMain_ReportContainsCoverageScopes(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	jsonOut := filepath.Join(repoRoot, "test-results", "scopes.json")

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-json-out", jsonOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}
	body, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	for _, key := range []string{"\"overall\"", "\"backend\"", "\"frontend\""} {
		if !strings.Contains(string(body), key) {
			t.Fatalf("expected %s in json: %s", key, string(body))
		}
	}
}

func TestMain_ReportIncludesGeneratedAt(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	jsonOut := filepath.Join(repoRoot, "test-results", "generated-at.json")

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-json-out", jsonOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}
	body, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	if !strings.Contains(string(body), "\"generated_at\"") {
		t.Fatalf("expected generated_at in json: %s", string(body))
	}
}

func TestMain_ReportIncludesMode(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	jsonOut := filepath.Join(repoRoot, "test-results", "mode.json")

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-json-out", jsonOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}
	body, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	if !strings.Contains(string(body), "\"mode\": \"warn\"") {
		t.Fatalf("expected warn mode in json: %s", string(body))
	}
}

func TestMain_ReportIncludesArtifactsPaths(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	jsonOut := filepath.Join(repoRoot, "test-results", "artifacts.json")
	mdOut := filepath.Join(repoRoot, "test-results", "artifacts.md")

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-json-out", jsonOut,
		"-md-out", mdOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}
	body, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	if !strings.Contains(string(body), "\"artifacts\"") {
		t.Fatalf("expected artifacts object in json: %s", string(body))
	}
}

func TestMain_FailsWhenGitRepoNotInitialized(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "backend"), 0o750); err != nil {
		t.Fatalf("mkdir backend: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "frontend", "coverage"), 0o750); err != nil {
		t.Fatalf("mkdir frontend: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "coverage.txt"), []byte("mode: atomic\nbackend/internal/sample.go:1.1,1.2 1 1\n"), 0o600); err != nil {
		t.Fatalf("write backend coverage: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "frontend", "coverage", "lcov.info"), []byte("TN:\nSF:frontend/src/sample.ts\nDA:1,1\nend_of_record\n"), 0o600); err != nil {
		t.Fatalf("write frontend lcov: %v", err)
	}

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
	)
	if result.exitCode == 0 {
		t.Fatalf("expected failure when repo is not initialized")
	}
	if !strings.Contains(result.stderr, "error generating git diff") {
		t.Fatalf("expected git diff error, got: %s", result.stderr)
	}
}

func TestMain_WritesWarningsToJSONWhenPresent(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "internal", "sample.go"), []byte("package internal\nvar Sample = 8\n"), 0o600); err != nil {
		t.Fatalf("update backend source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "coverage.txt"), []byte("mode: atomic\nbackend/internal/sample.go:1.1,2.20 1 0\n"), 0o600); err != nil {
		t.Fatalf("write backend coverage: %v", err)
	}

	jsonOut := filepath.Join(repoRoot, "test-results", "warnings.json")
	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD",
		"-json-out", jsonOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success with warnings: %s", result.stderr)
	}
	body, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("read warnings json: %v", err)
	}
	if !strings.Contains(string(body), "\"warnings\"") {
		t.Fatalf("expected warnings array in json: %s", string(body))
	}
}

func TestMain_CreatesOutputDirectoriesRecursively(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	jsonOut := filepath.Join(repoRoot, "nested", "json", "report.json")
	mdOut := filepath.Join(repoRoot, "nested", "md", "report.md")

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-json-out", jsonOut,
		"-md-out", mdOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}
	if _, err := os.Stat(jsonOut); err != nil {
		t.Fatalf("expected json output to exist: %v", err)
	}
	if _, err := os.Stat(mdOut); err != nil {
		t.Fatalf("expected markdown output to exist: %v", err)
	}
}

func TestMain_ReportMarkdownIncludesInputs(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	mdOut := filepath.Join(repoRoot, "test-results", "inputs.md")

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-md-out", mdOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}
	body, err := os.ReadFile(mdOut)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	if !strings.Contains(string(body), "- Backend coverage:") || !strings.Contains(string(body), "- Frontend coverage:") {
		t.Fatalf("expected inputs section in markdown: %s", string(body))
	}
}

func TestMain_ReportMarkdownIncludesThresholdTable(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	mdOut := filepath.Join(repoRoot, "test-results", "thresholds.md")

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-md-out", mdOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}
	body, err := os.ReadFile(mdOut)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	if !strings.Contains(string(body), "## Resolved Thresholds") {
		t.Fatalf("expected thresholds section in markdown: %s", string(body))
	}
}

func TestMain_ReportMarkdownIncludesCoverageSummary(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	mdOut := filepath.Join(repoRoot, "test-results", "summary.md")

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-md-out", mdOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}
	body, err := os.ReadFile(mdOut)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	if !strings.Contains(string(body), "## Coverage Summary") {
		t.Fatalf("expected coverage summary section in markdown: %s", string(body))
	}
}

func TestMain_ReportMarkdownIncludesArtifactsSection(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	mdOut := filepath.Join(repoRoot, "test-results", "artifacts.md")

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-md-out", mdOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}
	body, err := os.ReadFile(mdOut)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	if !strings.Contains(string(body), "## Artifacts") {
		t.Fatalf("expected artifacts section in markdown: %s", string(body))
	}
}

func TestMain_RepoRootAbsoluteAndRelativeCoveragePaths(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	absoluteBackend := filepath.Join(repoRoot, "backend", "coverage.txt")
	relativeFrontend := "frontend/coverage/lcov.info"

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
		"-backend-coverage", absoluteBackend,
		"-frontend-coverage", relativeFrontend,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success with mixed path styles: %s", result.stderr)
	}
}

func TestMain_StderrContainsContextOnGitFailure(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "not-a-baseline",
	)
	if result.exitCode == 0 {
		t.Fatalf("expected git failure")
	}
	if !strings.Contains(result.stderr, "error generating git diff") {
		t.Fatalf("expected context in stderr, got: %s", result.stderr)
	}
}

func TestMain_StderrContainsContextOnBackendParseFailure(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	if err := os.WriteFile(filepath.Join(repoRoot, "backend", "coverage.txt"), []byte(strings.Repeat("x", 3*1024*1024)), 0o600); err != nil {
		t.Fatalf("write large backend coverage: %v", err)
	}
	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
	)
	if result.exitCode == 0 {
		t.Fatalf("expected backend parse failure")
	}
	if !strings.Contains(result.stderr, "error parsing backend coverage") {
		t.Fatalf("expected backend parse context, got: %s", result.stderr)
	}
}

func TestMain_StderrContainsContextOnFrontendParseFailure(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	if err := os.WriteFile(filepath.Join(repoRoot, "frontend", "coverage", "lcov.info"), []byte(strings.Repeat("y", 3*1024*1024)), 0o600); err != nil {
		t.Fatalf("write large frontend coverage: %v", err)
	}
	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", "HEAD...HEAD",
	)
	if result.exitCode == 0 {
		t.Fatalf("expected frontend parse failure")
	}
	if !strings.Contains(result.stderr, "error parsing frontend coverage") {
		t.Fatalf("expected frontend parse context, got: %s", result.stderr)
	}
}

func TestMain_UsesConfiguredBaselineInOutput(t *testing.T) {
	repoRoot := createGitRepoWithCoverageInputs(t)
	jsonOut := filepath.Join(repoRoot, "test-results", "baseline-output.json")
	baseline := "HEAD...HEAD"

	result := runMainSubprocess(t,
		"-repo-root", repoRoot,
		"-baseline", baseline,
		"-json-out", jsonOut,
	)
	if result.exitCode != 0 {
		t.Fatalf("expected success: %s", result.stderr)
	}
	body, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("read json output: %v", err)
	}
	if !strings.Contains(string(body), fmt.Sprintf("\"baseline\": %q", baseline)) {
		t.Fatalf("expected baseline in output json, got: %s", string(body))
	}
}
