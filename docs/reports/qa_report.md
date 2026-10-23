# QA and Security Report: fix/hub-content-verification

(Replaces the previous report.) Scope: 4 commits over `origin/development`: `168043d5` (fix(security): harden verification of downloaded hub content), `0862beec` (show the hub-updating message), `bdbecc3e` (docs), `07e5421d` (log/test/docs tidy). 14 files, +398/-14. Audited against SECURITY.md, the Definition of Done in CLAUDE.md, and `docs/plans/crowdsec_hardening_batch5_spec.md` section 3e. The spec file was not staged or committed.

## Verdict

**PASS, no CRITICAL or HIGH findings.** Every local gate passed. Four Low findings and two informational notes are listed below. CodeQL and Trivy were deferred to CI, as this is a `fix(security)` with no new feature surface (DoD step 3). Nothing was committed, pushed or fixed by this audit.

## Gate results

| Gate | Result | Evidence |
|---|---|---|
| Bash preflight | PASS | `echo ok` returned `ok` |
| Backend race tests (`internal/crowdsec/...`, `internal/api/handlers/...`) | PASS | `go test -race -count=1` ok for both packages |
| Backend coverage (`scripts/go-test-coverage.sh`, min 87%) | PASS | 92.2% statements, 91.1% lines |
| Frontend coverage (`scripts/frontend-test-coverage.sh`, min 87%) | PASS | 91.77% lines, 90.73% statements, 84.41% branches. 3851 passed, 4 skipped, 2 todo. No skips added by this branch. |
| Local patch coverage (`scripts/local-patch-report.sh`) | PASS | Overall 93.5% (31 lines), backend 92.9%, frontend 100%. Artifacts written: `test-results/local-patch-report.md` and `.json`. Uncovered: `hub_sync.go` 745-746 (see F2). |
| `lefthook run pre-commit` | PASS (no-op) | Every hook skipped because nothing is staged on this committed branch. Equivalent checks run manually: golangci-lint, type-check and both builds below. |
| golangci-lint (`backend/.golangci.yml`) on changed packages | PASS | 0 issues |
| `go build ./...` | PASS | |
| Frontend `npm run type-check` and `npm run build` | PASS | |
| GORM security scan | N/A | No `backend/internal/models/**` or migration files changed |
| govulncheck on changed packages | PASS | 0 vulnerabilities called by the code. One module-level advisory exists in the dependency graph and is not reachable from these packages. |
| CodeQL Go/JS, Trivy | DEFERRED to CI | `fix(security)` with no new feature surface |
| Targeted Playwright: `tests/security/crowdsec-hub-preset-apply.spec.ts`, `--project=security-tests --project=firefox` | PASS, 11/11 | Run against a throwaway E2E container from this branch. The changelog file was reverted with `git checkout` afterwards. Full suite and other browsers not run locally (CI-only). |
| Gotify token hygiene | PASS | No Gotify references in the diff, test artifacts or logs of this audit |
| Commit hygiene | PASS | No session IDs or session links in commit messages. Subjects are conventional. Only one `(security)` subject, which is vague as required. |
| i18n | PASS | The new `applyHubUpdating` key is present in all 28 locale files |
| Skipped tests | PASS | No `t.Skip` or skip additions in changed Go tests |

## Real-hub and tamper gate

Run in a throwaway container from image `charon-qa-hubcontent:07e5421d` built from this branch. The container was removed afterwards. The live `charon` container was not touched and is still up, with its original start time.

**Default hub and mirror.** `hub-data.crowdsec.net` returned 404 from the container. The index was served from the GitHub mirror.

- Pulled 5 real presets with 200 responses: `collections/crowdsecurity/base-http-scenarios`, `collections/crowdsecurity/sshd`, `collections/sigmahq/windows_proc_creation`, `parsers/crowdsecurity/sshd-logs`, `parsers/crowdsecurity/http-logs`.
- Independent check: for all 5, the sha256 of the raw mirror bytes equals the index `versions[v].digest`, and the returned `item_sha256` is 64 hex characters.
- Applied `crowdsecurity/base-http-scenarios` through cscli: `status: applied`, `used_cscli: true`.

**Tamper test.** A TLS mock stood in for the mirror through a throwaway CA. The `sshd` collection bytes were altered by appending a comment line. The index digest was left intact.

- Result: **409 `preset_hub_updating`**. The debug log shows the pull refused with the "hub still updating" warning. Cache entries for `sshd`: 0.
- Control item `base-http-scenarios`, untouched: 200 with `digest_checked=true`.

**Index entries without a digest.** The same tampered `sshd` bytes were served with its `versions` record removed from the index.

- Result: **200**, accepted. The debug log shows `digest_checked=false` with "accepted unverified". This is the designed fallback for mirrors and custom endpoints.

Cache hits were not re-verified, by design. Pre-upgrade entries age out within the 24 h TTL and apply still binds to the cached bytes.

## Findings (ranked)

No CRITICAL or HIGH findings.

**F1. Low. The digest check is same-origin, and the docs overstate it.**
- Where: `backend/internal/crowdsec/hub_sync.go` `verifyIndexDigest`, and the wording in `docs/features/crowdsec.md` ("checks each downloaded item against the hub's official index") and `docs/troubleshooting/crowdsec.md` ("so a half-published update is never used").
- Evidence: the digest and the item bytes come from the same index origin. Whoever controls the index can publish a matching digest, so the check detects inconsistent publishing, torn updates and corruption. It does not authenticate content against an attacker who controls the index. Separately, a digestless index entry is accepted silently at default log levels (the only trace is at Debug).
- Remediation: reword to "detects a mismatch between the index and the downloaded item" and avoid "official", "never used" and any "tamper" claim. Keep the spec's wording rule. Record the Aikido A2(a) resolution as a consistency check, not a full mitigation.

**F2. Low. Two changed lines are not covered.**
- Where: `hub_sync.go` 745-746, the `hubItemUnverifiedError.Error()` method. Backend patch coverage is 92.9%.
- Remediation: assert the message in `hub_digest_test.go`. Not blocking.

**F3. Low. A missing digest is only logged at Debug.**
- Where: `verifyIndexDigest`, the `digest_checked=false` branch.
- Evidence: an operator on a custom `HUB_BASE_URL` or a mirror that strips `versions` loses the check with no visible signal at default log levels.
- Remediation: consider an Info-level line, once per pull, with slug only. Optional.

**F4. Low. Misleading `hub_base_url` in the pull-failure log (observability, likely pre-existing).**
- Where: the `crowdsec preset pull failed` warning.
- Evidence: the log reported `hub_base_url: https://hub-data.crowdsec.net` on a failed pull, though the index had come from the mirror.
- Remediation: log the endpoint that actually served the index. Out of scope for this PR; file as a normal issue.

**F5. Low. The E2E harness echoes the emergency token in failure output.**
- Where: `tests/security-shard.setup.ts` (around line 43, `emergencySecurityReset`).
- Evidence: when the shard setup failed on a closed port, the Playwright call log printed the `X-Emergency-Token` request header. The value was from a local `.env`. It is exposed only on failure, but CI logs are shared.
- Remediation: redact auth headers from the error path. Treat as security-sensitive, so use a private advisory per SECURITY.md, not a public issue. The token value is not reproduced in this report.

**Informational.**
- **I1.** The default hub index returned 404 during this audit. All real pulls resolved through the GitHub mirror, so the mirror is effectively the default path today.
- **I2.** The frontend suite has 4 skipped and 2 todo tests. None were added by this branch.
- **I3.** The E2E spec does not exercise the new pull-409 branch in `CrowdSecConfig.tsx`. Vitest (`CrowdSecConfig.presetOutcomes.test.tsx`) covers it, which is acceptable.

## Not verified

- CodeQL and Trivy (deferred to CI).
- Full-suite and cross-browser E2E (CI-only by policy).
- Digest-verified apply of all 5 presets. Only `base-http-scenarios` was applied locally.
- Pre-upgrade cache entries under real traffic. Covered by design, not by a test.

## Cleanup

Container `charon-qa-hubcontent`, image `charon-qa-hubcontent:07e5421d`, the mock hub process and the scratch directory were removed. The working tree is clean except this report.
