# Agent Skills Migration - Research Summary

**Date**: 2025-12-20
**Status**: Research Complete - Ready for Implementation

## What Was Accomplished

### 1. Complete Script Inventory

- Identified **29 script files** in `/scripts` directory
- Analyzed all scripts referenced in `.vscode/tasks.json`
- Classified scripts by priority, complexity, and use case

### 2. AgentSkills.io Specification Research

- Thoroughly reviewed the [agentskills.io specification](https://agentskills.io/specification)
- Understood the SKILL.md format requirements:
  - YAML frontmatter with required fields (name, description)
  - Optional fields (license, compatibility, metadata, allowed-tools)
  - Markdown body content with instructions
- Learned directory structure requirements:
  - Each skill in its own directory
  - SKILL.md is required
  - Optional subdirectories: `scripts/`, `references/`, `assets/`

### 3. Comprehensive Migration Plan Created

**Location**: `docs/plans/current_spec.md`

The plan includes:

#### A. Directory Structure

- Complete `.agentskills/` directory layout for all 24 skills
- Proper naming conventions (lowercase, hyphens, no special characters)
- Organized by category (testing, security, utility, linting, docker)

#### B. Detailed Skill Specifications

For each of the 24 skills to be created:

- Complete SKILL.md frontmatter with all required fields
- Skill-specific metadata (original script, exit codes, parameters)
- Documentation structure with purpose, usage, examples
- Related skills cross-references

#### C. Implementation Phases

**Phase 1** (Days 1-3): Core Testing & Build

- `test-backend-coverage`
- `test-frontend-coverage`
- `integration-test-all`

**Phase 2** (Days 4-7): Security & Quality

- 8 security and integration test skills
- CrowdSec, Coraza WAF, Trivy scanning

**Phase 3** (Days 8-9): Development Tools

- Version checking, cache clearing, version bumping, DB recovery

**Phase 4** (Days 10-12): Linting & Docker

- 12 linting and Docker management skills
- Complete migration and deprecation of `/scripts`

#### D. Task Configuration Updates

- Complete `.vscode/tasks.json` with all new paths
- Preserves existing task labels and behavior
- All 44 tasks updated to reference `.agentskills` paths

#### E. .gitignore Updates

- Added `.agentskills` runtime data exclusions
- Keeps skill definitions (SKILL.md, scripts) in version control
- Excludes temporary files, logs, coverage data

## Key Decisions Made

### 1. Skills to Create (24 Total)

Organized by category:

- **Testing**: 3 skills (backend, frontend, integration)
- **Security**: 8 skills (Trivy, CrowdSec, Coraza, WAF, rate limiting)
- **Utility**: 4 skills (version check, cache clear, version bump, DB recovery)
- **Linting**: 6 skills (Go, frontend, TypeScript, Markdown, Dockerfile)
- **Docker**: 3 skills (dev env, local env, build)

### 2. Scripts NOT to Convert (11 scripts)

Internal/debug utilities that don't fit the skill model:

- `check_go_build.sh`, `create_bulk_acl_issues.sh`, `debug_db.py`, `debug_rate_limit.sh`, `gopls_collect.sh`, `cerberus_integration.sh`, `install-go-1.25.5.sh`, `qa-test-auth-certificates.sh`, `release.sh`, `repo_health_check.sh`, `verify_crowdsec_app_config.sh`

### 3. Metadata Standards

Each skill includes:

- `author: Charon Project`
- `version: "1.0"`
- `category`: testing|security|build|utility|docker|linting
- `original-script`: Reference to source file
- `exit-code-0` and `exit-code-1`: Exit code meanings

### 4. Backward Compatibility

- Original `/scripts` kept for 1 release cycle
- Clear deprecation notices added
- Parallel run period in CI
- Rollback plan documented

## Next Steps

### Immediate Actions

1. **Review the Plan**: Team reviews `docs/plans/current_spec.md`
2. **Approve Approach**: Confirm phased implementation strategy
3. **Assign Resources**: Determine who implements each phase

### Phase 1 Kickoff (When Approved)

1. Create `.agentskills/` directory
2. Implement first 3 skills (testing)
3. Update tasks.json for Phase 1
4. Test locally and in CI
5. Get team feedback before proceeding

## Files Modified/Created

### Created

- `docs/plans/current_spec.md` - Complete migration plan (replaces old spec)
- `docs/plans/bulk-apply-security-headers-plan.md.backup` - Backup of old plan
- `AGENT_SKILLS_MIGRATION_SUMMARY.md` - This summary

### Modified

- `.gitignore` - Added `.agentskills` runtime data patterns

## Validation Performed

### Script Analysis

✅ Read and understood 8 major scripts:

- `go-test-coverage.sh` - Complex coverage filtering and threshold validation
- `frontend-test-coverage.sh` - npm test with Istanbul coverage
- `integration-test.sh` - Full E2E test with health checks and routing
- `coraza_integration.sh` - WAF testing with block/monitor modes
- `crowdsec_integration.sh` - Preset management testing
- `crowdsec_decision_integration.sh` - Comprehensive ban/unban testing
- `crowdsec_startup_test.sh` - Startup integrity checks
- `db-recovery.sh` - SQLite integrity and recovery

### Specification Compliance

✅ All proposed SKILL.md structures follow agentskills.io spec:

- Valid `name` fields (1-64 chars, lowercase, hyphens only)
- Descriptive `description` fields (1-1024 chars with keywords)
- Optional fields used appropriately (license, compatibility, metadata)
- `allowed-tools` lists all external commands
- Exit codes documented

### Task Configuration

✅ Verified all 44 tasks in `.vscode/tasks.json`
✅ Mapped each script reference to new `.agentskills` path
✅ Preserved task properties (labels, groups, problem matchers)

## Estimated Timeline

- **Research & Planning**: ✅ Complete (1 day)
- **Phase 1 Implementation**: 3 days
- **Phase 2 Implementation**: 4 days
- **Phase 3 Implementation**: 2 days
- **Phase 4 Implementation**: 2 days
- **Deprecation Period**: 18+ days (1 release cycle)
- **Cleanup**: After 1 release

**Total Migration**: ~12 working days
**Full Transition**: ~30 days including deprecation period

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Breaking CI workflows | Parallel run period, fallback to `/scripts` |
| Skills not AI-discoverable | Comprehensive keyword testing, iterate on descriptions |
| Script execution differences | Extensive testing in CI and local environments |
| Documentation drift | Clear deprecation notices, redirect updates |
| Developer confusion | Quick migration timeline, clear communication |

## Questions for Team

1. **Approval**: Does the phased approach make sense?
2. **Timeline**: Is 12 days reasonable, or should we adjust?
3. **Priorities**: Should any phases be reordered?
4. **Validation**: Do we have access to `skills-ref` validation tool?
5. **Rollout**: Should we do canary releases for each phase?

## Conclusion

Research is complete with a comprehensive, actionable plan. The migration to Agent Skills will:

- Make scripts AI-discoverable
- Improve documentation and maintainability
- Follow industry-standard specification
- Maintain backward compatibility
- Enable future enhancements (skill composition, versioning, analytics)

**Plan is ready for review and implementation approval.**

---

**Next Action**: Team review of `docs/plans/current_spec.md`
