package caddy

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// repoRootFromCaddyPkg is the repository root relative to this package directory.
const repoRootFromCaddyPkg = "../../.."

// adminAccessExemptScripts lists shell scripts that may publish the admin port.
// caddy-compat-matrix.sh starts its own stock Caddy containers, one per matrix
// cell, and publishes each admin API on a unique host port. It never reads the
// Charon container's admin interface, so the container-local rule does not apply.
var adminAccessExemptScripts = map[string]bool{
	"caddy-compat-matrix.sh": true,
}

// hostPublishedAdminPortPattern matches a `docker run -p <host>:2019` argument.
var hostPublishedAdminPortPattern = regexp.MustCompile(`-p\s+\S+:2019\b`)

// composeAdminPortEntryPattern matches a compose ports list item whose mapping
// ends in :2019. Environment items are excluded by the `=` guard, so
// CHARON_CADDY_ADMIN_API=http://localhost:2019 is not treated as a port.
var composeAdminPortEntryPattern = regexp.MustCompile(`^\s*-\s*"?[^"=]*:2019"?\s*(#.*)?$`)

// TestTestEnvironmentsUseContainerLocalAdminAccess verifies that no test
// environment publishes the proxy admin port to the host. Test tooling reads
// the admin interface from inside the Charon container with docker exec.
func TestTestEnvironmentsUseContainerLocalAdminAccess(t *testing.T) {
	root, err := filepath.Abs(repoRootFromCaddyPkg)
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "scripts")); statErr != nil {
		t.Fatalf("repository root %q does not contain scripts/: %v", root, statErr)
	}

	var violations []string

	scripts, err := filepath.Glob(filepath.Join(root, "scripts", "*.sh"))
	if err != nil {
		t.Fatalf("glob scripts: %v", err)
	}
	for _, path := range scripts {
		if adminAccessExemptScripts[filepath.Base(path)] {
			continue
		}
		violations = append(violations, scanFileLines(t, path, root, hostPublishedAdminPortPattern)...)
	}

	composes, err := filepath.Glob(filepath.Join(root, ".docker", "compose", "*.yml"))
	if err != nil {
		t.Fatalf("glob compose files: %v", err)
	}
	for _, path := range composes {
		violations = append(violations, scanFileLines(t, path, root, composeAdminPortEntryPattern)...)
	}

	if len(violations) > 0 {
		t.Errorf("test environments publish the proxy admin port; read it with "+
			"`docker exec <container> wget -qO- http://127.0.0.1:2019/config/` instead:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// scanFileLines returns "relative/path:line: text" for every line that matches pattern.
func scanFileLines(t *testing.T, path, root string, pattern *regexp.Regexp) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	var hits []string
	for i, line := range strings.Split(string(data), "\n") {
		if pattern.MatchString(line) {
			hits = append(hits, rel+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(line))
		}
	}
	return hits
}
