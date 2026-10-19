# QA and Security Report: fix/orthrus-first-docker-sources

(Overwrites the previous batch's report.) Branch vs development: 7 commits, 53 files.

| # | Item | Result |
|---|------|--------|
| 1 | Backend build, vet | PASS |
| 1 | Unit tests (services 92.9s, handlers 208.9s, routes 8.1s, -count=1) | PASS |
| 1 | scripts/go-test-coverage.sh | PASS: statements 92.2%, lines 91.1% (gate 87%) |
| 2 | Frontend type-check, build | PASS |
| 2 | Frontend coverage script | PASS: 303 files, 3808 tests passed; lines 91.7%, stmts 90.64%, branches 84.26% |
| 2 | Changed files | api/docker.ts 100% lines/branches; useDockerStatus.ts 100%; ProxyHostForm.tsx 98.3% lines, 88.3% branches |
| 3 | local-patch-report.sh | PASS: overall 49/49, backend 41/41, frontend 8/8, agent 0/0 (100%); no uncovered new lines; md and json present |
| 4 | CodeQL Go and JS (security-scan-codeql skill, codeql 2.26.4) | PASS: 0 errors, 0 warnings |
| 4 | Trivy (security-scan-trivy skill) | See note: exit non-zero, no findings in files touched by this branch |
| 5 | make lint-fast (backend, agent) | PASS: 0 issues |
| 5 | lefthook run pre-commit --all-files | PASS (plain run skips all hooks: nothing staged); semgrep 0 findings, frontend-lint 0 errors |
| 5 | GORM scan | Not required: diff touches no models, migrations, or queries |
| 6 | Security review of GET /api/v1/docker/status | PASS |
| 7 | docs-site untouched, docs claims | PASS |
| - | Playwright E2E | NOT RUN (port conflict, CI only). Specs list cleanly: 48 tests in 8 files, firefox. New specs have no test.fixme left. |

## Trivy note (pre-existing, not from this branch)
Skill exits non-zero on 4 findings, all outside the diff (git diff shows no changes under docs-site or backend/internal/api/routes/keys):
- docs-site/package-lock.json: braces 3.0.3, CVE-2026-93687 HIGH, no fixed version listed.
- backend/internal/api/routes/keys/hecate-ca.key: HIGH secret; file is gitignored (*.key), untracked local artifact.
- docs-site/build/assets/js/603ac66c.45842fcd.js and docs-site/build/docs/guides/dns-providers/google-cloud-dns/index.html: CRITICAL "gcp-service-account", false positive; gitignored build output of a docs example JSON.
Recommend: file an issue for the braces dev dependency (docs-site); consider a trivy secret skip for docs-site/build and the local key.

## Security review of /docker/status
- Auth/RBAC: registered via dockerHandler.RegisterRoutes(management) (routes.go:809), same group as /docker/containers, behind auth plus RequireManagementAccess. Route tests cover unauthenticated 401, passthrough 403, management 200.
- Disclosure: response is {available, reason, message} with canned messages and an enumerated reason; no path, uid, gid, or raw error text (raw err only in Debug log). docker_service.go:~499-570.
- Oracle/SSRF: takes no input; probes only the server-configured local Docker host with Ping, so no user-controlled target. Reason set reveals only coarse socket state to an already-authorised management user.
- Abuse: Ping bounded to 2s, ctx-derived, cheap; a timeout fails open ("slow") by design. Endpoint falls under the existing API rate limiting of the authenticated group; no extra limiter needed (optional hardening: none required).
- Raw-mount path (ListContainers) unchanged apart from the extracted message constant; unit tests pass.
- Secrets: scan of the added diff lines and commit messages found no credentials, tokens, or session IDs/claude.ai links (only test-helper token variable names).

## Docs
- No changes under docs-site (git diff empty).
- Same-host Orthrus section matches agent/main.go: connect path /api/v1/ws/orthrus/connect appended automatically (main.go:100-113); ws:// warning for non-localhost hosts (main.go:134-136); localhost allowed.

## Blocking issues
None.
