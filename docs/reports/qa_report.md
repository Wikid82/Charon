# QA Report: Encryption Key Handling

- Branch: `fix/encryption-rotation-coverage` (10 commits on `origin/development`, head `dfb644c51`)
- Plan: `docs/plans/pr26_encryption_rotation_plan.md` (rev 2), section 7 (test plan, container gates G1-G8,
  Definition of Done)
- Scope: one registry of encrypted columns; rotation re-encrypts every stored secret (DNS providers, DNS provider
  credentials, remote backup destinations, certificate private keys, tunnels, the stored backup passphrase) in one
  write-locked transaction; key versions follow the key itself, not its environment slot; every reader decrypts with
  all configured keys; the encryption page reports per key and per store which keys are still needed; writes that do
  not produce new ciphertext leave the secret columns alone; a live restore reloads the key versions; certificate
  private keys stored by older versions without encryption are encrypted at the first start with a key; a custom
  certificate upload without a valid key is refused.
- Image under test: the E2E image rebuilt from the head above (`charon:local`, also tagged `rotqa-pr` for the
  container gates). The previous image was `wikid82/charon:dev` at `39b3331` (an ancestor of the branch). All
  gate containers and volumes were throwaway (`rotqa-*`, own loopback ports) and were removed afterwards. The live
  containers `charon`, `charon-dev` and `orthrus-dev` were not touched.
- Toolchain: Go 1.27.4, CodeQL CLI 2.27.5, lefthook 2.2.3, Playwright 1.64.1.
- Date: 2026-11-24.

## Verdict

**Approved.** Every Definition of Done gate passes and every container gate matches the plan. QA added one test-only
file, `backend/internal/services/credential_service_cipher_errors_test.go` (four tests for the credential service:
create and update without a key store nothing, an update with new credentials stores the new secret, a test with an
unreadable secret reports a decryption error). It lifted patch coverage from 89.8% to 91.0%. QA also annotated two test fixture lines in
`backend/internal/services/secret_cipher_stale_writers_test.go` (141, 182) with `#nosec G117` and a justification,
because the full lint (gosec on test files, as in CI) flagged them. No application code was changed by QA.

Four low, in-scope observations (O1-O4) and two low, out-of-scope findings (F1, F2) are listed for the orchestrating
session. None blocks the merge.

## Findings

### In scope (non-blocking)

| Id | Severity | Observation | Remediation |
| --- | --- | --- | --- |
| O1 | Low | A legacy slot that holds the current key (for example `CHARON_ENCRYPTION_KEY_V1` equal to `CHARON_ENCRYPTION_KEY`) is not dropped and logs no Info line (`backend/internal/crypto/rotation_service.go:216-222`). Only a next key equal to the current key is dropped (`:208-213`). The status page then lists the slot as its own row with the current version and "Still needed". Versions stay correct, so nothing breaks | Drop a legacy slot whose key id matches the current key or an earlier slot, log Info with the slot name, and add a unit test next to the existing next-key case; or accept the behaviour and update the plan text |
| O2 | Low | While a next key is configured and no rotation has run yet, every start logs a Warn "Stored secrets use an encryption key other than the rotation target; rotation corrects them". The plan describes this Warn for rows that need a key other than the current key | Log Info instead of Warn when the only rows off target are on the current key and a next key is configured |
| O3 | Low (UX) | A next key that is configured but not used yet shows "No longer needed" on the encryption page (`removable` is "not current and in use 0"). This is true, but reads oddly just before the first rotation | Show a neutral label such as "Not used yet" for the next slot while its use count is 0 |
| O4 | Low (cosmetic) | On the start that encrypts legacy certificate keys, the reconcile summary runs first and reports `certificate_keys=0`. The status page is correct right after the start | Run the legacy certificate migration before the reconcile summary, or log the summary again after the migration |

### Out of scope

| Id | Severity | Finding | Remediation |
| --- | --- | --- | --- |
| F1 | Low | Two certificate deletes at the same moment: the second answers 500 "failed to create backup before deletion" because its pre-delete backup finds another backup running (`backend/internal/api/handlers/certificate_handler.go:536-540`). Seen in E2E: `tests/core/certificates.spec.ts:1075` and `:1136` fail with parallel workers and pass with `--workers=1`. Not changed by this PR | Answer 409 with a "try again" text for the busy case (or let the pre-delete backup wait for the lock), with a handler test; mark the two delete specs serial |
| F2 | Low (test environment) | The local E2E container keeps `/app/data` on a 512 MB tmpfs (`.docker/compose/docker-compose.playwright-local.yml`). Each backup is about 40 MB, so after about nine backups the pre-delete backup answers 507 and certificate delete specs fail | Raise the tmpfs size, or delete the backups each backup spec creates |

No CRITICAL, HIGH or MEDIUM findings in scope.

## Gate results

| # | Gate | Command | Result |
| --- | --- | --- | --- |
| 1 | Targeted E2E, encryption page | `npx playwright test tests/security-enforcement/zzz-security-ui/encryption-management.spec.ts --project=security-tests` | **Pass**: 20 passed, 3 skipped (rotation cases that need a next key; the E2E stack sets only the current key) |
| 1b | Targeted E2E, certificates and tunnels | `npx playwright test tests/core/certificates.spec.ts tests/certificate-export.spec.ts tests/certificate-delete.spec.ts tests/hecate-tunnel-manager.spec.ts --project=firefox` | **Pass after triage**: 94 passed, 1 skipped, 2 failed (`certificates.spec.ts:1075`, `:1136`). Root cause is F1 and F2, not this PR: the container data disk was full of backups, then two parallel deletes collided. After the backups were cleared, the `Delete Certificate` group passed 8 of 8 with `--workers=1` |
| 1c | Targeted E2E, DNS providers, backups, export | `npx playwright test tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts tests/manual-dns-provider.spec.ts tests/tasks/backups-restore.spec.ts tests/tasks/backups-encryption.spec.ts tests/certificate-export.spec.ts --project=firefox` | **Pass**: 85 passed |
| 2 | GORM security scan | `./scripts/scan-gorm-security.sh --check` | **Pass**: 0 critical, 0 high, 0 medium, 2 informational index suggestions (existing) |
| 3 | Local patch coverage | `bash scripts/local-patch-report.sh` | **Pass**: overall 91.0% (806/886 changed lines), backend 90.8%, frontend 100%; both artifacts written. Before the QA test file: 89.8% |
| 4 | Backend coverage | `scripts/go-test-coverage.sh` | **Pass**: statement 94.1%, line 93.1% (gate 87%); crypto 91.3%, services 92.5%, handlers 93.4%, routes 97.0%, caddy 98.1%, hecate 88.1% |
| 4b | Frontend coverage | `scripts/frontend-test-coverage.sh` | **Pass**: 378 files, 4711 tests passed; statements 92.04%, lines 92.89% |
| 4c | Race detector | `go test -race ./internal/crypto/... ./internal/api/handlers/ ./internal/api/routes/ ./internal/services/ ./internal/caddy/ ./internal/hecate/ ./internal/database/ -run 'Rotation\|Rotate\|Reconcile\|Rehydrat\|Restore\|Keyring\|Version\|Store\|Encryption\|Migrate\|ColumnOrder\|Stale\|Certificate\|PrivateKey\|Tunnel\|Remote\|WriteLock\|Cipher'` | **Pass**: all 8 packages; the QA test file also passes with `-race` |
| 5 | CodeQL Go and JS | `lefthook run codeql` | **Pass**: Go 1 result, suppressed by an existing entry in `codeql-suppressions.yml` (session cookie on local plain-HTTP origins, unrelated); JS 0 results; parity check passed |
| 6 | Pre-commit, all files | `lefthook run pre-commit --all-files` | **Pass**: all 20 hooks (golangci-lint fast, go vet, frontend lint, type-check and locales lint, semgrep, markdownlint, shellcheck, actionlint, and the rest) |
| 7 | Backend build | `cd backend && go build ./...` | **Pass** |
| 8 | Frontend type-check | `cd frontend && npm run type-check` | **Pass** |
| 9 | Frontend build | `cd frontend && npm run build` | **Pass** |
| 10 | Trivy | | Deferred to CI (no new feature surface; CI runs it on every PR) |
| 11 | Full lint, run last | `make lint-backend` | **Pass**: 0 issues on the final tree. The first run found 2 gosec G117 findings in the PR's test file `secret_cipher_stale_writers_test.go:141,182` (marshalled test secrets); QA added `#nosec G117` with a justification and re-ran (`agent/` unchanged, so `make lint-agent` was not required) |

## Container gates

Each gate used fresh keys from `openssl rand -base64 32` (A, B, C), a self-signed certificate, and one object per
store: a single-credential DNS provider, a multi-credential provider with two zone credentials, an S3 remote
destination, a custom certificate, a tunnel and a stored backup passphrase. Readers were exercised through the API:
the DNS provider and credential tests, the remote destination test, certificate export with the key, tunnel start,
and an encrypted backup that uses the stored passphrase. Database rows were read with read-only `sqlite3` in a
throwaway container.

| Gate | Check | Result |
| --- | --- | --- |
| G1 | Previous image: same flow (rotate with a next key, promote, restart) | **Done**: baseline recorded for comparison with G2 |
| G2 | PR image, fresh volume: key A, then `_NEXT=B`, rotate, rerun, promote (A to `V1`), remove A | **Pass**. Rotation re-encrypted 9 of 9 secrets across all 6 stores; the rerun answered 200 with `rotated 0`, `already_current 9`. After promotion every reader works and the Keys table shows A as version 1, in use 0, "No longer needed". After removing A every reader still works. The DNS challenge configuration sent to the proxy contained the decrypted single-provider token and the zone credential token |
| G3 | Upgrade: previous-image database with rows tagged 1 and 2 and no key registry, started on the PR image | **Pass**. Registry filled current = 1, next = 2; every store row byte-identical before and after the start; tunnel tags start at 0. Rotation re-encrypted 9 rows and fixed the tags of 2 rows already on the target (11 total); promotion works and only the Info summary is logged. Key A only, on a database without next-key rows: exactly one Info reconcile line, no Warn |
| G4 | Key A moved from `V1` to `V3` | **Pass**: still version 1 |
| G5 | Fresh install, no key | **Pass**: no "no such column" line; certificate upload answers 400 `{"error":"Set a valid encryption key (CHARON_ENCRYPTION_KEY) before uploading a certificate with a private key.","error_code":"encryption_key_required"}`; nothing stored |
| G6 | Legacy plaintext certificate key (`private_key` column added with `sqlite3`) | **Pass**. Without a key, every start logs "1 certificate private keys are stored without encryption. Set CHARON_ENCRYPTION_KEY and restart to encrypt them." and the key stays as it was. With key A: "encrypted plaintext private keys" (count 1), `private_key` is empty, `private_key_enc` set with version 1, export with the key works, and the proxy configuration loads the certificate with its key. `PRAGMA secure_delete` is a per-connection setting and cannot be read from outside the process; its restoration is covered by the unit tests |
| G7 | Restore of a previous-image backup on the PR image (with tunnel rows, and without) | **Pass**. Both answered `restart_required: true`, `live_rehydrate_applied: false`; live data unchanged until the restart. After the restart the restored secrets are readable, `tunnel_configs.key_version` exists (last column), and the status page is correct. A backup taken on the PR image restored live with `restart_required: false`; the secrets were readable at once (DNS tests, certificate export, tunnel start, remote test) and the log showed the reconcile summary before the uptime rehydrate |
| G8 | Writes after a rotation: remote destination test, certificate rename, tunnel rename without credentials | **Pass**: cipher and version columns of all three rows byte-identical to the rotated values; each write changed only its own field |
| Keys | Invalid, short and duplicate keys | **Pass**: `_V2` invalid base64 and `_V4` with 16 bytes log a Warn naming the slot only and show "could not be loaded"; `_NEXT` equal to the current key logs Info and is ignored (see O1 for a legacy slot); validate answers 400 "One or more encryption keys failed validation." with the slot names; rotate with nothing to do answers 409 `rotation_not_needed` |
| Logs | No key text in container logs | **Pass**: no key text (full or 16-character prefix), no private key PEM and no backup passphrase in any gate container log or in the E2E container log |
| Access | Admin-only endpoints | **Pass**: a `user` role answers 403 on status, rotate, validate and history; no token answers 401 |
