## PR-1 Caddy Compatibility Matrix

- Date: 2026-02-23
- Candidate version: 2.11.1
- Scope: PR-1 compatibility slice only

## Promotion Rule (PR-1)

- Promotion-gating rows: Scenario A on linux/amd64 and linux/arm64
- Evidence-only rows: Scenario B and C

## Matrix Summary

| Scenario | Platform | Status | Reviewer Action |
| --- | --- | --- | --- |
| A | linux/amd64 | PASS | Required for promotion |
| A | linux/arm64 | PASS | Required for promotion |
| B | linux/amd64 | PASS | Evidence-only |
| B | linux/arm64 | PASS | Evidence-only |
| C | linux/amd64 | PASS | Evidence-only |
| C | linux/arm64 | PASS | Evidence-only |

## Decision

- Promotion gate: PASS
- Runtime default drift: None observed in PR-1
- Candidate path: Opt-in only

## Artifacts

- Matrix CSV: test-results/caddy-compat-closure/matrix-summary.csv
- Module inventories: test-results/caddy-compat-closure/module-inventory-*-go-version-m.txt
- Module listings: test-results/caddy-compat-closure/module-inventory-*-modules.txt
