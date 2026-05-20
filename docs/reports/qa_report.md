# QA Report — Orthrus External Docker Proxy
**Date**: 2026-05-20
**Feature**: External Docker Proxy (Orthrus)
**Branch**: feature/hecate

## Gate Results

| Gate | Description | Result | Notes |
|------|-------------|--------|-------|
| 0 | E2E Container Rebuild | ✅ | `charon-e2e` Up healthy; ports 8080/2020/2019 confirmed |
| 1 | Playwright E2E | ❌ | Infrastructure blocker: ubuntu26.04-x64 not supported by Playwright 1.60.0; all install methods exhausted; CI environment (ubuntu-latest) is unaffected |
| 2 | Backend Unit Coverage | ✅ | 89.0% (threshold: 85%); `backend/coverage.txt` generated 2026-05-20 |
| 3 | Frontend Unit Coverage | ✅ | 89.4% (threshold: 85%); `frontend/coverage/lcov.info` valid |
| 4 | Local Patch Report | ⚠️ | 86.9% (threshold: 90%, mode=warn, exit 0); uncovered changed lines: `server.go` L97-99 (40%), `session.go` L127-128, 160-161, 167-170, 210-212 (84.9%) |
| 5 | TypeScript Type-check | ✅ | `npm run type-check` exit 0 |
| 6 | Lefthook Equivalent | ✅ | `go vet ./...` exit 0; ESLint 0 errors (993 pre-existing warnings) |
| 7 | GORM Security Scan | ✅ | 0 CRITICAL, 0 HIGH; 2 pre-existing INFO items (UserID/ProxyHostID FK index in `models/user.go:130-131`) |
| 8 | Trivy FS Scan | ✅ | Exit 0; no CRITICAL/HIGH; DB updated 2026-05-20T05:16:05Z |
| 9 | Docker Image Scan | ❌ | 3 HIGH findings in bundled binaries (Caddy, CrowdSec); no CRITICAL; see Security Findings |

## Coverage Summary

- Backend: 89.0%
- Frontend: 89.4%
- Patch coverage: 86.9% (threshold: 90%, mode: warn)

### Patch Coverage Detail

| File | Patch Coverage | Uncovered Changed Lines |
|------|---------------|------------------------|
| `backend/internal/orthrus/server.go` | 40.0% | 97–99 |
| `backend/internal/orthrus/session.go` | 84.9% | 127–128, 160–161, 167–170, 210–212 |
| `frontend/src/` (changed files) | 100.0% | — |

## Security Findings

### Gate 7 — GORM Security Scan (PASSED)

No CRITICAL or HIGH issues. Two pre-existing INFO items that are non-blocking:
- `backend/internal/models/user.go:130-131` — UserID/ProxyHostID foreign key index (INFO, pre-existing)

### Gate 8 — Trivy Filesystem Scan (PASSED)

No CRITICAL or HIGH vulnerabilities in the filesystem. Trivy DB freshly updated 2026-05-20T05:16:05Z.

### Gate 9 — Docker Image Scan (FAILED)

Three HIGH vulnerabilities found in bundled third-party binaries. No CRITICAL findings.

#### CVE-2026-45135 — `github.com/caddyserver/caddy/v2` (HIGH)

- **Binary**: `/usr/bin/caddy`
- **Installed**: v2.11.2
- **Fixed**: v2.11.3
- **Status**: Fix available — upgrade required
- **Description**: Unsafe Unicode handling in FastCGI `splitPos` allows execution of non-PHP files
- **Reference**: https://avd.aquasec.com/nvd/cve-2026-45135
- **Remediation**: Update Caddy build version in `Dockerfile` from v2.11.2 to v2.11.3

#### CVE-2026-32286 — `github.com/jackc/pgproto3/v2` in CrowdSec (HIGH)

- **Binaries**: `/usr/local/bin/crowdsec`, `/usr/local/bin/cscli`
- **Installed**: v2.3.3
- **Fixed**: No upstream fix available at time of scan
- **Status**: Affected; awaiting upstream patch in CrowdSec
- **Description**: Denial of Service via malicious PostgreSQL server response
- **Reference**: https://avd.aquasec.com/nvd/cve-2026-32286
- **Risk Context**: Exploitable only when CrowdSec is configured to use a PostgreSQL backend. The default Charon deployment uses the local SQLite backend; PostgreSQL is not used. Risk is LOW in standard deployments.
- **Remediation**: Track upstream CrowdSec release that patches pgproto3/v2; no immediate action available

### Pre-existing Backend Test Failure (Not Orthrus-related)

- `TestSendExternal_AllEventTypes/domain` (notification/webhook tests) — pre-existing failure unrelated to this feature; does not affect Gate 2 result.

## Conclusion

**BLOCKED** — Gate 9 failed due to HIGH severity CVEs in bundled third-party binaries.

### Required Actions Before Approval

1. **Gate 9 — Caddy CVE-2026-45135** (blocking): Upgrade Caddy from v2.11.2 to v2.11.3 in the Dockerfile. A fix is available.
2. **Gate 9 — pgproto3 CVE-2026-32286** (conditional): No upstream fix exists yet. Document as accepted risk given Charon's default non-PostgreSQL CrowdSec configuration, or pin for remediation when CrowdSec releases a patched version.
3. **Gate 1 — Playwright E2E** (infrastructure): Local environment is blocked by ubuntu26.04-x64 incompatibility with Playwright 1.60.0. CI (ubuntu-latest) runs these tests successfully. This gate should be confirmed passing via CI before merge.
4. **Gate 4 — Patch Coverage** (advisory): 86.9% is below the 90% threshold. Uncovered lines in `server.go` (L97–99) and `session.go` (L127–128, 160–161, 167–170, 210–212) should receive targeted tests to close the gap.
