# Repository Structure Reorganization Plan

**Date**: December 21, 2025 (Revised)
**Status**: Proposed
**Risk Level**: Medium (requires CI/CD updates, Docker path changes)

---

## Executive Summary

The repository root level currently contains **81 items**, making it difficult to navigate and maintain. This plan proposes moving files into logical directories to achieve a cleaner, more organized structure with only **~15 essential items** at the root level.

**Key Benefits**:

- Easier navigation for contributors
- Clearer separation of concerns
- Reduced cognitive load when browsing repository
- Better .gitignore and .dockerignore maintenance
- Improved CI/CD workflow clarity

---

## Current Root-Level Analysis

### Category Breakdown

| Category | Count | Examples | Status |
|----------|-------|----------|--------|
| **Docker Compose Files** | 5 | `docker-compose.yml`, `docker-compose.dev.yml`, etc. | 🔴 Scattered |
| **CodeQL SARIF Files** | 6 | `codeql-go.sarif`, `codeql-results-*.sarif` | 🔴 Build artifacts at root |
| **Implementation Docs** | 16 | `BULK_ACL_FEATURE.md`, `IMPLEMENTATION_SUMMARY.md`, etc. | 🔴 Should be in docs/ |
| **Config Files** | 8 | `eslint.config.js`, `.pre-commit-config.yaml`, `Makefile`, etc. | 🟡 Mixed - some stay, some move |
| **Docker Files** | 3 | `Dockerfile`, `docker-entrypoint.sh`, `DOCKER.md` | 🟡 Could group |
| **Core Docs** | 4 | `README.md`, `CONTRIBUTING.md`, `LICENSE`, `VERSION.md` | 🟢 Stay at root |
| **Hidden Config** | 15+ | `.github/`, `.vscode/`, `.gitignore`, `.dockerignore`, etc. | 🟢 Stay at root |
| **Source Directories** | 7 | `backend/`, `frontend/`, `docs/`, `scripts/`, etc. | 🟢 Stay at root |
| **Workspace File** | 1 | `Chiron.code-workspace` | 🟢 Stay at root |
| **Build Artifacts** | 3 | `codeql-db/`, `codeql-agent-results/`, `.trivy_logs/` | 🔴 Gitignored but present |

**Total Root Items**: ~60 items (files + directories)

### Problem Areas

1. **Docker Compose Sprawl**: 5 files at root when they should be grouped
2. **SARIF Pollution**: 6 CodeQL SARIF files are build artifacts (should be .gitignored)
3. **Documentation Chaos**: 16 implementation/feature docs scattered at root instead of `docs/`
4. **Mixed Purposes**: Docker files, configs, docs, code all at same level

---

## Proposed New Structure

### Root Level (Clean)

```
/projects/Charon/
├── .github/                    # GitHub workflows, templates, agents
├── .vscode/                    # VS Code workspace settings
├── backend/                    # Go backend source
├── configs/                    # Runtime configs (CrowdSec, etc.)
├── data/                       # Runtime data (gitignored)
├── docs/                       # Documentation (enhanced)
├── frontend/                   # React frontend source
├── logs/                       # Runtime logs (gitignored)
├── scripts/                    # Build/test/integration scripts
├── test-results/               # Test outputs (gitignored)
├── tools/                      # Development tools
│
├── .codecov.yml                # Codecov configuration
├── .dockerignore               # Docker build exclusions
├── .gitattributes              # Git attributes
├── .gitignore                  # Git exclusions
├── .goreleaser.yaml            # GoReleaser config
├── .markdownlint.json          # Markdown lint config
├── .markdownlintrc             # Markdown lint config
├── .pre-commit-config.yaml     # Pre-commit hooks
├── CHANGELOG.md                # Project changelog
├── Chiron.code-workspace       # VS Code workspace
├── CONTRIBUTING.md             # Contribution guidelines
├── CONTRIBUTING_TRANSLATIONS.md # Translation guidelines
├── LICENSE                     # License file
├── Makefile                    # Build automation
├── README.md                   # Project readme
├── VERSION.md                  # Version documentation
├── eslint.config.js            # ESLint config
├── go.work                     # Go workspace
├── go.work.sum                 # Go workspace checksums
└── package.json                # Root package.json (pre-commit, etc.)
```

### New Directory: `.docker/`

**Purpose**: Consolidate all Docker-related files except the primary Dockerfile

```
.docker/
├── compose/
│   ├── docker-compose.yml         # Main compose (moved from root)
│   ├── docker-compose.dev.yml     # Dev override (moved from root)
│   ├── docker-compose.local.yml   # Local override (moved from root)
│   ├── docker-compose.remote.yml  # Remote override (moved from root)
│   ├── docker-compose.override.yml   # Remote override (moved from root)
│   └── README.md                  # Compose file documentation
├── docker-entrypoint.sh           # Entrypoint script (moved from root)
└── README.md                      # Docker documentation (DOCKER.md renamed)
```

**Why `.docker/` with a dot?**

- Keeps it close to root-level Dockerfile (co-location)
- Hidden by default in file browsers (reduces clutter)
- Common pattern in monorepos (`.github/`, `.vscode/`)

**Alternative**: Could use `docker/` without dot, but `.docker/` is preferred for consistency

### Enhanced: `docs/`

**New subdirectory**: `docs/implementation/`

**Purpose**: Archive completed implementation documents that shouldn't be at root

```
docs/
├── implementation/                 # NEW: Implementation documents
│   ├── BULK_ACL_FEATURE.md        # Moved from root
│   ├── IMPLEMENTATION_SUMMARY.md  # Moved from root
│   ├── ISSUE_16_ACL_IMPLEMENTATION.md  # Moved from root
│   ├── QA_AUDIT_REPORT_LOADING_OVERLAYS.md  # Moved from root
│   ├── QA_MIGRATION_COMPLETE.md   # Moved from root
│   ├── SECURITY_CONFIG_PRIORITY.md  # Moved from root
│   ├── SECURITY_IMPLEMENTATION_PLAN.md  # Moved from root
│   ├── WEBSOCKET_FIX_SUMMARY.md   # Moved from root
│   └── README.md                  # Index of implementation docs
├── issues/                         # Existing: Issue templates
├── plans/                          # Existing: Planning documents
│   ├── structure.md               # THIS FILE
│   └── ...
├── reports/                        # Existing: Reports
├── troubleshooting/                # Existing: Troubleshooting guides
├── acme-staging.md
├── api.md
├── ...
└── index.md
```

### Enhanced: `.gitignore`

**New entries** to prevent SARIF files at root:

> **Note**: The `*.sarif` pattern may already exist in `.gitignore`. Verify before adding to avoid duplication. The explicit patterns below ensure comprehensive coverage.

```gitignore
# Add to "CodeQL & Security Scanning" section:
# -----------------------------------------------------------------------------
# CodeQL & Security Scanning
# -----------------------------------------------------------------------------
# ... existing entries ...

# Prevent SARIF files at root level
/*.sarif
/codeql-*.sarif

# Explicit gitignore for scattered SARIF files
/codeql-go.sarif
/codeql-js.sarif
/codeql-results-go.sarif
/codeql-results-go-backend.sarif
/codeql-results-go-new.sarif
/codeql-results-js.sarif

# Test artifacts at root
/block*.txt
/final_block_test.txt

# Debug/temp config files at root
/caddy_*.json
!package*.json

# Trivy scan outputs at root
/trivy-*.txt
```

#### Local Override Migration

**Important**: With the move to `.docker/compose/`, the standard `docker-compose.override.yml` behavior changes:

- **Previous behavior**: `docker-compose.override.yml` at repository root was auto-applied by Docker Compose
- **New behavior**: Override files at `.docker/compose/docker-compose.override.yml` must be explicitly referenced with `-f` flag

**Updated .gitignore entry**:

```gitignore
# Local docker-compose override (new location)
.docker/compose/docker-compose.override.yml
```

**Usage with new location**:

```bash
# Development with override
docker compose -f .docker/compose/docker-compose.yml -f .docker/compose/docker-compose.override.yml up -d
```

**Note**: Users with existing `docker-compose.override.yml` at root should move it to `.docker/compose/` and update their workflow scripts accordingly.

---

## File Migration Table

### Docker Compose Files → `.docker/compose/`

| Current Path | New Path | Type |
|-------------|----------|------|
| `/docker-compose.yml` | `/.docker/compose/docker-compose.yml` | Move |
| `/docker-compose.dev.yml` | `/.docker/compose/docker-compose.dev.yml` | Move |
| `/docker-compose.local.yml` | `/.docker/compose/docker-compose.local.yml` | Move |
| `/docker-compose.remote.yml` | `/.docker/compose/docker-compose.remote.yml` | Move |
| `/docker-compose.override.yml` | `/.docker/compose/docker-compose.override.yml` | Move (if exists) |

**Note**: `docker-compose.override.yml` is gitignored. Include in .gitignore update.

### Docker Support Files → `.docker/`

| Current Path | New Path | Type |
|-------------|----------|------|
| `/docker-entrypoint.sh` | `/.docker/docker-entrypoint.sh` | Move |
| `/DOCKER.md` | `/.docker/README.md` | Move + Rename |

### Implementation Docs → `docs/implementation/`

| Current Path | New Path | Type |
|-------------|----------|------|
| `/BULK_ACL_FEATURE.md` | `/docs/implementation/BULK_ACL_FEATURE.md` | Move |
| `/IMPLEMENTATION_SUMMARY.md` | `/docs/implementation/IMPLEMENTATION_SUMMARY.md` | Move |
| `/ISSUE_16_ACL_IMPLEMENTATION.md` | `/docs/implementation/ISSUE_16_ACL_IMPLEMENTATION.md` | Move |
| `/QA_AUDIT_REPORT_LOADING_OVERLAYS.md` | `/docs/implementation/QA_AUDIT_REPORT_LOADING_OVERLAYS.md` | Move |
| `/QA_MIGRATION_COMPLETE.md` | `/docs/implementation/QA_MIGRATION_COMPLETE.md` | Move |
| `/SECURITY_CONFIG_PRIORITY.md` | `/docs/implementation/SECURITY_CONFIG_PRIORITY.md` | Move |
| `/SECURITY_IMPLEMENTATION_PLAN.md` | `/docs/implementation/SECURITY_IMPLEMENTATION_PLAN.md` | Move |
| `/WEBSOCKET_FIX_SUMMARY.md` | `/docs/implementation/WEBSOCKET_FIX_SUMMARY.md` | Move |
| `/AGENT_SKILLS_MIGRATION_SUMMARY.md` | `/docs/implementation/AGENT_SKILLS_MIGRATION_SUMMARY.md` | Move |
| `/I18N_IMPLEMENTATION_SUMMARY.md` | `/docs/implementation/I18N_IMPLEMENTATION_SUMMARY.md` | Move |
| `/INVESTIGATION_SUMMARY.md` | `/docs/implementation/INVESTIGATION_SUMMARY.md` | Move |
| `/PHASE_0_COMPLETE.md` | `/docs/implementation/PHASE_0_COMPLETE.md` | Move |
| `/PHASE_3_COMPLETE.md` | `/docs/implementation/PHASE_3_COMPLETE.md` | Move |
| `/PHASE_4_COMPLETE.md` | `/docs/implementation/PHASE_4_COMPLETE.md` | Move |
| `/PHASE_5_COMPLETE.md` | `/docs/implementation/PHASE_5_COMPLETE.md` | Move |
| `/QA_PHASE5_VERIFICATION_REPORT.md` | `/docs/implementation/QA_PHASE5_VERIFICATION_REPORT.md` | Move |
| `/SECURITY_HEADERS_IMPLEMENTATION_SUMMARY.md` | `/docs/implementation/SECURITY_HEADERS_IMPLEMENTATION_SUMMARY.md` | Move |

### CodeQL SARIF Files → Delete (Add to .gitignore)

| Current Path | Action | Reason |
|-------------|--------|--------|
| `/codeql-go.sarif` | Delete + gitignore | Build artifact |
| `/codeql-js.sarif` | Delete + gitignore | Build artifact |
| `/codeql-results-go.sarif` | Delete + gitignore | Build artifact |
| `/codeql-results-go-backend.sarif` | Delete + gitignore | Build artifact |
| `/codeql-results-go-new.sarif` | Delete + gitignore | Build artifact |
| `/codeql-results-js.sarif` | Delete + gitignore | Build artifact |

**Note**: These are generated by CodeQL and should never be committed.

### Test/Debug Files → Delete + Gitignore

| Current Path | Action | Reason |
|-------------|--------|--------|
| `/block_test.txt` | Delete + gitignore | Test artifact |
| `/blocking_test.txt` | Delete + gitignore | Test artifact |
| `/final_block_test.txt` | Delete + gitignore | Test artifact |
| `/caddy_config_qa.json` | Delete + gitignore | Debug config |
| `/caddy_crowdsec_config.json` | Delete + gitignore | Debug config |
| `/trivy-image-scan.txt` | Delete + gitignore | Scan output |
| `/trivy-scan-output.txt` | Delete + gitignore | Scan output |

**Note**: These are test/debug artifacts that should never be committed.

### Files Staying at Root

| File | Reason |
|------|--------|
| `Dockerfile` | Primary Docker build file - standard location |
| `Makefile` | Build automation - standard location |
| `README.md` | Project entry point - standard location |
| `CONTRIBUTING.md` | Contributor guidelines - standard location |
| `CONTRIBUTING_TRANSLATIONS.md` | Translation contribution guidelines - standard location |
| `LICENSE` | License file - standard location |
| `VERSION.md` | Version documentation - standard location |
| `CHANGELOG.md` | Project changelog - standard location |
| `Chiron.code-workspace` | VS Code workspace - standard location |
| `go.work` | Go workspace - required at root |
| `go.work.sum` | Go workspace checksums - required at root |
| `package.json` | Root package (pre-commit, etc.) - required at root |
| `eslint.config.js` | ESLint config - required at root |
| `.codecov.yml` | Codecov config - required at root |
| `.goreleaser.yaml` | GoReleaser config - required at root |
| `.markdownlint.json` | Markdown lint config - required at root |
| `.pre-commit-config.yaml` | Pre-commit config - required at root |
| All `.git*` files | Git configuration - required at root |
| All hidden directories | Standard locations |

---

## Impact Analysis

### Files Requiring Updates

#### 1. GitHub Workflows (`.github/workflows/*.yml`)

**Files to Update**: 25+ workflow files

**Changes Needed**:

```yaml
# OLD (scattered references):
- 'Dockerfile'
- 'docker-compose.yml'
- 'docker-entrypoint.sh'
- 'DOCKER.md'

# NEW (centralized references):
- 'Dockerfile'                          # Stays at root
- '.docker/compose/docker-compose.yml'
- '.docker/compose/docker-compose.*.yml'
- '.docker/docker-entrypoint.sh'
- '.docker/README.md'
```

**Specific Files**:

- `.github/workflows/docker-lint.yml` - References Dockerfile (no change needed)
- `.github/workflows/docker-build.yml` - May reference docker-compose
- `.github/workflows/docker-publish.yml` - May reference docker-compose
- `.github/workflows/waf-integration.yml` - References Dockerfile (no change needed)

**Search Pattern**: `grep -r "docker-compose" .github/workflows/`

#### 2. Scripts (`scripts/*.sh`)

**Files to Update**: ~5 scripts

**Changes Needed**:

```bash
# OLD:
docker-compose -f docker-compose.local.yml up -d
docker compose -f docker-compose.yml -f docker-compose.dev.yml up

# NEW:
docker-compose -f .docker/compose/docker-compose.local.yml up -d
docker compose -f .docker/compose/docker-compose.yml -f .docker/compose/docker-compose.dev.yml up
```

**Specific Files**:

- `scripts/coraza_integration.sh` - Uses docker-compose.local.yml
- `scripts/crowdsec_integration.sh` - Uses docker-compose files
- `scripts/crowdsec_startup_test.sh` - Uses docker-compose files
- `scripts/integration-test.sh` - Uses docker-compose files

**Search Pattern**: `grep -r "docker-compose" scripts/`

#### 3. VS Code Tasks (`.vscode/tasks.json`)

**Changes Needed**:

```json
// OLD:
"docker compose -f docker-compose.override.yml up -d"
"docker compose -f docker-compose.local.yml up -d"

// NEW:
"docker compose -f .docker/compose/docker-compose.override.yml up -d"
"docker compose -f .docker/compose/docker-compose.local.yml up -d"
```

**Affected Tasks**:

- "Build & Run: Local Docker Image"
- "Build & Run: Local Docker Image No-Cache"
- "Docker: Start Dev Environment"
- "Docker: Stop Dev Environment"
- "Docker: Start Local Environment"
- "Docker: Stop Local Environment"

#### 4. Makefile

**Changes Needed**:

```makefile
# OLD:
docker-compose build
docker-compose up -d
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up
docker-compose down
docker-compose logs -f

# NEW:
docker-compose -f .docker/compose/docker-compose.yml build
docker-compose -f .docker/compose/docker-compose.yml up -d
docker-compose -f .docker/compose/docker-compose.yml -f .docker/compose/docker-compose.dev.yml up
docker-compose -f .docker/compose/docker-compose.yml down
docker-compose -f .docker/compose/docker-compose.yml logs -f
```

#### 5. Dockerfile

**Changes Needed**:

```dockerfile
# OLD:
COPY docker-entrypoint.sh /usr/local/bin/

# NEW:
COPY .docker/docker-entrypoint.sh /usr/local/bin/
```

**Line**: Search for `docker-entrypoint.sh` in Dockerfile

#### 6. Documentation Files

**Files to Update**:

- `README.md` - May reference docker-compose files or DOCKER.md
- `CONTRIBUTING.md` - May reference docker-compose files
- `docs/getting-started.md` - Likely references docker-compose
- `docs/debugging-local-container.md` - Likely references docker-compose
- Any docs referencing implementation files moved to `docs/implementation/`

**Search Pattern**:

- `grep -r "docker-compose" docs/`
- `grep -r "DOCKER.md" docs/`
- `grep -r "BULK_ACL_FEATURE\|IMPLEMENTATION_SUMMARY" docs/`

#### 7. .dockerignore

**Changes Needed**:

```dockerignore
# Add to "Documentation" section:
docs/implementation/

# Update Docker Compose exclusion:
.docker/
```

#### 8. .gitignore

**Changes Needed**:

```gitignore
# Add explicit SARIF exclusions at root:
/*.sarif
/codeql-*.sarif

# Update docker-compose.override.yml path:
.docker/compose/docker-compose.override.yml
```

---

## Migration Steps

### Phase 1: Preparation (No Breaking Changes)

1. **Create new directories**:

   ```bash
   mkdir -p .docker/compose
   mkdir -p docs/implementation
   ```

2. **Create README files**:
   - `.docker/README.md` (content from DOCKER.md + compose guide)
   - `.docker/compose/README.md` (compose file documentation)
   - `docs/implementation/README.md` (index of implementation docs)

3. **Update .gitignore** (add SARIF exclusions):

   ```bash
   # Add to .gitignore:
   /*.sarif
   /codeql-*.sarif
   .docker/compose/docker-compose.override.yml
   ```

4. **Commit preparation**:

   ```bash
   git add .docker/ docs/implementation/ .gitignore
   git commit -m "chore: prepare directory structure for reorganization"
   ```

### Phase 2: Move Files (Breaking Changes)

**⚠️ WARNING**: This phase will break existing workflows until all references are updated.

1. **Move Docker Compose files**:

   ```bash
   git mv docker-compose.yml .docker/compose/
   git mv docker-compose.dev.yml .docker/compose/
   git mv docker-compose.local.yml .docker/compose/
   git mv docker-compose.remote.yml .docker/compose/
   # docker-compose.override.yml is gitignored, no need to move
   ```

2. **Move Docker support files**:

   ```bash
   git mv docker-entrypoint.sh .docker/
   git mv DOCKER.md .docker/README.md
   ```

3. **Move implementation docs**:

   ```bash
   git mv BULK_ACL_FEATURE.md docs/implementation/
   git mv IMPLEMENTATION_SUMMARY.md docs/implementation/
   git mv ISSUE_16_ACL_IMPLEMENTATION.md docs/implementation/
   git mv QA_AUDIT_REPORT_LOADING_OVERLAYS.md docs/implementation/
   git mv QA_MIGRATION_COMPLETE.md docs/implementation/
   git mv SECURITY_CONFIG_PRIORITY.md docs/implementation/
   git mv SECURITY_IMPLEMENTATION_PLAN.md docs/implementation/
   git mv WEBSOCKET_FIX_SUMMARY.md docs/implementation/
   git mv AGENT_SKILLS_MIGRATION_SUMMARY.md docs/implementation/
   git mv I18N_IMPLEMENTATION_SUMMARY.md docs/implementation/
   git mv INVESTIGATION_SUMMARY.md docs/implementation/
   git mv PHASE_0_COMPLETE.md docs/implementation/
   git mv PHASE_3_COMPLETE.md docs/implementation/
   git mv PHASE_4_COMPLETE.md docs/implementation/
   git mv PHASE_5_COMPLETE.md docs/implementation/
   git mv QA_PHASE5_VERIFICATION_REPORT.md docs/implementation/
   git mv SECURITY_HEADERS_IMPLEMENTATION_SUMMARY.md docs/implementation/
   ```

4. **Delete test/debug files**:

   ```bash
   git rm block_test.txt
   git rm blocking_test.txt
   git rm final_block_test.txt
   git rm caddy_config_qa.json
   git rm caddy_crowdsec_config.json
   git rm trivy-image-scan.txt
   git rm trivy-scan-output.txt
   ```

5. **Delete SARIF files**:

   ```bash
   git rm codeql-go.sarif
   git rm codeql-js.sarif
   git rm codeql-results-go.sarif
   git rm codeql-results-go-backend.sarif
   git rm codeql-results-go-new.sarif
   git rm codeql-results-js.sarif
   ```

6. **Commit file moves**:

   ```bash
   git commit -m "chore: reorganize repository structure

   - Move docker-compose files to .docker/compose/
   - Move docker-entrypoint.sh to .docker/
   - Move DOCKER.md to .docker/README.md
   - Move implementation docs to docs/implementation/
   - Delete committed SARIF files (should be gitignored)
   - Delete test/debug artifacts (should be gitignored)
   "
   ```

### Phase 3: Update References (Fix Breaking Changes)

**Order matters**: Update in this sequence to minimize build failures.

1. **Update Dockerfile**:
   - Change `docker-entrypoint.sh` → `.docker/docker-entrypoint.sh`
   - Test: `docker build -t charon:test .`

2. **Update Makefile**:
   - Change all `docker-compose` commands to use `.docker/compose/docker-compose.yml`
   - Test: `make docker-build`, `make docker-up`

3. **Update .vscode/tasks.json**:
   - Change docker-compose paths in all tasks
   - Test: Run "Docker: Start Local Environment" task

4. **Update scripts/**:
   - Update `scripts/coraza_integration.sh`
   - Update `scripts/crowdsec_integration.sh`
   - Update `scripts/crowdsec_startup_test.sh`
   - Update `scripts/integration-test.sh`
   - Test: Run each script

5. **Update .github/workflows/**:
   - Update all workflows referencing docker-compose files
   - Test: Trigger workflows or dry-run locally

6. **Update .dockerignore**:
   - Add `.docker/` exclusion
   - Add `docs/implementation/` exclusion

7. **Update documentation**:
   - Update `README.md`
   - Update `CONTRIBUTING.md`
   - Update `docs/getting-started.md`
   - Update `docs/debugging-local-container.md`
   - Update any docs referencing moved files

8. **Commit all reference updates**:

   ```bash
   git add -A
   git commit -m "chore: update all references to reorganized files

   - Update Dockerfile to reference .docker/docker-entrypoint.sh
   - Update Makefile docker-compose paths
   - Update VS Code tasks with new compose paths
   - Update scripts with new compose paths
   - Update GitHub workflows with new paths
   - Update documentation references
   - Update .dockerignore and .gitignore
   "
   ```

### Phase 4: Verification

1. **Local build test**:

   ```bash
   docker build -t charon:test .
   docker compose -f .docker/compose/docker-compose.yml build
   ```

2. **Local run test**:

   ```bash
   docker compose -f .docker/compose/docker-compose.local.yml up -d
   # Verify Charon starts correctly
   docker compose -f .docker/compose/docker-compose.local.yml down
   ```

3. **Backend tests**:

   ```bash
   cd backend && go test ./...
   ```

4. **Frontend tests**:

   ```bash
   cd frontend && npm run test
   ```

5. **Integration tests**:

   ```bash
   scripts/integration-test.sh
   ```

6. **Pre-commit checks**:

   ```bash
   pre-commit run --all-files
   ```

7. **VS Code tasks**:
   - Test "Build & Run: Local Docker Image"
   - Test "Docker: Start Local Environment"

### Phase 5: CI/CD Monitoring

1. **Push to feature branch**:

   ```bash
   git checkout -b chore/reorganize-structure
   git push origin chore/reorganize-structure
   ```

2. **Create PR** with detailed description:
   - Link to this plan
   - List all changed files
   - Note breaking changes
   - Request review from maintainers

3. **Monitor CI/CD**:
   - Watch all workflow runs
   - Fix any failures immediately
   - Update this plan if new issues discovered

4. **After merge**:
   - Announce in project channels
   - Update any external documentation
   - Monitor for issues in next few days

### Phase 6: Documentation & Enforcement

1. **Create structure enforcement instructions**:
   Create `.github/instructions/structure.instructions.md` to enforce the clean structure going forward:

   ```markdown
   ---
   applyTo: '*'
   description: 'Repository structure guidelines to maintain organized file placement'
   ---

   # Repository Structure Guidelines

   ## Root Level Rules

   The repository root should contain ONLY:
   - Essential config files (`.gitignore`, `.pre-commit-config.yaml`, `Makefile`, etc.)
   - Standard project files (`README.md`, `CONTRIBUTING.md`, `LICENSE`, `CHANGELOG.md`)
   - Go workspace files (`go.work`, `go.work.sum`)
   - VS Code workspace (`Chiron.code-workspace`)
   - Primary `Dockerfile` (entrypoint and compose files live in `.docker/`)

   ## File Placement Rules

   ### Implementation/Feature Documentation
   - **Location**: `docs/implementation/`
   - **Pattern**: `*_SUMMARY.md`, `*_IMPLEMENTATION.md`, `*_COMPLETE.md`, `*_FEATURE.md`
   - **Never** place implementation docs at root

   ### Docker Compose Files
   - **Location**: `.docker/compose/`
   - **Files**: `docker-compose.yml`, `docker-compose.*.yml`
   - **Override**: Local overrides go in `.docker/compose/docker-compose.override.yml`

   ### Docker Support Files
   - **Location**: `.docker/`
   - **Files**: `docker-entrypoint.sh`, Docker documentation

   ### Test Artifacts
   - **Never commit**: `*.sarif`, `*_test.txt`, `*.cover` files at root
   - **Location**: Test outputs should go to `test-results/` or be gitignored

   ### Debug/Temp Config Files
   - **Never commit**: Temporary JSON configs like `caddy_*.json` at root
   - **Location**: Use `configs/` for persistent configs, gitignore temp files

   ## Before Creating New Files

   Ask yourself:
   1. Is this a standard project file? → Root is OK
   2. Is this implementation documentation? → `docs/implementation/`
   3. Is this Docker-related? → `.docker/` or `.docker/compose/`
   4. Is this a test artifact? → `test-results/` or gitignore
   5. Is this a script? → `scripts/`
   6. Is this runtime config? → `configs/`

   ## Enforcement

   Pre-commit hooks and CI will flag files placed incorrectly at root level.
   ```

2. **Update .pre-commit-config.yaml** (optional future enhancement):
   Consider adding a hook to detect new files at root that don't match allowed patterns.

3. **Announce changes**:
   - Update `CHANGELOG.md` with structure reorganization entry
   - Notify contributors of new file placement guidelines

---

## Risk Assessment

### High Risk Changes

| Change | Risk | Mitigation |
|--------|------|------------|
| **Docker Compose Paths** | CI/CD workflows may break | Test all workflows locally before merge |
| **Dockerfile COPY** | Docker build may fail | Test build immediately after change |
| **VS Code Tasks** | Local development disrupted | Update tasks before file moves |
| **Script References** | Integration tests may fail | Test all scripts after updates |

### Medium Risk Changes

| Change | Risk | Mitigation |
|--------|------|------------|
| **Documentation References** | Broken links | Use find-and-replace, verify all links |
| **Makefile Commands** | Local builds may fail | Test all make targets |
| **.dockerignore** | Docker image size may change | Compare before/after image sizes |

### Low Risk Changes

| Change | Risk | Mitigation |
|--------|------|------------|
| **Implementation Docs Move** | Internal docs, low impact | Update any cross-references |
| **SARIF Deletion** | Already gitignored | None needed |
| **.gitignore Updates** | Prevents future pollution | None needed |

### Rollback Plan

If critical issues arise after merge:

1. **Immediate**: Revert the merge commit
2. **Analysis**: Identify what was missed in testing
3. **Fix**: Update this plan with new requirements
4. **Re-attempt**: Create new PR with fixes

---

## Success Criteria

✅ **Before Merge**:

- [ ] All file moves completed
- [ ] All references updated
- [ ] Local Docker build succeeds
- [ ] Local Docker run succeeds
- [ ] Backend tests pass
- [ ] Frontend tests pass
- [ ] Integration tests pass
- [ ] Pre-commit checks pass
- [ ] All VS Code tasks work
- [ ] Documentation updated
- [ ] Structure instructions file created at `.github/instructions/structure.instructions.md`
- [ ] Test/debug files cleaned up and gitignored
- [ ] PR reviewed by maintainers

✅ **After Merge**:

- [ ] All CI/CD workflows pass
- [ ] Docker images build successfully
- [ ] No broken links in documentation
- [ ] No regressions reported
- [ ] Root level has ~15 items (down from 81)

---

## Alternative Approaches Considered

### Alternative 1: Keep Docker Files at Root

**Pros**: No breaking changes, familiar location
**Cons**: Doesn't solve the clutter problem

**Decision**: Rejected - doesn't meet goal of cleaning up root

### Alternative 2: Use `docker/` Instead of `.docker/`

**Pros**: More visible, no hidden directory
**Cons**: Less consistent with `.github/`, `.vscode/` pattern

**Decision**: Rejected - prefer hidden directory for consistency

### Alternative 3: Keep Implementation Docs at Root

**Pros**: Easier to find for contributors
**Cons**: Continues root-level clutter

**Decision**: Rejected - docs belong in `docs/`, can add index

### Alternative 4: Move All Config Files to `.config/`

**Pros**: Maximum organization
**Cons**: Many tools expect configs at root (eslint, pre-commit, etc.)

**Decision**: Rejected - tool requirements win over organization

### Alternative 5: Delete Old Implementation Docs

**Pros**: Maximum cleanup
**Cons**: Loses historical context, implementation notes

**Decision**: Rejected - prefer archiving to deletion

---

## Future Enhancements

After this reorganization, consider:

1. **`.config/` Directory**: For configs that don't need to be at root
2. **`build/` Directory**: For build artifacts and temporary files
3. **`deployments/` Directory**: For deployment configurations (Kubernetes, etc.)
4. **Submodule for Configs**: If `configs/` grows too large
5. **Documentation Site**: Consider moving docs to dedicated site structure

---

## References

- [Twelve-Factor App](https://12factor.net/) - Config management
- [GitHub's .github Directory](https://docs.github.com/en/communities/setting-up-your-project-for-healthy-contributions/creating-a-default-community-health-file)
- [VS Code Workspace](https://code.visualstudio.com/docs/editor/workspaces)
- [Docker Best Practices](https://docs.docker.com/develop/develop-images/dockerfile_best-practices/)

---

## Appendix: Search Commands

For agents implementing this plan, use these commands to find all references:

```bash
# Find docker-compose references:
grep -r "docker-compose\.yml" . --exclude-dir=node_modules --exclude-dir=.git

# Find docker-entrypoint.sh references:
grep -r "docker-entrypoint\.sh" . --exclude-dir=node_modules --exclude-dir=.git

# Find DOCKER.md references:
grep -r "DOCKER\.md" . --exclude-dir=node_modules --exclude-dir=.git

# Find implementation doc references:
grep -r "BULK_ACL_FEATURE\|IMPLEMENTATION_SUMMARY\|ISSUE_16_ACL" . --exclude-dir=node_modules --exclude-dir=.git

# Find SARIF references:
grep -r "\.sarif" . --exclude-dir=node_modules --exclude-dir=.git
```

---

**End of Plan**
