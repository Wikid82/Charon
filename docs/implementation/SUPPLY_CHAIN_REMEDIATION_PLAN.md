# Supply Chain Vulnerability Remediation Plan

**Created**: 2026-01-11
**Priority**: MEDIUM
**Target Completion**: Before next production release

## Summary

CI supply chain scans detected 4 HIGH-severity vulnerabilities in CrowdSec binaries (Go stdlib v1.25.1). Our application code is clean, but third-party binaries need updates.

## Vulnerabilities to Address

### 🔴 Critical Path Issues

#### 1. CrowdSec Binary Vulnerabilities (HIGH x4)

**Components Affected**:

- `/usr/local/bin/crowdsec`
- `/usr/local/bin/cscli`

**CVEs**:

1. **CVE-2025-58183** - archive/tar: Unbounded allocation in GNU sparse map parsing
2. **CVE-2025-58186** - net/http: Unbounded HTTP headers
3. **CVE-2025-58187** - crypto/x509: Name constraint checking performance
4. **CVE-2025-61729** - crypto/x509: HostnameError.Error() string construction

**Root Cause**: CrowdSec v1.6.5 compiled with Go 1.25.1 (vulnerable)

**Resolution**: Upgrade to CrowdSec v1.6.6+ (compiled with Go 1.25.2+)

## Action Items

### Phase 1: Immediate (This Sprint)

#### ✅ Action 1.1: Update CrowdSec Version in Dockerfile

**File**: [Dockerfile](../../Dockerfile)

```diff
- ARG CROWDSEC_VERSION=1.6.5
+ ARG CROWDSEC_VERSION=1.6.6
```

**Assignee**: @dev-team
**Effort**: 5 minutes
**Risk**: LOW - Version bump, tested upstream

#### ✅ Action 1.2: Verify CrowdSec Go Version

After rebuild, verify the Go version used:

```bash
docker run --rm charon:local /usr/local/bin/crowdsec version
docker run --rm charon:local /usr/local/bin/cscli version
```

**Expected Output**: Should show Go 1.25.2 or later

**Assignee**: @qa-team
**Effort**: 10 minutes

#### ✅ Action 1.3: Re-run Supply Chain Scan

```bash
# Local verification
docker build -t charon:local .
syft charon:local -o cyclonedx-json > sbom-verification.json
grype sbom:./sbom-verification.json --severity HIGH,CRITICAL
```

**Expected**: 0 HIGH/CRITICAL vulnerabilities in all binaries

**Assignee**: @security-team
**Effort**: 15 minutes

### Phase 2: CI/CD Enhancement (Next Sprint)

#### ⏳ Action 2.1: Add Vulnerability Severity Thresholds

**File**: [.github/workflows/supply-chain-verify.yml](../../.github/workflows/supply-chain-verify.yml)

Add component-level filtering to distinguish Charon vs third-party issues:

```yaml
- name: Analyze Vulnerability Report
  run: |
    # Parse and categorize vulnerabilities
    CHARON_CRITICAL=$(jq '[.matches[] | select(.artifact.name | test("charon|caddy")) | select(.vulnerability.severity == "Critical")] | length' vuln-scan.json)
    CHARON_HIGH=$(jq '[.matches[] | select(.artifact.name | test("charon|caddy")) | select(.vulnerability.severity == "High")] | length' vuln-scan.json)

    THIRDPARTY_HIGH=$(jq '[.matches[] | select(.artifact.name | test("crowdsec|cscli|dlv")) | select(.vulnerability.severity == "High")] | length' vuln-scan.json)

    echo "## Vulnerability Summary" >> $GITHUB_STEP_SUMMARY
    echo "| Component | Critical | High |" >> $GITHUB_STEP_SUMMARY
    echo "|-----------|----------|------|" >> $GITHUB_STEP_SUMMARY
    echo "| Charon/Caddy | ${CHARON_CRITICAL} | ${CHARON_HIGH} |" >> $GITHUB_STEP_SUMMARY
    echo "| Third-party | 0 | ${THIRDPARTY_HIGH} |" >> $GITHUB_STEP_SUMMARY

    # Fail on critical issues in our code
    if [[ ${CHARON_CRITICAL} -gt 0 || ${CHARON_HIGH} -gt 0 ]]; then
      echo "::error::Critical/High vulnerabilities detected in Charon components"
      exit 1
    fi

    # Warning for third-party (but don't fail build)
    if [[ ${THIRDPARTY_HIGH} -gt 0 ]]; then
      echo "::warning::${THIRDPARTY_HIGH} high-severity vulnerabilities in third-party binaries"
      echo "Review and schedule upgrade of affected components"
    fi
```

**Assignee**: @devops-team
**Effort**: 2 hours (implementation + testing)
**Benefit**: Prevent false-positive build failures

#### ⏳ Action 2.2: Create Vulnerability Suppression Policy

**File**: [.grype.yaml](../../.grype.yaml) (new file)

```yaml
# Grype vulnerability suppression configuration
# Review and update quarterly

match-config:
  # Ignore vulnerabilities in build artifacts (not in final image)
  - path: "**/.cache/**"
    ignore: true

  # Ignore test fixtures (private keys in test data)
  - path: "**/fixtures/**"
    ignore: true

ignore:
  # Template for documented exceptions
  # - vulnerability: CVE-YYYY-XXXXX
  #   package:
  #     name: package-name
  #     version: "1.2.3"
  #   reason: "Justification here"
  #   expiry: "2026-MM-DD"  # Auto-expire exceptions
```

**Assignee**: @security-team
**Effort**: 1 hour
**Review Cycle**: Quarterly

#### ⏳ Action 2.3: Add Pre-commit Hook for Local Scanning

**File**: [.pre-commit-config.yaml](../../.pre-commit-config.yaml)

Add Trivy hook for pre-push image scanning:

```yaml
  - repo: local
    hooks:
      - id: trivy-docker
        name: Trivy Docker Image Scan
        entry: sh -c 'trivy image --exit-code 1 --severity CRITICAL charon:local'
        language: system
        pass_filenames: false
        stages: [manual]  # Only run on explicit `pre-commit run --hook-stage manual`
```

**Usage**:

```bash
# Run before pushing
pre-commit run --hook-stage manual trivy-docker
```

**Assignee**: @dev-team
**Effort**: 30 minutes

### Phase 3: Long-term Hardening (Backlog)

#### 📋 Action 3.1: Multi-stage Build Optimization

**Goal**: Minimize attack surface by removing build artifacts from runtime image

**Changes**:

1. Separate builder and runtime stages
2. Remove development tools from final image
3. Use distroless base for Charon binary

**Effort**: 1 day
**Benefit**: Reduce image size ~50%, eliminate build-time vulnerabilities

#### 📋 Action 3.2: Implement SLSA Verification

**Goal**: Verify provenance of third-party binaries at build time

```dockerfile
# Verify CrowdSec signature before installing
RUN cosign verify --key crowdsec.pub \
    ghcr.io/crowdsecurity/crowdsec:${CROWDSEC_VERSION}
```

**Effort**: 4 hours
**Benefit**: Prevent supply chain tampering

#### 📋 Action 3.3: Dependency Version Pinning

**Goal**: Ensure reproducible builds with version/checksum verification

```dockerfile
# Instead of:
ARG CROWDSEC_VERSION=1.6.6

# Use:
ARG CROWDSEC_VERSION=1.6.6
ARG CROWDSEC_CHECKSUM=sha256:abc123...
```

**Effort**: 2 hours
**Benefit**: Prevent unexpected updates, improve audit trail

## Testing Strategy

### Unit Tests

- ✅ Existing Go tests continue to pass
- ✅ CrowdSec integration tests validate upgrade

### Integration Tests

```bash
# Run integration test suite
.github/skills/scripts/skill-runner.sh integration-test-all
```

**Expected**: All tests pass with CrowdSec v1.6.6

### Security Tests

```bash
# Verify no regressions
govulncheck ./...  # Charon code
trivy image --severity HIGH,CRITICAL charon:local  # Full image
grype sbom:./sbom.json  # SBOM analysis
```

**Expected**: 0 HIGH/CRITICAL in Charon, Caddy, and CrowdSec

### Smoke Tests (Post-deployment)

1. CrowdSec starts successfully
2. Logs show correct version
3. Decision engine processes alerts
4. WAF integration works correctly

## Rollback Plan

If CrowdSec v1.6.6 causes issues:

1. **Immediate**: Revert Dockerfile to v1.6.5
2. **Mitigation**: Accept risk temporarily, schedule hotfix
3. **Communication**: Update security team and stakeholders
4. **Timeline**: Re-attempt upgrade within 7 days

## Success Criteria

✅ **Deployment Approved** when:

- [ ] CrowdSec upgraded to v1.6.6+
- [ ] All HIGH/CRITICAL vulnerabilities resolved
- [ ] CI supply chain scan passes
- [ ] Integration tests pass
- [ ] Security team sign-off

## Communication

### Stakeholders

- **Development Team**: Implement Dockerfile changes
- **QA Team**: Verify post-upgrade functionality
- **Security Team**: Review scan results and sign off
- **DevOps Team**: Update CI/CD workflows
- **Product Owner**: Approve deployment window

### Status Updates

- **Daily**: Slack #security-updates
- **Weekly**: Include in sprint review
- **Completion**: Email to <security@company.com> with scan results

## Timeline

| Phase | Start Date | Target Completion | Status |
|-------|------------|-------------------|--------|
| Phase 1: Immediate Fixes | 2026-01-11 | 2026-01-13 | 🟡 In Progress |
| Phase 2: CI Enhancement | 2026-01-15 | 2026-01-20 | ⏳ Planned |
| Phase 3: Long-term | 2026-02-01 | 2026-03-01 | 📋 Backlog |

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| CrowdSec v1.6.6 breaks integration | LOW | MEDIUM | Test thoroughly in staging, have rollback ready |
| New vulnerabilities in v1.6.6 | LOW | LOW | Monitor CVE feeds, subscribe to CrowdSec security advisories |
| CI changes cause false negatives | MEDIUM | HIGH | Add validation step, peer review configuration |
| Delayed upgrade causes audit fail | LOW | MEDIUM | Document accepted risk, set expiry date |

## Appendix

### Related Documents

- [Supply Chain Scan Analysis](./SUPPLY_CHAIN_SCAN_ANALYSIS.md)
- [Security Policy](../../SECURITY.md)
- [CI/CD Documentation](../../.github/workflows/README.md)

### References

- [CrowdSec v1.6.6 Release Notes](https://github.com/crowdsecurity/crowdsec/releases/tag/v1.6.6)
- [Go 1.25.2 Security Fixes](https://go.dev/doc/devel/release#go1.25.2)
- [NIST CVE Database](https://nvd.nist.gov/)

---

**Last Updated**: 2026-01-11
**Next Review**: 2026-02-11 (or upon completion)
**Owner**: Security Team
