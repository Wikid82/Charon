# Supply Chain Vulnerability Remediation

Analyze vulnerability scan results, research each CVE, assess actual risk, and provide concrete remediation steps.

**Input**: $ARGUMENTS — provide ONE of:
1. PR comment (copy/paste from supply chain security bot)
2. GitHub Actions workflow run link
3. Raw Trivy/Grype scan output

## Execution Protocol

### Phase 1: Parse & Triage

Extract: CVE identifiers, affected packages + current versions, severity levels, fixed versions, package ecosystem.

Structure findings:
```
CRITICAL: CVE-2025-XXXXX: golang.org/x/net 1.22.0 → 1.25.5 (Buffer overflow)
HIGH:     CVE-2025-XXXXX: alpine-baselayout 3.4.0 → 3.4.3 (Privilege escalation)
```

Map to project files: Go → `go.mod` | npm → `package.json` | Alpine → `Dockerfile`

### Phase 2: Research & Risk Assessment

For each CVE (Critical → High → Medium → Low):
1. Research CVE details: attack vector, CVSS score, exploitability, PoC availability
2. Impact analysis: Is the vulnerable code path actually used? What's the attack surface?
3. Assign project-specific risk:
   - `CRITICAL-IMMEDIATE`: Exploitable, affects exposed services, no mitigations
   - `HIGH-URGENT`: Exploitable, limited exposure or partial mitigations
   - `MEDIUM-PLANNED`: Low exploitability or strong compensating controls
   - `ACCEPT`: No actual risk to this application (unused code path)

### Phase 3: Remediation

**Go modules**:
```bash
go get golang.org/x/net@v1.25.5
go mod tidy && go mod verify
govulncheck ./...
```

**npm packages**:
```bash
npm update package-name@version
npm audit fix && npm audit
```

**Alpine in Dockerfile**:
```dockerfile
FROM golang:1.25.5-alpine3.19 AS builder
RUN apk upgrade --no-cache affected-package
```

**Acceptance** (when vulnerability doesn't apply):
```yaml
# .trivyignore
CVE-2025-XXXXX # Risk accepted: Not using vulnerable code path — [explanation]
```

### Phase 4: Validation

1. `go test ./...` — full test suite passes
2. `cd frontend && npm test` — frontend tests pass
3. Re-run scan: `.github/skills/scripts/skill-runner.sh security-scan-go-vuln`
4. Re-run Docker image scan: `.github/skills/scripts/skill-runner.sh security-scan-docker-image`

### Phase 5: Documentation

Save report to `docs/security/vulnerability-analysis-[DATE].md` with:
- Executive summary (total found, fixed, mitigated, accepted)
- Per-CVE analysis with impact assessment
- Remediation actions with rationale
- Validation results

Update `SECURITY.md` and `CHANGELOG.md`.

## Constraints

- **Zero tolerance for Critical** without documented risk acceptance
- **Do NOT update major versions** without checking for breaking changes
- **Do NOT suppress warnings** without thorough analysis
- **Do NOT relax scan thresholds** to bypass checks
- All changes must pass the full test suite before being considered complete
