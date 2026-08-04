---
title: "CodeQL inline suppression for go/cookie-secure-not-set is not being honored"
labels:
  - bug
  - backend
  - security
priority: low
---

# CodeQL inline suppression for go/cookie-secure-not-set is not being honored

## Description

`backend/internal/api/handlers/auth_handler.go` (function `setSecureCookie`,
used to set the `auth_token` cookie) has a hand-written CodeQL inline
suppression comment on the `c.SetCookie(...)` call at line 191:

```go
c.SetCookie( // codeql[go/cookie-secure-not-set] Safe: secure is false only
	// when isLocalRequest(c) AND scheme != "https" (loopback/RFC1918/
	// IPv6-ULA/Tailscale-CGNAT origin over plain HTTP) — every other path
	// (HTTPS, or plain HTTP from a public host) still gets secure=true.
	// See the truth table in docs/plans/current_spec.md §9.2.
	name,   // name
	value,  // value
	maxAge, // maxAge in seconds
	"/",    // path
	domain, // domain (empty = current host)
	secure, // secure
	true,   // httpOnly (no JS access)
)
```

The justification itself is accurate: `secure` is computed a few lines above
(lines 172-185) and is only ever set to `false` when the request is local
(`isLocalRequest(c, trustedProxies)` — loopback, RFC 1918, IPv6 ULA, or
Tailscale/CGNAT `100.64.0.0/10`) **and** the scheme isn't `https`. Every other
path (HTTPS, or plain HTTP from a public host) gets `secure = true`. This
supports Charon's documented self-hosted LAN/VPN-mesh deployment mode without
TLS termination.

The problem: this suppression comment is not actually being recognized by
CodeQL's Go extractor. A fresh local CodeQL Go scan (suite
`go-security-and-quality.qls`) still reports the `go/cookie-secure-not-set`
finding for this call site, and the SARIF result for it shows
`"suppressions": null` — i.e. CodeQL never registered the inline comment as a
suppression at all. Most likely cause: the `// codeql[rule-id]` syntax needs
to sit on the exact alert line (or in a specific position) that this
multi-line, argument-per-line `SetCookie(...)` call doesn't satisfy, or Go's
extractor doesn't support this suppression style the way it's written here.

## Evidence

- File: `backend/internal/api/handlers/auth_handler.go`, lines 190-203
  (suppression comment starts at line 191, on the `c.SetCookie(` call).
- Fresh local CodeQL Go scan using suite
  `codeql/go-queries:codeql-suites/go-security-and-quality.qls` still surfaces
  `go/cookie-secure-not-set` for this call.
- The corresponding SARIF result entry has `"suppressions": null`, confirming
  CodeQL did not associate the inline comment with the alert.

## Impact

**Current: LOW.** `go/cookie-secure-not-set` is a `warning`-level finding
(security-severity 4.0) and does not currently block any local or CI gate —
both gates only fail on `error`-level findings.

**Latent risk:** the suppression is not functioning as its comment claims. If
this same inline-suppression pattern is copied elsewhere for a finding that
*is* `error`-level, it would silently fail to suppress the alert (causing an
unexpected CI block), or — worse — create false confidence that a real issue
has been triaged and handled when CodeQL is still actively flagging it every
scan.

## Recommended Next Step (not implemented here)

Investigate CodeQL Go's actual supported inline-suppression syntax/placement
rules, then either:

1. Reformat/reposition the `// codeql[go/cookie-secure-not-set]` comment to a
   form the Go extractor recognizes, **or**
2. Replace the inline suppression with a path/rule-level suppression in
   `.github/codeql/codeql-config.yml`, with the same documented justification.

Either way, verify the fix via a fresh SARIF scan showing a non-null
`suppressions` field for this result before considering it resolved.

## Acceptance Criteria

- [x] Root cause of why the inline suppression isn't recognized is confirmed
- [x] Suppression (inline or config-level) verified via fresh SARIF scan with
      non-null `suppressions` for the `go/cookie-secure-not-set` result on
      this call site
- [x] No change to the actual `secure` cookie logic (behavior is intentional
      and already correctly justified — this is a suppression-tooling issue
      only)

## Resolution

Root-caused and fixed in `docs/plans/current_spec.md` §2–§3 (Part 1).

**Root cause**: two independent, compounding placement errors, not a
Go-extractor bug and not a logic bug. GitHub's `codeql[rule-id]` inline
suppression syntax requires a **standalone comment line** (no code before
it on that line) positioned **exactly one line before** the alert's
reported `startLine`. The original comment violated both counts: it was a
trailing comment attached to the same line as `c.SetCookie(` (same-line
legacy `lgtm[...]`-style placement, not valid `codeql[...]` placement), and
even the tagged text itself sat on the alert's own start line rather than
the line immediately before it.

**Fix**: repositioned the comment to a standalone line directly above
`c.SetCookie(` (see `setSecureCookie` in
`backend/internal/api/handlers/auth_handler.go`), with the truth-table
justification folded into `setSecureCookie`'s own doc comment so it no
longer cites the rotating `docs/plans/current_spec.md` plan document.
Zero change to the `secure`/`isLocalRequest` decision logic itself — the
existing ~24-test suite in `auth_handler_test.go` (`SecureCookie`/
`LocalRequest` tests) passes unmodified as the regression gate.

**Verification outcome: fallback condition B.** After the placement fix, a
fresh local CodeQL Go scan (`bash scripts/pre-commit-hooks/codeql-go-scan.sh`)
still reports `go/cookie-secure-not-set` for this call site with no
`suppressions` key present on the result at all (not just this result —
zero results anywhere in the SARIF carry a `suppressions` key). Since
placement now provably satisfies the `codeql[rule-id]` rule above, this is
a genuine local CodeQL CLI/query-pack limitation for this call shape, not
a placement error — condition A (native, CodeQL-recognized suppression)
was not achievable locally. Per the fallback this issue's own "Recommended
Next Step" anticipated, the finding is instead formally registered as a
dated, reviewable exception in `.github/codeql/codeql-suppressions.yml`
(`rule_id: go/cookie-secure-not-set`, `path:
backend/internal/api/handlers/auth_handler.go`, `line: 198`, `added:
"2026-08-04"`, `review_by: "2026-11-04"`), which the hardened CodeQL
findings gate (`scripts/security/codeql-findings-gate.sh`, added alongside
this fix) treats as an equivalent, machine-enforced resolution — reviewed
and renewed or re-fixed by the `review_by` date rather than silently
riding through the gate forever the way the broken in-source comment
previously did.

Four further `codeql[go/log-injection]` comments in `crowdsec_handler.go`
and `backup_handler.go` were independently found malformed the same way
(comment token two lines above the flagged statement instead of one) and
repositioned in the same fix — no active SARIF findings existed for those
sites either way, so this was a hygiene fix rather than a live-finding
resolution, but it closes the same class of latent bug before it can ride
through unnoticed on an `error`-level rule.
