package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/patchreport"
)

type thresholdJSON struct {
	Overall  float64 `json:"overall_patch_coverage_min"`
	Backend  float64 `json:"backend_patch_coverage_min"`
	Frontend float64 `json:"frontend_patch_coverage_min"`
	Agent    float64 `json:"agent_patch_coverage_min"`
}

type thresholdSourcesJSON struct {
	Overall  string `json:"overall"`
	Backend  string `json:"backend"`
	Frontend string `json:"frontend"`
	Agent    string `json:"agent"`
}

type artifactsJSON struct {
	Markdown string `json:"markdown"`
	JSON     string `json:"json"`
}

type reportJSON struct {
	Baseline             string                           `json:"baseline"`
	GeneratedAt          string                           `json:"generated_at"`
	Mode                 string                           `json:"mode"`
	Thresholds           thresholdJSON                    `json:"thresholds"`
	ThresholdSources     thresholdSourcesJSON             `json:"threshold_sources"`
	Overall              patchreport.ScopeCoverage        `json:"overall"`
	Backend              patchreport.ScopeCoverage        `json:"backend"`
	Frontend             patchreport.ScopeCoverage        `json:"frontend"`
	Agent                patchreport.ScopeCoverage        `json:"agent"`
	FilesNeedingCoverage []patchreport.FileCoverageDetail `json:"files_needing_coverage,omitempty"`
	Warnings             []string                         `json:"warnings,omitempty"`
	Artifacts            artifactsJSON                    `json:"artifacts"`
}

func main() {
	repoRootFlag := flag.String("repo-root", ".", "Repository root path")
	baselineFlag := flag.String("baseline", "origin/development...HEAD", "Git diff baseline")
	backendCoverageFlag := flag.String("backend-coverage", "backend/coverage.txt", "Backend Go coverage profile")
	frontendCoverageFlag := flag.String("frontend-coverage", "frontend/coverage/lcov.info", "Frontend LCOV coverage report")
	agentCoverageFlag := flag.String("agent-coverage", "agent/coverage.txt", "Agent Go coverage profile")
	jsonOutFlag := flag.String("json-out", "test-results/local-patch-report.json", "Path to JSON output report")
	mdOutFlag := flag.String("md-out", "test-results/local-patch-report.md", "Path to markdown output report")
	advisoryFlag := flag.Bool("advisory", false, "Exit 0 even if any coverage scope is below threshold (advisory-only mode). Default is strict: non-zero exit on any below-threshold scope, per CLAUDE.md's Definition of Done Step 2.")
	flag.Parse()

	repoRoot, err := filepath.Abs(*repoRootFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error resolving repo root: %v\n", err)
		os.Exit(1)
	}

	backendCoveragePath := resolvePath(repoRoot, *backendCoverageFlag)
	frontendCoveragePath := resolvePath(repoRoot, *frontendCoverageFlag)
	agentCoveragePath := resolvePath(repoRoot, *agentCoverageFlag)
	jsonOutPath := resolvePath(repoRoot, *jsonOutFlag)
	mdOutPath := resolvePath(repoRoot, *mdOutFlag)

	err = assertFileExists(backendCoveragePath, "backend coverage file")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	err = assertFileExists(frontendCoveragePath, "frontend coverage file")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	err = assertFileExists(agentCoveragePath, "agent coverage file")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	diffContent, err := gitDiff(repoRoot, *baselineFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating git diff: %v\n", err)
		os.Exit(1)
	}

	backendChanged, frontendChanged, agentChanged, err := patchreport.ParseUnifiedDiffChangedLines(diffContent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing changed lines from diff: %v\n", err)
		os.Exit(1)
	}

	backendCoverage, err := patchreport.ParseGoCoverageProfile(backendCoveragePath, "backend")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing backend coverage: %v\n", err)
		os.Exit(1)
	}
	frontendCoverage, err := patchreport.ParseLCOVProfile(frontendCoveragePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing frontend coverage: %v\n", err)
		os.Exit(1)
	}
	agentCoverage, err := patchreport.ParseGoCoverageProfile(agentCoveragePath, "agent")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing agent coverage: %v\n", err)
		os.Exit(1)
	}

	overallThreshold := patchreport.ResolveThreshold("CHARON_OVERALL_PATCH_COVERAGE_MIN", 90, nil)
	backendThreshold := patchreport.ResolveThreshold("CHARON_BACKEND_PATCH_COVERAGE_MIN", 85, nil)
	frontendThreshold := patchreport.ResolveThreshold("CHARON_FRONTEND_PATCH_COVERAGE_MIN", 85, nil)
	agentThreshold := patchreport.ResolveThreshold("CHARON_AGENT_PATCH_COVERAGE_MIN", 85, nil)

	backendScope := patchreport.ComputeScopeCoverage(backendChanged, backendCoverage)
	frontendScope := patchreport.ComputeScopeCoverage(frontendChanged, frontendCoverage)
	agentScope := patchreport.ComputeScopeCoverage(agentChanged, agentCoverage)
	overallScope := patchreport.MergeScopeCoverage(backendScope, frontendScope, agentScope)
	backendFilesNeedingCoverage := patchreport.ComputeFilesNeedingCoverage(backendChanged, backendCoverage, backendThreshold.Value)
	frontendFilesNeedingCoverage := patchreport.ComputeFilesNeedingCoverage(frontendChanged, frontendCoverage, frontendThreshold.Value)
	agentFilesNeedingCoverage := patchreport.ComputeFilesNeedingCoverage(agentChanged, agentCoverage, agentThreshold.Value)
	filesNeedingCoverage := patchreport.MergeFileCoverageDetails(backendFilesNeedingCoverage, frontendFilesNeedingCoverage, agentFilesNeedingCoverage)

	backendScope = patchreport.ApplyStatus(backendScope, backendThreshold.Value)
	frontendScope = patchreport.ApplyStatus(frontendScope, frontendThreshold.Value)
	agentScope = patchreport.ApplyStatus(agentScope, agentThreshold.Value)
	overallScope = patchreport.ApplyStatus(overallScope, overallThreshold.Value)

	warnings := patchreport.SortedWarnings([]string{
		overallThreshold.Warning,
		backendThreshold.Warning,
		frontendThreshold.Warning,
		agentThreshold.Warning,
	})
	if overallScope.Status == "warn" {
		warnings = append(warnings, fmt.Sprintf("Overall patch coverage %.1f%% is below threshold %.1f%%", overallScope.PatchCoveragePct, overallThreshold.Value))
	}
	if backendScope.Status == "warn" {
		warnings = append(warnings, fmt.Sprintf("Backend patch coverage %.1f%% is below threshold %.1f%%", backendScope.PatchCoveragePct, backendThreshold.Value))
	}
	if frontendScope.Status == "warn" {
		warnings = append(warnings, fmt.Sprintf("Frontend patch coverage %.1f%% is below threshold %.1f%%", frontendScope.PatchCoveragePct, frontendThreshold.Value))
	}
	if agentScope.Status == "warn" {
		warnings = append(warnings, fmt.Sprintf("Agent patch coverage %.1f%% is below threshold %.1f%%", agentScope.PatchCoveragePct, agentThreshold.Value))
	}

	mode := "strict"
	if *advisoryFlag {
		mode = "advisory"
	}

	report := reportJSON{
		Baseline:    *baselineFlag,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Mode:        mode,
		Thresholds: thresholdJSON{
			Overall:  overallThreshold.Value,
			Backend:  backendThreshold.Value,
			Frontend: frontendThreshold.Value,
			Agent:    agentThreshold.Value,
		},
		ThresholdSources: thresholdSourcesJSON{
			Overall:  overallThreshold.Source,
			Backend:  backendThreshold.Source,
			Frontend: frontendThreshold.Source,
			Agent:    agentThreshold.Source,
		},
		Overall:              overallScope,
		Backend:              backendScope,
		Frontend:             frontendScope,
		Agent:                agentScope,
		FilesNeedingCoverage: filesNeedingCoverage,
		Warnings:             warnings,
		Artifacts: artifactsJSON{
			Markdown: relOrAbs(repoRoot, mdOutPath),
			JSON:     relOrAbs(repoRoot, jsonOutPath),
		},
	}

	if err := os.MkdirAll(filepath.Dir(jsonOutPath), 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "error creating json output directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(mdOutPath), 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "error creating markdown output directory: %v\n", err)
		os.Exit(1)
	}

	if err := writeJSON(jsonOutPath, report); err != nil {
		fmt.Fprintf(os.Stderr, "error writing json report: %v\n", err)
		os.Exit(1)
	}
	if err := writeMarkdown(mdOutPath, report, relOrAbs(repoRoot, backendCoveragePath), relOrAbs(repoRoot, frontendCoveragePath), relOrAbs(repoRoot, agentCoveragePath)); err != nil {
		fmt.Fprintf(os.Stderr, "error writing markdown report: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Local patch report generated (mode=%s)\n", report.Mode)
	fmt.Printf("JSON: %s\n", relOrAbs(repoRoot, jsonOutPath))
	fmt.Printf("Markdown: %s\n", relOrAbs(repoRoot, mdOutPath))
	for _, warning := range warnings {
		fmt.Printf("WARN: %s\n", warning)
	}

	// Strict-mode enforcement runs last, after both artifacts are written to
	// disk and the WARN diagnostic lines above are printed to stdout, so a
	// failing gate still leaves a developer a full report explaining exactly
	// which scope(s)/threshold(s) failed (Supervisor Correction 1) in
	// addition to this stderr message and the artifacts on disk.
	if !*advisoryFlag && patchreport.HasWarnStatus(overallScope, backendScope, frontendScope, agentScope) {
		fmt.Fprintln(os.Stderr, "Local patch coverage below threshold in strict mode (use -advisory to bypass); see report for details.")
		os.Exit(1)
	}
}

func resolvePath(repoRoot, configured string) string {
	if filepath.IsAbs(configured) {
		return configured
	}
	return filepath.Join(repoRoot, configured)
}

func relOrAbs(repoRoot, path string) string {
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func assertFileExists(path, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("missing %s at %s: %w", label, path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("expected %s to be a file but found directory: %s", label, path)
	}
	return nil
}

func gitDiff(repoRoot, baseline string) (string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "diff", "--unified=0", baseline)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff %s failed: %w (%s)", baseline, err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func writeJSON(path string, report reportJSON) error {
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report json: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write report json file: %w", err)
	}
	return nil
}

func writeMarkdown(path string, report reportJSON, backendCoveragePath, frontendCoveragePath, agentCoveragePath string) error {
	var builder strings.Builder
	builder.WriteString("# Local Patch Coverage Report\n\n")
	builder.WriteString("## Metadata\n\n")
	fmt.Fprintf(&builder, "- Generated: %s\n", report.GeneratedAt)
	fmt.Fprintf(&builder, "- Baseline: `%s`\n", report.Baseline)
	fmt.Fprintf(&builder, "- Mode: `%s`\n\n", report.Mode)

	builder.WriteString("## Inputs\n\n")
	fmt.Fprintf(&builder, "- Backend coverage: `%s`\n", backendCoveragePath)
	fmt.Fprintf(&builder, "- Frontend coverage: `%s`\n", frontendCoveragePath)
	fmt.Fprintf(&builder, "- Agent coverage: `%s`\n\n", agentCoveragePath)

	builder.WriteString("## Resolved Thresholds\n\n")
	builder.WriteString("| Scope | Minimum (%) | Source |\n")
	builder.WriteString("|---|---:|---|\n")
	fmt.Fprintf(&builder, "| Overall | %.1f | %s |\n", report.Thresholds.Overall, report.ThresholdSources.Overall)
	fmt.Fprintf(&builder, "| Backend | %.1f | %s |\n", report.Thresholds.Backend, report.ThresholdSources.Backend)
	fmt.Fprintf(&builder, "| Frontend | %.1f | %s |\n", report.Thresholds.Frontend, report.ThresholdSources.Frontend)
	fmt.Fprintf(&builder, "| Agent | %.1f | %s |\n\n", report.Thresholds.Agent, report.ThresholdSources.Agent)

	builder.WriteString("## Coverage Summary\n\n")
	builder.WriteString("| Scope | Changed Lines | Covered Lines | Patch Coverage (%) | Status |\n")
	builder.WriteString("|---|---:|---:|---:|---|\n")
	builder.WriteString(scopeRow("Overall", report.Overall))
	builder.WriteString(scopeRow("Backend", report.Backend))
	builder.WriteString(scopeRow("Frontend", report.Frontend))
	builder.WriteString(scopeRow("Agent", report.Agent))
	builder.WriteString("\n")

	if len(report.FilesNeedingCoverage) > 0 {
		builder.WriteString("## Files Needing Coverage\n\n")
		builder.WriteString("| Path | Patch Coverage (%) | Uncovered Changed Lines | Uncovered Changed Line Ranges |\n")
		builder.WriteString("|---|---:|---:|---|\n")
		for _, fileCoverage := range report.FilesNeedingCoverage {
			ranges := "-"
			if len(fileCoverage.UncoveredChangedLineRange) > 0 {
				ranges = strings.Join(fileCoverage.UncoveredChangedLineRange, ", ")
			}
			fmt.Fprintf(&builder, "| `%s` | %.1f | %d | %s |\n", fileCoverage.Path, fileCoverage.PatchCoveragePct, fileCoverage.UncoveredChangedLines, ranges)
		}
		builder.WriteString("\n")
	}

	if len(report.Warnings) > 0 {
		builder.WriteString("## Warnings\n\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&builder, "- %s\n", warning)
		}
		builder.WriteString("\n")
	}

	builder.WriteString("## Artifacts\n\n")
	fmt.Fprintf(&builder, "- Markdown: `%s`\n", report.Artifacts.Markdown)
	fmt.Fprintf(&builder, "- JSON: `%s`\n", report.Artifacts.JSON)

	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		return fmt.Errorf("write markdown file: %w", err)
	}
	return nil
}

func scopeRow(name string, scope patchreport.ScopeCoverage) string {
	return fmt.Sprintf("| %s | %d | %d | %.1f | %s |\n", name, scope.ChangedLines, scope.CoveredLines, scope.PatchCoveragePct, scope.Status)
}
