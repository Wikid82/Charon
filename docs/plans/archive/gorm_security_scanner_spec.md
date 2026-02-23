# GORM Security Scanner Implementation Plan

**Status**: READY FOR IMPLEMENTATION
**Priority**: HIGH 🟡
**Created**: 2026-01-28
**Security Issue**: GORM ID Leak Detection & Common GORM Mistake Scanning

---

## Executive Summary

**Objective**: Implement automated static analysis to detect GORM security issues and common mistakes in the codebase, focusing on ID leak prevention and proper model structure validation.

**Impact**:
- 🟡 Prevents exposure of internal database IDs in API responses
- 🟡 Catches common GORM misconfigurations before deployment
- 🟢 Improves code quality and security posture
- 🟢 Reduces code review burden

**Integration Points**:
- Pre-commit hook (manual stage initially, blocking after validation)
- VS Code task for quick developer checks
- Definition of Done requirement
- CI pipeline validation

---

## Current State Analysis

### Findings from Codebase Audit

#### ✅ Good Patterns Found
1. **Sensitive Fields Properly Hidden**:
   - `PasswordHash` in User model: `json:"-"`
   - `CredentialsEncrypted` in DNSProvider: `json:"-"`
   - `InviteToken` in User model: `json:"-"`
   - `BreakGlassHash` in SecurityConfig: `json:"-"`

2. **UUID Usage**:
   - Most models include a `UUID` field for external references
   - UUIDs properly indexed with `gorm:"uniqueIndex"`

3. **Proper Indexes**:
   - Foreign keys have indexes: `gorm:"index"`
   - Frequently queried fields indexed appropriately

4. **GORM Hooks**:
   - `BeforeCreate` hooks properly generate UUIDs
   - Hooks follow GORM conventions

#### 🔴 Critical Issues Found

**1. Widespread ID Exposure (20+ models)**

All the following models expose internal database IDs via `json:"id"`:

| Model | Location | Line | Issue |
|-------|----------|------|-------|
| User | `internal/models/user.go` | 23 | `json:"id" gorm:"primaryKey"` |
| ProxyHost | `internal/models/proxy_host.go` | 9 | `json:"id" gorm:"primaryKey"` |
| Domain | `internal/models/domain.go` | 11 | `json:"id" gorm:"primarykey"` |
| DNSProvider | (inferred) | - | `json:"id" gorm:"primaryKey"` |
| SSLCertificate | `internal/models/ssl_certificate.go` | 10 | `json:"id" gorm:"primaryKey"` |
| AccessList | (inferred) | - | `json:"id" gorm:"primaryKey"` |
| SecurityConfig | `internal/models/security_config.go` | 10 | `json:"id" gorm:"primaryKey"` |
| SecurityAudit | `internal/models/security_audit.go` | 9 | `json:"id" gorm:"primaryKey"` |
| SecurityDecision | `internal/models/security_decision.go` | 10 | `json:"id" gorm:"primaryKey"` |
| SecurityHeaderProfile | `internal/models/security_header_profile.go` | 10 | `json:"id" gorm:"primaryKey"` |
| SecurityRuleset | `internal/models/security_ruleset.go` | 9 | `json:"id" gorm:"primaryKey"` |
| NotificationTemplate | `internal/models/notification_template.go` | 13 | `json:"id" gorm:"primaryKey"` (string) |
| Notification | `internal/models/notification.go` | 20 | `json:"id" gorm:"primaryKey"` (string) |
| NotificationConfig | `internal/models/notification_config.go` | 12 | `json:"id" gorm:"primaryKey"` (string) |
| Location | `internal/models/location.go` | 9 | `json:"id" gorm:"primaryKey"` |
| Plugin | `internal/models/plugin.go` | 8 | `json:"id" gorm:"primaryKey"` |
| RemoteServer | `internal/models/remote_server.go` | 10 | `json:"id" gorm:"primaryKey"` |
| ImportSession | `internal/models/import_session.go` | 10 | `json:"id" gorm:"primaryKey"` |
| Setting | `internal/models/setting.go` | 10 | `json:"id" gorm:"primaryKey"` |
| UptimeMonitor | `internal/models/uptime.go` | 39 | `json:"id" gorm:"primaryKey"` |
| UptimeHost | `internal/models/uptime.go` | 11 | `json:"id" gorm:"primaryKey"` (string) |

**2. Response DTOs Still Expose IDs**

Response structs that embed models inherit the ID exposure:

```go
// ❌ BAD: Inherits json:"id" from models.ProxyHost
type ProxyHostResponse struct {
    models.ProxyHost
    Warnings []ProxyHostWarning `json:"warnings,omitempty"`
}

// ❌ BAD: Inherits json:"id" from models.DNSProvider
type DNSProviderResponse struct {
    models.DNSProvider
    HasCredentials bool `json:"has_credentials"`
}
```

**3. Non-service Structs Expose IDs Too**

Even non-database structs expose internal IDs:

```go
// internal/services/docker_service.go:47
type ContainerInfo struct {
    ID      string       `json:"id"`
    // ...
}

// internal/services/manual_challenge_service.go:337
type Challenge struct {
    ID                   string     `json:"id"`
    // ...
}

// internal/services/websocket_tracker.go:13
type Connection struct {
    ID             string    `json:"id"`
    // ...
}
```

---

## Security Rationale: Why ID Leaks Matter

### 1. **Information Disclosure**
- Internal database IDs reveal sequential patterns
- Attackers can enumerate resources by incrementing IDs
- Database structure and growth rate exposed

### 2. **Direct Object Reference (IDOR) Vulnerability**
- Makes IDOR attacks easier (guess valid IDs)
- Increases attack surface for authorization bypass
- Enables resource enumeration attacks

### 3. **Best Practice Violation**
- OWASP recommends using opaque identifiers for external references
- Industry standard: Use UUIDs/slugs for external APIs
- Internal IDs should never leave the application boundary

### 4. **Maintenance Issues**
- Harder to migrate databases (ID collisions)
- Difficult to merge data from multiple sources
- Complicates distributed systems

---

## Scanner Architecture

### Design Principles

1. **Single Responsibility**: Scanner is focused on GORM patterns only
2. **Fail Fast**: Exit on first detection in blocking mode
3. **Detailed Reports**: Provide file, line, and remediation guidance
4. **Extensible**: Easy to add new patterns
5. **Performance**: Fast enough for pre-commit (<5 seconds)

### Scanner Modes

| Mode | Trigger | Behavior | Exit Code |
|------|---------|----------|-----------|
| **Report** | Manual/VS Code | Lists all issues, doesn't fail | 0 |
| **Check** | Pre-commit (manual stage) | Reports issues, fails if found | 1 if issues |
| **Enforce** | Pre-commit (blocking) | Blocks commit on issues | 1 if issues |

---

## Detection Patterns

### Pattern 1: GORM Model ID Exposure (CRITICAL)

**What to Detect**:
```go
// ❌ BAD: Numeric ID exposed
type User struct {
    ID   uint   `json:"id" gorm:"primaryKey"`
    UUID string `json:"uuid" gorm:"uniqueIndex"`
}

// ✅ GOOD: String ID assumed to be UUID/opaque
type Notification struct {
    ID string `json:"id" gorm:"primaryKey"` // String IDs allowed (assumed UUID)
}
```

**Detection Logic**:
1. Find all `type XXX struct` declarations
2. **Apply GORM Model Detection Heuristics** (to avoid false positives):
   - File is in `internal/models/` directory, OR
   - Struct has 2+ fields with `gorm:` tags, OR
   - Struct embeds `gorm.Model`
3. Look for `ID` field with `gorm:"primaryKey"` or `gorm:"primarykey"`
4. Check if ID field type is `uint`, `int`, `int64`, or `*uint`, `*int`, `*int64`
5. Check if JSON tag exists and is NOT `json:"-"`
6. Report as **CRITICAL**

**String ID Policy Decision** (Option A - Recommended):
- **String-based primary keys are ALLOWED** and not flagged by the scanner
- **Rationale**: String IDs are typically UUIDs or other opaque identifiers, which are safe for external use
- **Assumption**: String IDs are non-sequential and cryptographically random (e.g., UUIDv4, ULID)
- **Manual Review Needed**: If a string ID is:
  - Sequential or auto-incrementing (e.g., "USER001", "ORDER002")
  - Generated from predictable patterns
  - Based on MD5/SHA hashes of enumerable inputs
  - Timestamp-based without sufficient entropy
- **Suppression**: Use `// gorm-scanner:ignore` comment if a legitimate string ID is incorrectly flagged in future

**Recommended Fix**:
```go
// ✅ GOOD: Hide internal numeric ID, use UUID for external reference
type User struct {
    ID   uint   `json:"-" gorm:"primaryKey"`
    UUID string `json:"uuid" gorm:"uniqueIndex"`
}
```

### Pattern 2: Response DTO Embedding Models (HIGH)

**What to Detect**:
```go
// ❌ BAD: Inherits ID from embedded model
type ProxyHostResponse struct {
    models.ProxyHost
    Warnings []string `json:"warnings"`
}
```

**Detection Logic**:
1. Find all structs with "Response" or "DTO" in the name
2. Look for embedded model types (type without field name)
3. Check if embedded type is from `models` package
4. Warn if the embedded model has an exposed ID field

**Recommended Fix**:
```go
// ✅ GOOD: Explicitly select fields
type ProxyHostResponse struct {
    UUID        string   `json:"uuid"`
    Name        string   `json:"name"`
    DomainNames string   `json:"domain_names"`
    Warnings    []string `json:"warnings"`
}
```

### Pattern 3: Missing Primary Key Tag (MEDIUM)

**What to Detect**:
```go
// ❌ BAD: ID field without primary key tag
type Thing struct {
    ID   uint   `json:"id"`
    Name string `json:"name"`
}
```

**Detection Logic**:
1. Find structs with an `ID` field
2. Check if GORM tags are present
3. If `gorm:` tag doesn't include `primaryKey`, report as warning

**Recommended Fix**:
```go
// ✅ GOOD
type Thing struct {
    ID   uint   `json:"-" gorm:"primaryKey"`
    UUID string `json:"uuid" gorm:"uniqueIndex"`
    Name string `json:"name"`
}
```

### Pattern 4: Foreign Key Without Index (LOW)

**What to Detect**:
```go
// ❌ BAD: Foreign key without index
type ProxyHost struct {
    CertificateID *uint `json:"certificate_id"`
}
```

**Detection Logic**:
1. Find fields ending with `ID` or `Id`
2. Check if field type is `uint`, `*uint`, or similar
3. If `gorm:` tag doesn't include `index` or `uniqueIndex`, report as suggestion

**Recommended Fix**:
```go
// ✅ GOOD
type ProxyHost struct {
    CertificateID *uint           `json:"-" gorm:"index"`
    Certificate   *SSLCertificate `json:"certificate" gorm:"foreignKey:CertificateID"`
}
```

### Pattern 5: Exposed API Keys/Secrets (CRITICAL)

**What to Detect**:
```go
// ❌ BAD: API key visible in JSON
type User struct {
    APIKey string `json:"api_key"`
}
```

**Detection Logic**:
1. Find fields with names: `APIKey`, `Secret`, `Token`, `Password`, `Hash`
2. Check if JSON tag is NOT `json:"-"`
3. Report as **CRITICAL**

**Recommended Fix**:
```go
// ✅ GOOD
type User struct {
    APIKey       string `json:"-" gorm:"uniqueIndex"`
    PasswordHash string `json:"-"`
}
```

### Pattern 6: Missing UUID for Models with Exposed IDs (HIGH)

**What to Detect**:
```go
// ❌ BAD: No external identifier
type Thing struct {
    ID   uint   `json:"id" gorm:"primaryKey"`
    Name string `json:"name"`
}
```

**Detection Logic**:
1. Find models with exposed `json:"id"`
2. Check if a `UUID` or similar external ID field exists
3. If not, recommend adding one

**Recommended Fix**:
```go
// ✅ GOOD
type Thing struct {
    ID   uint   `json:"-" gorm:"primaryKey"`
    UUID string `json:"uuid" gorm:"uniqueIndex"`
    Name string `json:"name"`
}
```

---

## Implementation Details

### File Structure

```
scripts/
└── scan-gorm-security.sh         # Main scanner script

scripts/pre-commit-hooks/
└── gorm-security-check.sh        # Pre-commit wrapper

.vscode/
└── tasks.json                    # Add VS Code task

.pre-commit-config.yaml           # Add hook definition

docs/implementation/
└── gorm_security_scanner_complete.md  # Implementation log

docs/plans/
└── gorm_security_scanner_spec.md      # This file
```

### Script: `scripts/scan-gorm-security.sh`

**Purpose**: Main scanner that detects GORM security issues

**Features**:
- Scans all .go files in `backend/`
- Detects all 6 patterns
- Supports multiple output modes (report/check/enforce)
- Colorized output with severity levels
- Provides file:line references and remediation guidance
- Exit codes for CI/pre-commit integration

**Usage**:
```bash
# Report mode (always exits 0)
./scripts/scan-gorm-security.sh --report

# Check mode (exits 1 if issues found)
./scripts/scan-gorm-security.sh --check

# Enforce mode (exits 1 on any issue)
./scripts/scan-gorm-security.sh --enforce
```

**Output Format**:
```
🔍 GORM Security Scanner v1.0
Scanning: backend/internal/models/*.go
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔴 CRITICAL: ID Field Exposed in JSON
  File: backend/internal/models/user.go:23
  Struct: User
  Issue: ID field has json:"id" tag (should be json:"-")
  Fix: Change `json:"id"` to `json:"-"` and use UUID for external references

🟡 HIGH: Response DTO Embeds Model With Exposed ID
  File: backend/internal/api/handlers/proxy_host_handler.go:30
  Struct: ProxyHostResponse
  Issue: Embeds models.ProxyHost which exposes ID field
  Fix: Explicitly define response fields instead of embedding

🟢 INFO: Foreign Key Without Index
  File: backend/internal/models/thing.go:15
  Struct: Thing
  Field: CategoryID
  Fix: Add gorm:"index" tag for better query performance

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Summary:
  3 CRITICAL issues found
  5 HIGH issues found
  12 INFO suggestions

🚫 FAILED: Security issues detected
```

### Script: `scripts/pre-commit-hooks/gorm-security-check.sh`

**Purpose**: Wrapper for pre-commit integration

**Implementation**:
```bash
#!/usr/bin/env bash
set -euo pipefail

# Pre-commit hook for GORM security scanning
cd "$(git rev-parse --show-toplevel)"

echo "🔒 Running GORM Security Scanner..."
./scripts/scan-gorm-security.sh --check
```

### VS Code Task Configuration

**Add to `.vscode/tasks.json`**:
```json
{
    "label": "Lint: GORM Security Scan",
    "type": "shell",
    "command": "./scripts/scan-gorm-security.sh --report",
    "group": {
        "kind": "test",
        "isDefault": false
    },
    "presentation": {
        "reveal": "always",
        "panel": "dedicated",
        "clear": true,
        "showReuseMessage": false
    },
    "problemMatcher": [],
    "options": {
        "cwd": "${workspaceFolder}"
    }
}
```

### Pre-commit Hook Configuration

**Add to `.pre-commit-config.yaml`**:
```yaml
- repo: local
  hooks:
    - id: gorm-security-scan
      name: GORM Security Scanner (Manual)
      entry: scripts/pre-commit-hooks/gorm-security-check.sh
      language: script
      files: '\.go$'
      pass_filenames: false
      stages: [manual]  # Manual stage initially
      verbose: true
      description: "Detects GORM ID leaks and common GORM security mistakes"
```

---

## Implementation Phases

### Phase 1: Core Scanner Development (60 minutes)

**Objective**: Create fully functional scanner script

**Tasks**:
1. ✅ Create `scripts/scan-gorm-security.sh`
2. ✅ Implement Pattern 1 detection (ID exposure)
3. ✅ Implement Pattern 2 detection (DTO embedding)
4. ✅ Implement Pattern 5 detection (API key exposure)
5. ✅ Add colorized output with severity levels
6. ✅ Add file:line reporting
7. ✅ Add remediation guidance
8. ✅ Test on current codebase

**Deliverables**:
- `scripts/scan-gorm-security.sh` (executable, ~300 lines)
- Test run output showing 20+ ID leak detections

**Success Criteria**:
- Detects all 20+ known ID exposures in models
- Detects Response DTO embedding issues
- Clear, actionable output
- Runs in <5 seconds

### Phase 2: Extended Pattern Detection (30 minutes)

**Objective**: Add remaining detection patterns

**Tasks**:
1. ✅ Implement Pattern 3 (missing primary key tags)
2. ✅ Implement Pattern 4 (missing foreign key indexes)
3. ✅ Implement Pattern 6 (missing UUID fields)
4. ✅ Add pattern enable/disable flags
5. ✅ Test each pattern independently

**Deliverables**:
- Extended `scripts/scan-gorm-security.sh`
- Pattern flags: `--pattern=id-leak`, `--pattern=all`, etc.

**Success Criteria**:
- All 6 patterns detect correctly
- No false positives on existing good patterns
- Patterns can be enabled/disabled individually

### Phase 3: Integration (8-12 hours)

**Objective**: Integrate scanner into development workflow

**Tasks**:
1. ✅ Create `scripts/pre-commit-hooks/gorm-security-check.sh`
2. ✅ Add entry to `.pre-commit-config.yaml` (manual stage)
3. ✅ Add VS Code task to `.vscode/tasks.json`
4. ✅ Test pre-commit hook manually
5. ✅ Test VS Code task

**Deliverables**:
- `scripts/pre-commit-hooks/gorm-security-check.sh`
- Updated `.pre-commit-config.yaml`
- Updated `.vscode/tasks.json`

**Success Criteria**:
- `pre-commit run gorm-security-scan --all-files` works
- VS Code task runs from command palette
- Exit codes correct for pass/fail scenarios

**Timeline Breakdown**:
- **Model Updates (6-8 hours)**: Update 20+ model files to hide numeric IDs
  - Mechanical changes: `json:"id"` → `json:"-"`
  - Verify UUID fields exist and are properly exposed
  - Update any direct `.ID` references in tests
- **Response DTO Updates (1-2 hours)**: Refactor embedded models to explicit fields
  - Identify all Response/DTO structs
  - Replace embedding with explicit field definitions
  - Update handler mapping logic
- **Frontend Coordination (1-2 hours)**: Update API clients to use UUIDs
  - Search for numeric ID usage in frontend code
  - Replace with UUID-based references
  - Test API integration
  - Update TypeScript types if needed

### Phase 4: Documentation & Testing (30 minutes)

**Objective**: Complete documentation and validate

**Tasks**:
1. ✅ Create `docs/implementation/gorm_security_scanner_complete.md`
2. ✅ Update `CONTRIBUTING.md` with scanner usage
3. ✅ Update Definition of Done with scanner requirement
4. ✅ Create test cases for false positives
5. ✅ Run full test suite

**Deliverables**:
- Implementation documentation
- Updated contribution guidelines
- Test validation report

**Success Criteria**:
- All documentation complete
- Scanner runs without errors on codebase
- Zero false positives on compliant code
- Clear usage instructions for developers

### Phase 5: CI Integration (Required - 15 minutes)

**Objective**: Add scanner to CI pipeline for automated enforcement

**Tasks**:
1. ✅ Add scanner step to `.github/workflows/test.yml`
2. ✅ Configure to run on PR checks
3. ✅ Set up failure annotations

**Deliverables**:
- Updated CI workflow

**Success Criteria**:
- Scanner runs on every PR
- Issues annotated in GitHub PR view

**GitHub Actions Workflow Configuration**:

Add to `.github/workflows/test.yml` after the linting steps:

```yaml
- name: GORM Security Scanner
  run: |
    chmod +x scripts/scan-gorm-security.sh
    ./scripts/scan-gorm-security.sh --check
  continue-on-error: false

- name: Annotate GORM Security Issues
  if: failure()
  run: |
    echo "::error title=GORM Security Issues::Run './scripts/scan-gorm-security.sh --report' locally for details"
```

---

## Testing Strategy

### Test Cases

| Test Case | Expected Result |
|-----------|----------------|
| Model with `json:"id"` | ❌ CRITICAL detected |
| Model with `json:"-"` on ID | ✅ PASS |
| Response DTO embedding model | ❌ HIGH detected |
| Response DTO with explicit fields | ✅ PASS |
| APIKey with `json:"api_key"` | ❌ CRITICAL detected |
| APIKey with `json:"-"` | ✅ PASS |
| Foreign key with index | ✅ PASS |
| Foreign key without index | ⚠️ INFO suggestion |
| Non-GORM struct with ID | ✅ PASS (ignored) |

### Test Validation Commands

```bash
# Test scanner on existing codebase
./scripts/scan-gorm-security.sh --report | tee gorm-scan-initial.txt

# Verify exit codes
./scripts/scan-gorm-security.sh --check
echo "Exit code: $?"  # Should be 1 (issues found)

# Test individual patterns
./scripts/scan-gorm-security.sh --pattern=id-leak
./scripts/scan-gorm-security.sh --pattern=dto-embedding
./scripts/scan-gorm-security.sh --pattern=api-key-exposure

# Test pre-commit integration
pre-commit run gorm-security-scan --all-files

# Test VS Code task
# Open Command Palette → Tasks: Run Task → Lint: GORM Security Scan
```

### Expected Initial Results

Based on codebase audit, the scanner should detect:
- **20+ CRITICAL**: Models with exposed IDs
- **2-5 HIGH**: Response DTOs embedding models
- **0 CRITICAL**: API key exposures (already compliant)
- **10-15 INFO**: Missing foreign key indexes

---

## Remediation Roadmap

### Priority 1: Fix Critical ID Exposures (2-3 hours)

**Approach**: Mass update all models to hide ID fields

**Script**: `scripts/fix-id-leaks.sh` (optional automation)

**Pattern**:
```bash
# For each model file:
# 1. Change json:"id" to json:"-" on ID field
# 2. Verify UUID field exists and is exposed
# 3. Update tests if they reference .ID directly
```

**Files to Update** (20+ models):
- `internal/models/user.go`
- `internal/models/proxy_host.go`
- `internal/models/domain.go`
- `internal/models/ssl_certificate.go`
- `internal/models/access_list.go`
- (see "Critical Issues Found" section for full list)

### Priority 2: Fix Response DTO Embedding (1 hour)

**Files to Update**:
- `internal/api/handlers/proxy_host_handler.go`
- `internal/services/dns_provider_service.go`

**Pattern**:
```go
// Before
type ProxyHostResponse struct {
    models.ProxyHost
    Warnings []string `json:"warnings"`
}

// After
type ProxyHostResponse struct {
    UUID        string   `json:"uuid"`
    Name        string   `json:"name"`
    DomainNames string   `json:"domain_names"`
    // ... copy all needed fields
    Warnings    []string `json:"warnings"`
}
```

### Priority 3: Add Scanner to Blocking Pre-commit (15 minutes)

**When**: After all critical issues fixed

**Change in `.pre-commit-config.yaml`**:
```yaml
- id: gorm-security-scan
  stages: [commit]  # Change from [manual] to [commit]
```

---

## Definition of Done Updates

### New Requirement

Add to project's Definition of Done:

```markdown
### GORM Security Compliance

- [ ] All GORM models have `json:"-"` on ID fields
- [ ] External identifiers (UUID/slug) exposed instead of internal IDs
- [ ] Response DTOs do not embed models with exposed IDs
- [ ] API keys, secrets, and credentials have `json:"-"` tags
- [ ] GORM security scanner passes: `./scripts/scan-gorm-security.sh --check`
- [ ] Pre-commit hook enforces GORM security: `pre-commit run gorm-security-scan`
```

### Integration into Existing DoD

Add to existing DoD checklist after "Tests pass" section:
```markdown
## Security

- [ ] No GORM ID leaks (run: `./scripts/scan-gorm-security.sh`)
- [ ] No secrets in code (run: `git secrets --scan`)
- [ ] No high/critical issues (run: `pre-commit run --all-files`)
```

---

## Monitoring & Maintenance

### Metrics to Track

1. **Issue Count Over Time**
   - Track decrease in ID leak issues
   - Goal: Zero critical issues within 1 week

2. **Scanner Performance**
   - Execution time per run
   - Target: <5 seconds for full scan

3. **False Positive Rate**
   - Track developer overrides
   - Target: <5% false positive rate

4. **Adoption Rate**
   - Percentage of PRs running scanner
   - Target: 100% compliance after blocking enforcement

### Maintenance Tasks

**Monthly**:
- Review new GORM patterns in codebase
- Update scanner to detect new antipatterns
- Review false positive reports

**Quarterly**:
- Audit scanner effectiveness
- Benchmark performance
- Update documentation

---

## Rollout Plan

### Week 1: Development & Testing

**Days 1-2**:
- Implement core scanner (Phase 1)
- Add extended patterns (Phase 2)
- Integration testing

**Days 3-4**:
- Documentation
- Team review and feedback
- Refinements

**Day 5**:
- Final testing
- Merge to main branch

### Week 2: Soft Launch

**Manual Stage Enforcement**:
- Scanner available via `pre-commit run gorm-security-scan --all-files`
- Developers can run manually or via VS Code task
- No blocking, only reporting

**Activities**:
- Send team announcement
- Demo in team meeting
- Gather feedback
- Monitor usage

### Week 3: Issue Remediation

**Focus**: Fix all critical ID exposures

**Activities**:
- Create issues for each model needing fixes
- Assign to developers
- Code review emphasis on scanner compliance
- Track progress daily

### Week 4: Hard Launch

**Blocking Enforcement**:
- Move scanner from manual to commit stage in `.pre-commit-config.yaml`
- Scanner now blocks commits with issues
- CI pipeline enforces scanner on PRs

**Activities**:
- Team announcement of enforcement
- Update documentation
- Monitor for issues
- Quick response to false positives

---

## Success Criteria

### Technical Success

- ✅ Scanner detects all 6 GORM security patterns
- ✅ Zero false positives on compliant code
- ✅ Execution time <5 seconds
- ✅ Integration with pre-commit and VS Code
- ✅ Clear, actionable error messages

### Team Success

- ✅ All developers trained on scanner usage
- ✅ Scanner runs on 100% of commits (after blocking)
- ✅ Zero critical issues in production code
- ✅ Positive developer feedback (>80% satisfaction)

### Security Success

- ✅ Zero GORM ID leaks in API responses
- ✅ All sensitive fields properly hidden
- ✅ Reduced IDOR attack surface
- ✅ Compliance with OWASP API Security Top 10

---

## Risk Assessment & Mitigation

### Risk 1: High False Positive Rate

**Probability**: MEDIUM
**Impact**: HIGH - Developer frustration, skip pre-commit

**Mitigation**:
- Extensive testing before rollout
- Quick response process for false positives
- Whitelist mechanism for exceptions
- Pattern refinement based on feedback

### Risk 2: Performance Impact on Development

**Probability**: LOW
**Impact**: MEDIUM - Slower commit process

**Mitigation**:
- Target <5 second execution time
- Cache scan results between runs
- Scan only modified files in pre-commit
- Manual stage during soft launch period

### Risk 3: Existing Codebase Has Too Many Issues

**Probability**: HIGH (already confirmed 20+ issues)
**Impact**: MEDIUM - Overwhelming fix burden

**Mitigation**:
- Start with manual stage (non-blocking)
- Create systematic remediation plan
- Fix highest priority issues first
- Phased rollout over 4 weeks
- Team collaboration on fixes

### Risk 4: Scanner Becomes Outdated

**Probability**: MEDIUM
**Impact**: LOW - Misses new patterns

**Mitigation**:
- Scheduled quarterly reviews
- Easy pattern addition mechanism
- Community feedback channel
- Version tracking and changelog

---

## Alternatives Considered

### Alternative 1: golangci-lint Custom Linter

**Pros**:
- Integrates with existing golangci-lint setup
- AST-based analysis (more accurate)
- Potential for broader Go community use

**Cons**:
- Requires Go plugin development
- More complex to maintain
- Harder to customize per-project
- Steeper learning curve

**Decision**: ❌ Rejected - Too complex for immediate needs

### Alternative 2: Manual Code Review Only

**Pros**:
- No tooling overhead
- Human judgment for edge cases
- Flexible

**Cons**:
- Not scalable
- Inconsistent enforcement
- Easy to miss in review
- Higher review burden

**Decision**: ❌ Rejected - Not reliable enough

### Alternative 3: GitHub Actions Only (No Local Check)

**Pros**:
- Centralized enforcement
- No local setup required

**Cons**:
- Delayed feedback (after push)
- Wastes CI resources
- Slower development cycle
- No offline development support

**Decision**: ❌ Rejected - Prefer shift-left security

### Selected Approach: Bash Script with Pre-commit ✅

**Pros**:
- Simple, maintainable
- Fast feedback (pre-commit)
- Easy to customize
- No dependencies
- Portable across environments
- Both automated and manual modes

**Cons**:
- Regex-based (less sophisticated than AST)
- Bash knowledge required
- Potential for false positives

**Decision**: ✅ Accepted - Best balance of simplicity and effectiveness

---

## Appendix A: Scanner Script Pseudocode

```bash
#!/usr/bin/env bash
set -euo pipefail

# Configuration
MODE="${1:---report}"  # --report, --check, --enforce
PATTERN="${2:-all}"    # all, id-leak, dto-embedding, etc.

# State
ISSUES_FOUND=0
CRITICAL_COUNT=0
HIGH_COUNT=0
INFO_COUNT=0

# Helper Functions

is_gorm_model() {
    local file="$1"
    local struct_name="$2"

    # Heuristic 1: File in internal/models/ directory
    if [[ "$file" == *"/internal/models/"* ]]; then
        return 0
    fi

    # Heuristic 2: Struct has 2+ fields with gorm: tags
    local gorm_tag_count=$(grep -A 20 "type $struct_name struct" "$file" | grep -c 'gorm:' || true)
    if [[ $gorm_tag_count -ge 2 ]]; then
        return 0
    fi

    # Heuristic 3: Embeds gorm.Model
    if grep -A 20 "type $struct_name struct" "$file" | grep -q 'gorm\.Model'; then
        return 0
    fi

    return 1
}

has_suppression_comment() {
    local file="$1"
    local line_num="$2"

    # Check for // gorm-scanner:ignore comment before or on the line
    local start_line=$((line_num - 1))
    if sed -n "${start_line},${line_num}p" "$file" | grep -q '// gorm-scanner:ignore'; then
        return 0
    fi

    return 1
}

# Pattern Detection Functions

detect_id_leak() {
    # Find: type XXX struct { ID ... json:"id" gorm:"primaryKey" }
    # Apply GORM model detection heuristics
    # Only flag numeric ID types (uint, int, int64, *uint, *int, *int64)
    # Allow string IDs (assumed to be UUIDs)
    # Exclude: json:"-"
    # Exclude: Lines with // gorm-scanner:ignore
    # Report: CRITICAL

    for file in $(find backend -name '*.go'); do
        while IFS= read -r line_num; do
            local struct_name=$(extract_struct_name "$file" "$line_num")

            # Skip if not a GORM model
            if ! is_gorm_model "$file" "$struct_name"; then
                continue
            fi

            # Skip if has suppression comment
            if has_suppression_comment "$file" "$line_num"; then
                continue
            fi

            # Extract ID field type
            local id_type=$(extract_id_type "$file" "$line_num")

            # Only flag numeric types
            if [[ "$id_type" =~ ^(\*?)u?int(64)?$ ]]; then
                # Check for json:"id" (not json:"-")
                if field_has_exposed_json_tag "$file" "$line_num"; then
                    report_critical "ID-LEAK" "$file" "$line_num" "$struct_name"
                    ((CRITICAL_COUNT++))
                    ((ISSUES_FOUND++))
                fi
            fi
        done < <(grep -n 'ID.*json:"id".*gorm:"primaryKey"' "$file" || true)
    done
}

detect_dto_embedding() {
    # Find: type XXX[Response|DTO] struct { models.YYY }
    # Check: If models.YYY has exposed ID
    # Report: HIGH
}

detect_exposed_secrets() {
    # Find: APIKey, Secret, Token, Password with json tags
    # Exclude: json:"-"
    # Report: CRITICAL
}

detect_missing_primary_key() {
    # Find: ID field without gorm:"primaryKey"
    # Report: MEDIUM
}

detect_foreign_key_index() {
    # Find: Fields ending with ID without gorm:"index"
    # Report: INFO
}

detect_missing_uuid() {
    # Find: Models with exposed ID but no UUID field
    # Report: HIGH
}

# Main Execution

print_header
scan_directory "backend/internal/models"
scan_directory "backend/internal/api/handlers"
scan_directory "backend/internal/services"
print_summary

# Exit based on mode and findings
exit $EXIT_CODE
```

---

## Appendix B: Example Scan Output

```
🔍 GORM Security Scanner v1.0.0
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📂 Scanning: backend/internal/models/
📂 Scanning: backend/internal/api/handlers/
📂 Scanning: backend/internal/services/

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔴 CRITICAL ISSUES (3)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[ID-LEAK-001] GORM Model ID Field Exposed in JSON
  📄 File: backend/internal/models/user.go:23
  🏗️  Struct: User
  📌 Field: ID uint
  🔖 Tags: json:"id" gorm:"primaryKey"

  ❌ Issue: Internal database ID is exposed in JSON serialization

  💡 Fix:
     1. Change json:"id" to json:"-" to hide internal ID
     2. Use the UUID field for external references
     3. Update API clients to use UUID instead of ID

  📝 Example:
     // Before
     ID   uint   `json:"id" gorm:"primaryKey"`
     UUID string `json:"uuid" gorm:"uniqueIndex"`

     // After
     ID   uint   `json:"-" gorm:"primaryKey"`
     UUID string `json:"uuid" gorm:"uniqueIndex"`

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[ID-LEAK-002] GORM Model ID Field Exposed in JSON
  📄 File: backend/internal/models/proxy_host.go:9
  🏗️  Struct: ProxyHost
  📌 Field: ID uint
  [... similar output ...]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🟡 HIGH PRIORITY ISSUES (2)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[DTO-EMBED-001] Response DTO Embeds Model With Exposed ID
  📄 File: backend/internal/api/handlers/proxy_host_handler.go:30
  🏗️  Struct: ProxyHostResponse
  📦 Embeds: models.ProxyHost

  ❌ Issue: Embedded model exposes internal ID field through inheritance

  💡 Fix:
     1. Remove embedded model
     2. Explicitly define only the fields needed in the response
     3. Map between model and DTO in handler

  📝 Example:
     // Before
     type ProxyHostResponse struct {
         models.ProxyHost
         Warnings []string `json:"warnings"`
     }

     // After
     type ProxyHostResponse struct {
         UUID        string   `json:"uuid"`
         Name        string   `json:"name"`
         DomainNames string   `json:"domain_names"`
         Warnings    []string `json:"warnings"`
     }

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🟢 INFORMATIONAL (5)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[FK-INDEX-001] Foreign Key Field Missing Index
  📄 File: backend/internal/models/thing.go:15
  🏗️  Struct: Thing
  📌 Field: CategoryID *uint

  ℹ️  Suggestion: Add index for better query performance

  📝 Example:
     // Before
     CategoryID *uint `json:"category_id"`

     // After
     CategoryID *uint `json:"category_id" gorm:"index"`

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 SUMMARY
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Scanned: 45 Go files (3,890 lines)
  Duration: 2.3 seconds

  🔴 CRITICAL: 3 issues
  🟡 HIGH:     2 issues
  🔵 MEDIUM:   0 issues
  🟢 INFO:     5 suggestions

  Total Issues: 5 (excluding informational)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

❌ FAILED: 5 security issues detected

Run with --help for usage information
Run with --pattern=<name> to scan specific patterns only
```

---

## Known Limitations and Edge Cases

### Pattern Detection Limitations

1. **Custom MarshalJSON Implementations**:
   - **Issue**: If a model implements custom `MarshalJSON()` method, the scanner won't detect ID exposure in the custom logic
   - **Example**:
     ```go
     type User struct {
         ID uint `json:"-" gorm:"primaryKey"`
     }

     // ❌ Scanner won't detect this ID leak
     func (u User) MarshalJSON() ([]byte, error) {
         return json.Marshal(map[string]interface{}{
             "id": u.ID,  // Leak not detected
         })
     }
     ```
   - **Mitigation**: Manual code review for custom marshaling, future enhancement to detect custom MarshalJSON

2. **Map Conversions and Reflection**:
   - **Issue**: Scanner can't detect ID leaks through runtime map conversions or reflection-based serialization
   - **Example**:
     ```go
     // ❌ Scanner won't detect this pattern
     data := map[string]interface{}{
         "id": user.ID,
     }
     ```
   - **Mitigation**: Code review, runtime logging/monitoring

3. **XML and YAML Tags**:
   - **Issue**: Scanner currently only checks `json:` tags, misses `xml:` and `yaml:` serialization
   - **Example**:
     ```go
     type User struct {
         ID uint `xml:"id" gorm:"primaryKey"` // Not detected
     }
     ```
   - **Mitigation**: Document as future enhancement (Pattern 7: XML tag exposure, Pattern 8: YAML tag exposure)

4. **Multi-line Tag Handling**:
   - **Issue**: Tags split across multiple lines may not be detected correctly
   - **Example**:
     ```go
     type User struct {
         ID uint `json:"id"
                  gorm:"primaryKey"`  // May not be detected
     }
     ```
   - **Mitigation**: Enforce single-line tags in code style guide

5. **Interface Implementations**:
   - **Issue**: Models returned through interfaces may bypass detection
   - **Example**:
     ```go
     func GetModel() interface{} {
         return &User{ID: 1}  // Exposed ID not detected in interface context
     }
     ```
   - **Mitigation**: Type-based analysis (future enhancement)

### False Positive Scenarios

1. **Non-GORM Structs with ID Fields**:
   - **Mitigation**: Implemented GORM model detection heuristics (file location, gorm tag count, gorm.Model embedding)

2. **External API Response Structs**:
   - **Issue**: Structs that unmarshal external API responses may need exposed ID fields
   - **Example**:
     ```go
     // This is OK - not a GORM model, just an API response
     type GitHubUser struct {
         ID int `json:"id"` // External API ID
     }
     ```
   - **Mitigation**: Use `// gorm-scanner:ignore` suppression comment

3. **Test Fixtures and Mock Data**:
   - **Issue**: Test structs may intentionally expose IDs for testing
   - **Mitigation**: Exclude `*_test.go` files from scanning (configuration option)

### Future Enhancements (Patterns 7 & 8)

**Pattern 7: XML Tag Exposure**
- Detect `xml:"id"` on primary key fields
- Same severity and remediation as JSON exposure

**Pattern 8: YAML Tag Exposure**
- Detect `yaml:"id"` on primary key fields
- Same severity and remediation as JSON exposure

---

## Performance Benchmarking Plan

### Benchmark Criteria

1. **Execution Time**:
   - **Target**: <5 seconds for full codebase scan
   - **Measurement**: Total wall-clock time from start to exit
   - **Failure Threshold**: >10 seconds (blocks developer workflow)

2. **Memory Usage**:
   - **Target**: <100 MB peak memory
   - **Measurement**: Peak RSS during scan
   - **Failure Threshold**: >500 MB (impacts system performance)

3. **File Processing Rate**:
   - **Target**: >50 files/second
   - **Measurement**: Total files / execution time
   - **Failure Threshold**: <10 files/second

4. **Pattern Detection Accuracy**:
   - **Target**: 100% recall on known issues, <5% false positive rate
   - **Measurement**: Compare against manual audit results
   - **Failure Threshold**: <95% recall or >10% false positives

### Test Scenarios

1. **Small Codebase (10 files, 500 lines)**:
   - Expected: <0.5 seconds
   - Use case: Single package scan during development

2. **Medium Codebase (50 files, 5,000 lines)**:
   - Expected: <2 seconds
   - Use case: Pre-commit hook on backend directory

3. **Large Codebase (200+ files, 20,000+ lines)**:
   - Expected: <5 seconds
   - Use case: Full repository scan in CI

4. **Very Large Codebase (1000+ files, 100,000+ lines)**:
   - Expected: <15 seconds
   - Use case: Monorepo or large enterprise codebase

### Benchmark Commands

```bash
# Basic timing
time ./scripts/scan-gorm-security.sh --report

# Detailed profiling with GNU time
/usr/bin/time -v ./scripts/scan-gorm-security.sh --report

# Benchmark script (run 10 times, report average)
for i in {1..10}; do
    time ./scripts/scan-gorm-security.sh --report > /dev/null
done 2>&1 | grep real | awk '{sum+=$2} END {print "Average: " sum/NR "s"}'

# Memory profiling
/usr/bin/time -f "Peak Memory: %M KB" ./scripts/scan-gorm-security.sh --report
```

### Expected Output

```
🔍 GORM Security Scanner - Performance Benchmark
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Test: Charon Backend Codebase
  Files Scanned: 87 Go files
  Lines Processed: 12,453 lines

Performance Metrics:
  ⏱️  Execution Time: 2.3 seconds
  💾 Peak Memory: 42 MB
  📈 Processing Rate: 37.8 files/second

Accuracy Metrics:
  ✅ Known Issues Detected: 23/23 (100%)
  ⚠️  False Positives: 1/24 (4.2%)

✅ PASS: All performance criteria met
```

### Performance Optimization Strategies

If benchmarks fail to meet targets:

1. **Parallel Processing**: Use `xargs -P` for multi-core scanning
2. **Incremental Scanning**: Only scan changed files in pre-commit
3. **Caching**: Cache previous scan results, only re-scan modified files
4. **Optimized Regex**: Profile and optimize slow regex patterns
5. **Compiled Scanner**: Rewrite in Go for better performance if needed

---

## Suppression Mechanism

### Overview

The scanner supports inline suppression comments for intentional exceptions to the detection patterns.

### Comment Format

```go
// gorm-scanner:ignore [optional reason]
```

**Placement**: Comment must appear on the line immediately before the flagged code, or on the same line.

### Use Cases

1. **External API Response Structs**:
   ```go
   // gorm-scanner:ignore External API response, not a GORM model
   type GitHubUser struct {
       ID int `json:"id"` // ID from GitHub API
   }
   ```

2. **Legacy Code During Migration**:
   ```go
   // gorm-scanner:ignore Legacy model, scheduled for refactor in #1234
   type OldModel struct {
       ID uint `json:"id" gorm:"primaryKey"`
   }
   ```

3. **Test Fixtures**:
   ```go
   // gorm-scanner:ignore Test fixture, ID needed for assertions
   type TestUser struct {
       ID uint `json:"id"`
   }
   ```

4. **Internal Services (Non-HTTP)**:
   ```go
   // gorm-scanner:ignore Internal service struct, never serialized to HTTP
   type InternalProcessorState struct {
       ID uint `json:"id"`
   }
   ```

### Best Practices

1. **Always Provide a Reason**: Explain why the suppression is needed
2. **Link to Issues**: Reference GitHub issues for planned refactors
3. **Review Suppressions**: Periodically audit suppression comments to ensure they're still valid
4. **Minimize Usage**: Suppressions are exceptions, not the rule. Fix the root cause when possible.
5. **Document in PR**: Call out suppressions in PR descriptions for review

### Scanner Behavior

When a suppression comment is encountered:
- The issue is **not reported** in scan output
- A debug log message is generated (if `--verbose` flag used)
- Suppression count is tracked and shown in summary

### Example Output with Suppressions

```
📊 SUMMARY
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Scanned: 45 Go files (3,890 lines)
  Duration: 2.3 seconds

  🔴 CRITICAL: 3 issues
  🟡 HIGH:     2 issues
  🔵 MEDIUM:   0 issues
  🟢 INFO:     5 suggestions

  🔇 Suppressed: 2 issues (see --verbose for details)

  Total Issues: 5 (excluding informational and suppressed)
```

---

## Error Handling

### Script Error Handling Patterns

The scanner must gracefully handle edge cases and provide clear error messages.

### Error Categories

1. **File System Errors**:
   - **Binary files**: Skip with warning
   - **Permission denied**: Report error and continue
   - **Missing directories**: Exit with error code 2
   - **Symbolic links**: Follow or skip based on configuration

2. **Malformed Go Code**:
   - **Syntax errors**: Report warning, skip file, continue scan
   - **Incomplete structs**: Warn and skip
   - **Missing closing braces**: Attempt recovery or skip

3. **Runtime errors**:
   - **Out of memory**: Exit with error code 3, suggest smaller batch
   - **Timeout**: Exit with error code 4, suggest `--fast` mode
   - **Signal interruption**: Cleanup and exit with code 130

### Error Handling Implementation

```bash
# Trap errors and cleanup
trap 'handle_error $? $LINENO' ERR
traps 'cleanup_and_exit' EXIT INT TERM

handle_error() {
    local exit_code=$1
    local line_num=$2
    echo "❌ ERROR: Scanner failed at line $line_num (exit code: $exit_code)" >&2
    echo "   Run with --debug for more details" >&2
    cleanup_and_exit $exit_code
}

cleanup_and_exit() {
    # Remove temporary files
    rm -f /tmp/gorm-scan-*.tmp 2>/dev/null || true
    exit ${1:-1}
}

# Safe file processing
process_file() {
    local file="$1"

    # Check if file exists
    if [[ ! -f "$file" ]]; then
        log_warning "File not found: $file"
        return 0
    fi

    # Check if file is readable
    if [[ ! -r "$file" ]]; then
        log_error "Permission denied: $file"
        return 0
    fi

    # Check if file is binary
    if file "$file" | grep -q "binary"; then
        log_debug "Skipping binary file: $file"
        return 0
    fi

    # Proceed with scan
    scan_file "$file"
}
```

### Error Exit Codes

| Code | Meaning | Action |
|------|---------|--------|
| 0 | Success, no issues found | Continue |
| 1 | Issues detected (normal failure) | Fix issues |
| 2 | Invalid arguments or configuration | Check command syntax |
| 3 | File system error | Check permissions/paths |
| 4 | Runtime error (OOM, timeout) | Optimize scan or increase resources |
| 5 | Internal script error | Report bug |
| 130 | Interrupted by user (SIGINT) | Normal interruption |

---

## Future Enhancements

### Planned Features

1. **Auto-fix Mode (`--fix` flag)**:
   - Automatically apply fixes for Pattern 1 (ID exposure)
   - Change `json:"id"` to `json:"-"` in-place
   - Create backup files before modification
   - Generate diff report of changes
   - **Estimated Effort**: 2-3 hours
   - **Priority**: High

2. **JSON Output Format (`--output=json`)**:
   - Machine-readable output for CI integration
   - Schema-based output for tool integration
   - Enables custom reporting dashboards
   - **Estimated Effort**: 1 hour
   - **Priority**: Medium

3. **Batch Remediation Script**:
   - `scripts/fix-all-gorm-issues.sh`
   - Applies fixes to all detected issues at once
   - Interactive mode for confirmation
   - Dry-run mode to preview changes
   - **Estimated Effort**: 3-4 hours
   - **Priority**: Medium

4. **golangci-lint Plugin**:
   - Implement as Go-based linter plugin
   - AST-based analysis for higher accuracy
   - Better performance on large codebases
   - Community contribution potential
   - **Estimated Effort**: 8-12 hours
   - **Priority**: Low (future consideration)

5. **Pattern Definitions in Config File**:
   - `.gorm-scanner.yml` configuration
   - Custom pattern definitions
   - Per-project suppression rules
   - Severity level customization
   - **Estimated Effort**: 2-3 hours
   - **Priority**: Low

6. **IDE Integration**:
   - Real-time detection in VS Code
   - Quick-fix suggestions
   - Inline warnings in editor
   - Language server protocol support
   - **Estimated Effort**: 8-16 hours
   - **Priority**: Low (nice to have)

7. **Pattern 7: XML Tag Exposure Detection**
8. **Pattern 8: YAML Tag Exposure Detection**
9. **Custom MarshalJSON Detection**
10. **Map Conversion Analysis (requires AST)**

### Community Contributions

If this scanner proves valuable:
- Open-source as standalone tool
- Submit to awesome-go list
- Create golangci-lint plugin
- Write blog post on GORM security patterns

---

## Appendix C: Reference Links

### GORM Documentation
- [GORM JSON Tags](https://gorm.io/docs/models.html#Fields-Tags)
- [GORM Data Serialization](https://gorm.io/docs/serialization.html)
- [GORM Indexes](https://gorm.io/docs/indexes.html)

### Security Best Practices
- [OWASP API Security Top 10](https://owasp.org/www-project-api-security/)
- [OWASP Direct Object Reference (IDOR)](https://owasp.org/www-community/attacks/Insecure_Direct_Object_References)
- [CWE-639: Authorization Bypass Through User-Controlled Key](https://cwe.mitre.org/data/definitions/639.html)

### Tool References
- [Pre-commit Framework](https://pre-commit.com/)
- [golangci-lint](https://golangci-lint.run/)
- [ShellCheck](https://www.shellcheck.net/) (for scanner script quality)

---

**Plan Status**: ✅ COMPLETE AND READY FOR IMPLEMENTATION

**Next Actions**:
1. Review and approve this plan
2. Create GitHub issue tracking implementation
3. Begin Phase 1: Core Scanner Development
4. Schedule team demo for Week 2

**Estimated Total Effort**:
- **Development**: 2.5 hours (scanner + patterns + docs)
- **Integration & Remediation**: 8-12 hours (model updates, DTOs, frontend coordination)
- **CI Integration**: 15 minutes (required)
- **Total**: 11-14.5 hours development + 1 week rollout

**Timeline**: 4 weeks from approval to full enforcement

---

**Questions or Concerns?**

Contact the security team or leave comments on the implementation issue.
