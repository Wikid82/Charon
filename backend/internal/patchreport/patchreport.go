package patchreport

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type LineSet map[int]struct{}

type FileLineSet map[string]LineSet

type CoverageData struct {
	Executable FileLineSet
	Covered    FileLineSet
}

type ScopeCoverage struct {
	ChangedLines     int     `json:"changed_lines"`
	CoveredLines     int     `json:"covered_lines"`
	PatchCoveragePct float64 `json:"patch_coverage_pct"`
	Status           string  `json:"status"`
}

type ThresholdResolution struct {
	Value   float64
	Source  string
	Warning string
}

var hunkPattern = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

const maxScannerTokenSize = 2 * 1024 * 1024

func newScannerWithLargeBuffer(input *strings.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScannerTokenSize)
	return scanner
}

func newFileScannerWithLargeBuffer(file *os.File) *bufio.Scanner {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScannerTokenSize)
	return scanner
}

func ResolveThreshold(envName string, defaultValue float64, lookup func(string) (string, bool)) ThresholdResolution {
	if lookup == nil {
		lookup = os.LookupEnv
	}

	raw, ok := lookup(envName)
	if !ok {
		return ThresholdResolution{Value: defaultValue, Source: "default"}
	}

	raw = strings.TrimSpace(raw)
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 0 || value > 100 {
		return ThresholdResolution{
			Value:   defaultValue,
			Source:  "default",
			Warning: fmt.Sprintf("Ignoring invalid %s=%q; using default %.1f", envName, raw, defaultValue),
		}
	}

	return ThresholdResolution{Value: value, Source: "env"}
}

func ParseUnifiedDiffChangedLines(diffContent string) (FileLineSet, FileLineSet, error) {
	backendChanged := make(FileLineSet)
	frontendChanged := make(FileLineSet)

	var currentFile string
	currentScope := ""
	currentNewLine := 0
	inHunk := false

	scanner := newScannerWithLargeBuffer(strings.NewReader(diffContent))
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "+++") {
			currentFile = ""
			currentScope = ""
			inHunk = false

			newFile := strings.TrimSpace(strings.TrimPrefix(line, "+++"))
			if newFile == "/dev/null" {
				continue
			}
			newFile = strings.TrimPrefix(newFile, "b/")
			newFile = normalizeRepoPath(newFile)
			if strings.HasPrefix(newFile, "backend/") {
				currentFile = newFile
				currentScope = "backend"
			} else if strings.HasPrefix(newFile, "frontend/") {
				currentFile = newFile
				currentScope = "frontend"
			}
			continue
		}

		if matches := hunkPattern.FindStringSubmatch(line); matches != nil {
			startLine, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, nil, fmt.Errorf("parse hunk start line: %w", err)
			}
			currentNewLine = startLine
			inHunk = true
			continue
		}

		if !inHunk || currentFile == "" || currentScope == "" || line == "" {
			continue
		}

		switch line[0] {
		case '+':
			if strings.HasPrefix(line, "+++") {
				continue
			}
			switch currentScope {
			case "backend":
				addLine(backendChanged, currentFile, currentNewLine)
			case "frontend":
				addLine(frontendChanged, currentFile, currentNewLine)
			}
			currentNewLine++
		case '-':
		case ' ':
			currentNewLine++
		case '\\':
		default:
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan diff content: %w", err)
	}

	return backendChanged, frontendChanged, nil
}

func ParseGoCoverageProfile(profilePath string) (data CoverageData, err error) {
	validatedPath, err := validateReadablePath(profilePath)
	if err != nil {
		return CoverageData{}, fmt.Errorf("validate go coverage profile path: %w", err)
	}

	// #nosec G304 -- validatedPath is cleaned and resolved to an absolute path by validateReadablePath.
	file, err := os.Open(validatedPath)
	if err != nil {
		return CoverageData{}, fmt.Errorf("open go coverage profile: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close go coverage profile: %w", closeErr)
		}
	}()

	data = CoverageData{
		Executable: make(FileLineSet),
		Covered:    make(FileLineSet),
	}

	scanner := newFileScannerWithLargeBuffer(file)
	firstLine := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if firstLine {
			firstLine = false
			if strings.HasPrefix(line, "mode:") {
				continue
			}
		}

		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}

		count, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}

		filePart, startLine, endLine, err := parseCoverageRange(fields[0])
		if err != nil {
			continue
		}

		normalizedFile := normalizeGoCoveragePath(filePart)
		if normalizedFile == "" {
			continue
		}

		for lineNo := startLine; lineNo <= endLine; lineNo++ {
			addLine(data.Executable, normalizedFile, lineNo)
			if count > 0 {
				addLine(data.Covered, normalizedFile, lineNo)
			}
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return CoverageData{}, fmt.Errorf("scan go coverage profile: %w", scanErr)
	}

	return data, nil
}

func ParseLCOVProfile(lcovPath string) (data CoverageData, err error) {
	validatedPath, err := validateReadablePath(lcovPath)
	if err != nil {
		return CoverageData{}, fmt.Errorf("validate lcov profile path: %w", err)
	}

	// #nosec G304 -- validatedPath is cleaned and resolved to an absolute path by validateReadablePath.
	file, err := os.Open(validatedPath)
	if err != nil {
		return CoverageData{}, fmt.Errorf("open lcov profile: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close lcov profile: %w", closeErr)
		}
	}()

	data = CoverageData{
		Executable: make(FileLineSet),
		Covered:    make(FileLineSet),
	}

	currentFiles := make([]string, 0, 2)
	scanner := newFileScannerWithLargeBuffer(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "SF:"):
			sourceFile := strings.TrimSpace(strings.TrimPrefix(line, "SF:"))
			currentFiles = normalizeFrontendCoveragePaths(sourceFile)
		case strings.HasPrefix(line, "DA:"):
			if len(currentFiles) == 0 {
				continue
			}
			parts := strings.Split(strings.TrimPrefix(line, "DA:"), ",")
			if len(parts) < 2 {
				continue
			}
			lineNo, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				continue
			}
			hits, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				continue
			}
			for _, filePath := range currentFiles {
				addLine(data.Executable, filePath, lineNo)
				if hits > 0 {
					addLine(data.Covered, filePath, lineNo)
				}
			}
		case line == "end_of_record":
			currentFiles = currentFiles[:0]
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return CoverageData{}, fmt.Errorf("scan lcov profile: %w", scanErr)
	}

	return data, nil
}

func ComputeScopeCoverage(changedLines FileLineSet, coverage CoverageData) ScopeCoverage {
	changedCount := 0
	coveredCount := 0

	for filePath, lines := range changedLines {
		executable, ok := coverage.Executable[filePath]
		if !ok {
			continue
		}
		coveredLines := coverage.Covered[filePath]

		for lineNo := range lines {
			if _, executableLine := executable[lineNo]; !executableLine {
				continue
			}
			changedCount++
			if _, isCovered := coveredLines[lineNo]; isCovered {
				coveredCount++
			}
		}
	}

	pct := 100.0
	if changedCount > 0 {
		pct = roundToOneDecimal(float64(coveredCount) * 100 / float64(changedCount))
	}

	return ScopeCoverage{
		ChangedLines:     changedCount,
		CoveredLines:     coveredCount,
		PatchCoveragePct: pct,
	}
}

func MergeScopeCoverage(scopes ...ScopeCoverage) ScopeCoverage {
	changed := 0
	covered := 0
	for _, scope := range scopes {
		changed += scope.ChangedLines
		covered += scope.CoveredLines
	}

	pct := 100.0
	if changed > 0 {
		pct = roundToOneDecimal(float64(covered) * 100 / float64(changed))
	}

	return ScopeCoverage{
		ChangedLines:     changed,
		CoveredLines:     covered,
		PatchCoveragePct: pct,
	}
}

func ApplyStatus(scope ScopeCoverage, minThreshold float64) ScopeCoverage {
	scope.Status = "pass"
	if scope.PatchCoveragePct < minThreshold {
		scope.Status = "warn"
	}
	return scope
}

func SortedWarnings(warnings []string) []string {
	filtered := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		if strings.TrimSpace(warning) != "" {
			filtered = append(filtered, warning)
		}
	}
	sort.Strings(filtered)
	return filtered
}

func parseCoverageRange(rangePart string) (string, int, int, error) {
	pathAndRange := strings.SplitN(rangePart, ":", 2)
	if len(pathAndRange) != 2 {
		return "", 0, 0, fmt.Errorf("invalid range format")
	}

	filePart := strings.TrimSpace(pathAndRange[0])
	rangeSpec := strings.TrimSpace(pathAndRange[1])
	coords := strings.SplitN(rangeSpec, ",", 2)
	if len(coords) != 2 {
		return "", 0, 0, fmt.Errorf("invalid coordinate format")
	}

	startParts := strings.SplitN(coords[0], ".", 2)
	endParts := strings.SplitN(coords[1], ".", 2)
	if len(startParts) == 0 || len(endParts) == 0 {
		return "", 0, 0, fmt.Errorf("invalid line coordinate")
	}

	startLine, err := strconv.Atoi(startParts[0])
	if err != nil {
		return "", 0, 0, fmt.Errorf("parse start line: %w", err)
	}
	endLine, err := strconv.Atoi(endParts[0])
	if err != nil {
		return "", 0, 0, fmt.Errorf("parse end line: %w", err)
	}
	if startLine <= 0 || endLine <= 0 || endLine < startLine {
		return "", 0, 0, fmt.Errorf("invalid line range")
	}

	return filePart, startLine, endLine, nil
}

func normalizeRepoPath(input string) string {
	cleaned := filepath.ToSlash(filepath.Clean(strings.TrimSpace(input)))
	cleaned = strings.TrimPrefix(cleaned, "./")
	return cleaned
}

func normalizeGoCoveragePath(input string) string {
	cleaned := normalizeRepoPath(input)
	if cleaned == "" {
		return ""
	}

	if strings.HasPrefix(cleaned, "backend/") {
		return cleaned
	}
	if idx := strings.Index(cleaned, "/backend/"); idx >= 0 {
		return cleaned[idx+1:]
	}

	repoRelativePrefixes := []string{"cmd/", "internal/", "pkg/", "api/", "integration/", "tools/"}
	for _, prefix := range repoRelativePrefixes {
		if strings.HasPrefix(cleaned, prefix) {
			return "backend/" + cleaned
		}
	}

	return cleaned
}

func normalizeFrontendCoveragePaths(input string) []string {
	cleaned := normalizeRepoPath(input)
	if cleaned == "" {
		return nil
	}

	seen := map[string]struct{}{}
	result := make([]string, 0, 3)
	add := func(value string) {
		value = normalizeRepoPath(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	add(cleaned)
	if idx := strings.Index(cleaned, "/frontend/"); idx >= 0 {
		frontendPath := cleaned[idx+1:]
		add(frontendPath)
		add(strings.TrimPrefix(frontendPath, "frontend/"))
	} else if strings.HasPrefix(cleaned, "frontend/") {
		add(strings.TrimPrefix(cleaned, "frontend/"))
	} else {
		add("frontend/" + cleaned)
	}

	return result
}

func addLine(set FileLineSet, filePath string, lineNo int) {
	if lineNo <= 0 || filePath == "" {
		return
	}
	if _, ok := set[filePath]; !ok {
		set[filePath] = make(LineSet)
	}
	set[filePath][lineNo] = struct{}{}
}

func roundToOneDecimal(value float64) float64 {
	return float64(int(value*10+0.5)) / 10
}

func validateReadablePath(rawPath string) (string, error) {
	trimmedPath := strings.TrimSpace(rawPath)
	if trimmedPath == "" {
		return "", fmt.Errorf("path is empty")
	}

	cleanedPath := filepath.Clean(trimmedPath)
	absolutePath, err := filepath.Abs(cleanedPath)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}

	return absolutePath, nil
}
