# Requirements - Dependency Digest Tracking Plan

## EARS Requirements

1. WHEN the nightly workflow executes, THE SYSTEM SHALL use container images pinned by digest for any external service images it runs.
2. WHEN a Docker Compose file is used in CI contexts, THE SYSTEM SHALL pin all third-party images by digest or provide a checksum verification step.
3. WHEN the Dockerfile downloads external artifacts, THE SYSTEM SHALL verify them with checksums.
4. WHEN Go tools are installed in build stages or scripts, THE SYSTEM SHALL pin a specific semantic version instead of `@latest`.
5. WHEN Renovate is configured, THE SYSTEM SHALL be able to update pinned digests and versioned tool installs without manual drift.
6. IF a dependency cannot be pinned by digest, THEN THE SYSTEM SHALL document the exception and compensating controls.
7. WHEN the Go toolchain shim is installed via `golang.org/dl/goX.Y.Z@latest`, THE SYSTEM SHALL allow this as an explicit exception and SHALL enforce compensating controls.
8. WHEN CI builds a self-hosted image, THE SYSTEM SHALL capture the resulting digest and propagate it to downstream jobs and tests.
9. WHEN CI starts the E2E compose stack, THE SYSTEM SHALL default to a digest-pinned image from workflow outputs while allowing a tag override for local runs.
