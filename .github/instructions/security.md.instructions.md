---
applyTo: SECURITY.md
---

# Instructions: Maintaining `SECURITY.md`

`SECURITY.md` is the project's living security record. It serves two audiences simultaneously: users who need to know what risks exist right now, and the broader community who need confidence that vulnerabilities are being tracked and remediated with discipline. Treat it like a changelog, but for security events — every known issue gets an entry, every resolved issue keeps its entry.

---

## File Structure

`SECURITY.md` must always contain the following top-level sections, in this order:

1. A brief project security policy preamble (responsible disclosure contact, response SLA)
2. **`## Known Vulnerabilities`** — active, unpatched issues
3. **`## Patched Vulnerabilities`** — resolved issues, retained permanently for audit trail

No other top-level sections are required. Do not collapse or remove sections even when they are empty — use the explicit empty-state placeholder defined below.

---

## Section 1: Known Vulnerabilities

This section lists every vulnerability that is currently unpatched or only partially mitigated. Entries must be sorted with the highest severity first, then by discovery date descending within the same severity tier.

### Entry Format

Each entry is an H3 heading followed by a structured block:

```markdown
### [SEVERITY] CVE-XXXX-XXXXX · Short Title

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-XXXX-XXXXX (or `CHARON-YYYY-NNN` if no CVE assigned yet) |
| **Severity** | Critical / High / Medium / Low · CVSS v3.1 score if known (e.g. `8.1 · High`) |
| **Status**   | Investigating / Fix In Progress / Awaiting Upstream / Mitigated (partial) |

**What**
One to three sentences describing the vulnerability class and its impact.
Be specific: name the weakness type (e.g. SQL injection, path traversal, SSRF).

**Who**
- Discovered by: [Reporter name or handle, or "Internal audit", or "Automated scan (tool name)"]
- Reported: YYYY-MM-DD
- Affects: [User roles, API consumers, unauthenticated users, etc.]

**Where**
- Component: [Module or service name]
- File(s): `path/to/affected/file.go`, `path/to/other/file.ts`
- Versions affected: `>= X.Y.Z` (or "all versions" / "prior to X.Y.Z")

**When**
- Discovered: YYYY-MM-DD
- Disclosed (if public): YYYY-MM-DD (or "Not yet publicly disclosed")
- Target fix: YYYY-MM-DD (or sprint/milestone reference)

**How**
A concise technical description of the attack vector, prerequisites, and exploitation
method. Omit proof-of-concept code. Reference CVE advisories or upstream issue
trackers where appropriate.

**Planned Remediation**
Describe the fix strategy: library upgrade, logic refactor, config change, etc.
If a workaround is available in the meantime, document it here.
Link to the tracking issue: [#NNN](https://github.com/owner/repo/issues/NNN)
```

### Empty State

When there are no known vulnerabilities:

```markdown
## Known Vulnerabilities

No known unpatched vulnerabilities at this time.
Last reviewed: YYYY-MM-DD
```

---

## Section 2: Patched Vulnerabilities

This section is a permanent, append-only ledger. Entries are never deleted. Sort newest-patched first. This section builds community trust by demonstrating that issues are resolved promptly and transparently.

### Entry Format

```markdown
### ✅ [SEVERITY] CVE-XXXX-XXXXX · Short Title

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-XXXX-XXXXX (or internal ID) |
| **Severity** | Critical / High / Medium / Low · CVSS v3.1 score |
| **Patched**  | YYYY-MM-DD in `vX.Y.Z` |

**What**
Same description carried over from the Known Vulnerabilities entry.

**Who**
- Discovered by: [Reporter or method]
- Reported: YYYY-MM-DD

**Where**
- Component: [Module or service name]
- File(s): `path/to/affected/file.go`
- Versions affected: `< X.Y.Z`

**When**
- Discovered: YYYY-MM-DD
- Patched: YYYY-MM-DD
- Time to patch: N days

**How**
Same technical description as the original entry.

**Resolution**
Describe exactly what was changed to fix the issue.
- Commit: [`abc1234`](https://github.com/owner/repo/commit/abc1234)
- PR: [#NNN](https://github.com/owner/repo/pull/NNN)
- Release: [`vX.Y.Z`](https://github.com/owner/repo/releases/tag/vX.Y.Z)

**Credit**
[Optional] Thank the reporter if they consented to attribution.
```

### Empty State

```markdown
## Patched Vulnerabilities

No patched vulnerabilities on record yet.
```

---

## Lifecycle: Moving an Entry from Known → Patched

When a fix ships:

1. Remove the entry from `## Known Vulnerabilities` entirely.
2. Add a new entry to the **top** of `## Patched Vulnerabilities` using the patched format above.
3. Carry forward all original fields verbatim — do not rewrite the history of the issue.
4. Add the `**Resolution**` and `**Credit**` blocks with patch details.
5. Update the `Last reviewed` date on the Known Vulnerabilities section if it is now empty.

Do not edit or backfill existing Patched entries once they are committed.

---

## Severity Classification

Use the following definitions consistently:

| Severity | CVSS Range | Meaning |
|----------|------------|---------|
| **Critical** | 9.0–10.0 | Remote code execution, auth bypass, full data exposure |
| **High**     | 7.0–8.9  | Significant data exposure, privilege escalation, DoS |
| **Medium**   | 4.0–6.9  | Limited data exposure, requires user interaction or auth |
| **Low**      | 0.1–3.9  | Minimal impact, difficult to exploit, defense-in-depth |

When a CVE CVSS score is not yet available, assign a preliminary severity based on these definitions and note it as `(preliminary)` until confirmed.

---

## Internal IDs

If a vulnerability has no CVE assigned, use the format `CHARON-YYYY-NNN` where `YYYY` is the year and `NNN` is a zero-padded sequence number starting at `001` for each year. Example: `CHARON-2025-003`. Assign a CVE ID in the entry retroactively if one is issued later, and add the internal ID as an alias in parentheses.

---

## Responsible Disclosure Preamble

The preamble at the top of `SECURITY.md` (before the vulnerability sections) must include:

- The preferred contact method for reporting vulnerabilities (e.g. a GitHub private advisory link, a security email address, or both)
- An acknowledgment-first response commitment: confirm receipt within 48 hours, even if the full investigation takes longer
- A statement that reporters will not be penalized or publicly named without consent
- A link to the full disclosure policy if one exists

Example:

```markdown
## Reporting a Vulnerability

To report a security issue, please use
[GitHub Private Security Advisories](https://github.com/owner/repo/security/advisories/new)
or email `security@example.com`.

We will acknowledge your report within **48 hours** and provide a remediation
timeline within **7 days**. Reporters are credited with their consent.
We do not pursue legal action against good-faith security researchers.
```

---

## Maintenance Rules

- **Review cadence**: Update the `Last reviewed` date in the Known Vulnerabilities section at least once per release cycle, even if no entries changed.
- **No silent patches**: Every security fix — no matter how minor — must produce an entry in `## Patched Vulnerabilities` before or alongside the release.
- **No redaction**: Do not redact or soften historical entries. Accuracy builds trust; minimizing past issues destroys it.
- **Dependency vulnerabilities**: Transitive dependency CVEs that affect Charon's exposed attack surface must be tracked here the same as first-party vulnerabilities. Pure dev-dependency CVEs with no runtime impact may be omitted at maintainer discretion, but must still be noted in the relevant dependency update PR.
- **Partial mitigations**: If a workaround is deployed but the root cause is not fixed, the entry stays in `## Known Vulnerabilities` with `Status: Mitigated (partial)` and the workaround documented in `**Planned Remediation**`.
