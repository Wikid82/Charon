# QA Report: Single-Backup Bulk Certificate Delete

- Branch: `fix/cert-bulk-delete-single-backup` (3 commits over `development` `9765dce2b`, HEAD `08870cca9`)
- Scope: new admin-only `POST /api/v1/certificates/bulk-delete`, refactor of `CertificateHandler.Delete`, frontend `bulkDeleteCertificates` client, `useBulkDeleteCertificates` hook, and ProxyHosts orphaned-certificate cleanup.
- Change type: `fix:` with no new feature surface. CodeQL and Trivy are deferred to CI per `CLAUDE.md`.
- Date: 2026-11-07

## Verdict

**PASS. No blockers.** All gates run locally passed. Findings below are low severity or informational.

## Gate Results

| Gate | Result | Evidence |
|---|---|---|
| GORM security scan | Skipped (not triggered) | No changes under `backend/internal/models/**`, no new GORM queries or migrations |
| Backend coverage (`scripts/go-test-coverage.sh`) | PASS | Statement 93.2%, line 92.1% (gate 87%). `internal/api/handlers` ran fresh (not cached): 92.3%. Zero failures |
| `go build ./...` | PASS | Clean |
| `go vet ./internal/api/...` | PASS | Clean |
| `make lint-fast` | PASS | 0 issues (backend and agent) |
| Frontend coverage (`scripts/frontend-test-coverage.sh`) | PASS | 328 files, 4332 tests passed (2 todo). Lines 92.5%, statements 91.6%, branches 86.31% |
| `npm run type-check` | PASS | Clean |
| `npm run build` | PASS | Built |
| Local patch coverage (`scripts/local-patch-report.sh`, strict) | PASS | Overall 95.2% (100/105), backend 94.7% (89/94), frontend 100% (11/11) |
| Targeted E2E (`tests/certificate-bulk-delete.spec.ts`) | Deferred to CI | Requires a rebuilt E2E container for the new route; Docker was out of scope for this run |
| CodeQL / Trivy | Deferred to CI | `fix:`-scoped change |

Coverage freshness: HEAD commit time 2026-11-07 03:39:23 UTC. `backend/coverage.txt` written 03:52:18 UTC and `frontend/coverage/lcov.info` written 04:05:54 UTC, both after HEAD, so the patch report used fresh coverage. Patch baseline `origin/development...HEAD`; `origin/development` equals local `development` and the merge base.

## Patch Coverage Gaps (non-blocking)

| File | Uncovered lines | What they are |
|---|---|---|
| `backend/internal/api/handlers/certificate_handler.go` | 469-471 | `Delete` (UUID path): `DeleteCertificate` returns an error after the usage check passed |
| `backend/internal/api/handlers/certificate_handler.go` | 535-536 | `respondCertificateDeleteError` default branch (generic 500 "failed to delete certificate") |

Both are the race/DB-failure branch of single delete (a host gets assigned, or the row vanishes, or the DB fails, between the usage check and the conditional delete). Recommended test: in a `Delete` test, use the existing `gormfault` test helper (or a callback that fails the `Delete` on `ssl_certificates`) so `IsCertificateInUseByUUID` succeeds and `DeleteCertificate` fails, then assert 500 `{"error":"failed to delete certificate"}` and that the raw DB error is not in the body.

## Security Review

### Authorization: PASS
- `routes.go:1151` registers `POST /certificates/bulk-delete` on the authenticated `management` group with `middleware.RequireRole(models.RoleAdmin)`, matching single `DELETE /certificates/:uuid`.
- The static `bulk-delete` segment does not collide with `/:uuid` routes (POST `/:uuid/export` has an extra segment); route registration ran in the backend suite without panic.
- `TestBulkDeleteCertificates_RoleGating` confirms a `user` role gets 403, no backup is taken, and nothing is deleted.

### Input validation: PASS
- Body bound to a typed struct; bind errors return a fixed `{"error":"invalid request"}` (no decoder text).
- 1 to 200 entries enforced (`maxBulkDeleteCertificates`); each entry parsed with `uuid.Parse` and canonicalized; duplicates (including case variants) dropped while keeping order. Numeric IDs are rejected, consistent with the UUID-only rule.
- Validation failures return before the backup, so a bad request cannot trigger a backup.

### No raw error text in responses: PASS
- 400 messages come from fixed sentinel errors (`errBulkDeleteCount`, `errBulkDeleteUUID`).
- Per-certificate failures map to the fixed enum `in_use` / `not_found` / `error`; `TestBulkDeleteCertificates_UsageCheckErrorReportsGenericError` asserts the SQLite "no such table" text does not leak.
- Backup and disk-space failures reuse the existing fixed messages.

### Log injection: PASS
- Every log field carrying a UUID goes through `util.SanitizeForLog`, and the values are already canonical UUIDs from `uuid.Parse`. `reason` is from a fixed set. `sendDeleteNotification` re-validates via `sanitizeCertRef`.

### DoS bounds: PASS with one low finding
- Work per request is capped at 200 certificates and one backup. The backup service's non-blocking lock still prevents concurrent backups.
- **QA-1 (Low):** the handler does not cap the request body size before decoding. The 200-entry limit is checked only after the whole JSON body has been read, so an authenticated admin can make the server buffer an arbitrarily large body. No global body limit exists in the server, and most JSON handlers share this gap, so the risk is small (admin-only, authenticated). Remediation: wrap `c.Request.Body` with `http.MaxBytesReader` (for example 64 KiB, which is ample for 200 UUIDs) before `ShouldBindJSON`, the same pattern as `crowdsec_body_limit.go`. A server-wide default JSON body limit would close it for every handler.

### `Delete` behavior unchanged: PASS
- Numeric-ID path: same order (usage check, backup, delete), same status codes and messages; the backup block moved into `backupBeforeDelete` unchanged.
- UUID path: usage check still runs before the backup; 404 / 409 / 500 "failed to check certificate usage" / 507 / 500 backup / 404, 409, 500 on delete map to the same statuses and messages as before via `respondCertificateDeleteError`. `errors.Is` works through the `%w: %w` wrap (Go 1.20+).
- Only difference: the UUID path's low-disk branch now logs a warning it previously did not. No response change.

## Informational Findings (no action required for this PR)

- **QA-2 (Info):** `BulkDelete` takes the backup before the per-certificate usage checks, so a batch where every certificate is in use or missing still takes a backup. Single delete checks usage first. Acceptable trade-off (one backup per batch); a pre-pass that skips the backup when nothing is deletable would avoid wasted backups.
- **QA-3 (Low):** neither the certificates page bulk action (`useBulkDeleteCertificates`) nor the ProxyHosts cleanup splits selections larger than 200. Above 200 the request returns 400 and the whole batch is reported as failed, with nothing deleted. Remediation: chunk into groups of 200 in `bulkDeleteCertificates` (one backup per chunk), or cap selection in the UI.
- **QA-4 (Info):** one external "Certificate Deleted" notification is still sent per deleted certificate (up to 200 per request). This matches the old sequential behavior; a single summary notification for a batch would be quieter.

## Documentation Review (`docs/api.md`)

The new "Bulk Delete Certificates" section matches the code: path, admin-only POST, request shape, 1 to 200 UUID limit, duplicate handling, response shape with `deleted` and `failed` arrays (both always arrays), reason enum, and the exact 400 / 500 / 507 messages.

- **QA-5 (Low, docs):** the new section was inserted between the single "Delete Certificate" responses and that section's trailing paragraph, "**Note:** A backup is automatically created before deletion. The certificate files are removed from disk along with the database record." That note now reads as part of the bulk section. Move it back above `#### Bulk Delete Certificates`.
- Minor gaps: the section does not say the endpoint is admin-only (403 for other roles), and does not mention that UUIDs are returned in canonical lowercase form.

## Gotify Token Review

No Gotify tokens, notification URLs, or secrets appear in the changed code, tests, or docs.

## Blockers

None.
