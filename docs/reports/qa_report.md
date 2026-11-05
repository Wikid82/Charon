# QA Report: WAF status counts a host's own WAF handler (GH #1840)

Branch `fix/waf-status-own-handler`, commit 693859d7a. Files: `backend/internal/caddy/config.go`, `backend/internal/caddy/config_waf_own_handler_test.go`.

## Gates

| Gate | Result | Detail |
|---|---|---|
| Backend coverage (`scripts/go-test-coverage.sh`) | PASS | Statement 93.2%, line 92.1% (gate 87% line, 85% minimum), zero failing packages |
| Local patch coverage (`scripts/local-patch-report.sh`) | PASS | Overall/backend 40 of 40 changed lines covered (100%); frontend and agent 0 changed lines. Artifacts `test-results/local-patch-report.md` and `.json` exist |
| `go build ./...` (backend) | PASS | |
| `make lint-staticcheck-only` | PASS | backend 0 issues, agent 0 issues |
| GORM scan | N/A | No model or migration changes |
| CodeQL / Trivy | Deferred to CI | Fix-scoped change |
| Container repro | SKIPPED | A full image build (Caddy with Coraza) is not cheap, and `charon:local` is not the new code. Behavior is covered by the unit tests in `config_waf_own_handler_test.go` instead |

## Diff review

- Refactor equivalence: PASS. `rulesetIncludeDirective` returns `Include <path>` only for a non-empty string `ruleset_name` that resolves to a non-empty path. A non-string name, an empty name, a missing key and a nil `rulesetPaths` all return "", which matches the old nested ifs. `delete(v, "ruleset_name")` is now unconditional. That is the same outcome as before, because deleting an absent key is a no-op and the old code deleted whenever the key existed. This holds in both the single-object and array branches. The generated config is unchanged.
- Own handler counted protected: PASS. `hostHasOwnWAFHandler` reads only the host, and no second global handler is appended. Only the `protected` flag passed to `report.WAF.record` and the `wafUnprotectedHosts` increment change.
- Unresolvable own handler still unprotected: PASS. A `waf` handler with blank or absent `directives` and an unresolved or missing `ruleset_name` returns false. Invalid JSON and an empty `advanced_config` also return false.
- Minor, non-blocking: if a handler has both explicit `directives` and a resolving `ruleset_name`, the generator overwrites `directives` with the Include. The count is true either way.

## Blockers

None.
