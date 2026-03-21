# CWE-614 Remediation — Sensitive Cookie Without 'Secure' Attribute

**Date**: 2026-03-21
**Scope**: `go/cookie-secure-not-set` CodeQL finding in `backend/internal/api/handlers/auth_handler.go`
**Status**: Draft — Awaiting implementation

---

## 1. Problem Statement

### CWE-614 Description

CWE-614 (*Sensitive Cookie Without 'Secure' Attribute*) describes the vulnerability where a
session or authentication cookie is issued without the `Secure` attribute. Without this attribute,
browsers are permitted to transmit the cookie over unencrypted HTTP connections, exposing the
token to network interception. A single cleartext transmission of an `auth_token` cookie is
sufficient for session hijacking.

### CodeQL Rule

The CodeQL query `go/cookie-secure-not-set` (security severity: **warning**) flags any call to
`http.SetCookie` or Gin's `c.SetCookie` where static analysis can prove there exists an execution
path in which the `secure` parameter evaluates to `false`. The rule does not require the path to
be reachable in production — it fires on reachability within Go's control-flow graph.

### SARIF Finding

The SARIF file `codeql-results-go.sarif` contains one result for `go/cookie-secure-not-set`:

| Field | Value |
|---|---|
| Rule ID | `go/cookie-secure-not-set` |
| Message | "Cookie does not set Secure attribute to true." |
| File | `internal/api/handlers/auth_handler.go` |
| Region | Lines 152–160, columns 2–3 |
| CWE tag | `external/cwe/cwe-614` |
| CVSS severity | Warning |

The flagged region is the `c.SetCookie(...)` call inside `setSecureCookie`, where the `secure`
variable (sourced from a `bool` modified at line 140 via `secure = false`) can carry `false`
through the call.

---

## 2. Root Cause Analysis

### The Offending Logic in `setSecureCookie`

`setSecureCookie` (auth_handler.go, line 133) constructs the `Secure` attribute value using
runtime heuristics:

```go
secure := true
sameSite := http.SameSiteStrictMode
if scheme != "https" {
    sameSite = http.SameSiteLaxMode
    if isLocalRequest(c) {
        secure = false   // ← line 140: CWE-614 root cause
    }
}
```

When both conditions hold — `requestScheme(c)` returns `"http"` AND `isLocalRequest(c)` returns
`true` — the variable `secure` is assigned `false`. This value then flows unmodified into:

```go
c.SetCookie( // codeql[go/cookie-secure-not-set]
    name, value, maxAge, "/", domain,
    secure, // ← false in the local-HTTP branch
    true,
)
```

CodeQL's dataflow engine traces the assignment on line 140 to the parameter on line 159 and emits
the finding. The `// codeql[go/cookie-secure-not-set]` inline suppression comment was added
alongside the logic, but the SARIF file pre-dates the suppression and the CI continues to report
the finding — indicating either that the suppression was committed after the SARIF was captured
in the repository, or that GitHub Code Scanning's alert dismissal has not processed it.

### Why the Suppression Is Insufficient

Inline suppression via `// codeql[rule-id]` tells CodeQL to dismiss the alert at that specific
callsite. It does not eliminate the code path that creates the security risk; it merely hides the
symptom. In a codebase with Charon's security posture (full supply-chain auditing, SBOM
generation, weekly CVE scanning), suppressing rather than fixing a cookie security issue is the
wrong philosophy. The authentic solution is to remove the offending branch.

### What `isLocalRequest` Detects

`isLocalRequest(c *gin.Context) bool` returns `true` if any of the following resolve to a local
or RFC 1918 private address: `c.Request.Host`, `c.Request.URL.Host`, the `Origin` header, the
`Referer` header, or any comma-delimited value in `X-Forwarded-Host`. It delegates to
`isLocalOrPrivateHost(host string) bool`, which checks for `"localhost"` (case-insensitive),
`ip.IsLoopback()`, or `ip.IsPrivate()` per the Go `net` package (10.0.0.0/8, 172.16.0.0/12,
192.168.0.0/16, ::1, fc00::/7).

### Why `secure = false` Was Introduced

The intent was to permit Charon to be accessed over HTTP on private networks (e.g., a developer
reaching `http://192.168.1.50:8080`). Browsers reject cookies with the `Secure` attribute on
non-HTTPS connections for non-localhost hosts, so setting `Secure = true` on a response to a
`192.168.x.x` HTTP request causes the browser to silently discard the cookie, breaking
authentication. The original author therefore conditionally disabled the `Secure` flag for these
deployments.

### Why This Is Now Wrong for Charon

Charon is a security-oriented reverse proxy manager designed to sit behind Caddy, which always
provides TLS termination in any supported deployment. The HTTP-on-private-IP access pattern breaks
down into three real-world scenarios:

1. **Local development (`http://localhost:8080`)** — All major browsers (Chrome 66+, Firefox 75+,
   Safari 14+) implement the *localhost exception*: the `Secure` cookie attribute is honoured and
   the cookie is accepted and retransmitted over HTTP to localhost. Setting `Secure = true` causes
   zero breakage here.

2. **Docker-internal container access (`http://172.x.x.x`)** — Charon is never reached directly
   from within the Docker network by a browser; health probes and inter-container calls do not use
   cookies. No breakage.

3. **Private-IP direct browser access (`http://192.168.x.x:8080`)** — This is explicitly
   unsupported as an end-user deployment mode. The Charon `ARCHITECTURE.md` describes the only
   supported path as via Caddy (HTTPS) or `localhost`. Setting `Secure = true` on these responses
   means the browser ignores the cookie; but this deployment pattern should not exist regardless.

The conclusion: removing `secure = false` unconditionally is both correct and safe for all
legitimate Charon deployments.

---

## 3. Affected Files

### Primary Change

| File | Function | Lines | Nature |
|---|---|---|---|
| `backend/internal/api/handlers/auth_handler.go` | `setSecureCookie` | 128–162 | Delete `secure = false` branch; update docstring; remove suppression comment |

No other file in the backend sets cookies directly. Every cookie write flows through
`setSecureCookie` or its thin wrapper `clearSecureCookie`. The complete call graph:

- `setSecureCookie` — canonical cookie writer (line 133)
- `clearSecureCookie` → `setSecureCookie(c, name, "", -1)` (line 166)
- `AuthHandler.Login` → `setSecureCookie(c, "auth_token", token, 3600*24)` (line 188)
- `AuthHandler.Logout` → `clearSecureCookie(c, "auth_token")`
- `AuthHandler.Refresh` → `setSecureCookie(c, "auth_token", token, 3600*24)` (line 252)

`clearSecureCookie` requires no changes; it already delegates through `setSecureCookie`.

### Test File Changes

| File | Test Function | Line | Change |
|---|---|---|---|
| `backend/internal/api/handlers/auth_handler_test.go` | `TestSetSecureCookie_HTTP_Loopback_Insecure` | 115 | `assert.False` → `assert.True` |
| `backend/internal/api/handlers/auth_handler_test.go` | `TestSetSecureCookie_HTTP_PrivateIP_Insecure` | 219 | `assert.False` → `assert.True` |
| `backend/internal/api/handlers/auth_handler_test.go` | `TestSetSecureCookie_HTTP_10Network_Insecure` | 237 | `assert.False` → `assert.True` |
| `backend/internal/api/handlers/auth_handler_test.go` | `TestSetSecureCookie_HTTP_172Network_Insecure` | 255 | `assert.False` → `assert.True` |
| `backend/internal/api/handlers/auth_handler_test.go` | `TestSetSecureCookie_HTTP_IPv6ULA_Insecure` | 291 | `assert.False` → `assert.True` |

The five tests named `*_Insecure` were authored to document the now-removed behaviour; their
assertions flip from `False` to `True`. Their names remain unchanged — renaming is cosmetic and
out of scope for a security fix.

Tests that must remain unchanged:

- `TestSetSecureCookie_HTTPS_Strict` — asserts `True`; unaffected.
- `TestSetSecureCookie_HTTP_Lax` — asserts `True`; unaffected (192.0.2.0/24 is TEST-NET-1, not
  an RFC 1918 private range, so `isLocalRequest` already returned `false` here).
- `TestSetSecureCookie_ForwardedHTTPS_LocalhostForcesInsecure` — asserts `True`; unaffected.
- `TestSetSecureCookie_ForwardedHTTPS_LoopbackForcesInsecure` — asserts `True`; unaffected.
- `TestSetSecureCookie_ForwardedHostLocalhostForcesInsecure` — asserts `True`; unaffected.
- `TestSetSecureCookie_OriginLoopbackForcesInsecure` — asserts `True`; unaffected.
- `TestSetSecureCookie_HTTPS_PrivateIP_Secure` — asserts `True`; unaffected.
- `TestSetSecureCookie_HTTP_PublicIP_Secure` — asserts `True`; unaffected.

---

## 4. Implementation Details

### 4.1 Changes to `setSecureCookie` in `auth_handler.go`

**Before** (lines 128–162):

```go
// setSecureCookie sets an auth cookie with security best practices
// - HttpOnly: prevents JavaScript access (XSS protection)
// - Secure: true for HTTPS; false for local/private network HTTP requests
// - SameSite: Lax for any local/private-network request (regardless of scheme),
//   Strict otherwise (public HTTPS only)
func setSecureCookie(c *gin.Context, name, value string, maxAge int) {
	scheme := requestScheme(c)
	secure := true
	sameSite := http.SameSiteStrictMode
	if scheme != "https" {
		sameSite = http.SameSiteLaxMode
		if isLocalRequest(c) {
			secure = false
		}
	}

	if isLocalRequest(c) {
		sameSite = http.SameSiteLaxMode
	}

	// Use the host without port for domain
	domain := ""

	c.SetSameSite(sameSite)
	// secure is intentionally false for local/private network HTTP requests; always true for external or HTTPS requests.
	c.SetCookie( // codeql[go/cookie-secure-not-set]
		name,   // name
		value,  // value
		maxAge, // maxAge in seconds
		"/",    // path
		domain, // domain (empty = current host)
		secure, // secure
		true,   // httpOnly (no JS access)
	)
}
```

**After**:

```go
// setSecureCookie sets an auth cookie with security best practices
// - HttpOnly: prevents JavaScript access (XSS protection)
// - Secure: always true; the localhost exception in Chrome, Firefox, and Safari
//   permits Secure cookies over HTTP to localhost/127.0.0.1 without issue
// - SameSite: Lax for any local/private-network request (regardless of scheme),
//   Strict otherwise (public HTTPS only)
func setSecureCookie(c *gin.Context, name, value string, maxAge int) {
	scheme := requestScheme(c)
	sameSite := http.SameSiteStrictMode
	if scheme != "https" || isLocalRequest(c) {
		sameSite = http.SameSiteLaxMode
	}

	// Use the host without port for domain
	domain := ""

	c.SetSameSite(sameSite)
	c.SetCookie(
		name,   // name
		value,  // value
		maxAge, // maxAge in seconds
		"/",    // path
		domain, // domain (empty = current host)
		true,   // secure (always; satisfies CWE-614)
		true,   // httpOnly (no JS access)
	)
}
```

**What changed**:

1. The `secure := true` variable is removed entirely; `true` is now a literal at the callsite,
   making the intent unmistakable to both humans and static analysis tools.
2. The `if scheme != "https" { ... if isLocalRequest(c) { secure = false } }` block is replaced
   by a single `if scheme != "https" || isLocalRequest(c)` guard for the `sameSite` value only.
   The two previously separate `isLocalRequest` calls collapse into one.
3. The `// secure is intentionally false...` comment is removed — it described dead logic.
4. The `// codeql[go/cookie-secure-not-set]` inline suppression is removed — it is no longer
   needed and should not persist as misleading dead commentary.
5. The function's docstring bullet for `Secure:` is updated to reflect the always-true policy
   and cite the browser localhost exception.

### 4.2 Changes to `auth_handler_test.go`

Five `assert.False(t, cookie.Secure)` assertions become `assert.True(t, cookie.Secure)`.
The SameSite assertions on the lines immediately following each are correct and untouched.

| Line | Before | After |
|---|---|---|
| 115 | `assert.False(t, cookie.Secure)` | `assert.True(t, cookie.Secure)` |
| 219 | `assert.False(t, cookie.Secure)` | `assert.True(t, cookie.Secure)` |
| 237 | `assert.False(t, cookie.Secure)` | `assert.True(t, cookie.Secure)` |
| 255 | `assert.False(t, cookie.Secure)` | `assert.True(t, cookie.Secure)` |
| 291 | `assert.False(t, cookie.Secure)` | `assert.True(t, cookie.Secure)` |

### 4.3 No Changes Required

The following functions are call-through wrappers or callers of `setSecureCookie` and require
zero modification:

- `clearSecureCookie` — its contract ("remove the cookie") is satisfied by any `maxAge = -1`
  call, regardless of the `Secure` attribute value.
- `AuthHandler.Login`, `AuthHandler.Logout`, `AuthHandler.Refresh` — callsites are unchanged.
- `isLocalRequest`, `isLocalOrPrivateHost`, `requestScheme`, `normalizeHost`, `originHost` —
  all remain in use for the `sameSite` determination.
- `codeql-config.yml` — no query exclusions are needed; the root cause is fixed in code.

---

## 5. Test Coverage Requirements

### 5.1 Existing Coverage — Sufficient After Amendment

The five amended tests continue to exercise the local-HTTP branch of `setSecureCookie`:

- They confirm `SameSiteLaxMode` is still applied for local/private-IP HTTP requests.
- They now additionally confirm `Secure = true` even on those requests.

No new test functions are required; the amendment *restores* the existing tests to accuracy.

### 5.2 Regression Check

After the change, run the full `handlers` package test suite:

```bash
cd backend && go test ./internal/api/handlers/... -run TestSetSecureCookie -v
```

All tests matching `TestSetSecureCookie*` must pass. Pay particular attention to:

- `TestSetSecureCookie_HTTP_Loopback_Insecure` — `Secure = true`, `SameSite = Lax`
- `TestSetSecureCookie_HTTPS_Strict` — `Secure = true`, `SameSite = Strict`
- `TestSetSecureCookie_HTTP_PublicIP_Secure` — `Secure = true`, `SameSite = Lax`

### 5.3 No New Tests

A new test asserting `Secure = true` for all request types would be redundant — the amended
assertions across 5 existing tests already cover loopback, private-IPv4 (three RFC 1918 ranges),
and IPv6 ULA. There is no behavioural gap that requires new coverage.

---

## 6. Commit Slicing Strategy

This remediation ships as a **single commit on a single PR**. It touches exactly two files and
changes exactly one category of behaviour (the cookie `Secure` attribute). Splitting it would
create a transient state where the production code and the unit tests are inconsistent.

**Commit message**:

```
fix(auth): always set Secure attribute on auth cookies (CWE-614)

Remove the conditional secure = false path that CodeQL flags as
go/cookie-secure-not-set. The Secure flag is now unconditionally
true on all SetCookie calls.

Browsers apply the localhost exception (Chrome 66+, Firefox 75+,
Safari 14+), so Secure cookies over HTTP to 127.0.0.1 and localhost
work correctly in development. Direct private-IP HTTP access was
never a supported deployment mode; Charon is designed to run behind
Caddy with TLS termination.

Removes the inline codeql[go/cookie-secure-not-set] suppression which
masked the finding without correcting it, and updates the five unit
tests that previously asserted Secure = false for local-network HTTP.
```

**PR title**: `fix(auth): set Secure attribute unconditionally on auth cookies (CWE-614)`

**PR labels**: `security`, `fix`

---

## 7. Acceptance Criteria

A successful remediation satisfies all of the following:

### 7.1 CodeQL CI Passes

1. The `CodeQL - Analyze (go)` workflow job completes with zero results for rule
   `go/cookie-secure-not-set`.
2. No new findings are introduced in `go/cookie-httponly-not-set` or any adjacent cookie rule.
3. The `Verify CodeQL parity guard` step (`check-codeql-parity.sh`) succeeds.

### 7.2 Unit Tests Pass

```bash
cd backend && go test ./internal/api/handlers/... -count=1
```

All tests in the `handlers` package pass, including the five amended `*_Insecure` tests that
now assert `Secure = true`.

### 7.3 Build Passes

```bash
cd backend && go build ./...
```

The backend compiles cleanly with no errors or vet warnings.

### 7.4 No Suppression Comments Remain

```bash
grep -r 'codeql\[go/cookie-secure-not-set\]' backend/
```

Returns no matches. The finding is resolved at the source, not hidden.

### 7.5 SARIF Regenerated

After the CI run, the `codeql-results-go.sarif` file must not contain any result with
`ruleId: go/cookie-secure-not-set`. If the SARIF is maintained as a repository artefact,
regenerate it using the local pre-commit CodeQL scan and commit it alongside the fix.

---

## 8. Out of Scope

- Renaming the five `*_Insecure` test functions. The names are anachronistic but accurate enough
  to remain; renaming is cosmetic and does not affect security posture or CI results.
- Changes to `codeql-config.yml`. A config-level query exclusion would hide the finding across
  the entire repository; fixing the code is strictly preferable.
- Changes to Caddy configuration or TLS termination. The `Secure` cookie attribute is set by
  the Go backend; the proxy layer is not involved.
- Changes to `isLocalRequest` or its helpers. They remain correct and necessary for the
  `SameSite` determination.
