---
post_title: "GHSA-69x3-g4r3-p962 Remediation Options"
categories: ["security", "ci"]
tags: ["ghsa-69x3-g4r3-p962", "nebula", "caddy", "risk-acceptance", "docker-scan"]
summary: "Remediation options memo for GHSA-69x3-g4r3-p962 in Charon when direct nebula upgrade is blocked by upstream dependency incompatibility."
post_date: "2026-02-19"
---

## Context and Current Evidence

- Vulnerability: `GHSA-69x3-g4r3-p962` (`github.com/slackhq/nebula`, fixed in `1.10.3`).
- Current scanner evidence in this repo indicates:
  - package/version: `github.com/slackhq/nebula@v1.9.7`
  - artifact location: `/usr/bin/caddy`
  - source: `grype-results.json`
- `backend` module does not directly require `nebula` (`go mod why -m github.com/slackhq/nebula` returns main module does not need it).
- Docker build logic explicitly pins `nebula@v1.9.7` in Caddy builder with a compatibility note stating `v1.10+` currently breaks compilation in upstream chain.
- Prior repository analysis reports show forced upgrade attempts failing on `smallstep/certificates` API mismatch and `ipstore` compatibility issues.

## Root Dependency Chain Hypotheses

The exact chain may vary by Caddy/plugin version, but these are the most plausible paths.

| Hypothesis | Why Plausible | Confirmability Checks | What Would Falsify It |
|---|---|---|---|
| H1: `caddy-security` path pulls `smallstep/certificates` which pulls `nebula` | Caddy builder includes `github.com/greenpau/caddy-security`; prior logs mention `smallstep/certificates` compile failures against `nebula` API changes | Rebuild only Caddy stage and inspect generated module graph and `go mod graph` | No `smallstep/certificates` in generated graph |
| H2: `caddy-crowdsec-bouncer` path pulls `ipstore` which pulls `nebula` | Builder includes crowdsec bouncer; prior scan artifacts and historical reports show `bouncer -> ipstore -> nebula` | Inspect generated module graph from xcaddy temp build and grep for `hslatman/ipstore` and `slackhq/nebula` | `ipstore` absent from graph or no path to `nebula` |
| H3: stale artifact mismatch between current Dockerfile and scan metadata | Dockerfile currently references newer plugin/version combinations than some older reports | Regenerate SBOM and scan from a clean build, compare package versions and chains | Fresh SBOM/scan matches old chain exactly |
| H4: vulnerability exists in binary metadata but runtime path is non-reachable in Charon’s active features | Vulnerable package is in `caddy` binary; exploit preconditions may not be met in deployed config | Validate loaded Caddy modules and active config; verify no Nebula-related cert/blocklist flows configured | Runtime config shows Nebula-related path active with matching exploit preconditions |

## Confirmability Checks (Team Runnable)

Use these checks to move from hypothesis to evidence.

### A) Chain attribution checks

```bash
# 1) Confirm backend is not direct source
cd /projects/Charon/backend
go mod why -m github.com/slackhq/nebula

# 2) Confirm Docker build currently pins nebula in Caddy stage
cd /projects/Charon
rg -n "go get github.com/slackhq/nebula|caddy-crowdsec-bouncer|smallstep/certificates|ipstore" Dockerfile

# 3) Confirm scanner sees vulnerable package in caddy binary
jq '.matches[] | select(.vulnerability.id=="GHSA-69x3-g4r3-p962") |
    {package:.artifact.name, version:.artifact.version, locations:.artifact.locations, fix:.vulnerability.fix.versions}' \
  /projects/Charon/grype-results.json
```

### B) Fresh-build verification checks

```bash
# 4) Rebuild Caddy stage with full logs to capture current dependency behavior
cd /projects/Charon
docker build --target caddy-builder --progress=plain -t charon-caddy-builder-debug . 2>&1 | tee /tmp/charon-caddy-builder.log

# 5) Rebuild full image and regenerate SBOM + grype report for current state
# (Use existing project tasks/skills where available)
.github/skills/scripts/skill-runner.sh security-scan-docker-image
```

### C) Reachability/exploitability-context checks (confidence-building, not proof)

```bash
# 6) Inspect loaded Caddy modules at runtime (if container is running)
docker exec -it charon caddy list-modules | rg -i "crowdsec|security|step|nebula"

# 7) Inspect active Caddy config for handlers/modules that could traverse vulnerable paths
curl -s http://localhost:2019/config/ | jq '.. | objects | select(has("handler") or has("module"))'

# 8) Search Charon code/config for explicit Nebula-specific usage or config assumptions
cd /projects/Charon
rg -n "NebulaCAPool|NewCAPoolFromBytes|UnmarshalNebulaCertificate|nebula" backend frontend configs .docker
```

## Mitigation Options (Ranked by Feasibility/Risk)

### Short-term compensating controls

1. **Time-boxed temporary exception with strict evidence and controls**
   - Keep CI gate logically strict, but allow a temporary exception for this specific GHSA while blocked upstream.
   - Add expiry date, named owner, weekly reassessment, and mandatory upstream tracking issue.
2. **Exposure reduction while exception is active**
   - Prefer minimal plugin surface in environments that do not require affected functionality.
   - Restrict admin/API exposure and enforce existing hardening controls (network policy, auth, least privilege).
3. **Continuous monitoring and trigger-based revocation**
   - Revoke exception immediately on: upstream fix available and buildable, exploit PoC increasing practical risk, or widened runtime reachability evidence.

### Medium-term engineering paths

1. **Adopt upstream-compatible Caddy/plugin chain that supports `nebula >= 1.10.3`**
   - Preferred sustainable fix; lowest long-term maintenance burden.
2. **Fork/patch transitive dependency chain to restore compatibility with fixed nebula**
   - Higher engineering burden; useful if upstream SLA is too slow.
3. **Re-architect/remove specific plugin causing chain inclusion (if feature trade-off acceptable)**
   - Can eliminate vulnerable chain, but may reduce security/features (for example CrowdSec integration path).

## Decision Matrix

| Option | Security Impact | Build/Runtime Risk | Effort | Time-to-implement | CI policy impact | Recommendation |
|---|---|---|---|---|---|---|
| O1: Force `nebula@1.10.3` now (direct override) | High positive if successful | High (known compile break risk) | Medium | Short for attempt, uncertain for success | Keeps strict block if works; currently causes failures | **Not recommended now** |
| O2: Temporary exception with compensating controls + expiry | Medium (risk accepted, bounded) | Low-to-medium | Low | Fast | Requires scoped allow/exception in PR1 gate | **Recommended short-term** |
| O3: Remove/disable chain-inducing plugin(s) | Medium-to-high (if chain removed) | Medium (feature/security behavior change risk) | Medium | Medium | Could restore strict block if finding removed | Conditional backup option |
| O4: Fork/patch transitive deps for compatibility | High if delivered correctly | Medium-high (maintenance/fork drift) | High | Medium-long | Keeps strict block once merged | Recommended only if O5 stalls |
| O5: Upgrade to upstream Caddy/plugin versions that naturally include fixed chain | High (clean long-term fix) | Medium (upgrade regression risk) | Medium | Medium | Best path to remove exception and keep block | **Recommended medium-term target** |
| O6: Keep current state with no formal exception policy | Low (unbounded accepted risk) | Low immediate, high governance risk | Low | Immediate | Undermines CI policy consistency | **Not recommended** |

## PR1 Gate Handling Recommendation (Block vs Temporary Exception)

### Default posture

- **Block on High/Critical remains the default policy.**

### Temporary exception criteria (all required)

Use a temporary exception only if all conditions below are met:

1. **Attribution evidence** proves finding is transitive in `/usr/bin/caddy`, not direct app module (`backend` no direct `nebula` dependency).
2. **Reproduction evidence** shows attempted fixed upgrade path currently breaks build (with retained logs).
3. **Reachability assessment evidence** shows no confirmed direct runtime exploit path in Charon configuration (stated as confidence, not certainty).
4. **Compensating controls** are documented and active.
5. **Expiry and owner** are explicit (for example 30 days, named maintainer).
6. **Upstream tracking** issue(s) and review cadence are active.

### Evidence package required to justify exception

- Fresh scan artifact showing exact GHSA finding and location.
- Backend `go mod why` output showing no direct dependency.
- Build logs from attempted `nebula@1.10.3` path showing current incompatibility.
- Runtime/config inspection outputs used for reachability assessment.
- Signed-off exception document with expiry, owner, and revocation triggers.

### Revocation triggers (exception automatically invalid)

- Upstream compatible version is available and build passes in test branch.
- New exploitability evidence indicates practical Charon runtime exposure.
- Exception expires without renewed approval and updated evidence.

## Recommended Path

- **Short-term (PR1):** apply **O2** (time-boxed temporary exception) with strict evidence package and compensating controls.
- **Medium-term (next engineering slice):** execute **O5** as primary remediation path (upstream-compatible upgrade), with **O4** as fallback if upstream timelines are unacceptable.
- Keep the CI security posture intact by treating this as a narrowly scoped governance exception, not a policy downgrade.

## Local Validation Checklist for Reachability/Exploitability Context

These checks help estimate practical risk and verify assumptions. They do **not** prove non-exploitability.

1. Confirm finding attribution to binary/package/version/location.
2. Confirm direct backend dependency absence.
3. Confirm active Caddy modules and handlers in running environment.
4. Confirm whether relevant feature paths/configurations are enabled in deployment.
5. Attempt fixed-version build path and preserve failure evidence.
6. Re-run scans after any dependency/build-chain change.
7. Reassess exception validity on each CI security scan cycle.

## Notes

- As of the testing on 2026-02-19, just updating nebula to `1.10.3` in the Dockerfile causes build failures due to upstream incompatibilities, which supports the attribution and reproduction evidence for the temporary exception path.
- The conflict between `smallstep/certificates` and `nebula` API changes is a known issue in the ecosystem, which adds external validity to the hypothesis about the dependency chain.
- Will need to monitor upstream releases of `smallstep/certificates` and `Caddy` for compatible versions that allow upgrading `nebula` without breaking builds.
- Current `smallstep/certificates` version is `v0.29`. Will try nebula `1.10.3` update again once `smallstep/certificates` `v0.30+` is released.
