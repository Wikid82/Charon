# QA Audit Report

**Date:** 2026-05-26
**Branch:** `development`
**Scope:** Focused audit of two commits — new docs pages (Orthrus, Hecate, Remote Docker Setup) and FeedbackWidget UI update (docs link addition)
**Auditor:** QA Security Agent

---

## Verdict: APPROVED ✅

All critical checks pass. One broken internal link was identified and **fixed in-place** during this audit.

---

## Checks

### Documentation

| Check | File | Result | Notes |
|---|---|---|---|
| Frontmatter valid | `docs/features/orthrus.md` | PASS | `title`, `description`, `category: features` present |
| Frontmatter valid | `docs/features/hecate.md` | PASS | `title`, `description`, `category: features` present |
| Frontmatter valid | `docs/guides/remote-docker-setup.md` | PASS | `title`, `description`, `category: guides` present |
| Internal links resolve | `docs/features/orthrus.md` | PASS *(fixed)* | See ISSUE-1 — broken link corrected |
| Internal links resolve | `docs/features/hecate.md` | PASS | `orthrus.md`, `uptime-monitoring.md` both verified |
| Internal links resolve | `docs/guides/remote-docker-setup.md` | PASS | `../features/hecate.md` resolves correctly |
| Internal links resolve | `docs/features/uptime-monitoring.md` | PASS | Cross-reference `../guides/remote-docker-setup.md` appended and resolves correctly |
| Internal links resolve | `docs/features.md` | PASS | `features/orthrus.md`, `features/hecate.md` both verified |
| Internal links resolve | `docs/index.md` | PASS | All three new Remote Access links resolve correctly |
| Cross-reference appended | `docs/features/uptime-monitoring.md` | PASS | Footer link to Remote Docker Setup guide present |
| Orthrus entry added | `docs/features.md` | PASS | Entry and `Learn More` link present |
| Hecate link fixed | `docs/features.md` | PASS | `features/hecate.md` (corrected) present |
| Remote Access section | `docs/index.md` | PASS | Section with three links present |

### Frontend — FeedbackWidget

| Check | Result | Notes |
|---|---|---|
| `DOCS_URL` is a static constant | PASS | `https://wikid82.github.io/Charon/` — not derived from user input |
| Docs link uses `target="_blank"` | PASS | Tab-nabbing vector closed |
| Docs link uses `rel="noopener noreferrer"` | PASS | OWASP A05 compliant |
| `aria-label` present on docs link | PASS | Uses i18n key `feedback.viewDocsAriaLabel` |
| `BookOpen` icon has `aria-hidden="true"` | PASS | Decorative icon correctly hidden from AT |
| No hardcoded secrets or API keys | PASS | None present |
| TypeScript type-check (`tsc --noEmit`) | PASS | Exit 0, no errors |

### Tests

| Check | Result | Notes |
|---|---|---|
| FeedbackWidget tests (vitest) | PASS | 20/20 tests passing |
| Tests cover new docs link `href` | PASS | Test 16 verifies `DOCS_URL` |
| Tests cover `target="_blank"` | PASS | Test 17 |
| Tests cover `rel="noopener noreferrer"` | PASS | Test 18 |
| Tests cover `aria-label` | PASS | Test 19 — verifies i18n string resolves correctly |

### Localisation

| Check | Result | Notes |
|---|---|---|
| `en/translation.json` — 3 new feedback keys | PASS | `viewDocs`, `viewDocsDescription`, `viewDocsAriaLabel` present |
| `de/translation.json` — 3 new feedback keys | PASS | All present (English strings — consistent with project pattern) |
| `fr/translation.json` — 3 new feedback keys | PASS | All present |
| `zh/translation.json` — 3 new feedback keys | PASS | All present |
| `es/translation.json` — 3 new feedback keys | PASS | All present |
| No duplicate key conflict | PASS | `viewDocs` at line ~1395 is in a different JSON namespace (`dns`), not `feedback` |

---

## Issues

### ISSUE-1 — Broken Internal Link *(FIXED)*

- **Severity:** LOW
- **File:** `docs/features/orthrus.md`, line 74
- **Original:** `[adding a Remote Server](remote-docker-setup.md)`
- **Problem:** Relative path resolved to `docs/features/remote-docker-setup.md` — file does not exist.
- **Fix applied:** Changed to `[adding a Remote Server](../guides/remote-docker-setup.md)`
- **Status:** RESOLVED ✅

---

## Notes

- Non-English locale files (`de`, `fr`, `zh`, `es`) carry the three new `feedback` keys in English. This is consistent with the existing project pattern for untranslated strings and is not a blocking issue.
- `docs/troubleshooting/` directory confirmed present; references to it from `uptime-monitoring.md` are valid.
- `docs/features/notifications.md` confirmed present; reference from `uptime-monitoring.md` is valid.
