# QA / Security Report: chore/lint-full-config-1391

Branch HEAD 0b923ef9, 10 commits on a51d9eb4 (107 files, +860/-830). Date 2026-09-29. Scope: chore(lint), no new feature surface.

## Verdict: PASS (one non-blocking note)

| Check | Result |
|---|---|
| Backend `go test ./... -count=1` | 38 packages ok, 0 fail (about 9,079 passing test events) |
| Race: `routes -run ApplyInitialCaddyConfig -race -count=3` | ok |
| Backend coverage gate (`go-test-coverage.sh`) | statements 92.2%, line 88.8% (gate 87%): met |
| Agent `build` / `vet` / `test` | clean; cert, leash, muzzle, protocol ok |
| golangci-lint full config v2.14.0, backend | 0 issues |
| golangci-lint full config v2.14.0, agent | 0 issues |
| `make lint-fast` | 0 issues (backend + agent) |
| `lefthook run pre-commit --all-files` | all hooks pass (incl. semgrep: 0 findings, 367 rules, 1735 files) |
| GORM scan `--check` | PASSED, 0 issues (1 suppressed informational) |
| Frontend type-check / build | pass / pass |
| Frontend Vitest with coverage (all files) | 282 files passed; 3530 passed, 4 skipped, 2 todo; includes Security page + useSecurity tests |
| Frontend coverage | statements 90.05%, branches 83.38%, functions 87.9%, lines 91.21% |

## Patch coverage (`scripts/local-patch-report.sh`, baseline origin/development)

Artifacts exist: `test-results/local-patch-report.md` and `.json`.

- Overall 89.8% (114/127): below the local strict 90.0% threshold by 0.2 points (warn; script exit 1 in strict mode). This is non-blocking: backend 88.5% (100/113) and frontend 100% (14/14) both pass their 85% thresholds; agent has 0 changed instrumented lines.
- Uncovered changed lines, all in error or log branches:
  - `backend/cmd/api/main.go` 178
  - `proxy_host_handler.go` 911, 927, 1062, 1094
  - `crowdsec_handler.go` 1773-1774 (testKeyAgainstLAPI body-close error log), 2635, 2667 (renamed `validationErrs` appends)
  - `backup_restore_safe.go` 556-557 (`readManifestEntry` open-error return)
  - `routes.go` 65-66 (the "Failed to apply initial Caddy config" error log in `applyInitialCaddyConfig`)
- hub_sync.go, leash.go and the ExportConfig symlink skip have no uncovered changed lines.
- Optional follow-up: add tests for `readManifestEntry` open error and the `ApplyConfig` error path to reach 90%.

## Security review of `git diff a51d9eb4..HEAD -- backend agent`

- hub_sync `extractTarGz`: the prefix check is replaced by `filepath.Rel` containment (rejects `..` and `../`), and `Perm()` strips setuid/setgid/sticky bits from archive modes. This is stricter than before and fixes the old sibling-prefix bypass. No regression found.
- ExportConfig: symlinks are skipped (with a sanitized log) before `os.Open`. This closes symlink-escape reads. No regression.
- routes `applyInitialCaddyConfig`: now honors the passed ctx (returns on cancel, re-checks ctx before ApplyConfig), and the 30s/1s timing is unchanged. This is better than the prior `context.Background()` goroutine, and the race tests are clean.
- leash.go: the 101 response body is closed after reading the `X-Orthrus-Write-Enabled` header, and the fail-closed semantics are preserved.
- backup_restore_safe: extracted `readManifestEntry`, which fixes a defer-in-loop; behavior is the same.
- LAPI key validation: the body is closed per attempt, and logs still use `maskAPIKey`. No secrets are logged.
- Suppressions: 56 `nolint` additions in the diff (plus one removed `#nosec` pair converted). Non-test ones are 15: G204 on fixed or operator-configured binaries ("cscli", caddy path, git in a dev tool), G703/G304 on operator-configured env paths or paths contained by the Rel check, G117 for secrets encrypted before persisting, and G115 with a guarded bound. Each has an accurate justification. The remaining ones are test fixtures (dummy tokens, G101). No suppression masks a real vulnerability.
- Secrets: the grep of added lines shows only test fixtures and the masked-key logging. No credentials added.
- Commit messages: no `(security)` scope, and no session IDs or claude.ai/code links (grep empty).

## Not run (by instruction or per CLAUDE.md)

- E2E / Playwright and `docker-rebuild-e2e`: the user has not approved touching Docker on this host (live containers, dual daemons). Left to CI.
- CodeQL and Trivy: deferred to CI, because this is a chore(lint) change with no new feature surface (CLAUDE.md DoD step 3). CI runs both on every PR.
- No live-infrastructure access, no branch switch, no push and no commit; this report is left untracked.

## Notes

- The npm `pretype-check` and `pretest:coverage` hooks run `npm ci`, which touches only node_modules. `git status` was clean at the last check.
- `scripts/frontend-test-coverage.sh` hit my 590s cap at the wrapper level (its output is sparse). I completed the same `vitest run --coverage` (the same command it invokes) as a blocking job with polling; exit 0.
