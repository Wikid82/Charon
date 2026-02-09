# Local Scan Hygiene (CodeQL + Trivy)

This plan captures local scan-hygiene items that are not the SSRF remediation itself, but commonly cause CI-aligned local security tasks to fail due to generated artifacts or scanning scope.

## Goal

- Keep local CI-aligned tasks deterministic and aligned with CI behavior.
- Prevent generated artifacts (coverage, dist outputs, tool DBs) from being treated as source code during scans.

## CodeQL JS: prevent scanning generated artifacts

### Problem

Local CodeQL JS scans can fail if coverage/build artifacts exist on disk under `frontend/` (example: a finding under `frontend/coverage/lcov-report/...`).

### Plan

- Ensure generated artifacts are not treated as source:
  - Confirm `.gitignore` excludes `frontend/coverage/**` and other build outputs.
- Add a deterministic cleanup step in local CodeQL JS entrypoints:
  - Remove if present:
    - `frontend/coverage/`
    - `frontend/dist/`
    - `playwright-report/`
    - `test-results/`
    - `coverage/` (root-level, if present)

Likely scripts involved (verify current wiring before editing):

- [scripts/pre-commit-hooks/codeql-js-scan.sh](scripts/pre-commit-hooks/codeql-js-scan.sh)
- [.github/skills/security-scan-codeql-scripts/run.sh](.github/skills/security-scan-codeql-scripts/run.sh)

### Notes

- `.github/codeql/codeql-config.yml` already has `paths-ignore` entries for several generated paths (e.g., `frontend/coverage/**`, `frontend/dist/**`, `test-results/**`). Cleanup is still recommended because it protects local runs even if a given invocation does not consistently apply a config file.

## Trivy FS: exclude tool/cache databases from scan scope

### Problem

Trivy can scan non-project directories and produce noise or scanner errors when it traverses:

- local caches (`.cache/`, including Go module caches)
- CodeQL databases (`codeql-db-*`)
- agent outputs (`codeql-agent-results/`)

### Plan

- Update the local Trivy entrypoint to skip non-project directories using explicit `--skip-dirs` options.

Primary script:

- [.github/skills/security-scan-trivy-scripts/run.sh](.github/skills/security-scan-trivy-scripts/run.sh)

Suggested skip set (keep explicit; no globs):

- `.cache/`
- `codeql-db-go/`
- `codeql-db-js/`
- `my-codeql-db/`
- `codeql-agent-results/`
- `codeql-custom-queries-go/` (optional for noise/speed)
- `test-results/` (optional; only if it creates findings)

### Keep local behavior CI-aligned

- Ensure findings fail the scan without unnecessary noise:
  - Set `--exit-code 1`
  - Default severity threshold: `CRITICAL,HIGH` (allow override via `TRIVY_SEVERITY`)
- Prefer skip-dirs for non-project content; use ignorefiles only for true false positives.

## Repo hygiene follow-up (separate PR)

The repo root currently contains scan artifacts such as `codeql-results-*.sarif` and `trivy-*.txt`. Follow the repo structure guidance by moving these under `test-results/` and/or adding appropriate `.gitignore` entries.
