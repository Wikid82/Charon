# Remaining Contract Tasks — Charon (feature/beta-release)

This document lists open items that must be completed to finish the current contract work: backend functionality, tests, coverage, front-end tasks, and documentation.

## High-priority Backend Tasks

- **Certificate handler: backup-before-delete**
  - Add `BackupService` constructor injection into `CertificateHandler`.
  - On delete: check "in-use" first; if not in-use, call `BackupService.CreateBackup()`. On backup failure return 500 and don't delete; if success, call delete and return 200.
  - Update `routes.go` to wire `backupService` to `NewCertificateHandler`.
  - Add unit tests: backup created before delete, deletion blocked if in use, backup failure prevents deletion.

- **Break-glass / Security**
  - Add handler-level tests for `GenerateBreakGlass` and `VerifyBreakGlass` endpoints.
  - Cover scenarios: no config, no hash, wrong token, rotated tokens, hash preservation across `Upsert`.
  - Ensure service-level tests are comprehensive (already added), extend to cover handler behavior and response codes.

- **Increase handler coverage to >=80%**
  - Target handlers with low coverage:
    - `proxy_host_handler.go` — Create/Update flows (54%/41% coverage);
    - `certificate_handler.go` — Upload handler coverage low, add success path tests;
    - `security_handler.go` — Upsert/DeleteRuleSet, Enable/Disable flows (48-60% coverage)
    - `import_handler.go` — DetectImports, UploadMulti and commit flows (low coverage);
    - `crowdsec_handler.go` — ReadFile, WriteFile tests;
    - `uptime_handler.go` — Sync, Delete, GetHistory error cases (more edge coverage)
  - Add negative tests for each: invalid input, not found, permission/FS errors.

## Medium-priority Backend Tasks

- **Notification Handler**
  - Add more tests for error flows, preview invalid payloads, provider CRUD tests.

- **Uptime Handler**
  - Edge cases: ensure `Sync` reports errors on DB/FS issues and handled correctly; verify metrics reporting for monitor creation/removal.

- **User, ACLs, and Remote Server**
  - Add tests for API keys regeneration, user setup flows, bulk ACL updates and remote server test connection flows.

## Frontend Tasks

- **Fix TypeScript issues and tests**
  - We've resolved `useQueryClient` unused import error in `CertificateList.test.tsx`. Continue running `npm run type-check` and fix other errors.
  - Run `npm test`/`vitest` for all component tests; update mocks for API clients where needed.

- **Component Test Coverage**
  - Add unit tests for components relying on API services/wrappers: `CertificateList`, security handlers, notification templates, and proxy host forms.

- **Integration / E2E**
  - Add or expand Cypress e2e tests for the main user flows (Login, Create Proxy Host, Upload Certificates, Backup/Restore workflows).

## CI & Lint

- Run/verify all linters and hooks:
  1. `pre-commit run --all-files`
  2. `cd frontend && npm run type-check && npm test`
  3. `cd backend && go test ./... -coverprofile=coverage.txt` and `bash scripts/go-test-coverage.sh` to ensure coverage >=80%.
  4. `golangci-lint` for Go linting.

## Docs & PR

- Update `docs/features.md` with the new features and implementation summary.
- Add test coverage updates and final review checklist in the PR description.

## Acceptance Criteria

- All tests pass with coverage >= 80%.
- No TypeScript errors across frontend.
- `CertificateHandler.Delete` performs backup before delete when safe (not in-use) and returns proper errors otherwise.
- `GenerateBreakGlass` / `VerifyBreakGlass` endpoints tested and behaving per the spec.
- CI passes pre-commit and linters.

---

### Quick Commands

- Run all backend tests + coverage:

```bash
cd /projects/Charon/backend
bash scripts/go-test-coverage.sh
```

- Run all frontend checks:

```bash
cd /projects/Charon/frontend
npm run type-check
npm test
```

---

If you want, I can pick one of these tasks to implement first tomorrow (suggested priority: finish `CertificateHandler.Delete` backup-before-delete and corresponding tests, then handler coverage work).
