# QA Report

Branch: `fix/localhost-allowance-followups` (4 commits on `development`)

## Scope

Hardening outbound client configuration in the update checker and hub sync, removing unused LAPI helpers. Backend and documentation changes only (plus a one-line addition to `docs/troubleshooting/crowdsec.md`).

## Results

| Gate | Result | Notes |
|---|---|---|
| `go build ./...` | PASS | |
| Local patch coverage preflight | PASS | 18/18 changed lines covered (100%); artifacts in `test-results/` |
| Backend coverage (`scripts/go-test-coverage.sh`) | PASS | 92.4% statements, 89.5% lines; gate met |
| `lefthook run pre-commit --all-files` | PASS | All hooks green, Semgrep 0 findings |
| `make lint-fast` | PASS | 0 issues (backend and agent) |
| Race tests: `services`, `crowdsec` | PASS | `-race -count=1`, all packages ok |
| `go test ./internal/api/...` | PASS | all packages ok |
| GORM scan | N/A | No models, queries or migrations touched |
| Playwright / frontend gates | N/A | Backend and docs only |
| CodeQL / Trivy | Deferred to CI | Fix-scoped change |
| Docs | PASS | Troubleshooting note is accurate; `docs-site/docs/` untouched |

## Verdict

PASS. No blocking issues.
