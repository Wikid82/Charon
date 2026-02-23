# Nebula v1.10.3 Upgrade Compilation Failure Analysis

Date: 2026-02-10

## Scope

This report analyzes the Caddy build-time compilation failures observed when forcing github.com/slackhq/nebula to v1.10.3 in the Docker build stage and documents options and a recommendation. No fixes are implemented here.

## Evidence Sources

- Caddy builder dependency overrides in [Dockerfile](Dockerfile)
- Workspace pin for nebula in [go.work.sum](go.work.sum)
- Security scan context and prior remediation plan in [docs/reports/qa_report.md](docs/reports/qa_report.md) and [docs/plans/dod_remediation_spec.md](docs/plans/dod_remediation_spec.md)
- Caddy upgrade notes indicating prior smallstep/certificates changes in [CHANGELOG.md](CHANGELOG.md)

## 1. Exact Error Messages

### Error Output

Build failed in the Caddy builder stage during `go build` for the xcaddy-generated module. Compiler output:

```
# github.com/smallstep/certificates/authority/provisioner
/go/pkg/mod/github.com/smallstep/certificates@v0.30.0-rc2/authority/provisioner/nebula.go:51:18: undefined: nebula.NebulaCAPool
/go/pkg/mod/github.com/smallstep/certificates@v0.30.0-rc2/authority/provisioner/nebula.go:67:37: undefined: nebula.NewCAPoolFromBytes
/go/pkg/mod/github.com/smallstep/certificates@v0.30.0-rc2/authority/provisioner/nebula.go:306:76: undefined: nebula.NebulaCertificate
/go/pkg/mod/github.com/smallstep/certificates@v0.30.0-rc2/authority/provisioner/nebula.go:325:19: undefined: nebula.UnmarshalNebulaCertificate
# github.com/hslatman/ipstore
/go/pkg/mod/github.com/hslatman/ipstore@v0.3.1-0.20241030220615-1e8bac326f71/ipstore.go:83:23: s.table.GetAndDelete undefined (type *bart.Table[T] has no field or method GetAndDelete)
```

### Failing Packages/Files and Missing APIs

- github.com/smallstep/certificates/authority/provisioner
  - /go/pkg/mod/github.com/smallstep/certificates@v0.30.0-rc2/authority/provisioner/nebula.go:51:18
    - missing: nebula.NebulaCAPool
  - /go/pkg/mod/github.com/smallstep/certificates@v0.30.0-rc2/authority/provisioner/nebula.go:67:37
    - missing: nebula.NewCAPoolFromBytes
  - /go/pkg/mod/github.com/smallstep/certificates@v0.30.0-rc2/authority/provisioner/nebula.go:306:76
    - missing: nebula.NebulaCertificate
  - /go/pkg/mod/github.com/smallstep/certificates@v0.30.0-rc2/authority/provisioner/nebula.go:325:19
    - missing: nebula.UnmarshalNebulaCertificate

- github.com/hslatman/ipstore
  - /go/pkg/mod/github.com/hslatman/ipstore@v0.3.1-0.20241030220615-1e8bac326f71/ipstore.go:83:23
    - missing: s.table.GetAndDelete (type *bart.Table[T] has no field or method GetAndDelete)

## 2. Affected Components

### smallstep/certificates

- Current override: v0.30.0-rc2 in the Caddy builder stage via [Dockerfile](Dockerfile)
- Previously referenced in the changelog as no longer needing a manual patch (historical v0.29.x noted in [CHANGELOG.md](CHANGELOG.md))
- Likely mismatch: Caddy (or a plugin) may still depend on the v0.29.x API surface, and the v0.30.0-rc2 API changes could break compilation
- Need from logs: exact missing symbols and their call sites

### ipstore

- Current override: v0.3.1-0.20241030220615-1e8bac326f71 in the Caddy builder stage via [Dockerfile](Dockerfile)
- Questioned API: GetAndDelete
- Likely consumer: github.com/hslatman/caddy-crowdsec-bouncer (same author as ipstore, included via xcaddy in [Dockerfile](Dockerfile))
- Alternative methods: unknown without API docs or logs; likely a change to ipstore’s store interface or method renaming

### Caddy core vs plugins

- Caddy core and plugins are built together in the xcaddy temporary module. The override failures are most likely in plugin packages because the Caddy core dependency graph is stable for v2.11.0-beta.2, while the overrides force newer versions.
- The most likely plugin impact is the CrowdSec bouncer module (github.com/hslatman/caddy-crowdsec-bouncer), given the ipstore override.

## 3. Options Analysis

### Option A: Patch affected code in Dockerfile build stage

- What code needs patching:
  - The generated xcaddy build module under /tmp/buildenv_* (temporary). This would involve applying sed or patch operations against the generated module source (Caddy core or plugin code) after `xcaddy build` but before `go build`.
- Complexity:
  - Likely moderate. A simple find/replace may work for API rename (for example, GetAndDelete to a new method), but API surface changes in smallstep/certificates could require more than a rename.
- Risk:
  - Medium to high. Patching generated third-party code introduces fragility and can break functionality if the semantic behavior changed.
- Maintainability:
  - Low. The patch is tied to transient xcaddy build output; any Caddy or plugin update can invalidate the patch.

### Option B: Find compatible dependency versions

- Goal:
  - Align versions so Caddy core and its plugins compile without patching generated source.
- Feasibility:
  - Potentially high if a compatible smallstep/certificates version exists that supports nebula v1.10.3 or if the nebula upgrade can be isolated to the dependency that pulls it.
- What to look for:
  - smallstep/certificates version compatible with Caddy v2.11.0-beta.2 or the plugin API set used in the xcaddy build
  - ipstore version that still provides GetAndDelete (if that is the failing method)
- Trade-offs:
  - Using older dependency versions may reintroduce known vulnerabilities or leave the nebula CVE unaddressed in the runtime image.

### Option C: Alternative approaches

- Exclude nebula from Caddy builder:
  - If nebula is only present in build-stage module metadata (not required for runtime), it may be possible to avoid pulling it into the build graph.
  - This depends on which plugin or dependency is bringing nebula in; logs are required to confirm.
- Use a Caddy release with nebula v1.10.3+ already pinned:
  - If upstream Caddy (or a specific plugin release) already pins nebula v1.10.3+, upgrading to that release would be cleaner than manual overrides.
- Swap the plugin:
  - If the dependency chain originates from a plugin that is not required, removing it or replacing it with a supported alternative avoids the nebula dependency.
  - This must be validated against the current Charon feature set (CrowdSec support suggests the bouncer plugin is required).

## 4. Recommendation

Recommended option: Option B first, with Option A as a short-term fallback.

Reasoning:
- The Dockerfile already applies dependency overrides; a compatible version alignment avoids source patching and reduces risk.
- It preserves maintainability by removing build-stage patching of third-party code.
- If version alignment is not possible, a narrow patch in the build stage can unblock the build, but should be treated as temporary.

Risk assessment:
- Medium. The primary risk is selecting older versions that eliminate compilation errors but reintroduce security findings or break runtime behavior.

Fallback plan:
- If version alignment fails, apply a temporary, minimal patch in the xcaddy build directory and track it with a dedicated changelog note and a follow-up task to remove it after upstream releases catch up.

## 5. Testing Plan

After any fix, validate the full Caddy build and runtime behavior:

- Build validation
  - Docker build of the Caddy builder stage succeeds without compilation errors
- Runtime validation
  - Caddy starts with all required modules enabled
  - Security stack middleware loads successfully (CrowdSec, WAF, ACL, rate limiting)
  - Core proxy flows work (HTTP/HTTPS, certificate issuance, DNS challenge)
- Specific endpoints/features
  - Emergency recovery port (2019) accessibility
  - Certificate issuance flows for ACME and DNS-01
  - CrowdSec bouncer behavior under known block/allow cases

## 6. Version Compatibility Test Results

### Research Summary

- smallstep/certificates releases at v0.30.0 or newer are limited to v0.30.0-rc1 and v0.30.0-rc2. Both rc tags (and master) pin nebula v1.9.7 and still reference the removed nebula APIs.
- ipstore latest tag is v0.3.0; main still calls GetAndDelete and pins bart v0.13.0.
- caddy-crowdsec-bouncer latest tag is v0.9.2 and depends on ipstore v0.3.0 (bart v0.13.0 indirect).

### Working Version Combination

None found. All tested approaches failed due to smallstep/certificates referencing removed nebula APIs and ipstore triggering a GetAndDelete mismatch. Logs were written to the requested locations.

### Build Command (Dockerfile Changes Tested)

- Approach A: add go get github.com/slackhq/nebula@v1.10.3 and go get github.com/smallstep/certificates@v0.30.0-rc2 before go mod tidy in the Caddy builder stage.
- Approach B: Approach A plus go get github.com/hslatman/ipstore@v0.3.0.
- Approach C: Approach A plus go get github.com/hslatman/caddy-crowdsec-bouncer@v0.9.2.

### Test Results

- Approach A: failed with undefined nebula symbols in smallstep/certificates and GetAndDelete missing in ipstore.
- Approach B: failed with the same nebula and GetAndDelete errors.
- Approach C: failed with the same nebula and GetAndDelete errors.

## Requested Missing Inputs

To complete Section 1 with exact compiler output and concrete API mismatches, provide the Caddy build log from the nebula v1.10.3 upgrade attempt (CI log or local Docker build output). This will enable precise file/package attribution and accurate API change mapping.

## 7. Decision and Path Forward

### Decision
Path 4 selected: Document as known issue and accept risk for nebula v1.9.7.

### Rationale
- High severity risk applies to components within our control; this is upstream dependency breakage
- Updating dependencies breaks CrowdSec bouncer compilation
- No compatible upstream versions exist as of 2026-02-10
- Loss of reliability outweighs theoretical vulnerability in a build-time dependency

### Next Steps
- Track upstream fixes per [docs/security/SECURITY-EXCEPTION-nebula-v1.9.7.md](../security/SECURITY-EXCEPTION-nebula-v1.9.7.md)
- Reassess if dependency chain updates enable nebula v1.10.3+ without build breakage
