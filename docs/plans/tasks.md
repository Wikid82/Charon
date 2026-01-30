# Tasks - Dependency Digest Tracking Plan

## Phase 2 - Pinning & Verification Updates

- [x] Pin `dlv` and `xcaddy` versions in Dockerfile.
- [x] Add checksum verification for CrowdSec fallback tarball.
- [x] Add checksum verification for GeoLite2 database download.
- [x] Pin CI compose images by digest.
- [x] Default Playwright CI compose to workflow digest output with tag override for local runs.
- [x] Pin whoami test service image by digest in docker-build workflow.
- [x] Propagate nightly image digest to smoke tests and scans.
- [x] Pin `govulncheck` and `gopls` versions in scripts.
- [x] Add Renovate regex managers for pinned tool versions and go.work.

## Follow-ups

- [ ] Add policy linting to detect unpinned tags in CI-critical files.
- [ ] Update security documentation for digest policy and exceptions.
