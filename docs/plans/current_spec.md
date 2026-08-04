# CodeQL Cookie-Suppression Fix + Findings-Gate Hardening

Status: Implemented — all 4 commits landed on
`fix/codeql-cookie-suppression-gate-hardening` (2554c129, 88763c79,
a06ef9ed), verified independently: 8/8 bats tests pass, fresh
`lefthook run codeql` passes end-to-end with the cookie finding shown as
SUPPRESSED (dated, justified, review by 2026-11-04), a deliberate
unsuppressed regression correctly hard-fails the hardened gate, and
`go build`/`go test ./internal/api/handlers/...` pass.
Date: 2026-08-04
Scope: Backend (three handlers — `auth_handler.go`, `crowdsec_handler.go`,
`backup_handler.go` — all comment-only/no-behavior-change, §3.5) +
CI/tooling (gate scripts, CI workflow, new ignore-list mechanism). No
frontend, no DB schema, no API contract changes.

## 0. Grounding / Verification Findings (read this before anything else)

**Revision history of this section, stated plainly.** An earlier revision
of this plan asserted the prior CodeQL path-injection fix (PR #1216,
branch `fix/codeql`) was **unmerged** — `development` HEAD at `7b5c156a`,
`fix/codeql` 7 commits ahead. That finding was independently reproduced
twice by separate review passes and, at the time, both were correct about
what `git log`/`git branch -a` showed *locally*. What neither pass caught
is that this is a **shared working directory** and nobody had run `git
fetch origin` before inspecting it — so both verifications were reading a
stale local `development` ref that predated the actual merge on GitHub.
This was not a fabricated or careless claim; it was a correct read of
out-of-date local state. It's recorded here, rather than silently
corrected, so the plan's own history stays honest about what happened and
why the branching/porting instructions changed between revisions.

**Corrected state, verified directly against `origin` after running `git
fetch origin`:**

| Check | Result |
|---|---|
| `origin/development` HEAD | `379a6401` ("Merge pull request #1216 from Wikid82/fix/codeql") — **PR #1216 is genuinely merged.** |
| `fix/codeql` vs `origin/development` | **0 commits unique to `fix/codeql`** (fully absorbed by the merge) — **14 commits unique to `origin/development`** (real work landed since: dependency bumps, a `feat: add changelog` merge, `refactor: consolidate allowlist-rejection result construction`, etc.) |
| Current branch | `fix/codeql-cookie-suppression-gate-hardening`, created from `origin/development` post-merge and tracking it (`git branch --show-current` / `git status` both confirmed at the start of this revision) |
| `isWithinAllowlistBounds` in `system_permissions_handler.go`? | **No**, confirmed still true post-merge (`grep -rn isWithinAllowlistBounds backend/` → no matches). The mechanism that shipped is `firstAllowlistPrefix` + an inline `strings.HasPrefix` guard at each sink, plus the pre-existing `isWithinAllowlist` (`filepath.Rel`-based) check — both present in `system_permissions_handler.go` today, confirmed by direct read. |
| `docs/issues/codeql-cookie-suppression-not-honored.md` exists on this branch? | **Yes** — it came in as part of the PR #1216 merge (`379a6401`'s diffstat includes it as a 96-line addition) and is present on `origin/development`, hence present here without any porting step. |

**Implication for this plan:** `fix/codeql` no longer needs any special
handling — it is fully absorbed into `origin/development` (0 unique
commits) and is now just an ordinary ancestor, like any other merged
branch. There is nothing left to avoid stacking on or accidentally
inheriting. The "must not reuse `fix/codeql`" branching concern from the
prior revision is moot; see §6 for the (already-complete) branch state.
Everywhere this plan previously said "port `docs/issues/codeql-cookie-
suppression-not-honored.md` from `fix/codeql`" (old §3.4/§7/§13 Commit 3),
that step is now unnecessary — the file is already here.

The second, unrelated stale branch noted in the prior revision,
`fix/cwe-614-secure-cookie-attribute` (local + remote, single unrelated
`package-lock.json`-bump commit `cf81040b`), is unaffected by this
correction — still not a prerequisite, still disregardable, still not
referenced further in this plan.

A **fresh** local CodeQL scan was run at the start of this revision
(`bash scripts/pre-commit-hooks/codeql-go-scan.sh` and
`bash scripts/pre-commit-hooks/codeql-js-scan.sh`, both run to completion;
SARIF files are gitignored, so these are local-only artifacts, not
committed) against the current `fix/codeql-cookie-suppression-gate-
hardening` branch state — i.e., `origin/development` post-#1216-merge plus
the 14 subsequent commits — specifically to re-ground Part 1 and Part 2 in
current data rather than trust the prior planning pass's now-superseded
snapshot:

| SARIF file | Findings | Detail |
|---|---|---|
| `codeql-results-go.sarif` | 1 non-error result, 0 error results | `go/cookie-secure-not-set` at `internal/api/handlers/auth_handler.go`, SARIF region `startLine: 191, endLine: 203`, `"suppressions": null` — identical location/shape to the prior scan |
| `codeql-results-js.sarif` | 0 results | — |

This re-confirms, on current data: (a) the cookie finding is real and
currently unsuppressed, exactly as `docs/issues/codeql-cookie-suppression-
not-honored.md` describes, and — notably — it **shipped all the way
through PR #1216 into `origin/development` and is still live there today**,
not merely "sitting on an unmerged branch" as the prior revision's framing
had it; (b) it is still the **only** non-error CodeQL finding in the repo,
which directly re-confirms Part 2's migration plan (§5.6 — nothing else
needs to move into the new ignore-list mechanism) still holds against
current `development`, not a stale snapshot; (c) the merged path-injection
fix (`firstAllowlistPrefix` + inline `strings.HasPrefix` guards)
introduced **zero** new findings of any kind — confirmed rather than
assumed, since it had already been verified clean before merge and it
would have been easy to let that assumption ride; (d) there are zero
`go/log-injection` results in the fresh scan (0, not suppressed-and-
present — the query simply didn't fire for any of the **six** existing
`codeql[go/log-injection]` comments in `crowdsec_handler.go`/
`backup_handler.go` — a recount during an earlier revision found an even-
earlier draft undercounted this as "five"; see §5.7), of which four are
independently verifiable as malformed by static inspection and are now
folded into Commit 2's scope rather than deferred (see §5.7).

**Bonus finding from re-running the automation check (informs §3.4):** the
prior revision's automation caveat about `docs-to-issues.yml` auto-filing
an issue for this file assumed the file was new to the branch. It isn't
(see above), which already changes that caveat's premise. Investigating
further via `gh run view` on the actual `Convert Docs to Issues` run
triggered by the PR #1216 merge commit (`379a6401`) turned up a more
concrete fact: the workflow's "Detect changed files" step correctly found
`docs/issues/codeql-cookie-suppression-not-honored.md` as a changed file
in that commit, but the "Process issue files" step then **failed to create
an issue for it** — logged error: `Function yaml.safeLoad is removed in
js-yaml 4. Use yaml.load instead, which is now safe by default.` This is a
pre-existing bug in the workflow's own tooling (an incompatible `gray-
matter`/`js-yaml` version pairing from its unpinned `npm install gray-
matter` step), unrelated to anything in this PR, and it's why the file is
still sitting at its original path rather than having been auto-filed and
moved to `docs/issues/created/` already. See §3.4 for what this means for
Commit 3.

---

## 1. Introduction

### 1.1 Overview

Two related pieces of work, both required, in this order:

- **Part 1**: `backend/internal/api/handlers/auth_handler.go`'s
  `go/cookie-secure-not-set` CodeQL finding carries an inline suppression
  comment that CodeQL's Go extractor does not honor (`"suppressions": null`
  in a fresh SARIF scan). Root-cause the placement bug, fix it (or fix the
  underlying logic if inspection reveals it's not as safe as claimed — it
  is not; see §2), and verify via SARIF.
- **Part 2**: The local/CI CodeQL gate (`scripts/pre-commit-hooks/codeql-check-findings.sh`
  and `.github/workflows/codeql.yml`) currently treats all
  `warning`-level CodeQL findings as non-blocking by policy
  (`.github/security-severity-policy.yml`). That policy is exactly how the
  broken cookie suppression rode all the way through an otherwise-clean PR
  into `development` — confirmed merged and still live there today (§0) —
  without ever failing a gate. Close that gap: findings of any
  severity should fail the gate by default, with an explicit, dated,
  reviewable exception mechanism for genuinely accepted/upstream-blocked
  findings — mirroring the existing `.trivyignore`/`.grype.yaml` pattern.

### 1.2 Objective

1. Make the `go/cookie-secure-not-set` finding either (a) genuinely
   suppressed in CodeQL's own terms (non-null `suppressions` in a fresh
   SARIF scan), or (b) formally registered in the new Part 2 ignore-list
   mechanism if native suppression turns out not to be achievable for this
   call shape — either way, resolved for real, not just non-blocking by
   accident of policy.
2. Change the CodeQL findings gate (local pre-commit script + CI workflow)
   so that **any** CodeQL finding (error or warning) fails the gate unless
   it has a matching, valid, non-expired entry in a new repo-local,
   version-controlled ignore-list file — and make local and CI enforce the
   *same* logic via one shared script, closing a duplication/drift risk
   found during this research (§4.1).
3. Close out `docs/issues/codeql-cookie-suppression-not-honored.md`.

### 1.3 Non-goals

- No change to `setSecureCookie`'s actual `secure`/`SameSite` decision
  logic — root-cause analysis (§2) confirms it is sound.
- **Superseded during supervisor review — see §5.7.** An earlier draft of
  this plan excluded the `go/log-injection` suppression comments in
  `crowdsec_handler.go`/`backup_handler.go` on the reasoning that "0
  results in the fresh scan means nothing to root-cause." That reasoning
  was incomplete: Part 1's own placement rule (§2.3) is independently
  checkable by static inspection, with no dependency on whether the query
  currently fires. Re-inspection (§5.7) found 4 of the 6 existing
  `codeql[go/log-injection]` comments are already provably malformed the
  same way the cookie comment was. Those four comment repositions are now
  **in scope**, folded into Commit 2 alongside the cookie fix — not a
  violation of this PR's one-feature boundary, since the fix pattern,
  risk profile (comment-only, zero behavior change), and verification
  method are identical to Part 1's own change. The two already-correctly-
  formed sites (`crowdsec_handler.go:1135`, `:1139`) remain untouched. No
  active `go/log-injection` SARIF finding exists for any of the six sites
  today (confirmed via fresh scan) — this stays a comment-hygiene fix, not
  a vulnerability response, exactly like Part 1.
- No retrofitting of a hard expiry-enforcement script onto the existing
  `.trivyignore`/`.grype.yaml` mechanism (confirmed via grep: no such
  script exists today; their `exp:`/`expiry:` fields are review dates
  followed by convention, not machine-enforced). The new CodeQL mechanism
  *will* enforce expiry programmatically (§5.2) — a deliberate
  improvement, scoped to CodeQL only in this PR.
- No changes to `.grype.yaml`/Trivy tooling.

---

## 2. Part 1 — Root Cause Analysis

### 2.1 The code as it stands today

`backend/internal/api/handlers/auth_handler.go`:

```go
// lines 172-204
func setSecureCookie(c *gin.Context, name, value string, maxAge int, trustedProxies []string) {
	scheme := requestScheme(c, trustedProxies)
	secure := true
	sameSite := http.SameSiteStrictMode
	if scheme != "https" {
		sameSite = http.SameSiteLaxMode
	}

	if isLocalRequest(c, trustedProxies) {
		sameSite = http.SameSiteLaxMode
		if scheme != "https" {
			secure = false
		}
	}

	// Use the host without port for domain
	domain := ""

	c.SetSameSite(sameSite)
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
}

// lines 206-209
func clearSecureCookie(c *gin.Context, name string, trustedProxies []string) {
	setSecureCookie(c, name, "", -1, trustedProxies)
}
```

`secure` (line 174, mutated line 183) is `false` if and only if
**both** hold: `isLocalRequest(c, trustedProxies)` is true, **and**
`requestScheme(c, trustedProxies) != "https"`.

### 2.2 Is the underlying logic actually safe? (root-cause protocol: entry → transformation → persistence → exit)

Traced the full call chain per CLAUDE.md's Root Cause Analysis Protocol,
not just the flagged line:

- **Entry point**: `requestScheme` (lines 54-69) and `isLocalRequest`
  (lines 124-155) both consult `c.Request` — Host, URL, `RemoteAddr`, and
  conditionally `X-Forwarded-Proto`/`X-Forwarded-Host`.
- **Transformation / trust gate**: `isTrustedPeer` (lines 43-52) is the
  single chokepoint both functions call before honoring *any*
  client-suppliable header. It checks the request's **raw TCP
  `RemoteAddr`** — never a header — against the admin-configured
  `trustedProxies` CIDR list (`security.IsIPInCIDRList`). Empty
  `trustedProxies` ⇒ always `false` ("trust nobody"), matching Gin's
  `SetTrustedProxies(nil)` default (documented at lines 38-42).
  - **If the peer is untrusted**: `isLocalRequest` falls back to
    `isLocalOrPrivateHost(peerIP)` using `RemoteAddr` alone (lines
    147-154, with an explicit comment explaining Host/X-Forwarded-Host/
    Origin/Referer are all client-controlled and untrustworthy without a
    trusted peer). An attacker cannot spoof this — `RemoteAddr` is set by
    Go's `net/http` server from the actual TCP connection, not from any
    header.
  - **If the peer is trusted** (i.e., an admin explicitly configured this
    IP/CIDR as a reverse proxy in front of Charon): `isLocalRequest` and
    `requestScheme` honor `X-Forwarded-Host`/`X-Forwarded-Proto`. This
    does let a trusted proxy assert "the original client's host was
    local," which could theoretically be wrong if the proxy itself is
    misconfigured or compromised — but that risk is identical to (and no
    broader than) the trust already extended to `X-Forwarded-Proto` for
    HTTPS detection, and `trustedProxies` is an explicit admin opt-in, not
    a default-on trust. This is consistent with the rest of the codebase's
    threat model for reverse-proxy deployments, not a new gap introduced
    by this cookie logic.
- **Persistence/exit**: `secure=false` only ever reaches `c.SetCookie` for
  a `Set-Cookie` response header — no further propagation.

**Conclusion: the logic is genuinely sound as designed.** This matches
what `docs/issues/codeql-cookie-suppression-not-honored.md` already
concluded ("The justification itself is accurate"). This is a
suppression-tooling problem, not a vulnerability — Part 1's fix must not
touch the `secure`/`isLocalRequest` decision logic.

This is also independently corroborated by existing test coverage:
`backend/internal/api/handlers/auth_handler_test.go` already has ~15
table-style tests exercising this exact truth table (`TestSetSecureCookie_HTTPS_Strict`,
`_HTTP_Lax`, `_HTTP_Loopback_Insecure`, `_ForwardedHTTPS_LocalhostForcesInsecure`,
`_ForwardedHostLocalhostForcesInsecure`, `_HTTP_PrivateIP_Insecure`,
`_HTTP_10Network_Insecure`, `_HTTP_172Network_Insecure`,
`_HTTPS_PrivateIP_Secure`, `_HTTP_IPv6ULA_Insecure`, `_HTTP_PublicIP_Secure`,
`_HTTP_TailscaleCGNAT_Insecure`, plus `TestIsLocalRequest_UntrustedPeer_IgnoresForwardedHost`
and `TestIsLocalRequest_TrustedPeer_HonorsForwardedHost`). Part 1's fix is
comment-only, so this suite is the regression guard and needs no new
logic-level test cases — see §3.

### 2.3 Why the suppression comment isn't recognized (CodeQL syntax research)

Researched GitHub/CodeQL's actual inline-suppression matching rules
(`github/codeql`'s shared `AlertSuppression.qll`, plus GitHub's public
docs/changelog on `codeql[rule-id]` vs legacy `lgtm[rule-id]`):

| Form | Placement rule |
|---|---|
| `// lgtm[rule-id]` (legacy) | Same line as the alert, OR the line immediately after, provided no other code sits between the comment and the alert location. |
| `// codeql[rule-id]` (current, GitHub-recommended) | Must be a **standalone comment line** — no code preceding it on that line — positioned **exactly one line before** the alert's reported start line. Internally: `hasLocationInfo(filepath, _, _, startline - 1, _)`. GitHub explicitly recommends `codeql[...]` over same-line `lgtm[...]` specifically *because* a same-line comment changes that line's content/hash and causes alert churn. |

A single `codeql[rule-id]` comment does **not** spread over an entire
multi-line statement/block — it matches one specific preceding line only.

Cross-referencing against the fresh SARIF captured in §0: the
`go/cookie-secure-not-set` result's primary location is
`internal/api/handlers/auth_handler.go`, region `startLine: 191, endLine:
203` — i.e., CodeQL anchors the alert to the **opening line of the
`c.SetCookie(...)` statement** (line 191, where `c.SetSameSite(sameSite)`
is on line 190 today).

The current comment fails on **two independent counts**, either one of
which alone would be fatal:

1. **It's a trailing/same-line comment attached to code**
   (`c.SetCookie( // codeql[...]`), not a standalone comment line. The
   `codeql[...]` matching rule requires no preceding code on that line —
   this format is actually the *legacy `lgtm[...]`-style* same-line
   placement, not valid `codeql[...]` placement, despite using the
   `codeql[...]` token.
2. **Even ignoring (1), it sits on line 191 itself**, not on line 190 (the
   line immediately *before* 191). `codeql[...]` requires `startline - 1`,
   never the same line.

This fully explains the `"suppressions": null` result without needing to
assume a Go-extractor-specific bug — it's a straightforward, mechanically
reproducible placement error.

### 2.4 A second, pre-existing hygiene bug found in the same code (in scope to fix alongside)

Lines 91-98 (comment above `tailscaleCGNAT`) and line 195 (inside the
`SetCookie` call comment) both cite `docs/plans/current_spec.md §9.1.5`
and `§9.2` respectively as the source of a "truth table" and "threat
model" justification. **Neither section exists in the current
`docs/plans/current_spec.md`** (confirmed by grep — the file currently in
place, before this rewrite, is the path-injection plan from §0, whose §9
is "Acceptance Criteria," not a cookie truth table) — and by design,
`docs/plans/current_spec.md` is explicitly documented as *"Current active
plan"*, a single rotating document that gets fully overwritten by each new
feature's plan (as this very document is doing right now). Citing it from
a permanent code comment as a stable reference is a latent hygiene bug:
the citation was accurate only for as long as one specific historical
version of that file existed, and every future overwrite invalidates it
silently (no build/lint catches a stale prose cross-reference). Per
CLAUDE.md's "actively refactor code you encounter, even outside of your
immediate task scope" and DRY/READABLE guidance, Part 1's fix removes
these two dangling references and folds the truth table fully into the
(already largely self-contained) `setSecureCookie` doc comment at lines
157-171, so the justification no longer depends on any file outside
`auth_handler.go` itself.

---

## 3. Part 1 — Proposed Fix

### 3.1 Decision

The logic is safe (§2.2); only the suppression mechanism is broken (§2.3).
Fix the placement, do not touch behavior. Provide an explicit fallback to
Part 2's new ignore-list mechanism in case native suppression still fails
after correct placement (verified via a fresh scan before merge — see §3.3).

### 3.2 Exact change — `backend/internal/api/handlers/auth_handler.go`

Replace lines 186-204 (`// Use the host without port...` through the
closing `)` of `SetCookie`) with:

```go
	// Use the host without port for domain
	domain := ""

	c.SetSameSite(sameSite)

	// secure is false only when isLocalRequest(c) AND scheme != "https"
	// (loopback/RFC1918/IPv6-ULA/Tailscale-CGNAT origin over plain HTTP) —
	// every other path (HTTPS, or plain HTTP from a public host) still
	// gets secure=true. See the doc comment on setSecureCookie above for
	// the full truth table and threat-model justification.
	// codeql[go/cookie-secure-not-set]
	c.SetCookie(
		name,   // name
		value,  // value
		maxAge, // maxAge in seconds
		"/",    // path
		domain, // domain (empty = current host)
		secure, // secure
		true,   // httpOnly (no JS access)
	)
```

Notes for the implementer (`backend-dev`):

- The `codeql[go/cookie-secure-not-set]` comment **must be the line
  immediately before** `c.SetCookie(` with nothing else on that line. As
  written above it is — but insertions/edits elsewhere in this function
  before merge could shift line numbers; this is a *relative* placement
  rule (line N-1 to whatever line the call statement lands on), not tied
  to line 191 specifically, so it is robust to reformatting as long as the
  comment stays the line directly above the call.
- Also remove the two dangling `docs/plans/current_spec.md §9.1.5` / `§9.2`
  references described in §2.4: the comment block above `tailscaleCGNAT`
  (currently ends "...consistent with Charon's self-hosted/LAN/VPN-mesh
  threat model (see docs/plans/current_spec.md §9.1.5)."   → drop the
  parenthetical, keep the sentence) and the one folded into the `SetCookie`
  comment above (already removed in the replacement block shown above).
- No changes to `requestScheme`, `isLocalRequest`, `isTrustedPeer`,
  `isLocalOrPrivateHost`, `tailscaleCGNAT`, or `clearSecureCookie`.

### 3.3 Verification plan

1. Regenerate SARIF locally: `lefthook run codeql` (runs go-scan → js-scan
   → check-findings → parity in sequence per `lefthook.yml`), or directly
   `bash scripts/pre-commit-hooks/codeql-go-scan.sh` to just refresh
   `codeql-results-go.sarif`.
2. Inspect the specific result:
   ```bash
   jq '.runs[].results[] | select(.ruleId=="go/cookie-secure-not-set")' codeql-results-go.sarif
   ```
3. **Success condition A (preferred)**: the result is still present (a
   suppressed result is not removed from `results`, it's annotated) with a
   non-null `suppressions` array, e.g. `[{"kind": "inSource", ...}]`. This
   is genuine, CodeQL-native suppression — verifies both the placement fix
   and closes the acceptance criteria in
   `docs/issues/codeql-cookie-suppression-not-honored.md` as originally
   scoped.
4. **Fallback condition B**: if `suppressions` is *still* null after
   correct placement (i.e., a genuine Go-extractor limitation with this
   specific call shape, not a placement error) — do not keep
   reformatting speculatively. Instead, register this exact finding
   (`ruleId=go/cookie-secure-not-set`, `path=backend/internal/api/handlers/auth_handler.go`,
   current `startLine`) in the new `.github/codeql/codeql-suppressions.yml`
   from Part 2 (§5), with the same justification text, dated today, and a
   `review_by` date. Either outcome is an acceptable, real resolution —
   what's not acceptable is leaving it unresolved as today.
5. Confirm no regression: `go test ./backend/internal/api/handlers/... -run 'SecureCookie|LocalRequest'` (existing suite from §2.2) plus the full `go test ./...` gate.
6. Confirm no unrelated CodeQL delta: diff the full findings list
   before/after (`jq '[.runs[].results[].ruleId] | sort'`) to confirm the
   only change is this one result's `suppressions` field (or its move
   into the ignore-list under fallback condition B) — nothing else should
   appear or disappear.

### 3.4 Closing out the tracked issue

**No porting needed.** Per §0's corrected grounding, `docs/issues/codeql-
cookie-suppression-not-honored.md` came in as part of the PR #1216 merge
(`379a6401`) and is already present on this branch — confirmed directly
(`ls docs/issues/codeql-cookie-suppression-not-honored.md` succeeds, file
read in full at the start of this revision). The prior revision's "port
the file from `fix/codeql` via `git checkout fix/codeql -- <path>`"
sub-step is unnecessary and removed. Commit 3 is now a single, simple
step:

Once verification (§3.3) succeeds under either condition A or B, update
the existing file:

- Check off all three "Acceptance Criteria" boxes.
- Add a short "Resolution" section stating which condition (A or B)
  applied, the actual root cause (§2.3 — standalone-comment + off-by-one
  line placement, not a Go-extractor bug), and a link/reference to the
  commit that fixed it.
- Do **not** delete the file — it's useful history for anyone who searches
  for this pattern again (e.g., if the `go/log-injection` comments touched
  in §5.7 ever start firing again elsewhere and need the same treatment).
  Move it into a "resolved" state rather than removing it, consistent with
  how this repo already treats other `docs/issues/*.md` entries (checked
  `docs/issues/README.md` conventions — resolved issues stay in place with
  their checklist completed, not deleted).

**Automation caveat — re-evaluated against actual trigger/diff logic, not
assumed.** The prior revision's concern was that porting the file in as a
*new* file would trip `.github/workflows/docs-to-issues.yml`'s auto-file
automation. That premise no longer applies (nothing is being newly added),
but the underlying question — does *editing* an already-tracked file under
`docs/issues/` still trigger the automation — needed its own answer, so
the workflow file and its actual recent run history were both read
directly rather than assumed:

- **Trigger/diff logic** (`.github/workflows/docs-to-issues.yml`, "Detect
  changed files" step): fires on `workflow_run` completion of "Docker
  Build, Publish & Test", then diffs the *single triggering commit*
  (`getCommit(ref: head_sha)`) for files under `docs/issues/`, excluding
  `docs/issues/created/**`, `_TEMPLATE`, `README`, non-`.md` files, and
  anything with `status === 'removed'`. **Modified files are not
  excluded** — only `removed` status is filtered out — so editing this
  file in Commit 3 and having that land in a `development`-bound merge
  commit will, in principle, still surface it to "Process issue files" as
  a changed file. The caveat is not automatically moot just because the
  file already exists.
- **But**: checked what actually happened the *last* time this exact file
  was surfaced to the automation — the PR #1216 merge commit itself
  (`379a6401`), via `gh run view` on the resulting `Convert Docs to
  Issues` run. "Detect changed files" correctly found
  `docs/issues/codeql-cookie-suppression-not-honored.md`, but "Process
  issue files" then **errored out** before creating anything: `Function
  yaml.safeLoad is removed in js-yaml 4. Use yaml.load instead, which is
  now safe by default.` This is a pre-existing bug in the workflow's own
  tooling — its `npm install gray-matter` step has no lockfile/version
  pin, and picked up a `gray-matter`/`js-yaml` pairing where `gray-
  matter`'s frontmatter parser still calls the removed `js-yaml`
  v3 API — unrelated to this PR's content and not something introduced by
  this plan's work. No issue was created and the file was never moved to
  `docs/issues/created/`, which is also why it's still sitting at its
  original path today rather than already resolved-and-archived.
- **Net effect for this PR**: because that failure is a dependency-version
  problem in the automation itself (not keyed to file content), the same
  failure will most likely reproduce identically the next time "Docker
  Build, Publish & Test" completes against a commit that touches this file
  — including the Commit 3 edit here — so the realistic expectation is
  another silent-to-us, logged-as-a-warning failure, not a duplicate
  issue. **This is not a guarantee**, though: if the workflow's `gray-
  matter`/`js-yaml` pinning is fixed independently (by anyone, in an
  unrelated PR) before this PR's Commit 3 lands and its triggering Docker
  build completes, the automation would then successfully process the
  edited file and — since the script has no dedup-by-filename or
  dedup-by-existing-issue check — could file a fresh issue for it. That
  residual risk is small and outside this PR's control, so the mitigation
  is unchanged in spirit from the prior revision's: if a
  `docs-to-issues.yml`-created issue does appear for this file after
  Commit 3 merges, close it with a comment cross-referencing this PR
  ("Resolved by <PR link> — see the file's own Resolution section") rather
  than leaving it open as an untriaged duplicate. The `gray-matter`/`js-
  yaml` bug itself is out of scope for this PR (unrelated tooling, not
  CodeQL) — worth a short follow-up issue for whoever owns
  `docs-to-issues.yml`, but not blocking here.

### 3.5 Additional comment-placement fixes (folded in per §5.7)

Independently re-read all six existing `codeql[go/log-injection]` sites
(not just the cited five — see §5.7 for the corrected count) and applied
§2.3's placement rule by static inspection. Four are malformed the same
way the cookie comment was — a standalone comment block whose
`codeql[...]`-tagged line sits `startline - 2` (two lines above the
statement it annotates) rather than `startline - 1`:

| Site | Current structure | Fix |
|---|---|---|
| `backend/internal/api/handlers/crowdsec_handler.go:1121-1123` | 2-line comment (1121 tagged, 1122 explanatory), statement at 1123 | Collapse/reorder so the tagged line lands at line 1122 (directly above the statement) |
| `backend/internal/api/handlers/crowdsec_handler.go:1129-1131` | 2-line comment (1129 tagged, 1130 explanatory), statement at 1131 | Same treatment, tagged line lands directly above the statement |
| `backend/internal/api/handlers/crowdsec_handler.go:1235-1237` | 2-line comment (1235 tagged, 1236 explanatory), statement at 1237 | Same treatment, tagged line lands directly above the statement |
| `backend/internal/api/handlers/backup_handler.go:286-288` | 2-line comment (286 tagged, 287 explanatory), statement starts at 288 (multi-line chained `middleware.GetRequestLogger(c).WithField(...)` call) | Same treatment, tagged line lands directly above line 288 |

The two correctly-formed sites (`crowdsec_handler.go:1135`, single-line
comment directly above a single-line statement at 1136; and `:1139`,
same shape above 1140) are left untouched — they already satisfy §2.3's
rule.

Implementer note: because these are today 2-line comment blocks (an
explanatory second line follows the `codeql[...]`-tagged line), the fix
isn't simply "move the block up one line" — specifically the *tagged*
line must land at `startline - 1`. Simplest, most consistent option
(matches §3.2's cookie-comment fix pattern): reorder so the explanatory
line comes first and the `codeql[...]`-tagged line comes second,
immediately adjacent to the statement it annotates. No change to any
logged field, `util.SanitizeForLog(...)` call, or control flow at any of
the four sites — comment reposition only, same risk profile as §3.2.

Verification: same method as §3.3 — a fresh SARIF scan should continue to
show zero `go/log-injection` results for all six sites (comment-only
reposition introduces no new taint paths), confirming no regression; if
the query ever does fire for one of these sites in the future, its
`suppressions` field should now be non-null given the corrected
placement, per §2.3's rule.

---

## 4. Part 2 — Current Gate Behavior (research findings)

### 4.1 Two independent, hand-duplicated implementations of the same logic

`scripts/pre-commit-hooks/codeql-check-findings.sh` (manual — invoked via
`lefthook run codeql`, step `3-check-findings`, **not** part of the
blocking pre-commit pipeline) computes, per SARIF file, an
"effective level" for each result via a fallback chain
(`result.level` → `rules[ruleIndex].defaultConfiguration.level` → lookup
by `ruleId` in the rules array → `""`), then:

- `BLOCKING_COUNT` = results where effective level == `"error"`.
- `WARNING_COUNT` = results where effective level == `"warning"` (printed,
  never blocks).
- Exits 1 only if `BLOCKING_COUNT > 0` (or SARIF file missing, or `jq`
  missing).

`.github/workflows/codeql.yml` has its **own, separately-maintained copy**
of the identical effective-level jq expression, duplicated across two
steps ("Check CodeQL Results" — report-only, writes
`$GITHUB_STEP_SUMMARY`; "Fail on High-Severity Findings" — computes
`ERROR_COUNT` with the same jq and fails the job if `> 0`). It does **not**
call `codeql-check-findings.sh` at all — it's a fully independent
implementation that happens to compute the same thing today.

This is precisely the "two independent hand-maintained
[implementations] that silently drifted" failure pattern already on
record for this repo (Orthrus's dual Docker-API allowlists, tracked in
GH #1160/#1161) — the exact mechanism by which the cookie finding's
suppression bug was able to ride all the way through PR #1216 into
`development` cleanly, where it remains live today (§0's fresh scan):
nothing ever hard-failed because both independent copies agreed the
finding was merely a non-blocking warning.

`scripts/ci/check-codeql-parity.sh` (lefthook step `4-parity-check`,
also invoked as its own CI step before `Initialize CodeQL` in
`codeql.yml`, matrix-gated to the `go` leg) exists specifically to prevent
exactly this class of drift — but today it only asserts: workflow trigger
branches, query-suite pinning (`security-and-quality`, not
`security-experimental`), and `.vscode/tasks.json` label/command parity
with the pre-commit scripts. **It does not assert anything about the
blocking-logic jq being identical between the local script and the CI
workflow** — that's the actual gap that let this specific drift happen.
Part 2 closes this structurally (§5.4), not just by convention.

### 4.2 Policy source of truth

`.github/security-severity-policy.yml` (`version: 1`,
`effective_date: 2026-02-25`) is the documented, authoritative policy both
scripts are implementing:

```yaml
codeql:
  severity_mapping:
    error: high_or_critical
    warning: medium_or_lower
    note: informational
  blocking_levels:
    - error
  warning_policy:
    default_action: report
    escalation_high_signal_rule_ids:
      - go/request-forgery
      - js/missing-rate-limiting
      - js/insecure-randomness
```

`blocking_levels: [error]` and `warning_policy.default_action: report` are
exactly the "warnings don't block" rule the user's new standing policy
wants overturned. This file must change (§5.3) — it's the single
documented source both the shell logic and (per its stated `scope`) any
future contributor should consult.

### 4.3 A second, related gap found during this research (fold into Part 2, not a separate PR)

Neither the local script, the CI workflow steps, nor the policy file
consult SARIF `suppressions` at all today. A genuinely-suppressed
**error**-level finding (non-null `suppressions`, e.g. a correctly-placed
`codeql[...]` comment on a real error-level rule) would still count toward
`BLOCKING_COUNT`/`ERROR_COUNT` and fail the gate today — a false failure.
This is directly adjacent to Part 1 (native suppression only becomes a
useful, low-overhead mechanism if the gate actually respects it) and small
enough to fix in the same pass rather than opening a second PR for it —
folded into the shared gate script's design in §5.4.

### 4.4 Existing ignore-list pattern (`.trivyignore` / `.grype.yaml`)

Confirmed both files exist at repo root. Format:

- **`.trivyignore`**: flat text, one entry per line — either a bare path
  glob (e.g. `backend/internal/api/routes/keys/hecate-ca.key`) or a bare
  CVE/GHSA ID (e.g. `CVE-2026-25793`). Each ID is preceded by a `#`-comment
  block: title, `Severity:` + package, root-cause/no-fix explanation,
  exploitability/reachability argument specific to Charon's deployment,
  `Review by: <date>`, `# exp: <date>` (machine-parseable-looking but **no
  script actually parses/enforces it** — confirmed via repo-wide grep for
  `expiry|exp:|stale` across `scripts/` and `.github/workflows/`; nothing
  reads `.trivyignore`'s `exp:` lines programmatically today). Consumed
  natively by Trivy's `--ignorefile`/`trivyignores:` input.
- **`.grype.yaml`**: structured YAML under `ignore:`, each entry
  `vulnerability:` + `package: {name, version, type}` + `reason:` (long
  free-text block, same content as the `.trivyignore` comment) +
  `expiry: "<date>"` (also not programmatically enforced — same grep
  result). Consumed natively by Grype's own config-file convention.
- No CI step currently flags stale/expired entries in either file — review
  dates are a human-process convention today, not a gate.

---

## 5. Part 2 — Proposed Design

### 5.1 Ignore-list mechanism: recommendation and reasoning

**Considered**: rely on GitHub's native CodeQL alert-dismissal (Security
tab: dismiss with reason — false positive / won't fix / used in tests).

**Rejected as the primary mechanism** (though it remains a fine
*complementary* action once native in-source suppression is picked up
automatically — see §5.4's suppression-awareness fix, which makes
dismissal happen for free when a suppression comment is valid). Reasons:

1. **Local gate has no path to it.** `codeql-check-findings.sh` runs
   entirely offline against a freshly-generated local SARIF file — it has
   no GitHub API/auth dependency today, and adding one (to query
   dismissed-alert status) would mean either shipping `gh` CLI auth
   requirements into every dev's pre-commit flow, or the local and CI
   gates enforcing *different* policies depending on network access. Both
   are worse than what exists today.
2. **No enforced dated review.** GH's dismissal UI captures a reason enum
   and an optional comment, dismisser identity, and timestamp, but nothing
   analogous to `review_by`/`expiry` that a script can check and fail on.
3. **Not portable/legible in PR review.** Dismissal happens out-of-band in
   a UI, not as a diff in the PR that requests the exception — this repo's
   whole existing pattern (`.trivyignore`, `.grype.yaml`) is explicitly
   the opposite: a reviewable, git-blamable text file changed in the same
   PR as the code it excuses.
4. **Keying granularity.** GH dismissal is keyed by an opaque per-repo
   alert number. CodeQL findings need a composite key (rule + file + line)
   since one rule can have many call-site instances with very different
   risk profiles — a flat "dismiss this rule" or "dismiss this alert
   number" doesn't compose cleanly with reviewing a text diff.

**Decision**: repo-local, version-controlled YAML file:
`.github/codeql/codeql-suppressions.yml` — co-located with the existing
`.github/codeql/codeql-config.yml` (CodeQL-specific policy artifacts
already live there; repo root is reserved for the Trivy/Grype pair by
existing convention, and there's no reason to further crowd repo root).

### 5.2 File format

```yaml
# CodeQL findings ignore-list.
#
# Policy (.github/security-severity-policy.yml): CodeQL findings of ANY
# severity fail the local (lefthook `codeql`) and CI (.github/workflows/
# codeql.yml) gates by default. A finding is only allowed to pass if it
# has a matching, non-expired entry here.
#
# Matching: an entry suppresses a SARIF result iff
#   result.ruleId == rule_id
#   AND result.locations[0].physicalLocation.artifactLocation.uri == path
#   AND result.locations[0].physicalLocation.region.startLine is within
#       [line] or [line_range.start, line_range.end]
#
# Expiry: entries past review_by are treated as EXPIRED and stop
# suppressing (the finding reverts to blocking, printed distinctly from a
# brand-new/never-triaged finding so it's obvious a renewal decision is
# needed). Bump review_by (with a dated "Extended ..." note in the reason,
# mirroring .trivyignore's convention) or fix the underlying issue.
#
# This mechanism intentionally goes one step further than the existing
# .trivyignore/.grype.yaml pattern: it is the *first* ignore-list in this
# repo whose expiry is actually machine-enforced (see
# scripts/security/codeql-findings-gate.sh). See docs/plans/current_spec.md
# [dated 2026-08-04] for why — not retrofitted onto Trivy/Grype in the
# same change.
suppressions: []
  # Example entry (uncomment/copy when a real exception is needed):
  # - rule_id: go/cookie-secure-not-set
  #   path: backend/internal/api/handlers/auth_handler.go
  #   line: 191
  #   reason: >
  #     Secure is false only for loopback/RFC1918/IPv6-ULA/Tailscale-CGNAT
  #     origins over plain HTTP, by design, for Charon's documented
  #     self-hosted LAN/VPN-mesh deployment mode without TLS termination.
  #     See setSecureCookie's doc comment for the full truth table.
  #   added: "2026-08-04"
  #   review_by: "2026-11-04"
```

Per §0/§5.6, this file ships with an **empty `suppressions: []` list** —
the one finding this whole plan exists to fix is expected to be resolved
natively in Part 1 (condition A), not routed through this file. The
schema/example stays as a comment for the next time it's actually needed.

**Line-drift guidance**: an entry keyed by a bare `line:` stops matching
the moment a later, unrelated code edit shifts line numbers in the same
file — the finding then silently reverts to blocking, indistinguishable
from a brand-new finding unless the gate script itself surfaces the
distinction (it does — see §5.4's `LIKELY-STALE ENTRY` case, and §12's
Risks table). For any entry expected to survive routine refactors — which
is most entries, since code around a suppressed line rarely stays frozen
forever — prefer `line_range: {start: ..., end: ...}` (already in the
schema above) over a bare `line:` pin, sized generously enough to absorb
minor reformatting. Reserve bare `line:` for genuinely one-off sites where
drift risk is low (e.g. the last line of a function that's unlikely to
grow).

### 5.3 `.github/security-severity-policy.yml` changes

```yaml
codeql:
  severity_mapping:
    error: high_or_critical
    warning: medium_or_lower
    note: informational
  # CHANGED 2026-08-04: findings of any level block by default. Prior
  # policy (blocking_levels: [error] only, warnings "report") let a
  # broken CodeQL suppression ship unnoticed — see docs/plans/current_spec.md.
  blocking_levels:
    - error
    - warning
    - note
  exceptions:
    mechanism: .github/codeql/codeql-suppressions.yml
    description: >
      A finding at any level is excluded from blocking only if it has a
      matching, non-expired entry in the file above, OR the SARIF result
      itself carries a non-null `suppressions` field (a correctly-placed
      in-source `codeql[rule-id]` comment CodeQL's own extractor
      recognized). Expired or unmatched findings always block.
```

`warning_policy`/`escalation_high_signal_rule_ids` is removed — escalation
tiers no longer apply once every level blocks by default; the
`codeql-suppressions.yml` review cadence replaces it.

### 5.4 Shared gate script (closes §4.1's duplication and §4.3's suppression-awareness gap)

New file: `scripts/security/codeql-findings-gate.sh`.

**Inputs**: `$1` = SARIF file path, `$2` = language label (for messages).
**Behavior** (single source of truth, used by both local and CI):

1. Load SARIF; compute each result's effective level via the existing
   fallback chain (unchanged logic, just relocated).
2. For each result:
   - If `result.suppressions` is non-null → **natively suppressed**;
     excluded from blocking; printed as `SUPPRESSED (in-source): <rule>
     <file>:<line>`.
   - Else, look up `(ruleId, path, line)` against
     `.github/codeql/codeql-suppressions.yml`:
     - **Full match** (`rule_id` + `path` agree, and the result's
       `startLine` falls within the entry's `line` or `line_range`),
       `review_by` in the future → excluded from blocking; printed as
       `SUPPRESSED (codeql-suppressions.yml, reason: "<reason>", review by
       <date>): <rule> <file>:<line>`.
     - Full match, `review_by` in the past → **blocking**; printed as
       `EXPIRED SUPPRESSION (review_by <date> has passed — renew or fix):
       <rule> <file>:<line>`.
     - **Partial match** — an entry exists with the same `rule_id` +
       `path`, but the result's `startLine` falls *outside* that entry's
       `line`/`line_range` (the entry has almost certainly drifted, most
       likely because a later code edit shifted line numbers out from
       under a `line`-keyed entry — see §12's line-drift risk) →
       **blocking**; printed as `LIKELY-STALE ENTRY (line moved? check
       codeql-suppressions.yml): <rule> <file>:<line>` — deliberately
       distinct from the generic `NEW FINDING` message below, since the
       rule+path partial match is already computed as part of the lookup
       and costs nothing extra to surface distinctly.
     - **No match at all** (no entry for this `rule_id`+`path` pair) →
       **blocking**; printed as `NEW FINDING (no exception on file):
       <effective-level> <rule> <file>:<line>`.
3. Exit non-zero if any result is blocking; 0 otherwise. Print a final
   summary count of suppressed vs. blocking vs. total.

**`scripts/pre-commit-hooks/codeql-check-findings.sh`** becomes a thin
wrapper: drop its own `BLOCKING_COUNT`/`WARNING_COUNT` jq blocks, call
`scripts/security/codeql-findings-gate.sh codeql-results-go.sarif go` and
the same for the JS SARIF (keeping its existing dual-filename fallback for
`codeql-results-js.sarif` / legacy `codeql-results-javascript.sarif`),
`FAILED=1` if either call exits non-zero.

**`.github/workflows/codeql.yml`** — "Check CodeQL Results" (report step)
and "Fail on High-Severity Findings" (blocking step) both call
`scripts/security/codeql-findings-gate.sh sarif-results/${{
matrix.language }}/<file>.sarif ${{ matrix.language }}` instead of their
own inline jq; the report step additionally pipes the script's output into
`$GITHUB_STEP_SUMMARY`, the blocking step just checks its exit code.

### 5.5 `scripts/ci/check-codeql-parity.sh` — new assertion

Add a check that both `scripts/pre-commit-hooks/codeql-check-findings.sh`
and `.github/workflows/codeql.yml` reference the shared script by its
canonical path (`grep -Fq 'scripts/security/codeql-findings-gate.sh'` in
each), failing parity with a clear message if either has drifted back to
inline/duplicated logic. This is what makes "local and CI enforce the
same policy" a structurally-checked invariant instead of a convention that
can silently drift again (§4.1).

### 5.6 Migration plan

Per §0's fresh-scan snapshot: **zero** findings currently need migrating
into `.github/codeql/codeql-suppressions.yml` beyond what Part 1 resolves
natively. `codeql-results-go.sarif` had exactly one non-error result
(the cookie finding, resolved in Part 1) and zero error-level results;
`codeql-results-js.sarif` had zero results of any level. The file ships
with `suppressions: []` (§5.2).

**Re-confirmed against current `development`, not the original planning
snapshot.** The prior revision's scan was taken when local `development`
was believed to be at `7b5c156a` (pre-#1216-merge, per §0's now-corrected
history). This revision re-ran both scans (`codeql-go-scan.sh`,
`codeql-js-scan.sh`) against the actual current branch state — post-#1216-
merge plus 14 further commits — and got the **identical** result: same one
non-error finding, same location, same zero JS results, and zero new
findings introduced by the now-merged path-injection fix. The migration
assumption below holds against real current data, not a stale snapshot —
but the re-verify-at-implementation-time step is still required, since
more commits will land on `development` between this planning pass and
actual implementation.

**Must re-verify at implementation time, not assume from this planning
snapshot** — `development` will have moved on by the time this is
implemented. Implementation step: after wiring the new gate logic but
*before* flipping `blocking_levels` to include `warning`/`note`, run a
fresh `lefthook run codeql` and inspect the full findings list:

```bash
jq -r '.runs[].results[] | "\(.ruleId) \(.locations[0].physicalLocation.artifactLocation.uri):\(.locations[0].physicalLocation.region.startLine)"' codeql-results-go.sarif codeql-results-js.sarif | sort -u
```

If this list is empty (Part 1's fix landed, no new findings appeared since
this planning pass), proceed with an empty `codeql-suppressions.yml` as
designed. If it is *not* empty, each remaining finding needs either a real
code fix (preferred, if trivial) or a dated, justified
`codeql-suppressions.yml` entry before the stricter gate is turned on —
never a silent drop and never a bare hard-fail with no documented path
forward.

### 5.7 The six existing `go/log-injection` suppression comments — independently verified and folded into Commit 2's scope

Supervisor review flagged that an earlier draft of this section
under-reasoned this: "0 SARIF results means nothing to root-cause" treats
SARIF output as the only source of truth, but Part 1's own newly
established placement rule (§2.3 — a standalone `codeql[rule-id]` comment
must sit at exactly `startline - 1`, with no code preceding it on that
line) is independently checkable by static inspection, with no dependency
on whether the query currently fires. Re-read all six actual sites
directly (`backend/internal/api/handlers/crowdsec_handler.go` lines 1121,
1129, 1135, 1139, 1235; `backend/internal/api/handlers/backup_handler.go`
line 286) and applied that rule by hand:

| Site | Structure | Comment token line vs. statement start | Verdict |
|---|---|---|---|
| `crowdsec_handler.go:1121-1123` | 2-line standalone comment (1121-1122), statement at 1123 | token line 1121 = `startline - 2` | **Malformed** |
| `crowdsec_handler.go:1129-1131` | 2-line standalone comment (1129-1130), statement at 1131 | token line 1129 = `startline - 2` | **Malformed** |
| `crowdsec_handler.go:1135-1136` | 1-line standalone comment, statement immediately below | token line 1135 = `startline - 1` | Correctly formed |
| `crowdsec_handler.go:1139-1140` | 1-line standalone comment, statement immediately below | token line 1139 = `startline - 1` | Correctly formed |
| `crowdsec_handler.go:1235-1237` | 2-line standalone comment (1235-1236), statement at 1237 | token line 1235 = `startline - 2` | **Malformed** |
| `backup_handler.go:286-288` | 2-line standalone comment (286-287), statement starts at 288 (multi-line chained call) | token line 286 = `startline - 2` | **Malformed** |

**Verified count: six sites total**, not five — this plan's earlier draft
and the reviewer's own summary both undercounted this as "five" (`grep -n
"codeql\[go/log-injection\]"` across both files returns six matches: five
in `crowdsec_handler.go`, one in `backup_handler.go`). Of those six,
**four are malformed** (1121, 1129, 1235 in `crowdsec_handler.go`; 286 in
`backup_handler.go` — all share the identical "2-line standalone comment
block, token line 2 above the statement" shape) and **two are correctly
formed** (1135, 1139 in `crowdsec_handler.go` — single-line comment
directly above a single-line statement).

A fresh local SARIF scan (`codeql-results-go.sarif`, same scan referenced
in §0) confirms **zero** `go/log-injection` results for any of these six
sites today (`jq '[.runs[].results[] | select(.ruleId=="go/log-injection")]
| length'` → `0`). So unlike Part 1's cookie finding, there is no
`"suppressions": null` result actively riding through the old policy for
these — the malformation is latent, not currently exploited by the old
gate's gap. That said, per CLAUDE.md's "actively refactor code you
encounter, even outside of your immediate task scope" — a principle this
same plan already invokes in §2.4 for two dangling doc cross-references —
and given the fix is mechanically identical to Part 1 (reposition a
standalone comment to `startline - 1`, zero behavior change, same
verification method), there is no good reason to defer this to a
fast-follow issue while it's already been independently root-caused here.

**Decision: fold the four malformed-site fixes into Commit 2's scope**
(§3.5), alongside the cookie fix, rather than exclude them into a separate
issue. The two already-correct sites (1135, 1139) are left untouched. This
changes Commit 2's file scope (§7, §13) from one file to three, but not
its risk profile: every change in Commit 2 remains comment-only, zero
behavior change, verified the same way (inspect placement, confirm no
diff to any logged field, sanitization call, or control flow).

---

## 6. Branching

**Done, not a future step.** Per CLAUDE.md, no worktrees — work happens
directly on a branch, and per §0's corrected grounding that branch already
exists and is already correctly positioned:

- **Branch**: `fix/codeql-cookie-suppression-gate-hardening`
- **Created from**: `origin/development` **after** the PR #1216 merge
  (`379a6401`) — i.e., it already includes the merged path-injection fix
  and the 14 commits that landed after it — not from the stale pre-merge
  ref the prior revision was working against.
- **Tracking**: confirmed via `git status` → "Your branch is up to date
  with 'origin/development'."
- **No further branch setup required.** There is no `fix/codeql` or
  `fix/cwe-614-secure-cookie-attribute` reuse risk to guard against — both
  were already ruled out in the prior revision, and `fix/codeql` is now
  moot as a distinct concern anyway (§0: 0 commits unique to it, fully
  absorbed into `development`). Implementation begins directly on the
  current branch; there is no `git checkout -b` step left to perform.

---

## 7. Files Affected

| File | Part | Change |
|---|---|---|
| `backend/internal/api/handlers/auth_handler.go` | 1 | Reposition suppression comment (standalone line, correct offset); remove 2 dangling `docs/plans/current_spec.md` cross-references. No logic change. |
| `backend/internal/api/handlers/crowdsec_handler.go` | 1 | Reposition 3 malformed `codeql[go/log-injection]` standalone comments (lines 1121, 1129, 1235) to `startline-1`. No logic/behavior change. Folded in per §5.7/§3.5. |
| `backend/internal/api/handlers/backup_handler.go` | 1 | Reposition 1 malformed `codeql[go/log-injection]` standalone comment (line 286) to `startline-1`. No logic/behavior change. Folded in per §5.7/§3.5. |
| `docs/issues/codeql-cookie-suppression-not-honored.md` | 1 | **Already present on this branch** (came in with the PR #1216 merge, §0 — no porting needed). Mark acceptance criteria complete and add Resolution section. Not deleted. `docs-to-issues.yml` automation is currently broken for this file (verified `js-yaml`-compat error on the last run that saw it, §3.4) so no duplicate auto-filed issue is expected, but see §3.4's residual-risk note if that automation bug is fixed independently before this PR merges. |
| `docs/plans/current_spec.md` | 1 & 2 | This plan (already written by this planning pass). |
| `.github/codeql/codeql-suppressions.yml` | 2 | **New file.** Empty ignore-list with documented schema. |
| `scripts/security/codeql-findings-gate.sh` | 2 | **New file.** Shared blocking-logic script (SARIF + suppressions.yml aware). |
| `scripts/security/tests/codeql-findings-gate.bats` | 2 | **New file.** bats-core fixture tests for the shared gate script (7 cases, §9.2). |
| `scripts/security/testdata/*.sarif`, `scripts/security/testdata/*suppressions*.yml` | 2 | **New files.** Fixture SARIF/suppressions inputs consumed by the bats tests above. |
| `.gitignore` | 2 | Add `!scripts/security/testdata/*.sarif` exception directly below the existing blanket `*.sarif` line (§7.1) — without it, the new fixture files above cannot be staged/committed. |
| `scripts/pre-commit-hooks/codeql-check-findings.sh` | 2 | Refactor to thin wrapper calling the shared script; remove duplicated jq. |
| `.github/workflows/codeql.yml` | 2 | "Check CodeQL Results" and "Fail on High-Severity Findings" steps call the shared script instead of inline jq. |
| `.github/security-severity-policy.yml` | 2 | `codeql.blocking_levels` → `[error, warning, note]`; add `exceptions` block; remove `warning_policy`/`escalation_high_signal_rule_ids`. |
| `scripts/ci/check-codeql-parity.sh` | 2 | Add assertion that both local script and CI workflow reference the shared gate script path. |
| `backend/internal/api/handlers/auth_handler_test.go` | 1 (verify only) | No new tests required (existing suite already covers the truth table, §2.2) — run as regression gate. |

### 7.1 Ignore-file / config audit (explicitly checked, per task instructions)

- **`.gitignore`**: `.github/codeql/codeql-suppressions.yml` must **not**
  be gitignored — it needs to be tracked/committed exactly like
  `.trivyignore`/`.grype.yaml` (both currently tracked, confirmed via
  `git status`/repo presence). Checked `.gitignore` for any existing
  `.github/codeql/**` or `*suppressions*` pattern that would need an
  exception — none found; no `.gitignore` change needed for that file.
  **However**: `.gitignore` line 189 is a blanket `*.sarif` (confirmed via
  `grep -n sarif .gitignore`), which **would silently prevent
  `scripts/security/testdata/*.sarif` (Commit 1's fixture files, §7/§9.2)
  from being staged at all** — unlike `codeql-results-*.sarif` at the
  repo root, these are deliberately committed test fixtures, not scan
  artifacts, and the blanket pattern doesn't distinguish the two. Required
  `.gitignore` change (add directly beneath the existing `*.sarif` line):
  ```
  *.sarif
  !scripts/security/testdata/*.sarif
  ```
  Verify with `git check-ignore -v scripts/security/testdata/*.sarif`
  returning nothing (i.e., not ignored) once the exception is added — this
  is a required part of Commit 1's validation, not optional cleanup.
- **`.codecov.yml`**: checked existing `ignore:` list — it already
  excludes `backend/codeql-db/**`, `codeql-db/**`, `codeql-db-*/**`,
  `codeql-agent-results/**`, `codeql-custom-queries-*/**`, `*.sarif`. The
  new `.github/codeql/codeql-suppressions.yml` and
  `scripts/security/codeql-findings-gate.sh` are not application code
  subject to patch-coverage (YAML data file; shell script already outside
  Go/TS coverage scope like every other `scripts/**` file) — no change
  needed, but `scripts/security/codeql-findings-gate.sh` should be
  exercised by shellcheck (already globbed via `*.sh` in `lefthook.yml`)
  and ideally a bats/manual test (see §9 Test Plan) even though it's not
  part of the Go/frontend coverage percentage.
- **`.dockerignore`**: new files live under `.github/` and `scripts/`,
  both already outside the Docker build context relevance path; existing
  `*.sarif`, `codeql-db/`, etc. entries are unaffected and don't need
  updating for these two new, non-artifact files.
- **`Dockerfile`**: no reference to CodeQL tooling; no change needed.

---

## 8. Implementation Plan (phased)

### Phase 1 — Tests-first framing (adapted; see rationale below)

CLAUDE.md's default commit sequence starts with "E2E specs for new
behavior (as `test.fixme`)." This PR introduces **no new user-facing
behavior** — Part 1 is comment-only, Part 2 is CI/tooling with no UI
surface — so there is no new Playwright spec to write. Instead, Phase 1 is
a **regression-scope identification** step:

- Confirm which existing Playwright specs exercise login/cookie-setting
  behavior (auth flows touch `setSecureCookie` via `Login`/`Refresh`
  handlers) and must be run as the regression gate for Part 1's
  comment-only change. Identify via `grep -rl "login\|auth_token" frontend/e2e` (or wherever specs live) at implementation time.
- These existing specs run unmodified as part of DoD step 1
  (`npx playwright test --project=firefox`) — no `test.fixme` needed since
  no behavior is pending implementation.

### Phase 2 — Foundation (no behavior change)

- Add `.github/codeql/codeql-suppressions.yml` (empty, documented schema).
- Add `scripts/security/codeql-findings-gate.sh` with
  `scripts/security/tests/codeql-findings-gate.bats` (7 fixture cases,
  §9.2) exercising it against fixture SARIF/suppressions files — written
  and tested standalone, **not yet wired** into the pre-commit script or
  CI workflow, so this commit changes zero enforced behavior.

### Phase 3 — Backend (Part 1)

- Apply the exact `auth_handler.go` change from §3.2.
- Apply the four `crowdsec_handler.go`/`backup_handler.go` comment
  repositions from §3.5 (folded in per §5.7).
- Run verification steps from §3.3 (may require iterating on comment
  placement if condition A doesn't hold on the first attempt — allowed
  and expected per §3.3 step 4's fallback).
- Update `docs/issues/codeql-cookie-suppression-not-honored.md` (§3.4 —
  already present on this branch, no porting needed; see §3.4's
  re-evaluated automation caveat).

### Phase 4 — Gate hardening (Part 2, behavior change)

- Wire `codeql-check-findings.sh` and `.github/workflows/codeql.yml` to
  call the shared script (§5.4).
- Update `.github/security-severity-policy.yml` (§5.3).
- Extend `scripts/ci/check-codeql-parity.sh` (§5.5).
- Run the migration check from §5.6 **before** this commit is considered
  done — confirm the ignore-list stays empty or gains exactly the entries
  needed, never silently dropping a real finding.

### Phase 5 — Verification, docs, DoD

- Full Definition of Done per CLAUDE.md (§10 below).
- Update `docs/features/security.md` / `docs/security.md` if either
  documents the CodeQL gate's current "warnings are non-blocking" behavior
  user-facing/contributor-facing (check at implementation time — not
  confirmed to reference this specific policy during this planning pass,
  but both files exist and are plausible homes for a policy-behavior
  change; verify and update if so).

---

## 9. Test Plan

### 9.1 Part 1

- No new unit tests (existing suite in `auth_handler_test.go` already
  covers the full truth table per §2.2) — run unmodified as the
  regression gate: `go test ./backend/internal/api/handlers/... -run 'SecureCookie|LocalRequest' -v`.
- SARIF-based verification per §3.3 (not a unit test — a manual/CI
  verification step, documented as such).

### 9.2 Part 2 — new script needs real tests

`scripts/security/codeql-findings-gate.sh` is new logic and must have
accompanying tests per CLAUDE.md's "All new code MUST include accompanying
unit tests." Since this is a bash script (not Go/TS, so outside
`scripts/go-test-coverage.sh`/`scripts/frontend-test-coverage.sh`'s 85%
gates), design as fixture-driven functional tests:

- **Test framework: `bats-core`, committed now, not left open.** Per
  CLAUDE.md's LEVERAGE principle, this repo already has an adopted,
  working convention for exactly this kind of script test: `scripts/tests/local-patch-report_baseline.bats`
  and `scripts/history-rewrite/tests/*.bats` (the latter run in CI via
  `bats ./scripts/history-rewrite/tests` in
  `.github/workflows/history-rewrite-tests.yml`, which also `apt-get
  install`s `bats`; confirmed locally installed as Bats 1.13.0). New file:
  `scripts/security/tests/codeql-findings-gate.bats` — colocated with the
  script under test the same way `scripts/history-rewrite/tests/` is
  colocated with `scripts/history-rewrite/*.sh` (a subdirectory-scoped
  script family gets its own `tests/` subdirectory; this is the closer
  match to the new script's layout than the flat `scripts/tests/`
  directory, which holds tests for root-level `scripts/*.sh` files like
  `local-patch-report.sh`). Each of the 7 fixture cases below becomes a
  `@test` block asserting exit code plus a distinguishing output
  substring, following `local-patch-report_baseline.bats`'s `setup()`
  pattern of staging fixture files per test.
- New fixture SARIF files (and matching `codeql-suppressions.yml`
  fixtures for cases 4-6) under `scripts/security/testdata/`, covering:
  1. A single error-level, unsuppressed result → script exits non-zero.
  2. A single warning-level, unsuppressed result → script exits non-zero
     (this is the exact regression test for the bug this PR closes —
     under the *old* policy this fixture would have passed).
  3. A result with non-null `suppressions` (native) → script exits 0,
     output shows `SUPPRESSED (in-source)`.
  4. A result matching a non-expired `codeql-suppressions.yml` fixture
     entry → script exits 0, output shows the reason/review date.
  5. A result matching an *expired* `codeql-suppressions.yml` fixture
     entry → script exits non-zero, output shows `EXPIRED SUPPRESSION`.
  6. A result whose `ruleId`+`path` match a fixture `codeql-suppressions.yml`
     entry, but whose `startLine` falls outside that entry's `line`/
     `line_range` (simulated line drift, §5.2/§5.4/§12) → script exits
     non-zero, output shows `LIKELY-STALE ENTRY` — distinguishable from
     fixture 2's `NEW FINDING` output.
  7. Empty results array → script exits 0.
- `shellcheck --severity=error` on the new script (already enforced by
  `lefthook.yml`'s `shellcheck` pre-commit command via its `*.sh` glob —
  no config change needed, just needs to pass).

### 9.3 Parity guard test

- Extend `scripts/ci/check-codeql-parity.sh`'s own invocation
  (`lefthook run codeql`, step 4) to prove the new assertion actually
  fires: temporarily (during implementation/review, not committed) revert
  one of the two call sites to inline jq and confirm
  `check-codeql-parity.sh` fails with the new drift message, then restore
  it. Document this as a one-time manual verification in the PR
  description rather than a permanent automated test (the parity script
  itself has no existing test harness in this repo — consistent with its
  current state).

---

## 10. Validation Gates (run in this order before considering the work done)

1. `go build ./...` (backend) — confirm `auth_handler.go` compiles.
2. `go test ./backend/internal/api/handlers/... -run 'SecureCookie|LocalRequest' -v` — Part 1 regression.
3. `go test ./...` — full backend suite.
4. `scripts/go-test-coverage.sh` — ≥85% (Part 1 is comment-only so should
   be a no-op on coverage; confirm no regression).
5. `bats scripts/security/tests/codeql-findings-gate.bats` — 7 fixture cases (§9.2).
6. `shellcheck --severity=error scripts/security/codeql-findings-gate.sh scripts/pre-commit-hooks/codeql-check-findings.sh scripts/ci/check-codeql-parity.sh`
7. `lefthook run codeql` (full sequential pipeline: go-scan → js-scan →
   check-findings → parity) — must pass cleanly against fresh scans, with
   the cookie finding resolved per §3.3's condition A or B.
8. Manual expired/blocking negative test per §9.2 fixture 5, and the
   deliberately-reintroduced-bad-pattern check below.
9. `lefthook run pre-commit` — full fast-linter pass (this PR doesn't
   touch anything in the blocking pre-commit set beyond what's already
   covered by shellcheck/staticcheck globs, but must still pass clean).
10. `bash scripts/local-patch-report.sh` — patch coverage artifacts.
11. `npx playwright test --project=firefox` (scoped to auth/login specs
    per §8 Phase 1, full suite if time permits) — confirm zero regression
    in cookie-setting behavior end-to-end.
12. `cd frontend && npm run type-check` — no frontend files touched, but
    run as a cheap confirmation nothing was inadvertently affected.
13. Deliberate-regression test (Part 2's own "does the gate actually
    gate" check): temporarily reintroduce a trivially-detectable
    `go/cookie-secure-not-set`-shaped pattern (e.g., a scratch handler
    with `c.SetCookie(name, value, maxAge, "/", "", false, true)` and no
    suppression at all) in a throwaway file, run `lefthook run codeql`,
    confirm it now hard-fails (where under the *old* policy a
    warning-level finding like this would have passed) — then delete the
    scratch file before committing. This is the concrete verification
    case requested for "a deliberately-reintroduced known-bad pattern
    should make the gate genuinely fail."
14. Ignore-list visibility check: add a temporary fixture entry to
    `.github/codeql/codeql-suppressions.yml` matching the scratch
    finding from step 13, confirm the gate now passes *and* the script's
    output still prints the suppressed finding (not silently swallowed —
    visible/auditable per the task's requirement), then remove the
    temporary entry.

---

## 11. Acceptance Criteria

**Part 1**

- [ ] `backend/internal/api/handlers/auth_handler.go`'s `secure`/
      `isLocalRequest`/`requestScheme`/`isTrustedPeer` logic is
      byte-for-byte unchanged (comment-only diff plus the two dangling
      cross-reference removals).
- [ ] `crowdsec_handler.go`'s and `backup_handler.go`'s logged fields,
      `util.SanitizeForLog(...)` calls, and control flow are byte-for-byte
      unchanged at all four repositioned sites (comment-only diff, §3.5).
- [ ] A fresh CodeQL Go SARIF scan shows the `go/cookie-secure-not-set`
      result for this call site either (A) with non-null `suppressions`,
      or (B) absent from the blocking set because it's registered in
      `.github/codeql/codeql-suppressions.yml` — not both null-suppressed
      and unregistered as it is today.
- [ ] `docs/issues/codeql-cookie-suppression-not-honored.md`'s acceptance
      criteria are checked off and a Resolution section is added.
- [ ] Existing `auth_handler_test.go` suite passes unmodified.

**Part 2**

- [ ] `.github/security-severity-policy.yml` documents CodeQL findings of
      any level blocking by default, with the ignore-list as the only
      exception path.
- [ ] `scripts/pre-commit-hooks/codeql-check-findings.sh` and
      `.github/workflows/codeql.yml` both call the same
      `scripts/security/codeql-findings-gate.sh` — verified by
      `check-codeql-parity.sh`'s new assertion.
- [ ] A fresh warning-level finding with no ignore-list entry and no
      native suppression fails `lefthook run codeql` and would fail CI
      (validation gate step 13).
- [ ] A finding with a valid, non-expired `codeql-suppressions.yml` entry
      passes the gate but remains visible in script output (validation
      gate step 14) — not silently dropped.
- [ ] An expired `codeql-suppressions.yml` entry does **not** suppress —
      the gate fails and the output distinguishes "expired" from "new."
- [ ] Migration check (§5.6) confirms no other currently-known finding was
      silently dropped or left with no documented path forward.

**Both**

- [ ] Full Definition of Done (CLAUDE.md) passes: Playwright, patch
      coverage preflight, CodeQL/Trivy security scans, lefthook, coverage
      ≥85%, type-check, builds, cleanup.

---

## 12. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Correct-looking placement fix still doesn't produce non-null `suppressions` (unknown Go-extractor edge case with this specific 7-argument `SetCookie` call shape) | §3.3 condition B fallback: route through the new ignore-list mechanism instead of open-ended guessing; either outcome is a real, verifiable resolution. |
| Flipping `blocking_levels` to include `warning`/`note` surfaces *other*, previously-unknown findings at implementation time that weren't in this planning pass's snapshot (`development` moves between now and implementation) | §5.6 migration check is an explicit, required implementation step *before* the stricter policy is turned on — never flip the switch blind. |
| New shared `codeql-findings-gate.sh` becomes a third place with its own drift risk if `check-codeql-parity.sh`'s new assertion is ever weakened/removed | §5.5's assertion is itself covered by the manual one-time verification in §9.3; recommend (not required for this PR) eventually giving `check-codeql-parity.sh` its own regression test harness — noted as a limitation consistent with its current, pre-existing untested state, not a new gap introduced here. |
| ~~The five `go/log-injection` suppression comments are latently exposed to the same placement bug~~ — **superseded**: independent static-inspection re-analysis (§5.7) found 4 of the actual 6 sites already malformed, not merely "latently exposed." | No longer a deferred risk — the four malformed sites are fixed directly in Commit 2 (§3.5), same commit as the cookie fix. Residual risk is limited to the two already-correct sites (1135, 1139) regressing in some *future* edit, which is now covered going forward by the *new*, stricter gate policy (Part 2) rather than left to ride silently as before. |
| Removing the `docs/plans/current_spec.md §9.x` cross-references in Part 1 could look like scope creep beyond "just fix the suppression" | Directly justified by CLAUDE.md's Root Cause Analysis Protocol and "actively refactor code you encounter" — the dangling reference was discovered while tracing the exact code this task required understanding in full, not sought out separately; kept minimal (comment text only, zero behavior change). |
| Empty `.github/codeql/codeql-suppressions.yml` at ship time means the mechanism is untested against a *real* accepted finding, only fixtures | Acceptable for this PR — §5.6 explicitly re-checks at implementation time and would populate a real entry if one turns out to be needed; fixture coverage (§9.2) exercises the matching/expiry logic thoroughly regardless of whether a real entry exists yet. |
| ~~Branch-hygiene risk: implementer starts from `fix/codeql` out of habit since it's the currently-checked-out branch~~ — **moot**: `fix/codeql` is now fully merged (§0), and the actual working branch, `fix/codeql-cookie-suppression-gate-hardening`, was already created correctly from post-merge `origin/development` before this revision began (§6). | No action needed — there is no unmerged branch left to accidentally start from, and the correct branch already exists and is in use. |
| `codeql-suppressions.yml` entries keyed by an exact `line:` silently stop matching if a later, unrelated code edit shifts line numbers in the same file — the finding then reverts to blocking with no way to tell "line moved" apart from "genuinely new finding" | §5.4's gate script distinguishes this case explicitly: a `(ruleId, path)` match with a non-matching `line`/`line_range` prints `LIKELY-STALE ENTRY (line moved? check codeql-suppressions.yml)` instead of the generic `NEW FINDING` message, at no extra computation cost (the partial match is already being evaluated). §5.2 recommends `line_range` (already in the schema) over bare `line:` for any entry expected to survive routine refactors; §9.2 fixture 6 exercises this path directly. |
| Editing `docs/issues/codeql-cookie-suppression-not-honored.md` in Commit 3 could, in principle, still surface it to `docs-to-issues.yml`'s "Detect changed files" step (modified files aren't filtered out, only `removed`-status ones are) | §3.4's re-evaluated automation caveat: verified via `gh run view` that the automation already tried and failed to process this exact file on the PR #1216 merge commit (`js-yaml`-compat error in its own tooling, unrelated to this PR), so the realistic expectation is another silent failure, not a duplicate issue. Residual risk if that unrelated bug gets fixed independently before this PR merges: close any resulting auto-filed issue with a cross-reference comment to this PR rather than leaving it open as an untriaged duplicate. |

---

## 13. Commit Slicing Strategy (single PR, ordered commits — per CLAUDE.md)

**Decision**: single PR, `fix/codeql-cookie-suppression-gate-hardening` →
`development`, containing both parts as one cohesive story ("resolve the
cookie finding for real, and stop this class of gap from recurring" is one
feature, not two — Part 2 exists *because of* Part 1's root cause).
Ordered, reviewable commits within it, adapted from CLAUDE.md's suggested
sequence (test → foundation → backend → frontend → hardening) since there
is no frontend leg and no new user-facing behavior to spec via E2E:

### Commit 1 — Foundation: shared CodeQL gate script + empty ignore-list (no behavior change)
- **Scope**: `scripts/security/codeql-findings-gate.sh` (new),
  `scripts/security/tests/codeql-findings-gate.bats` (new, 7 fixture
  cases per §9.2), `scripts/security/testdata/*.sarif` +
  `*suppressions*.yml` fixtures (new), `.github/codeql/codeql-suppressions.yml`
  (new, empty), `.gitignore` (add `!scripts/security/testdata/*.sarif`
  exception, §7.1 — required or the fixture files above can't be staged).
- **Not yet wired** into `codeql-check-findings.sh` or `codeql.yml` — this
  commit is purely additive, zero enforced-behavior change, reviewable in
  isolation.
- **Dependencies**: none.
- **Validation gate**: `bats scripts/security/tests/codeql-findings-gate.bats`
  (7 cases, §9.2) passes; `shellcheck` clean; `git check-ignore -v
  scripts/security/testdata/*.sarif` confirms the fixtures are not
  ignored (§7.1).
- **Routing**: `devops`.
- **Commit message**: `feat: add shared CodeQL findings-gate script and ignore-list schema` (no `(security)` scope — pure tooling scaffolding, not yet enforcing anything).

### Commit 2 — Backend: fix the CodeQL suppression comment placement bug (Part 1, broadened per §5.7)
- **Scope**: `backend/internal/api/handlers/auth_handler.go` (comment
  reposition + dangling-reference cleanup, §3.2), plus
  `backend/internal/api/handlers/crowdsec_handler.go` and
  `backend/internal/api/handlers/backup_handler.go` (four additional
  malformed `codeql[go/log-injection]` comment repositions, §3.5 — folded
  in during this revision per §5.7's re-analysis; all four share the
  identical root cause and fix pattern as the cookie comment).
- **Dependencies**: none (independent of Commit 1; ordered here per
  CLAUDE.md's foundation-then-backend convention, and because Commit 4's
  gate-flip should land after Part 1 is verified resolved, per §5.6).
- **Validation gate**: `go build`, `go test ./backend/internal/api/handlers/...`
  (existing suite unmodified/passing), fresh SARIF scan per §3.3 shows
  condition A or B satisfied for the cookie finding, and zero
  `go/log-injection` results (no regression) for the four repositioned
  sites.
- **Routing**: `backend-dev`.
- **`(security)` commit-scope decision (resolves a supervisor-flagged
  inconsistency)**: **drop `(security)` scope entirely.** The plan's own
  root-cause analysis (§2.2) concludes "the logic is genuinely sound as
  designed... this is a suppression-tooling problem, not a vulnerability,"
  and §3.1 restates "the logic is safe; only the suppression mechanism is
  broken." Commit 4 already withholds `(security)` scope on the identical
  reasoning ("it changes policy enforcement strength... not a vulnerability
  fix"). Keeping `(security)` on Commit 2 while withholding it from Commit
  4 was an unexplained inconsistency — by the plan's own stated test, a
  comment-placement fix with a verified-safe underlying logic doesn't meet
  CLAUDE.md's bar ("real vulnerability fixes, new protective mechanisms —
  not general bug fixes"). Applying it consistently means dropping it
  here too. (This also makes the "how vague must the subject line be"
  question moot for this commit — that constraint only applies to
  `(security)`-scoped subjects, since those are the ones displayed
  verbatim in the What's New changelog; a non-security `fix:` subject can
  describe the mechanism plainly.)
- **Commit message**: `fix: correct CodeQL suppression comment placement in auth, crowdsec, and backup handlers`

### Commit 3 — Docs: close out the tracked issue
- **Scope**: `docs/issues/codeql-cookie-suppression-not-honored.md` only.
  Single step (§3.4 — no porting sub-step needed; the file is already
  present on this branch via the PR #1216 merge, §0): update the existing
  file — check off Acceptance Criteria, add a Resolution section.
- **Dependencies**: Commit 2 (needs the actual resolution to describe).
- **Validation gate**: none beyond doc review — no code impact.
- **Operational note**: §3.4's re-evaluated automation caveat — editing
  this file could in principle still surface it to `docs-to-issues.yml`,
  but the automation is currently verified-broken for this exact file (a
  `js-yaml`-compat error in its own tooling, unrelated to this PR, seen on
  the PR #1216 merge run), so no duplicate-issue auto-filing is expected.
  If one does appear anyway (e.g. the automation's dependency bug gets
  fixed independently before this PR merges), close it with a
  cross-reference comment to this PR rather than leaving it open as an
  untriaged duplicate.
- **Routing**: `backend-dev` (small enough not to need `docs-writer`; it's
  closing out a technical issue doc, not user-facing docs).
- **Commit message**: `docs: close out CodeQL cookie-suppression tracking issue`

### Commit 4 — Hardening: wire the stricter gate + policy flip (Part 2)
- **Scope**: `scripts/pre-commit-hooks/codeql-check-findings.sh` (thin
  wrapper refactor), `.github/workflows/codeql.yml` (both steps call
  shared script), `.github/security-severity-policy.yml` (policy flip),
  `scripts/ci/check-codeql-parity.sh` (new assertion).
- **Dependencies**: Commit 1 (script must exist), Commit 2 (Part 1 must be
  resolved *before* this flips `warning`/`note` to blocking, per §5.6 —
  otherwise this commit would immediately break the gate on its own
  branch).
- **Validation gate**: §5.6 migration check run and confirmed empty (or
  populated with real, justified entries — not silently dropped);
  `lefthook run codeql` full pipeline passes; parity assertion verified
  per §9.3's manual drift-injection test.
- **Routing**: `devops`.
- **Commit message**: `feat: fail CodeQL findings gate on any severity by default, add documented exceptions` (no `(security)` scope — this is process/gate tooling, not a vulnerability fix or new protective mechanism against an attacker; it changes *policy enforcement strength*, which CLAUDE.md's `(security)` scope guidance reserves for "genuinely security-relevant... real vulnerability fixes, new protective mechanisms," not CI gate stringency).

### Commit 5 — Verification artifacts + full DoD pass
- **Scope**: no functional file changes expected; this commit exists to
  capture `test-results/local-patch-report.{md,json}`, any
  `docs/features/security.md`/`docs/security.md` updates found necessary
  during Phase 5 (§8), and confirmation the deliberate-regression test
  (validation gate 13/14) was run and reverted cleanly.
- **Dependencies**: Commits 1-4.
- **Validation gate**: full DoD (§10, all 14 gates) green.
- **Routing**: `qa-security` for the security-scan/coverage legs,
  `docs-writer` if any user-facing doc needed a touch.
- **Commit message**: `chore: verify CodeQL gate hardening and update security docs` (only if there's an actual doc delta — otherwise fold this verification into Commit 4 rather than create an empty commit).

### Rollback / contingency

- Each commit is independently revertable without breaking `development`
  at any intermediate point *except* Commit 4 depends on Commit 2 being
  merged first within the same PR (not across PRs) — if Part 1's fix
  turns out to need condition B (ignore-list fallback) rather than
  condition A (native suppression), Commit 4 still lands cleanly since the
  ignore-list mechanism (Commit 1) already exists and Commit 2 would have
  populated it.
- If native suppression (condition A) is confirmed impossible for this
  call shape during implementation, no separate PR is needed — Commit 2 is
  simply amended, within the same feature branch, before merge, to route
  through the ignore-list instead; this is anticipated in §3.3 and not a
  scope change.
- If Part 2's stricter policy surfaces an unexpected real finding during
  the §5.6 migration check that can't be trivially fixed or justified
  before this PR is ready to merge, the correct contingency is to **add a
  properly dated, justified ignore-list entry** (not to revert Part 2 or
  merge with the gate silently softened) — consistent with §5.6's explicit
  "never silently drop, never bare hard-fail with no path forward" rule.
- Emergency bypass (`git commit --no-verify`) is not anticipated to be
  needed for this work and, per CLAUDE.md, would require a follow-up issue
  if used.

---

## 14. Handoff

This plan is ready for `supervisor` review. On approval, delegate:

- Commit 1, Commit 4 → `devops`
- Commit 2, Commit 3 → `backend-dev`
- Commit 5 → `qa-security` (+ `docs-writer` only if a doc delta is found necessary)

`management` orchestrates the sequence per §13's dependency order; per
CLAUDE.md, all of it lands as ordered commits within the single PR
described above — no splitting across multiple PRs.
