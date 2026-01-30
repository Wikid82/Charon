# Dependency Digest Tracking Plan: Nightly Build Supply-Chain Hardening

**Version:** 1.0
**Status:** Research Complete - Phase 2 In Progress
**Priority:** HIGH
**Created:** 2026-01-30
**Source:** Nightly build readiness review

---

## Executive Summary

The nightly build pipeline is wired and waiting; now the supply chain needs a sharper edge. This plan catalogs every dependency used by the nightly workflow and its supporting build paths, highlights those not tracked by digest or checksum, and lays out a phased strategy to lock them down. The objective is simple: when the nightly build wakes up, it should pull only what we intended—no silent drift, no invisible updates, and no mystery bytes.

---

## Goals

1. **Digest-Tracked Dependencies**: Ensure all container images and external artifacts used in nightly build paths are pinned by digest or verified by checksum.
2. **Repeatable Nightly Builds**: Make the nightly build reproducible by eliminating unpinned tags and `@latest` installs.
3. **Clear Ownership**: Centralize digest updates via Renovate where feasible.
4. **Minimal Change Surface**: Only adjust files necessary for dependency integrity.

## Non-Goals

- Redesigning the nightly workflow logic.
- Changing release tagging or publishing conventions.
- Reworking the Docker build pipeline beyond dependency pinning.

---

## Research Inventory (Current State)

### Workflows

- Nightly workflow: [.github/workflows/nightly-build.yml](.github/workflows/nightly-build.yml)
- Docker build workflow: [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)
- Playwright workflow (nightly test support): [.github/workflows/playwright.yml](.github/workflows/playwright.yml)

### Docker & Compose

- Runtime image build: [Dockerfile](Dockerfile)
- Compose (E2E CI): [.docker/compose/docker-compose.playwright-ci.yml](.docker/compose/docker-compose.playwright-ci.yml)
- Compose (primary): [.docker/compose/docker-compose.yml](.docker/compose/docker-compose.yml)
- Compose (dev): [.docker/compose/docker-compose.dev.yml](.docker/compose/docker-compose.dev.yml)
- Compose (remote): [.docker/compose/docker-compose.remote.yml](.docker/compose/docker-compose.remote.yml)

### Scripts & Tooling

- Security scan helper: [scripts/security-scan.sh](scripts/security-scan.sh)
- Local Go installer: [scripts/install-go-1.25.6.sh](scripts/install-go-1.25.6.sh)
- Go version updater skill: [.github/skills/utility-update-go-version-scripts/run.sh](.github/skills/utility-update-go-version-scripts/run.sh)
- Renovate rules: [.github/renovate.json](.github/renovate.json)

---

## Findings: Dependencies Not Yet Tracked by Digest/Checksum

### Dependency Table (Phase 1 Requirement)

| File path | Dependency | Current pin state | Target pin method |
| --- | --- | --- | --- |
| .docker/compose/docker-compose.playwright-ci.yml | crowdsecurity/crowdsec:latest | Tag `latest` | Tag + digest (Renovate-managed) |
| .docker/compose/docker-compose.playwright-ci.yml | mailhog/mailhog:latest | Tag `latest` | Tag + digest (Renovate-managed) |
| .docker/compose/docker-compose.playwright-ci.yml | CHARON_E2E_IMAGE (charon:e2e-test) | Tag only | Default to workflow digest output; allow tag override |
| .docker/compose/docker-compose.remote.yml | alpine/socat | Tagless (defaults to latest) | Tag + digest (Renovate-managed) |
| .docker/compose/docker-compose.yml | ghcr.io/wikid82/charon:latest | Tag `latest` | Tag + digest, allow local override |
| .docker/compose/docker-compose.dev.yml | ghcr.io/wikid82/charon:dev | Tag only | Tag + digest, allow local override |
| .github/workflows/docker-build.yml | traefik/whoami | Tagless (defaults to latest) | Tag + digest (Renovate-managed) |
| Dockerfile (backend-builder) | dlv@latest | Go tool `@latest` | Pinned version (Renovate-managed) |
| Dockerfile (caddy-builder) | xcaddy@latest | Go tool `@latest` | Pinned version (Renovate-managed) |
| Dockerfile (crowdsec-fallback) | crowdsec-release.tgz | No checksum | SHA256 verification |
| Dockerfile (final runtime) | GeoLite2-Country.mmdb | No checksum | SHA256 verification |
| scripts/security-scan.sh | govulncheck@latest | Go tool `@latest` | Pinned version (Renovate-managed) |
| scripts/install-go-1.25.6.sh | gopls@latest | Go tool `@latest` | Pinned version (Renovate-managed) |
| .github/skills/utility-update-go-version-scripts/run.sh | golang.org/dl/go${REQUIRED_VERSION}@latest | Allowed exception | Exception + compensating controls |

### A. Container Images (Compose & Workflows)

1. **E2E Playwright Compose**
	- File: [.docker/compose/docker-compose.playwright-ci.yml](.docker/compose/docker-compose.playwright-ci.yml)
	- Images:
	  - `crowdsecurity/crowdsec:latest`
	  - `mailhog/mailhog:latest`
	  - `CHARON_E2E_IMAGE_DIGEST` from workflow output (default)
	  - `CHARON_E2E_IMAGE` tag override for local runs
2. **Remote Docker socket proxy**
	- File: [.docker/compose/docker-compose.remote.yml](.docker/compose/docker-compose.remote.yml)
	- Image: `alpine/socat`
3. **Dev and prod compose images**
	- File: [.docker/compose/docker-compose.yml](.docker/compose/docker-compose.yml)
	- Image: `ghcr.io/wikid82/charon:latest`
	- File: [.docker/compose/docker-compose.dev.yml](.docker/compose/docker-compose.dev.yml)
	- Image: `ghcr.io/wikid82/charon:dev`
4. **Workflow test service image**
	- File: [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)
	- Image: `traefik/whoami` (tagless, latest by default)

### B. Dockerfile External Downloads & Unpinned Go Installs

1. **Go tools installed with @latest**
	- Stage: `backend-builder`
	- File: [Dockerfile](Dockerfile)
	- Tool: `github.com/go-delve/delve/cmd/dlv@latest`
2. **Caddy builder uses @latest for xcaddy**
	- Stage: `caddy-builder`
	- File: [Dockerfile](Dockerfile)
	- Tool: `github.com/caddyserver/xcaddy/cmd/xcaddy@latest`
3. **CrowdSec fallback download without checksum**
	- Stage: `crowdsec-fallback`
	- File: [Dockerfile](Dockerfile)
	- Artifact: `crowdsec-release.tgz` (no sha256 verification)
4. **GeoLite2 database download without checksum**
	- Stage: final runtime
	- File: [Dockerfile](Dockerfile)
	- Artifact: `GeoLite2-Country.mmdb` (raw GitHub download)

### C. Scripts Installing Go Tools with @latest

1. [scripts/security-scan.sh](scripts/security-scan.sh)
	- `golang.org/x/vuln/cmd/govulncheck@latest`
2. [scripts/install-go-1.25.6.sh](scripts/install-go-1.25.6.sh)
	- `golang.org/x/tools/gopls@latest`
3. [.github/skills/utility-update-go-version-scripts/run.sh](.github/skills/utility-update-go-version-scripts/run.sh)
	- `golang.org/dl/go${REQUIRED_VERSION}@latest`
	- **Exception candidate:** Go toolchain installer (requires `@latest` for versioned shim)

---

## Requirements (EARS Notation)

1. WHEN the nightly workflow executes, THE SYSTEM SHALL use container images pinned by digest for any external service images it runs (e.g., `traefik/whoami`).
2. WHEN a Docker Compose file is used in CI contexts, THE SYSTEM SHALL pin all third-party images by digest or provide a checksum verification step.
3. WHEN the Dockerfile downloads external artifacts, THE SYSTEM SHALL verify them with checksums or pinned release asset digests.
4. WHEN Go tools are installed in build stages or scripts, THE SYSTEM SHALL pin a specific semantic version instead of `@latest`.
5. WHEN Renovate is configured, THE SYSTEM SHALL be able to update pinned digests and versioned tool installs without manual drift.
6. IF a dependency cannot be pinned by digest (e.g., variable build outputs), THEN THE SYSTEM SHALL document the exception and the compensating control (checksum, SBOM, or provenance).
7. WHEN the Go toolchain shim is installed via `golang.org/dl/goX.Y.Z@latest`, THE SYSTEM SHALL allow this as an explicit exception and SHALL enforce compensating controls (pinned `goX.Y.Z`, checksum or provenance validation for the installed toolchain, and Renovate visibility).
8. WHEN CI builds a self-hosted image, THE SYSTEM SHALL capture the resulting digest and propagate it to downstream jobs and tests as an immutable reference.

---

## Design Decisions (Draft)

1. **Digest Pinning Strategy**
	- Use `image: name:tag@sha256:...` for compose and workflow `docker run` usage when possible.
	- For the self-built nightly image, keep the tag for readability but capture and propagate the digest to downstream verification steps.
	- Use tag+digest pairs consistently to preserve human-readable tags while enforcing immutability.
2. **Checksum Verification for Artifacts**
	- Add `ARG` + `SHA256` environment variables for CrowdSec tarball and GeoLite2 DB.
	- Verify downloads in Dockerfile with `sha256sum -c`.
	- GeoLite2 checksum provenance: prefer MaxMind-provided SHA256 from the official GeoLite2 download API (license-key gated) and document the applicable GeoLite2 EULA/licensing source.
3. **Version Pinning for Go Tools**
	- Replace `@latest` installs with pinned versions and Renovate annotations.
4. **Exception: `golang.org/dl/goX.Y.Z@latest`**
	- Allow the go toolchain shim to use `@latest` for the specific `goX.Y.Z` target version.
	- Compensating controls: ensure `REQUIRED_VERSION` is pinned, verify the resulting toolchain provenance (Go checksum database or release manifest), and add Renovate monitoring for `REQUIRED_VERSION` updates.

---

## Planned Updates (Files & Components)

### Workflows

1. **Nightly Build**
	- File: [.github/workflows/nightly-build.yml](.github/workflows/nightly-build.yml)
	- Component: `test-nightly-image` job
	- Capture the nightly image digest from the build step and export it as a job output (e.g., `nightly_image_digest`).
	- Propagate the digest to downstream jobs via `needs.<job>.outputs.nightly_image_digest` and use `image: tag@sha256:...` where possible.
	- Record the tag+digest pair in job summary for auditability.

2. **Docker Build Workflow**
	- File: [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)
	- Component: `Run Upstream Service (whoami)` step
	- Replace `traefik/whoami` with `traefik/whoami:tag@sha256:...` and document digest ownership.
	- Capture the built image digest from buildx output (or `docker buildx imagetools inspect`) and expose it as a workflow output for reuse in later jobs.

### Dockerfile

1. **Stage: backend-builder**
	- Replace `dlv@latest` with a pinned version (e.g., `@v1.x.y`) tracked by Renovate.
2. **Stage: caddy-builder**
	- Replace `xcaddy@latest` with pinned version; add Renovate directive.
3. **Stage: crowdsec-fallback**
	- Add checksum verification for `crowdsec-release.tgz` using `sha256sum`.
4. **Stage: final runtime**
	- Add checksum verification for GeoLite2 DB, preferably from a fixed release artifact or vendor checksum list.
	- Document GeoLite2 checksum provenance in the Dockerfile or plan (MaxMind GeoLite2 download API + EULA source).

### Compose Files

1. **E2E CI Compose**
	- File: [.docker/compose/docker-compose.playwright-ci.yml](.docker/compose/docker-compose.playwright-ci.yml)
	- Pin `crowdsecurity/crowdsec`, `mailhog/mailhog` by digest.
	- Default to `CHARON_E2E_IMAGE_DIGEST` from workflow outputs with `CHARON_E2E_IMAGE` tag override for local runs.
2. **Remote Socket Proxy**
	- File: [.docker/compose/docker-compose.remote.yml](.docker/compose/docker-compose.remote.yml)
	- Pin `alpine/socat` by digest.
3. **Dev & Prod Compose**
	- File: [.docker/compose/docker-compose.yml](.docker/compose/docker-compose.yml)
	- File: [.docker/compose/docker-compose.dev.yml](.docker/compose/docker-compose.dev.yml)
	- Decide whether to:
	  - Keep tags for local convenience, OR
	  - Provide commented tag+digest options and Renovate-managed examples.

### Renovate Configuration

1. **Enable Digest Pinning for Docker Compose**
	- File: [.github/renovate.json](.github/renovate.json)
	- Ensure docker digest pinning is enabled for compose images and tag+digest pairs are preserved.
2. **Add Custom Managers for Go Tools**
	- Track pinned versions for `dlv` and `xcaddy` in Dockerfile.
	- Track `REQUIRED_VERSION` for `golang.org/dl/goX.Y.Z@latest` exception to keep the target version current.

---

## Review Notes for Supporting Files

1. **.gitignore**
	- No immediate changes required. If a new dependency lock manifest is introduced (e.g., `dependency-digests.json`), ensure it is not ignored.
2. **.dockerignore**
	- No blocking issues found. Consider excluding any new digest manifest artifacts only if they are not required in image builds.
3. **codecov.yml**
	- No changes required for dependency tracking. Coverage ignore patterns are acceptable for this effort.
4. **Dockerfile**
	- Changes required (pin `@latest` tools, verify external downloads with checksums).

---

## Risks & Mitigations

1. **Digest Rotation**
	- Risk: pinned digests require updates.
	- Mitigation: Renovate updates digests on schedule.
2. **Checksum Source Reliability**
	- Risk: upstream artifacts lack stable checksum URLs.
	- Mitigation: use release checksums or vendor-provided signed assets; document exceptions.
3. **Local Developer Friction**
	- Risk: digest pinning may slow dev iteration.
	- Mitigation: keep optional tag paths or override vars for local use.

---

## Implementation Plan (Phased, Minimal Requests)

### Phase 1 — Inventory & Decision Map (Single Request)

**Objective:** Establish the canonical list of digest-tracked dependencies and confirm which files will be modified.

**Status:** Complete (dependency table added; dev/prod compose pinning decision set)

**Actions:**
- Create a dependency table in `docs/plans/current_spec.md` (this file) with:
  - File path
  - Dependency name
  - Current pin state (tag, digest, checksum, latest)
  - Target pin method
- Decide whether dev compose files are pinned or left flexible with documented overrides.
	- **Owner:** DevOps
	- **Decision Date:** 2026-01-30
	- **Decision:** Pin dev/prod compose images with tag+digest defaults while allowing local overrides via env vars.

**Deliverables:**
- Finalized dependency inventory and pinning policy.

### Phase 2 — Pinning & Verification Updates (Single Request)

**Objective:** Apply digest pinning, version pinning, and checksum verification changes across build and CI surfaces.

**Actions:**
- Update Dockerfile stages:
  - Pin `dlv` and `xcaddy` versions.
  - Add checksum verification for GeoLite2 and CrowdSec tarball.
- Update compose images to digest form where required.
- Update workflow `docker run` test image to digest form.
- Update Renovate config to keep digests and Go tool versions fresh.

**Deliverables:**
- All dependencies in nightly path pinned or checksum-verified.

### Phase 3 — Validation & Guardrails (Single Request)

**Objective:** Ensure policy compliance and prevent regression.

**Actions:**
- Add documentation in `docs/` or `SECURITY.md` describing digest policy.
- Verify SBOM generation still succeeds with pinned dependencies.
	- Add a lint check (required) to detect unpinned tags and `@latest` in CI-critical files.
		- Scope files:
		  - `.github/workflows/*.yml`
		  - `.docker/compose/*.yml`
		  - `Dockerfile`
		  - `scripts/*.sh`
		- Patterns to flag (non-exhaustive):
		  - `:latest` image tags (except explicitly documented local-only compose examples)
		  - `@latest` in Go tool installs (except `golang.org/dl/goX.Y.Z@latest`)
		  - Docker image references lacking `@sha256:` in CI/test contexts

**Deliverables:**
- Policy documentation and validation evidence.

---

## Acceptance Criteria

1. All external images referenced by CI workflows or CI compose files are pinned by digest.
2. All Dockerfile external downloads are checksum-verified.
3. No `@latest` installs remain in Dockerfile or CI-critical scripts without explicit exception.
4. The Go toolchain shim exception is documented with compensating controls and Renovate visibility.
5. CI workflows capture and propagate self-built image digests for downstream usage.
6. Renovate can update digests and pinned tool versions automatically.
7. Documentation clearly states which files must use digests and why.

---

## Handoff Contract (JSON)

```json
{
	"plan": "Dependency Digest Tracking Plan: Nightly Build Supply-Chain Hardening",
	"phase": "Phase 1 — Inventory & Decision Map",
	"status": "In Progress",
	"owner": "DevOps",
	"handoffTargets": ["Backend_Dev", "DevOps", "QA_Security"],
	"decisionRequired": "Dev compose pinning policy",
	"decisionDate": "2026-01-30",
	"dependencies": [
		".github/workflows/nightly-build.yml",
		".github/workflows/docker-build.yml",
		".docker/compose/docker-compose.playwright-ci.yml",
		".docker/compose/docker-compose.yml",
		".docker/compose/docker-compose.dev.yml",
		".docker/compose/docker-compose.remote.yml",
		"Dockerfile",
		".github/renovate.json",
		"scripts/security-scan.sh",
		"scripts/install-go-1.25.6.sh",
		".github/skills/utility-update-go-version-scripts/run.sh"
	],
	"notes": "Digest pinning and checksum verification must align with Acceptance Criteria and Renovate ownership."
}
```

---

## Handoff Notes

Once this plan is accepted, delegate implementation to `DevOps` and `Backend_Dev` for Dockerfile and workflow changes, and `QA_Security` for validation and policy checks.
