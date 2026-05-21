# QA Audit Report — PR #1031: External Docker Proxy (feature/hecate)

| Field           | Value                                    |
|-----------------|------------------------------------------|
| **PR**          | #1031                                    |
| **Branch**      | `feature/hecate`                         |
| **Audit Date**  | 2026-05-20                               |
| **Auditor**     | QA Security (automated)                  |
| **Feature**     | External Docker Proxy (Orthrus subsystem)|
| **Verdict**     | **BLOCKED — Required fixes before merge**|

---

## Audit Summary

PR #1031 introduces the External Docker Proxy feature to the Orthrus agent subsystem. It adds
TCP port forwarding through the yamux session layer, a new frontend configuration dialog in
`OrthrusAgentManager`, and badge rendering for proxy-enabled agents.

**Overall verdict: BLOCKED.** Gates 1 and 6 fail on PR-scope code. Gate 9 has an actionable
HIGH CVE (Caddy v2.11.2 in the stale local image; the Dockerfile already pins the fix at
v2.11.3 — a rebuild is required). Six required fixes are enumerated in
[Required Actions](#required-actions).

---

## Gate Results

| Gate | Name                       | Result             | Threshold            |
|------|----------------------------|--------------------|----------------------|
| 1    | E2E Playwright             | **FAIL**           | All tests pass       |
| 2    | Backend Unit Coverage      | **PASS**           | 85% (≥91.2%)         |
| 3    | Frontend Unit Coverage     | **PASS**           | 87% lines (≥89.35%)  |
| 4    | Local Patch Coverage       | **WARN**           | 85% back (82.9%)     |
| 5    | TypeScript Type-check      | **PASS**           | Zero errors          |
| 6    | Lint (lefthook lint-full)  | **FAIL**           | Zero issues (3 found)|
| 7    | GORM Security Scan         | **PASS**           | 0 CRITICAL/HIGH      |
| 8    | Trivy Filesystem Scan      | **PASS**           | 0 CRITICAL/HIGH      |
| 9    | Trivy Image Scan           | **WARN**           | 0 CRITICAL (1 fixable HIGH via rebuild) |

---

## Gate 1 — E2E Playwright

**Result: FAIL**
**Duration:** 41.1 s · Tests passed: 2/9 · Tests failed: 7/9

### Passing tests
- `auth setup` — session file written
- `PROXY badge appears in agent row when external_proxy_port > 0`

### Failing tests (6) — Root Cause 1: Missing i18n key

All calls to `openProxyDialog()` in `tests/orthrus-external-proxy.spec.ts` (line 88) match
the button by regex `Configure external proxy`. At runtime `aria-label` resolves to the raw
i18n key string `hecate.agentManager.configureProxy` because the key does not exist in any of
the five locale files. The regex never matches, the locator times out, and six tests abort.

**Affected tests:**
- `opens the proxy configuration dialog`
- `shows validation error when port is invalid`
- `saves proxy configuration successfully`
- `clears proxy configuration`
- `disables save button when proxy port is empty`
- `shows loading state while saving`

**Root cause:** `frontend/src/components/hecate/OrthrusAgentManager.tsx` line 177:

```tsx
aria-label={t('hecate.agentManager.configureProxy', { name: agent.name })}
```

The key `configureProxy` is absent from all five locale files:
`frontend/src/locales/{en,es,fr,de,zh}/translation.json`

**Fix required:** Add the key to every locale file (see [Required Actions](#required-actions)).

### Failing test (1) — Root Cause 2: Strict-mode locator violation

`tests/orthrus-external-proxy.spec.ts` line 283:

```ts
const proxyBadge = page.getByText('PROXY');
```

The locator resolves to two elements (badge text + heading text), triggering Playwright strict
mode. The test throws `Error: strict mode violation`.

**Fix required:** Add `{ exact: true }` to the locator (see [Required Actions](#required-actions)).

---

## Gate 2 — Backend Unit Coverage

**Result: PASS**
**Coverage:** 91.2% (threshold: 85%)
**Artifact:** `backend/coverage.txt`

All backend packages pass. No action required.

---

## Gate 3 — Frontend Unit Coverage

**Result: PASS**
**Coverage (lines):** 89.35% (threshold: 87%)
**Artifact:** `frontend/coverage/lcov.info`

| Metric     | Value  | Threshold |
|------------|--------|-----------|
| Lines      | 89.35% | 87%       |
| Statements | 88.37% | —         |
| Functions  | 85.43% | —         |
| Branches   | 81.55% | —         |

---

## Gate 4 — Local Patch Coverage

**Result: WARN (non-blocking, mode=warn)**
**Generated:** 2026-05-20T10:16:33Z
**Artifacts:** `test-results/local-patch-report.md`, `test-results/local-patch-report.json`

| Scope    | Changed Lines | Covered Lines | Patch Coverage | Threshold | Status |
|----------|---------------|---------------|----------------|-----------|--------|
| Overall  | 111           | 92            | 82.9%          | 90.0%     | warn   |
| Backend  | 111           | 92            | 82.9%          | 85.0%     | warn   |
| Frontend | 0             | 0             | 100.0%         | 85.0%     | pass   |

### Files needing coverage

| File                                          | Patch Coverage | Uncovered Lines         |
|-----------------------------------------------|----------------|-------------------------|
| `backend/internal/orthrus/server.go`          | 60.0%          | 96–97                   |
| `backend/internal/orthrus/session.go`         | 77.9%          | 127–128, 160–161, 167–175, 199–203 |

**`server.go` lines 96–97:** Error logging path inside
`if err := session.StartDockerProxy(); err != nil`. Error path not exercised by existing tests.

**`session.go` uncovered paths:** yamux error path in `NewAgentSession` (127–128), double-check
pattern for closed-session in `StartDockerProxy` (160–161, 167–175), and transient retry logic
in `runProxyListener` (199–203).

Patch coverage is non-blocking (mode=warn) but is below both thresholds. Adding targeted error
path tests is strongly recommended before merge.

---

## Gate 5 — TypeScript Type-check

**Result: PASS**
Zero type errors across the entire frontend. `tsc --noEmit` succeeded.

---

## Gate 6 — Lint (lefthook lint-full)

**Result: FAIL**

### golangci-lint — PR-scope issues (3)

| File                                                        | Line | Linter    | Issue                                                   |
|-------------------------------------------------------------|------|-----------|---------------------------------------------------------|
| `backend/internal/orthrus/session.go`                       | 237  | gosec     | G104: Errors unhandled — `io.Copy` (nolint present for errcheck only) |
| `backend/internal/orthrus/session.go`                       | 242  | gosec     | G104: Errors unhandled — `io.Copy` (nolint present for errcheck only) |
| `backend/internal/orthrus/session_external_proxy_test.go`   | 62   | errcheck  | Error return of `conn.Close()` not checked in `defer`   |
| `backend/internal/orthrus/session_proxy_test.go`            | 19   | gocritic  | `paramTypeCombine` — two adjacent `*websocket.Conn` params can be combined |

> Note: `session.go` lines 237 and 242 carry `//nolint:errcheck` but gosec G104 is not
> suppressed. The directive must become `//nolint:errcheck,gosec`.

### hadolint — Dockerfile (7 info-level, non-blocking)

All findings are `DL3059` (consecutive `RUN` instructions) or `SC2012` (use `find` instead of
`ls`). Severity: info. No warnings or errors. Non-blocking.

### markdownlint — Auto-generated artifacts (non-blocking)

Lint ran on files under `test-results/` and `docs/reports/` that are generated artifacts.
Non-blocking.

---

## Gate 7 — GORM Security Scan

**Result: PASS**
Zero findings across all severity levels (CRITICAL, HIGH, MEDIUM, LOW).

The `external_proxy_port` field added to `backend/internal/models/orthrus_agent.go` is a
non-sensitive integer with `gorm:"default:0"`. `ID` is correctly `json:"-"`.
`AuthKeyHash` is correctly `json:"-"`. No data exposure risk detected.

---

## Gate 8 — Trivy Filesystem Scan

**Result: PASS**
Zero CRITICAL or HIGH findings in project source dependencies (`backend/go.sum`, `package.json`).

All CRITICAL/HIGH findings reported by Trivy were confined to `.cache/go/pkg/mod/` (developer
tool cache). These packages are not shipped in the production image and do not constitute a
vulnerability in the distributed artifact.

---

## Gate 9 — Trivy Image Scan

**Result: WARN — 0 CRITICAL, 3 HIGH (2 unique CVEs)**
**Scan target:** `charon:local` (image tar via `docker save`)

| CVE               | Component                    | Version   | CVSS | Fixed In | Status       |
|-------------------|------------------------------|-----------|------|----------|--------------|
| CVE-2026-45135    | `github.com/caddyserver/caddy/v2` | 2.11.2 | 8.1 | 2.11.3   | **Dockerfile already updated — rebuild required** |
| CVE-2026-32286    | `github.com/jackc/pgproto3/v2`   | 2.3.3  | 7.5 | None     | Pre-existing, no fix available |

### CVE-2026-45135 (HIGH · CVSS 8.1)

- **Component:** Caddy v2.11.2 (embedded in Charon binary via xcaddy)
- **Description:** Unsafe Unicode handling in FastCGI `splitPos` function. Network-reachable
  (AV:N), high complexity (AC:H), no privileges required. Potential for confidentiality and
  integrity impact.
- **Affects:** Any Charon deployment using the FastCGI reverse proxy capability.
- **Fix available:** v2.11.3
- **Status:** The Dockerfile `ARG CADDY_VERSION` is **already pinned to 2.11.3**. The
  `charon:local` image is a stale build from before this change. A container image rebuild
  will apply the fix.
- **Action required:** Rebuild the container image before publishing/deploying.

### CVE-2026-32286 (HIGH · CVSS 7.5)

- **Component:** `github.com/jackc/pgproto3/v2` v2.3.3
- **Description:** DoS via malicious PostgreSQL server sending a DataRow message with a
  negative field length in `DataRow.Decode`. AV:N, AC:L, no privileges required.
- **Fix available:** None (upstream unresolved)
- **Pre-existing:** This CVE predates PR #1031 and is not introduced by this change.
- **Action required:** Document in `SECURITY.md` (see [SECURITY.md Updates](#securitymd-updates)).

---

## Required Actions

The following issues **must** be resolved before merging PR #1031.

### Fix 1 — Add missing i18n translation key (BLOCKS Gate 1)

Add `configureProxy` to the `hecate.agentManager` section in all five locale files:

**`frontend/src/locales/en/translation.json`**
```json
"configureProxy": "Configure external proxy for {{name}}"
```

**`frontend/src/locales/es/translation.json`**
```json
"configureProxy": "Configurar proxy externo para {{name}}"
```

**`frontend/src/locales/fr/translation.json`**
```json
"configureProxy": "Configurer le proxy externe pour {{name}}"
```

**`frontend/src/locales/de/translation.json`**
```json
"configureProxy": "Externen Proxy für {{name}} konfigurieren"
```

**`frontend/src/locales/zh/translation.json`**
```json
"configureProxy": "为 {{name}} 配置外部代理"
```

### Fix 2 — Resolve Playwright strict-mode violation (BLOCKS Gate 1)

**`tests/orthrus-external-proxy.spec.ts` line 283:**

```ts
// Before
const proxyBadge = page.getByText('PROXY');

// After
const proxyBadge = page.getByText('PROXY', { exact: true });
```

### Fix 3 — Suppress gosec G104 in nolint directives (BLOCKS Gate 6)

**`backend/internal/orthrus/session.go` lines 237 and 242:**

```go
// Before
_, _ = io.Copy(dst, src) //nolint:errcheck

// After
_, _ = io.Copy(dst, src) //nolint:errcheck,gosec
```

### Fix 4 — Suppress errcheck on deferred close in test (BLOCKS Gate 6)

**`backend/internal/orthrus/session_external_proxy_test.go` line 62:**

```go
// Before
defer conn.Close()

// After
defer conn.Close() //nolint:errcheck
```

### Fix 5 — Combine websocket.Conn parameter types (BLOCKS Gate 6)

**`backend/internal/orthrus/session_proxy_test.go` line 19** — combine adjacent
`*websocket.Conn` parameters per gocritic `paramTypeCombine`:

```go
// Before
func proxyHelper(c1 *websocket.Conn, c2 *websocket.Conn) { ... }

// After
func proxyHelper(c1, c2 *websocket.Conn) { ... }
```

*(Adjust function signature to match actual source; the pattern is the same.)*

### Fix 6 — Rebuild container image to apply Caddy 2.11.3 (BLOCKS Gate 9 for deployment)

The Dockerfile already specifies `ARG CADDY_VERSION=2.11.3`. The `charon:local` image is
a stale build. Rebuild before publishing or deploying:

```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e
```

Or the standard Docker build process for the release image.

---

## Recommended Actions

The following issues are non-blocking but are strongly recommended before merge.

### Recommendation 1 — Increase patch coverage for error paths

`backend/internal/orthrus/server.go` (60.0% patch) and `session.go` (77.9% patch) have
uncovered error paths in PR-scope code. Adding targeted unit tests for these paths would:

- Bring backend patch coverage above the 85% threshold
- Ensure error propagation in `StartDockerProxy` and `runProxyListener` is verified

Key paths to cover:
- `server.go` 96–97: `session.StartDockerProxy()` error return
- `session.go` 127–128: yamux session error in `NewAgentSession`
- `session.go` 160–161, 167–175: `StartDockerProxy` closed-session and double-check paths
- `session.go` 199–203: `runProxyListener` transient retry logic

### Recommendation 2 — Add CVE-2026-32286 to SECURITY.md

`github.com/jackc/pgproto3/v2` v2.3.3 is not yet documented in `SECURITY.md`. See
[SECURITY.md Updates](#securitymd-updates).

---

## SECURITY.md Updates

The following entry should be added to `SECURITY.md` under **Known Vulnerabilities**.

```markdown
### [HIGH] CVE-2026-32286 · pgproto3 DoS via Negative DataRow Field Length

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2026-32286 |
| **Severity** | High · 7.5 |
| **Status**   | Awaiting Upstream |

**What**
`github.com/jackc/pgproto3/v2` v2.3.3 — `DataRow.Decode` does not validate field length
before allocation. A malicious PostgreSQL server can send a negative field length to trigger
a panic or excessive memory allocation, causing denial of service.

**Who**

- Discovered by: Automated scan (Trivy image scan)
- Reported: 2026-05-20
- Affects: Charon deployments connecting to untrusted PostgreSQL servers (not the default
  SQLite deployment)

**Where**

- Component: `github.com/jackc/pgproto3/v2` v2.3.3 (transitive dependency)
- Versions affected: pgproto3/v2 < patched version

**When**

- Discovered: 2026-05-20
- Disclosed (if public): Public
- Target fix: When upstream publishes a patched release

**How**
Exploitation requires a malicious or compromised PostgreSQL server to send a crafted DataRow
message. Charon's default installation uses SQLite and does not connect to PostgreSQL. Users
deploying Charon with a PostgreSQL backend (non-standard) are potentially exposed if the
database server is untrusted. EPSS score not yet available.

**Planned Remediation**
Monitor https://github.com/jackc/pgproto3 for a fix release. Upgrade the indirect dependency
once a patched version is available.
```

CVE-2026-45135 (Caddy) does not require a `SECURITY.md` entry because the Dockerfile already
pins the fix (v2.11.3) — it is remediated at the source level pending an image rebuild.

---

## Appendix — PR Scope Files

| File                                                          | Type     | Notes                        |
|---------------------------------------------------------------|----------|------------------------------|
| `backend/internal/orthrus/session.go`                         | Modified | Core proxy implementation    |
| `backend/internal/orthrus/server.go`                          | Modified | Agent server integration     |
| `backend/internal/orthrus/session_external_proxy_test.go`     | New      | Unit tests for external proxy|
| `backend/internal/orthrus/session_proxy_test.go`              | New      | Proxy helper tests           |
| `backend/internal/models/orthrus_agent.go`                    | Modified | `ExternalProxyPort` field    |
| `frontend/src/components/hecate/OrthrusAgentManager.tsx`      | Modified | Proxy dialog + badge         |
| `frontend/src/locales/{en,es,fr,de,zh}/translation.json`      | Modified | i18n — key missing           |
| `tests/orthrus-external-proxy.spec.ts`                        | New      | E2E tests (2/9 pass)         |
| `Dockerfile`                                                  | Modified | Caddy ARG updated to 2.11.3  |
