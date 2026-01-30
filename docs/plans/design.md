# Design - Dependency Digest Tracking Plan

## Architecture Overview

This change set hardens the nightly build and CI surfaces by pinning container images to digests, pinning Go tool installs to fixed versions, and verifying external artifact downloads with SHA256 checksums.

## Data Flow

1. Build workflows produce an image digest via buildx and expose it as a job output.
2. Downstream jobs and tests consume the digest to pull and run immutable images.
3. CI compose files reference third-party images as `name:tag@sha256:digest`.
4. Dockerfile download steps verify artifacts using SHA256 checksums before extraction.

## Interfaces

- GitHub Actions job outputs:
  - `build-and-push-nightly.outputs.digest`
- Compose overrides:
  - `CHARON_E2E_IMAGE_DIGEST` (preferred, digest-pinned from workflow output)
  - `CHARON_E2E_IMAGE` (tag-based local override)
  - `CHARON_IMAGE`, `CHARON_DEV_IMAGE` (local override for tag-only usage)

## Error Handling

- Dockerfile checksum verification uses `sha256sum -c` to fail fast on mismatches.
- CI workflows rely on digest references; failure to resolve a digest fails the job early.

## Implementation Considerations

- Tag+digest pairs preserve human-readable tags while enforcing immutability.
- Renovate regex managers track pinned versions for Go tools and go.work toolchain version.
- The Go toolchain shim uses `@latest` by exception and reads the pinned version from go.work.
