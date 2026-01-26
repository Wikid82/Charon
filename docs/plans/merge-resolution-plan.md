# Merge Conflict Resolution Plan: `feature/beta-release` → `main`

**Plan ID**: MERGE-2026-001
**Status**: 🔄 PENDING
**Priority**: High
**Created**: 2026-01-25

---

## 🔴 Workflow Failure Analysis (Added 2026-01-25)

### Issue Identified: docker-build.yml Failure

**Workflow Run**: https://github.com/Wikid82/Charon/actions/runs/21326638353

**Root Cause**: Base image mismatch after Debian Trixie migration (PR #550)

| Component | Before Fix | After Fix |
|-----------|-----------|-----------|
| Workflow `docker-build.yml` | `debian:bookworm-slim` | `debian:trixie-slim` |
| Dockerfile `CADDY_IMAGE` | `debian:trixie-slim` | `debian:trixie-slim` ✓ |

**Problem**: The workflow step "Resolve Debian base image digest" was still pulling `debian:bookworm-slim` while the Dockerfile was updated to use `debian:trixie-slim`. This caused inconsistency in the build.

**Fix Applied**: Updated [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml#L54-L57):
```yaml
- name: Resolve Debian base image digest
  run: |
    docker pull debian:trixie-slim
    DIGEST=$(docker inspect --format='{{index .RepoDigests 0}}' debian:trixie-slim)
```

---

## Summary

This plan addresses merge conflicts in the `feature/beta-release` branch that need resolution against `main`. After analyzing all conflicting files, here is the recommended resolution strategy.

---

## File Analysis

### 1. `.github/workflows/codeql.yml`

**Conflict Likelihood**: Low-Medium
**Current State**: No visible conflict markers

**Key Features in Current Version**:
- Go version: `1.25.6`
- Forked PR handling (skips when `fork == true`)
- CodeQL config file: `.github/codeql/codeql-config.yml`
- SARIF analysis with error/warning/note counting

**Resolution Strategy**: **Accept feature branch changes**
- Feature branch likely has updated Go version and security improvements
- Verify `GO_VERSION` env var matches other workflows after merge

---

### 2. `.github/workflows/docker-build.yml`

**Conflict Likelihood**: Medium
**Current State**: No visible conflict markers

**Key Features in Current Version**:
- SBOM generation and attestation
- CVE-2025-68156 verification for Caddy/CrowdSec
- Feature branch detection and artifact handling
- Multi-platform builds (amd64/arm64)
- Trivy vulnerability scanning

**Resolution Strategy**: **Accept feature branch changes**
- Feature branch contains critical security patches
- Verify image tag logic matches expected patterns
- Confirm `SYFT_VERSION` and `GRYPE_VERSION` are current

---

### 3. `Dockerfile`

**Conflict Likelihood**: High (likely PR #550 Debian Trixie migration)
**Current State**: Already using `debian:trixie-slim`

**Key Features in Current Version**:
- Base image: `debian:trixie-slim` (Debian 13 testing)
- Go version: `1.25` (builder stages)
- Caddy version: `2.11.0-beta.2`
- CrowdSec version: `1.7.6`
- gosu version: `1.17`
- Security patches for `expr-lang/expr@v1.17.7`
- Multi-stage build with cross-compilation helpers

**Resolution Strategy**: **Accept feature branch changes (post-Trixie migration)**
- If main still uses `bookworm-slim`, take feature branch version
- Critical: Preserve all CVE patches (CVE-2025-68156, CVE-2025-58183, etc.)
- Ensure all `renovate:` comments are preserved for automated updates

---

### 4. `backend/go.sum`

**Conflict Likelihood**: High
**Current State**: 167 packages, no conflict markers

**Key Versions Detected**:
- `golang.org/x/crypto@v0.47.0`
- `google.golang.org/grpc@v1.75.0`
- `gorm.io/gorm@v1.31.1`
- `github.com/gin-gonic/gin@v1.11.0`

**Resolution Strategy**: **Regenerate after merge**
- Dependency lock files should never be manually merged
- After resolving other conflicts, run:
  ```bash
  cd backend && go mod tidy && go mod download
  ```

---

### 5. `frontend/package-lock.json` ⚠️ (Not `backend/`)

**Conflict Likelihood**: High
**Current State**: 7499 lines, lockfileVersion 3

**Resolution Strategy**: **Regenerate after merge**
- Delete the file and regenerate:
  ```bash
  cd frontend && rm package-lock.json && npm install
  ```

---

### 6. `frontend/package.json` ⚠️ (Not `backend/`)

**Conflict Likelihood**: Medium
**Current State**: Version `0.3.0`, no conflict markers

**Key Dependencies**:
- React: `^19.2.3`
- Vite: `^7.3.1`
- Playwright: `^1.57.0`
- TypeScript: `^5.9.3`

**Resolution Strategy**: **Manual review required**
- Compare `main` and feature branch versions
- Keep higher version numbers when there are conflicts
- Ensure no duplicate entries

---

## Command Sequence for Resolution

```bash
# 1. Ensure you're on the feature branch
git checkout feature/beta-release

# 2. Fetch latest main
git fetch origin main

# 3. Start the merge (this will show conflicts)
git merge origin/main

# 4. For workflow files (if conflicts exist):
# Accept feature branch changes, then verify
git checkout --theirs .github/workflows/codeql.yml
git checkout --theirs .github/workflows/docker-build.yml
git add .github/workflows/

# 5. For Dockerfile (if conflicts exist):
# Accept feature branch (Trixie migration)
git checkout --theirs Dockerfile
git add Dockerfile

# 6. For Go dependencies:
git checkout --theirs backend/go.sum
cd backend && go mod tidy
cd ..
git add backend/go.sum backend/go.mod

# 7. For frontend dependencies:
cd frontend
rm -f package-lock.json
# Manually resolve package.json if needed
npm install
cd ..
git add frontend/package.json frontend/package-lock.json

# 8. Complete the merge
git commit -m "Merge main into feature/beta-release - resolve conflicts"

# 9. Validate
make lint
make test
```

---

## Post-Merge Validation Checklist

- [ ] `go mod tidy` completes without errors
- [ ] `npm install` (frontend) completes without errors
- [ ] Docker build succeeds: `docker build -t charon:test .`
- [ ] CI workflows pass on push
- [ ] Go version consistent across all workflows (`1.25.6`)
- [ ] Debian Trixie base image in Dockerfile

---

## Notes

1. **File Path Correction**: The conflicting package files are in `frontend/`, not `backend/`. The Go backend uses `go.mod`/`go.sum`, not npm.

2. **Conflict markers not visible**: The files read don't show `<<<<<<<` markers, suggesting either:
   - The merge hasn't been attempted yet
   - Conflicts would appear after running `git merge`

3. **PR #550 Reference**: The Dockerfile already shows Trixie migration is complete in the current branch.
