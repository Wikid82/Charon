# GoReleaser v2 Migration & Nightly Build Failure Remediation

**Status**: Active  
**Created**: 2026-01-30  
**Priority**: CRITICAL (Blocking Nightly Builds)

---

## Executive Summary

The nightly build workflow (`nightly-build.yml`) is failing with multiple issues:

1. **GoReleaser v2 Compatibility**: Config uses deprecated v1 syntax
2. **Zig Cross-Compilation**: Incorrect macOS target triple format
3. **🆕 CGO/SQLite Dependency**: Disabling CGO for darwin breaks SQLite (`mattn/go-sqlite3` requires CGO)

**Error Messages**:
```
only version: 2 configuration files are supported, yours is version: 1, please update your configuration
```

**Deprecation Warnings**:
- `snapshot.name_template` is deprecated
- `archives.format` is deprecated
- `archives.builds` is deprecated
- `nfpms.builds` is deprecated

**Build Error** (Zig):
```
error: unable to find or provide libc for target 'x86_64-macos.11.7.1...13.3-gnu'
info: zig can provide libc for related target x86_64-macos.11-none
```

---

## 🔴 Critical Dependency: SQLite CGO Issue

### Problem Statement

The current SQLite driver (`gorm.io/driver/sqlite`) depends on `mattn/go-sqlite3`, which is a CGO-based library. This means:

- **CGO_ENABLED=0** will cause build failures when SQLite is used
- **Cross-compilation** for darwin from Linux is blocked by CGO complexity
- The proposed fix of disabling CGO for darwin builds **will break the application**

### Solution: Migrate to Pure-Go SQLite

**Recommended Migration Path:**

| Current | New | Notes |
|---------|-----|-------|
| `gorm.io/driver/sqlite` | `github.com/glebarez/sqlite` | GORM-compatible pure-Go driver |
| `mattn/go-sqlite3` (indirect) | `modernc.org/sqlite` (indirect) | Pure-Go SQLite implementation |

**Benefits:**
1. ✅ No CGO required for any platform
2. ✅ Simplified cross-compilation (no Zig needed for SQLite)
3. ✅ Smaller binary size
4. ✅ Faster build times
5. ✅ Same GORM API - minimal code changes required

### Files Requiring SQLite Driver Changes

| File | Line | Change Required |
|------|------|-----------------|
| [backend/internal/database/database.go](../../backend/internal/database/database.go#L10) | 10 | `gorm.io/driver/sqlite` → `github.com/glebarez/sqlite` |
| [backend/internal/testutil/db_test.go](../../backend/internal/testutil/db_test.go#L6) | 6 | `gorm.io/driver/sqlite` → `github.com/glebarez/sqlite` |
| [backend/cmd/seed/main.go](../../backend/cmd/seed/main.go#L13) | 13 | `gorm.io/driver/sqlite` → `github.com/glebarez/sqlite` |
| [backend/go.mod](../../backend/go.mod#L19) | 19 | Replace `gorm.io/driver/sqlite` with `github.com/glebarez/sqlite` |

---

## Issue 1: GoReleaser v1 → v2 Migration (CRITICAL)

### Problem Statement

GoReleaser v2 (currently v2.13.3) no longer supports `version: 1` configuration files. The nightly workflow uses GoReleaser `~> v2` which requires v2 config syntax.

### Root Cause Analysis

Current `.goreleaser.yaml` uses deprecated v1 syntax:

```yaml
version: 1  # ❌ v2 requires "version: 2"
```

Multiple deprecated fields need updating:
| Deprecated Field | v2 Replacement |
|-----------------|----------------|
| `snapshot.name_template` | `snapshot.version_template` |
| `archives.format` | `archives.formats` (array) |
| `archives.builds` | `archives.ids` |
| `nfpms.builds` | `nfpms.ids` |

### GoReleaser Deprecation Reference

From [goreleaser.com/deprecations](https://goreleaser.com/deprecations):

1. **`snapshot.name_template`** → `snapshot.version_template`
   - Changed in v2.0.0
   - The template generates a version string, not a "name"

2. **`archives.format`** → `archives.formats`
   - Changed to array to support multiple formats per archive config
   - Must be `formats: [tar.gz]` not `format: tar.gz`

3. **`archives.builds`** → `archives.ids`
   - Renamed for clarity: it filters by build `id`, not "builds"

4. **`nfpms.builds`** → `nfpms.ids`
   - Same rationale as archives

### Required Changes

```diff
--- a/.goreleaser.yaml
+++ b/.goreleaser.yaml
@@ -1,4 +1,4 @@
-version: 1
+version: 2

 project_name: charon

@@ -62,10 +62,10 @@
       - -X github.com/Wikid82/charon/backend/internal/version.BuildTime={{.Date}}

 archives:
-  - format: tar.gz
+  - formats: [tar.gz]
     id: nix
-    builds:
+    ids:
       - linux
       - darwin
     name_template: >-
@@ -76,9 +76,9 @@
       - LICENSE
       - README.md

-  - format: zip
+  - formats: [zip]
     id: windows
-    builds:
+    ids:
       - windows
     name_template: >-
       {{ .ProjectName }}_
@@ -90,7 +90,7 @@

 nfpms:
   - id: packages
-    builds:
+    ids:
       - linux
     package_name: charon
     vendor: Charon
@@ -116,7 +116,7 @@
   name_template: 'checksums.txt'

 snapshot:
-  name_template: "{{ .Tag }}-next"
+  version_template: "{{ .Tag }}-next"

 changelog:
   sort: asc
```

---

## Issue 2: Zig Cross-Compilation for macOS

### Problem Statement

The nightly build fails during GoReleaser release step when cross-compiling for macOS (darwin) using Zig:

```text
error: unable to find or provide libc for target 'x86_64-macos.11.7.1...13.3-gnu'
info: zig can provide libc for related target x86_64-macos.11-none
```

### Root Cause Analysis

The `.goreleaser.yaml` darwin build uses **`-macos-none`** which is correct, but examining the actual file shows **`-macos-none`** is already in place. The error message suggests something is injecting version numbers.

**Wait** - Re-reading the current config, I see it actually says `-macos-none` already. Let me check if there's a different issue.

Actually, looking at the error more carefully:
```
target 'x86_64-macos.11.7.1...13.3-gnu'
```

This suggests the **Go runtime/cgo is detecting the macOS version range** and passing it to Zig incorrectly. The `-gnu` suffix shouldn't be there for macOS.

**Current Configuration**:
```yaml
CC=zig cc -target {{ if eq .Arch "amd64" }}x86_64{{ else }}aarch64{{ end }}-macos-none
```

The current config is correct (`-macos-none`), but CGO may be interfering.

### ~~Recommended Fix: Disable CGO for Darwin~~

> **⚠️ UPDATE:** This section is superseded by the SQLite driver migration (see "Critical Dependency: SQLite CGO Issue" above). Simply disabling CGO for darwin **breaks SQLite functionality**.

### ✅ Actual Fix: Migrate to Pure-Go SQLite

By migrating from `gorm.io/driver/sqlite` (CGO) to `github.com/glebarez/sqlite` (pure-Go):

1. **Zig is no longer required** for any platform
2. **CGO_ENABLED=0** can be used for ALL platforms (linux, darwin, windows)
3. **Cross-compilation is trivial** - standard Go cross-compilation works
4. **Build times are faster** - no C compiler invocation

This completely eliminates Issue 2 as a side effect of fixing the SQLite dependency issue.

---

## Complete Updated `.goreleaser.yaml`

> **Note:** After migrating to pure-Go SQLite (`github.com/glebarez/sqlite`), Zig cross-compilation is no longer required. All platforms now use `CGO_ENABLED=0` for simpler, faster builds.

```yaml
version: 2

project_name: charon

builds:
  - id: linux
    dir: backend
    main: ./cmd/api
    binary: charon
    env:
      - CGO_ENABLED=0
    goos:
      - linux
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X github.com/Wikid82/charon/backend/internal/version.Version={{.Version}}
      - -X github.com/Wikid82/charon/backend/internal/version.GitCommit={{.Commit}}
      - -X github.com/Wikid82/charon/backend/internal/version.BuildTime={{.Date}}

  - id: windows
    dir: backend
    main: ./cmd/api
    binary: charon
    env:
      - CGO_ENABLED=0
    goos:
      - windows
    goarch:
      - amd64
    ldflags:
      - -s -w
      - -X github.com/Wikid82/charon/backend/internal/version.Version={{.Version}}
      - -X github.com/Wikid82/charon/backend/internal/version.GitCommit={{.Commit}}
      - -X github.com/Wikid82/charon/backend/internal/version.BuildTime={{.Date}}

  - id: darwin
    dir: backend
    main: ./cmd/api
    binary: charon
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X github.com/Wikid82/charon/backend/internal/version.Version={{.Version}}
      - -X github.com/Wikid82/charon/backend/internal/version.GitCommit={{.Commit}}
      - -X github.com/Wikid82/charon/backend/internal/version.BuildTime={{.Date}}

archives:
  - formats: [tar.gz]
    id: nix
    ids:
      - linux
      - darwin
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}
    files:
      - LICENSE
      - README.md

  - formats: [zip]
    id: windows
    ids:
      - windows
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}
    files:
      - LICENSE
      - README.md

nfpms:
  - id: packages
    ids:
      - linux
    package_name: charon
    vendor: Charon
    homepage: https://github.com/Wikid82/charon
    maintainer: Wikid82
    description: "Charon - A powerful reverse proxy manager"
    license: MIT
    formats:
      - deb
      - rpm
    contents:
      - src: ./backend/data/
        dst: /var/lib/charon/data/
        type: dir
      - src: ./frontend/dist/
        dst: /usr/share/charon/frontend/
        type: dir
    dependencies:
      - libc6
      - ca-certificates

checksum:
  name_template: 'checksums.txt'

snapshot:
  version_template: "{{ .Tag }}-next"

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
```

---

## Implementation Plan

### Phase 0: SQLite Driver Migration (PREREQUISITE)

**Objective:** Migrate from CGO-dependent SQLite to pure-Go implementation.

**Files to Modify:**

| File | Change | Reason |
|------|--------|--------|
| `backend/go.mod` | Replace `gorm.io/driver/sqlite` with `github.com/glebarez/sqlite` | Pure-Go SQLite driver |
| `backend/internal/database/database.go` | Update import statement | New driver package |
| `backend/internal/testutil/db_test.go` | Update import statement | New driver package |
| `backend/cmd/seed/main.go` | Update import statement | New driver package |

**Steps:**

```bash
# 1. Update go.mod - replace CGO driver with pure-Go driver
cd backend
go get github.com/glebarez/sqlite
go mod edit -droprequire gorm.io/driver/sqlite

# 2. Update import statements in Go files
# (Manual step - update imports in 3 files listed above)

# 3. Tidy dependencies
go mod tidy

# 4. Verify build works without CGO
CGO_ENABLED=0 go build ./cmd/api
CGO_ENABLED=0 go build ./cmd/seed

# 5. Run tests to verify SQLite functionality
CGO_ENABLED=0 go test ./internal/database/... -v
CGO_ENABLED=0 go test ./internal/testutil/... -v
```

**Validation:**
- ✅ `CGO_ENABLED=0 go build ./backend/cmd/api` succeeds
- ✅ `CGO_ENABLED=0 go build ./backend/cmd/seed` succeeds  
- ✅ All database tests pass with CGO disabled
- ✅ `go.mod` no longer references `gorm.io/driver/sqlite` or `mattn/go-sqlite3`

---

### Phase 1: Update GoReleaser Config

**Files to Modify:**

| File | Change | Reason |
|------|--------|--------|
| `.goreleaser.yaml` | Update to version 2 syntax | Required for GoReleaser ~> v2 |
| `.goreleaser.yaml` | Remove Zig cross-compilation | No longer needed with pure-Go SQLite |
| `.goreleaser.yaml` | Set `CGO_ENABLED=0` for ALL platforms | Consistent pure-Go builds |

**Simplified Build Configuration (No Zig Required):**

```yaml
builds:
  - id: linux
    dir: backend
    main: ./cmd/api
    binary: charon
    env:
      - CGO_ENABLED=0
    goos:
      - linux
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X github.com/Wikid82/charon/backend/internal/version.Version={{.Version}}
      - -X github.com/Wikid82/charon/backend/internal/version.GitCommit={{.Commit}}
      - -X github.com/Wikid82/charon/backend/internal/version.BuildTime={{.Date}}

  - id: windows
    dir: backend
    main: ./cmd/api
    binary: charon
    env:
      - CGO_ENABLED=0
    goos:
      - windows
    goarch:
      - amd64
    ldflags:
      - -s -w
      - -X github.com/Wikid82/charon/backend/internal/version.Version={{.Version}}
      - -X github.com/Wikid82/charon/backend/internal/version.GitCommit={{.Commit}}
      - -X github.com/Wikid82/charon/backend/internal/version.BuildTime={{.Date}}

  - id: darwin
    dir: backend
    main: ./cmd/api
    binary: charon
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X github.com/Wikid82/charon/backend/internal/version.Version={{.Version}}
      - -X github.com/Wikid82/charon/backend/internal/version.GitCommit={{.Commit}}
      - -X github.com/Wikid82/charon/backend/internal/version.BuildTime={{.Date}}
```

---

### Phase 2: Verification Steps

```bash
# 1. Verify SQLite migration (Phase 0 complete)
cd backend
CGO_ENABLED=0 go build ./cmd/api
CGO_ENABLED=0 go test ./... -count=1

# 2. Validate the GoReleaser config locally
goreleaser check

# 3. Test snapshot build locally (no Zig required!)
goreleaser release --snapshot --skip=publish --clean

# 4. Trigger nightly workflow manually
gh workflow run nightly-build.yml -f reason="Test GoReleaser v2 migration with pure-Go SQLite"

# 5. Monitor workflow execution
gh run watch
```

---

### Phase 3: Rollback Plan

If the fix fails:

**SQLite Rollback:**
1. Revert `go.mod` to use `gorm.io/driver/sqlite`
2. Revert import statement changes
3. Re-enable CGO in GoReleaser config

**GoReleaser Rollback:**
1. Revert `.goreleaser.yaml` changes
2. Pin GoReleaser to v1.x in workflows:
   ```yaml
   version: '1.26.2'  # Last v1 release
   ```

---

## Requirements (EARS Notation)

1. WHEN building for any platform, THE SYSTEM SHALL use `CGO_ENABLED=0` (pure-Go builds).
2. WHEN importing the SQLite driver, THE SYSTEM SHALL use `github.com/glebarez/sqlite` (pure-Go driver).
3. WHEN GoReleaser executes, THE SYSTEM SHALL use version 2 configuration syntax.
4. WHEN archiving builds, THE SYSTEM SHALL use `formats` (array) instead of deprecated `format`.
5. WHEN referencing build IDs in archives/nfpms, THE SYSTEM SHALL use `ids` instead of deprecated `builds`.
6. WHEN generating snapshot versions, THE SYSTEM SHALL use `version_template` instead of deprecated `name_template`.

---

## Acceptance Criteria

**Phase 0 (SQLite Migration):**
- [ ] `backend/go.mod` uses `github.com/glebarez/sqlite` instead of `gorm.io/driver/sqlite`
- [ ] No references to `mattn/go-sqlite3` in `go.mod` or `go.sum`
- [ ] `CGO_ENABLED=0 go build ./backend/cmd/api` succeeds
- [ ] `CGO_ENABLED=0 go build ./backend/cmd/seed` succeeds
- [ ] All database tests pass with `CGO_ENABLED=0`

**Phase 1 (GoReleaser v2):**
- [ ] `goreleaser check` passes without errors or deprecation warnings
- [ ] Nightly build workflow completes successfully
- [ ] Linux amd64/arm64 binaries are produced
- [ ] Windows amd64 binary is produced
- [ ] Darwin amd64/arm64 binaries are produced
- [ ] .deb and .rpm packages are produced for Linux
- [ ] No deprecation warnings in CI logs
- [ ] No Zig-related errors in build logs

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Pure-Go SQLite has different behavior | Low | Medium | Run full test suite; compare query results |
| Pure-Go SQLite performance differs | Low | Low | Run benchmarks; acceptable for typical workloads |
| Other undocumented v2 breaking changes | Low | Medium | Monitor GoReleaser changelog; test locally first |
| Import statement missed in some file | Low | High | Use grep to find all `gorm.io/driver/sqlite` imports |

---

## References

- [glebarez/sqlite - Pure Go SQLite driver for GORM](https://github.com/glebarez/sqlite)
- [modernc.org/sqlite - Pure Go SQLite implementation](https://pkg.go.dev/modernc.org/sqlite)
- [GoReleaser v2 Migration Guide](https://goreleaser.com/deprecations/)
- [GoReleaser Builds Documentation](https://goreleaser.com/customization/build/)

---

# ARCHIVED: Other CI Issues (Separate from GoReleaser)

The following issues are documented separately and may be addressed in future PRs:

1. **Playwright E2E - Emergency Server Connectivity** - See [docs/plans/e2e_remediation_spec.md](e2e_remediation_spec.md)
2. **Trivy Scan - Image Reference Validation** - See [docs/plans/docker_compose_ci_fix.md](docker_compose_ci_fix.md)
