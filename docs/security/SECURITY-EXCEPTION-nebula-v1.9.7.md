# Security Exception: Nebula v1.9.7 (GHSA-69x3-g4r3-p962)

**Date:** 2026-02-10
**Status:** ACCEPTED RISK
**CVE:** GHSA-69x3-g4r3-p962
**Severity:** High
**Package:** github.com/slackhq/nebula@v1.9.7
**Fixed Version:** v1.10.3

## Decision
Accept the High severity vulnerability in nebula v1.9.7 as a documented known issue.

## Rationale
- Nebula is a transitive dependency via CrowdSec bouncer -> ipstore chain
- Upgrading to v1.10.3 breaks compilation:
  - smallstep/certificates removed nebula APIs (NebulaCAPool, NewCAPoolFromBytes, etc.)
  - ipstore missing GetAndDelete method compatibility
- No compatible upstream versions exist as of 2026-02-10
- Patching dependencies during build is high-risk and fragile
- High severity risk classification applies to vulnerabilities within our control
- This is an upstream dependency management issue beyond our immediate control

## Dependency Chain
- Caddy (xcaddy builder)
  - github.com/hslatman/caddy-crowdsec-bouncer@v0.9.2
    - github.com/hslatman/ipstore@v0.3.0
      - github.com/slackhq/nebula@v1.9.7 (vulnerable)

## Exploitability Assessment
- Nebula is present in Docker image build artifacts
- Used by CrowdSec bouncer for IP address management
- Attack surface: [Requires further analysis - see monitoring plan]

## Monitoring Plan
Watch for upstream fixes in:
- github.com/hslatman/caddy-crowdsec-bouncer (primary)
- github.com/hslatman/ipstore (secondary)
- github.com/smallstep/certificates (nebula API compatibility)
- github.com/slackhq/nebula (direct upgrade if dependency chain updates)

Check quarterly (or when Dependabot/security scans alert):
- CrowdSec bouncer releases: https://github.com/hslatman/caddy-crowdsec-bouncer/releases
- ipstore releases: https://github.com/hslatman/ipstore/releases
- smallstep/certificates releases: https://github.com/smallstep/certificates/releases

## Remediation Trigger
Revisit and remediate when ANY of:
- caddy-crowdsec-bouncer releases version with nebula v1.10.3+ support
- smallstep/certificates releases version compatible with nebula v1.10.3
- ipstore releases version fixing GetAndDelete compatibility
- GHSA-69x3-g4r3-p962 severity escalates to CRITICAL
- Proof-of-concept exploit published targeting Charon's attack surface

## Alternative Mitigation (Future)
If upstream remains stalled:
- Consider removing CrowdSec bouncer plugin (loss of CrowdSec integration)
- Evaluate alternative IP blocking/rate limiting solutions
- Implement CrowdSec integration at reverse proxy layer instead of Caddy

## References
- CVE Details: https://github.com/advisories/GHSA-69x3-g4r3-p962
- Analysis Report: [docs/reports/nebula_upgrade_analysis.md](../reports/nebula_upgrade_analysis.md)
- Version Test Results: [docs/reports/nebula_upgrade_analysis.md](../reports/nebula_upgrade_analysis.md#6-version-compatibility-test-results)
