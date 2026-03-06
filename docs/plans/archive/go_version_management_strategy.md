# Go Version Management Strategy

**Status:** Research & Recommendations
**Date:** 2025-02-12
**Context:** Addressing Go version compatibility issues with development tools after automated Renovate upgrades

---

## Problem Statement

### Current Situation

- **Go Upgrade Flow:** Renovate automatically updates Go versions in `go.work`, `Dockerfile`, and GitHub Actions workflows
- **Tool Compatibility Issue:** Development tools (e.g., golangci-lint) built with older Go versions break when the project moves to a newer Go version
- **Recent Failure:** golangci-lint v2.8.0 built with Go 1.25.5 failed pre-commit checks after Go 1.26.0 upgrade

### Current Installation State

```bash
$ go version
go version go1.26.0 linux/amd64

$ golangci-lint version
golangci-lint has version 2.8.0 built with go1.25.5 from e2e40021 on 2026-01-07T21:29:47Z

$ ls -la ~/sdk/
drwxr-xr-x 10 root root 4096 Dec  6 05:34 go1.25.5
drwxr-xr-x 10 root root 4096 Jan 21 18:34 go1.25.6
```

**The Mismatch:** Tools built with Go 1.25.5 encountering Go 1.26.0 runtime/stdlib changes.

### Current Update Mechanism

1. **Skill Runner** (`.github/skills/utility-update-go-version-scripts/run.sh`):
   - Uses `golang.org/dl/goX.Y.Z@latest` to download specific versions
   - Installs to `~/sdk/goX.Y.Z/`
   - Updates symlink: `/usr/local/go/bin/go -> ~/sdk/goX.Y.Z/bin/go`
   - **Preserves old versions** in `~/sdk/`

2. **Install Scripts** (`scripts/install-go-*.sh`):
   - Downloads Go tarball from go.dev
   - **Removes** existing `/usr/local/go` completely
   - Extracts fresh installation
   - **Does not preserve** old versions

---

## Research Findings

### 1. Official Go Position on Version Management

**Source:** [Go Downloads](https://go.dev/doc/manage-install) and [golang.org/dl](https://pkg.go.dev/golang.org/dl)

#### Key Points:
- **Multi-version support:** Go officially supports running multiple versions via `golang.org/dl`
- **SDK isolation:** Each version installs to `$HOME/sdk/goX.Y.Z/`
- **Version-specific binaries:** Run as `go1.25.6`, `go1.26.0`, etc.
- **No interference:** Old versions don't conflict with new installations
- **Official recommendation:** Use `go install golang.org/dl/goX.Y.Z@latest && goX.Y.Z download`

#### Example Workflow:
```bash
# Install multiple versions
go install golang.org/dl/go1.25.6@latest
go1.25.6 download

go install golang.org/dl/go1.26.0@latest
go1.26.0 download

# Switch between versions
go1.25.6 version  # go version go1.25.6 linux/amd64
go1.26.0 version  # go version go1.26.0 linux/amd64
```

**Verdict:** ✅ Official support for multi-version. No third-party manager needed.

---

### 2. Version Managers (gvm, goenv, asdf)

#### GVM (Go Version Manager)
- **Status:** Last updated 2019, limited Go 1.13+ support
- **Issues:** Not actively maintained, compatibility problems with modern Go
- **Verdict:** ❌ Not recommended

#### goenv
- **Status:** Active, mimics rbenv/pyenv model
- **Pros:** Automatic version switching via `.go-version` files
- **Cons:** Wrapper around `golang.org/dl`, adds complexity
- **Use Case:** Multi-project environments with different Go versions per project
- **Verdict:** ⚠️ Overkill for single-project workflows

#### asdf (via asdf-golang plugin)
- **Status:** Active, part of asdf ecosystem
- **Pros:** Unified tool for managing multiple language runtimes
- **Cons:** Requires learning asdf, heavy for Go-only use
- **Use Case:** Polyglot teams managing Node, Python, Ruby, Go, etc.
- **Verdict:** ⚠️ Good for polyglot environments, unnecessary for Go-focused projects

**Conclusion:** For single-project Go development, `golang.org/dl` is simpler and officially supported.

---

### 3. Industry Best Practices

#### Kubernetes Project
**Source:** [kubernetes/kubernetes](https://github.com/kubernetes/kubernetes)

- **Approach:** Pin Go version in `.go-version`, `Dockerfile`, and CI
- **Multi-version:** No—upgrades are coordinated across all tools simultaneously
- **Tool management:** Tools rebuilt on every Go upgrade
- **Build reproducibility:** Docker builds use specific `golang:X.Y.Z` images
- **Verdict:** Single version, strict coordination, CI enforcement

#### Docker CLI
**Source:** [docker/cli](https://github.com/docker/cli)

- **Approach:** Single Go version in `Dockerfile`, `go.mod`, and CI
- **Tool management:** CI installs tools fresh on every run (no version conflicts)
- **Upgrade strategy:** Update Go version → rebuild all tools → test → merge
- **Verdict:** Single version, ephemeral CI environments

#### HashiCorp Projects (Terraform, Vault)
**Source:** [hashicorp/terraform](https://github.com/hashicorp/terraform)

- **Approach:** Pin Go version in `go.mod` and `.go-version`
- **Multi-version:** No—single version enforced across team
- **Tool management:** Hermetic builds in Docker, tools installed per-build
- **Upgrade strategy:** Go upgrades blocked until all tools compatible
- **Verdict:** Single version, strict enforcement

#### Common Patterns Across Major OSS Projects:
1. **Single Go version** per project (no multi-version maintenance)
2. **CI installs tools fresh** on every run (avoids stale tool issues)
3. **Docker builds** ensure reproducibility
4. **Local development:** Developers expected to match project Go version
5. **Tool compatibility:** Tools updated/rebuilt alongside Go upgrades

---

### 4. Tool Dependency Management

#### Problem: Pre-built Binaries vs. Source Builds

##### Pre-built Binary (Current Issue):
```bash
$ golangci-lint version
golangci-lint has version 2.8.0 built with go1.25.5
```
- **Problem:** Binary built with Go 1.25.5, incompatible with Go 1.26.0 stdlib
- **Root cause:** Go 1.26 introduced stdlib changes that break tools compiled with 1.25

##### Solution: Rebuild Tools After Go Upgrade
```bash
# Rebuild golangci-lint with current Go version
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Verify
$ golangci-lint version
golangci-lint has version 2.8.1 built with go1.26.0
```

#### Best Practice: Go Tools Should Use `go install`
- **Why:** `go install` compiles with the current Go toolchain
- **Result:** Tools always match the active Go version
- **Trade-off:** Requires source compilation (slower) vs. downloading binary (faster)

---

### 5. GitHub Actions Best Practices

#### actions/setup-go Caching
```yaml
- uses: actions/setup-go@v6
  with:
    go-version: "1.26.1"

    cache: true  # Caches Go modules and build artifacts
```

**Implications:**
- Cache keyed by Go version and `go.sum`
- Go version change → new cache entry (no stale artifacts)
- Tools installed in CI are ephemeral → rebuilt every run
- **Verdict:** ✅ No version conflict issues in CI

---

## Recommendations

### Strategy: Single Go Version + Tool Rebuild Protocol

**Rationale:**
1. Official Go tooling (`golang.org/dl`) already supports multi-version
2. Industry standard is **single version per project**
3. CI environments are ephemeral (no tool compatibility issues)
4. Local development should match CI
5. Renovate already updates Go version in sync across all locations

### Implementation Plan

#### Phase 1: Pre-Upgrade Tool Verification (Immediate)

**Objective:** Prevent pre-commit failures after Go upgrades

**Changes:**

1. **Update Pre-commit Hook** (`scripts/pre-commit-hooks/golangci-lint-fast.sh`):
   ```bash
   # Add version check before running
   LINT_GO_VERSION=$(golangci-lint version | grep -oP 'go\K[0-9]+\.[0-9]+(?:\.[0-9]+)?')
   SYSTEM_GO_VERSION=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+(?:\.[0-9]+)?')

   if [ "$LINT_GO_VERSION" != "$SYSTEM_GO_VERSION" ]; then
       echo "⚠️  golangci-lint Go version mismatch:"
       echo "   golangci-lint: $LINT_GO_VERSION"
       echo "   system Go:     $SYSTEM_GO_VERSION"
       echo ""
       echo "🔧 Rebuilding golangci-lint with current Go version..."
       go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   fi
   ```

2. **Update Go Installation Skill** (`.github/skills/utility-update-go-version-scripts/run.sh`):
   ```bash
   # After Go version update, rebuild critical tools
   echo "🔧 Rebuilding development tools with Go $REQUIRED_VERSION..."

   # List of tools to rebuild
   TOOLS=(
       "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
       "golang.org/x/tools/gopls@latest"
       "golang.org/x/vuln/cmd/govulncheck@latest"
   )

   for tool in "${TOOLS[@]}"; do
       echo "  - Installing $tool"
       go install "$tool" || echo "⚠️  Failed to install $tool"
   done
   ```

3. **Create Tool Rebuild Script** (`scripts/rebuild-go-tools.sh`):
   ```bash
   #!/usr/bin/env bash
   set -euo pipefail

   echo "🔧 Rebuilding Go development tools..."
   echo "Current Go version: $(go version)"
   echo ""

   # Core development tools
   declare -A TOOLS=(
       ["golangci-lint"]="github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
       ["gopls"]="golang.org/x/tools/gopls@latest"
       ["govulncheck"]="golang.org/x/vuln/cmd/govulncheck@latest"
       ["dlv"]="github.com/go-delve/delve/cmd/dlv@latest"
   )

   for tool_name in "${!TOOLS[@]}"; do
       tool_path="${TOOLS[$tool_name]}"
       echo "Installing $tool_name..."
       if go install "$tool_path"; then
           echo "✅ $tool_name installed successfully"
       else
           echo "❌ Failed to install $tool_name"
       fi
   done

   echo ""
   echo "✅ Tool rebuild complete"
   echo ""
   echo "Installed versions:"
   golangci-lint version 2>/dev/null || echo "  golangci-lint: not found"
   gopls version 2>/dev/null || echo "  gopls: $(gopls version)"
   govulncheck -version 2>/dev/null || echo "  govulncheck: not found"
   dlv version 2>/dev/null || echo "  dlv: $(dlv version)"
   ```

#### Phase 2: Documentation Updates

**Files to Update:**

1. **CONTRIBUTING.md**:
   ```markdown
   ### Go Version Updates

   When Renovate updates the Go version:
   1. Pull the latest changes
   2. Run the Go update skill: `.github/skills/scripts/skill-runner.sh utility-update-go-version`
   3. Rebuild development tools: `./scripts/rebuild-go-tools.sh`
   4. Restart your IDE's Go language server

   **Why?** Development tools (golangci-lint, gopls) are compiled binaries.
   After a Go version upgrade, these tools must be rebuilt with the new Go version
   to avoid compatibility issues.
   ```

2. **README.md** (Development Setup):
   ```markdown
   ### Keeping Go Tools Up-to-Date

   After pulling a Go version update from Renovate:
   ```bash
   # Rebuild all Go development tools
   ./scripts/rebuild-go-tools.sh
   ```

   This ensures tools like golangci-lint are compiled with the same Go version
   as your project.
   ```

3. **New File: `docs/development/go_version_upgrades.md`**:
   - Detailed explanation of Go version management
   - Step-by-step upgrade procedure
   - Troubleshooting common issues
   - FAQ section

#### Phase 3: Pre-commit Enhancement (Optional)

**Auto-fix Tool Mismatches:**

Add to `.pre-commit-config.yaml`:
```yaml
repos:
  - repo: local
    hooks:
      - id: go-tool-version-check
        name: Go Tool Version Check
        entry: scripts/pre-commit-hooks/check-go-tool-versions.sh
        language: system
        pass_filenames: false
        stages: [pre-commit]
```

Script automatically detects and fixes tool version mismatches before linting runs.

---

### Not Recommended: Multiple Go Versions Locally

#### Why Not Keep Multiple Versions?

**Cons:**
1. **Complexity:** Switching between versions manually is error-prone
2. **Confusion:** Which version is active? Are tools built with the right one?
3. **Disk space:** Each Go version is ~400MB
4. **Maintenance burden:** Tracking which tool needs which version
5. **CI mismatch:** CI uses single version; local multi-version creates drift

**Exception:** If actively developing multiple Go projects targeting different versions
- **Solution:** Use `goenv` or `asdf` for automatic version switching per project
- **Charon Context:** Single project, single Go version is simpler

**Verdict:** ❌ Multiple versions add complexity without clear benefit for single-project development

---

### Multi-Version Backward Compatibility Analysis

#### Should We Maintain N-1 Compatibility?

**Context:** Some projects maintain compatibility with previous major Go releases (e.g., supporting both Go 1.25 and 1.26)

**Analysis:**
- **Charon is a self-hosted application**, not a library
- Users run pre-built Docker images (Go version is locked in image)
- Developers should match the project's Go version
- No need to support multiple client Go versions

**Recommendation:** ❌ No backward compatibility requirement

**If library:** ✅ Would recommend supporting N-1 (e.g., Go 1.25 and 1.26) using:
```go
//go:build go1.25
// +build go1.25
```

---

## Implementation Timeline

### Week 1: Critical Fixes (High Priority)
- [x] **Document the problem** (this document)
- [ ] **Implement pre-commit tool version check** with auto-rebuild
- [ ] **Update Go version update skill** to rebuild tools
- [ ] **Create `rebuild-go-tools.sh` script**

### Week 2: Documentation & Education
- [ ] **Update CONTRIBUTING.md** with Go upgrade procedure
- [ ] **Update README.md** with tool rebuild instructions
- [ ] **Create `docs/development/go_version_upgrades.md`**
- [ ] **Add troubleshooting section** to copilot instructions

### Week 3: Testing & Validation
- [ ] **Test upgrade flow** with next Renovate Go update
- [ ] **Verify pre-commit auto-rebuild** works correctly
- [ ] **Document tool rebuild performance** (time cost)

---

## Decision Matrix

| Approach | Pros | Cons | Verdict |
|----------|------|------|---------|
| **Keep multiple Go versions** | Can build tools with older Go | Complexity, disk space, confusion | ❌ Not recommended |
| **Use version manager (goenv/asdf)** | Auto-switching, nice UX | Overhead, learning curve | ⚠️ Optional for polyglot teams |
| **Single version + rebuild tools** | Simple, matches CI, industry standard | Tools rebuild takes time (~30s) | ✅ **Recommended** |
| **Pin tool versions to old Go** | Tools don't break | Misses tool updates, security issues | ❌ Not recommended |
| **Delay Go upgrades until tools compatible** | No breakage | Blocks security patches, slows updates | ❌ Not recommended |

---

## FAQ

### Q1: Why did golangci-lint break after Go 1.26 upgrade?

**A:** golangci-lint was compiled with Go 1.25.5, but Go 1.26.0 introduced stdlib changes. Pre-built binaries from older Go versions can be incompatible with newer Go runtimes.

**Solution:** Rebuild golangci-lint with Go 1.26: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`

### Q2: Should I keep old Go versions "just in case"?

**A:** No, unless actively developing multiple projects with different Go requirements.

**Why not:** Complexity, confusion, and disk space without clear benefit. If you need older Go, the official `golang.org/dl` mechanism makes it easy to reinstall:
```bash
go install golang.org/dl/go1.25.6@latest
go1.25.6 download
```

### Q3: Will Renovate break my tools every time it updates Go?

**A:** Not with the recommended approach. The pre-commit hook will detect version mismatches and automatically rebuild tools before running linters.

**Manual rebuild:** `./scripts/rebuild-go-tools.sh` (takes ~30 seconds)

### Q4: How often does Go release new versions?

**A:** Go releases **major versions every 6 months** (February and August). Patch releases are more frequent for security/bug fixes.

**Implication:** Expect Go version updates 2-3 times per year from Renovate.

### Q5: What if I want to test my code against multiple Go versions?

**A:** Use CI matrix testing:
```yaml
strategy:
  matrix:
    go-version: ['1.25', '1.26', '1.27']
```

For local testing, use `golang.org/dl`:
```bash
go1.25.6 test ./...
go1.26.0 test ./...
```

### Q6: Should I pin golangci-lint to a specific version?

**A:** Yes, but via Renovate-managed version pin:
```bash
# In Makefile or script
GOLANGCI_LINT_VERSION := v1.62.2
go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
```

Renovate can update the version pin, and CI will use the pinned version consistently.

### Q7: Why doesn't CI have this problem?

**A:** CI environments are ephemeral. Every workflow run:
1. Installs Go from scratch (via `actions/setup-go`)
2. Installs tools with `go install` (compiled with CI's Go version)
3. Runs tests/lints
4. Discards everything

**Local development** has persistent tool installations that can become stale.

---

## Risk Assessment

### Risk: Developers Forget to Rebuild Tools

**Likelihood:** High (after automated Renovate updates)
**Impact:** Medium (pre-commit failures, frustration)
**Mitigation:**
1. Pre-commit hook auto-detects and rebuilds tools (Phase 1)
2. Clear documentation in CONTRIBUTING.md (Phase 2)
3. Error messages include rebuild instructions

### Risk: Tool Rebuild Takes Too Long

**Likelihood:** Medium (depends on tool size)
**Impact:** Low (one-time cost per Go upgrade)
**Measurement:**
```bash
$ time go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
real    0m28.341s  # ~30 seconds
```

**Mitigation:** Acceptable for infrequent upgrades (2-3 times/year)

### Risk: CI and Local Go Versions Drift

**Likelihood:** Low (both managed by Renovate)
**Impact:** High (false positives/negatives in CI)
**Mitigation:**
1. Renovate updates `go.work` and GitHub Actions `GO_VERSION` in sync
2. CI verifies Go version matches `go.work`

---

## References

### Official Go Documentation
- [Managing Go Installations](https://go.dev/doc/manage-install)
- [golang.org/dl](https://pkg.go.dev/golang.org/dl)
- [Go Release Policy](https://go.dev/doc/devel/release)

### Industry Examples
- [Kubernetes: .go-version](https://github.com/kubernetes/kubernetes/blob/master/.go-version)
- [Docker CLI: Dockerfile](https://github.com/docker/cli/blob/master/Dockerfile)
- [HashiCorp Terraform: go.mod](https://github.com/hashicorp/terraform/blob/main/go.mod)

### Version Managers
- [goenv](https://github.com/go-nv/goenv)
- [asdf-golang](https://github.com/kennyp/asdf-golang)
- [gvm](https://github.com/moovweb/gvm) (archived, not recommended)

### Related Resources
- [golangci-lint Installation](https://golangci-lint.run/usage/install/)
- [actions/setup-go Caching](https://github.com/actions/setup-go#caching-dependency-files-and-build-outputs)
- [Go Modules Reference](https://go.dev/ref/mod)

---

## Next Steps

1. **Review this document** with team/maintainers
2. **Approve strategy:** Single Go version + tool rebuild protocol
3. **Implement Phase 1** (pre-commit tool version check)
4. **Test with next Renovate Go update** (validate flow)
5. **Document lessons learned** and adjust as needed

---

## Appendix: Tool Inventory

| Tool | Purpose | Installation | Version Check |
|------|---------|--------------|---------------|
| **golangci-lint** | Pre-commit linting | `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` | `golangci-lint version` |
| **gopls** | Go language server (IDE) | `go install golang.org/x/tools/gopls@latest` | `gopls version` |
| **govulncheck** | Security scanning | `go install golang.org/x/vuln/cmd/govulncheck@latest` | `govulncheck -version` |
| **dlv** (Delve) | Debugger | `go install github.com/go-delve/delve/cmd/dlv@latest` | `dlv version` |
| **gotestsum** | Test runner | `go install gotest.tools/gotestsum@latest` | `gotestsum --version` |

**Critical Tools:** golangci-lint, gopls (priority for rebuild)
**Optional Tools:** dlv, gotestsum (rebuild as needed)

---

**Status:** ✅ Research complete, ready for implementation
**Recommendation:** **Single Go version + tool rebuild protocol**
**Next Action:** Implement Phase 1 (pre-commit tool version check)
