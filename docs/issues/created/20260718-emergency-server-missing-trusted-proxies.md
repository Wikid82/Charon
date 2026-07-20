---
title: "emergency_server.go's Gin router never calls SetTrustedProxies — audit-log IPs spoofable"

labels:
  - bug
  - security
  - backend

priority: low

milestone: ""

assignees: []
---

# emergency_server.go's Gin router never calls SetTrustedProxies

## Description

`backend/internal/server/emergency_server.go` constructs its own `gin.New()`
engine for the break-glass emergency-access server. Unlike
`internal/server/server.go`'s `NewRouter` — which (as of commit `3b1cd2bb`,
see `docs/plans/current_spec.md` §13) explicitly calls `SetTrustedProxies`,
gated by the `CHARON_TRUSTED_PROXIES` config value, and falls back to
`SetTrustedProxies(nil)` when unset — `emergency_server.go` never calls
`SetTrustedProxies` at all, not even an explicit `nil`. Confirmed via grep:
no such call exists anywhere in the file.

Per Gin's own default behavior, a router that never sets its trusted-proxy
list trusts `X-Forwarded-For` from any peer. This affects every
`ClientIP()`-based call in this file, including the `SecurityReset` handler
and the request-logging middleware (around line 118).

## Why It Matters

The emergency server is a high-trust, high-scrutiny break-glass path — it
exists specifically for situations where normal auth/security enforcement
may be compromised or unavailable. Its audit trail is meant to be one of the
more trustworthy records in the system. But because `ClientIP()` currently
trusts a forged `X-Forwarded-For` header from anyone able to reach the
endpoint, an attacker can make their requests show up in the audit log under
an arbitrary IP address, undermining that audit trail for the one path where
it matters most.

This is **not** an authentication bypass and does not expose any data by
itself — it's an audit-trail integrity gap.

## Suggested Fix Direction

Apply the same pattern this PR just added to `internal/server/server.go`'s
`NewRouter` (commit `3b1cd2bb`; design documented in
`docs/plans/current_spec.md` §13, "Trusted-Proxy Header Hardening for Cookie
Security Decisions") to `emergency_server.go`'s router construction: read
`CHARON_TRUSTED_PROXIES` and call `SetTrustedProxies` with that list (or
explicit `nil` when unset), rather than leaving the call out entirely. Not
fully designed here — just pointing at the existing precedent to mirror.

## Severity

**Low / informational.** This is a rarely-used break-glass path, and the gap
affects audit-log accuracy, not authentication or data exposure.

## Tasks

- [ ] Add an explicit `SetTrustedProxies` call (gated by `CHARON_TRUSTED_PROXIES`, or explicit `nil`) to `emergency_server.go`'s router construction
- [ ] Add a regression test asserting `ClientIP()` in the emergency server does not trust forwarded headers by default
- [ ] Confirm `SecurityReset` and the request-logging middleware (~line 118) record the correct peer IP after the fix

## Acceptance Criteria

- [ ] `emergency_server.go`'s `gin.New()` instance has an explicit `SetTrustedProxies` call matching the pattern in `internal/server/server.go`'s `NewRouter`
- [ ] Default behavior (no `CHARON_TRUSTED_PROXIES` set) trusts no proxies, matching Gin's safe default
- [ ] Existing emergency-server tests continue to pass

## Related Issues

- Found during QA/Supervisor review of PR #1136 (`feature/backuprestore`), commit `3b1cd2bb`'s trusted-proxy security hardening work. That fix correctly scoped itself to `internal/server/server.go`'s `NewRouter` only, since `emergency_server.go` is a separate file with its own router — this issue tracks the follow-up for the latter.
