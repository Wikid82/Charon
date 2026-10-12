# Container Registry Prune

`.github/workflows/container-prune.yml` runs weekly (Sundays 03:00 UTC, or manually) and
prunes the `charon` and `orthrus` images on GHCR and Docker Hub using
`scripts/prune-ghcr.sh` and `scripts/prune-dockerhub.sh` (shared helpers in
`scripts/lib/prune-common.sh`).

## Retention policy

| Class | Matches | Retention |
| --- | --- | --- |
| Protected | `PROTECTED_REGEX`: semver (`1.2.3`, `v1.2.3`), `latest`, `main`, `develop` | Never pruned. On GHCR, a version with any protected tag is kept. |
| Per-build | `BUILD_TAG_REGEX`: `build-*`, `sha256-*` (SBOM/attestation), `development-<sha>`, `main-<sha>`, `development-<runid>`, bare 7-40 char hex | `BUILD_KEEP_DAYS` (default 7). Exempt from the keep-last-N shield, so the limit is absolute. |
| Everything else | `nightly-*`, `pr-*`, `dev`, `beta`, ... | `KEEP_DAYS` (default 30), except the `KEEP_LAST_N` (default 30) newest. |
| Untagged (GHCR) | versions with no tags | `KEEP_DAYS` when `PRUNE_UNTAGGED=true`. |

`workflow_dispatch` inputs: `keep_days`, `build_keep_days`, `keep_last_n`, `dry_run`.

## Verified deletes

Each delete checks the HTTP status. 2xx counts as deleted; 404 counts as already gone.
Anything else (for example 403) is logged as `delete FAILED http=<code>`, counted in
`TOTAL_FAILED`, and the script exits 1 after writing `prune-summary-*.env`. The workflow
summary shows a Failed column.

If Docker Hub deletes fail with 403, the `DOCKERHUB_TOKEN` access token lacks delete
rights; create a token with **Read, Write, Delete** permission.

## Known limitations

- Docker Hub untagged manifests are not pruned. Docker Hub's tags API never lists them and
  no documented API to enumerate or delete individual manifest digests could be verified
  (`/v2/repositories/{ns}/{repo}/images` and `/manifests/{digest}` return 404, and the
  published Hub API spec has neither).
- GHCR's versions API reports no size, so GHCR runs report counts only.
- Tests: `bats scripts/tests/prune-ghcr.bats scripts/tests/prune-dockerhub.bats` (stub `curl`,
  no network).
