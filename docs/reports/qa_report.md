# QA & Security Report — Docusaurus `docs-site/` Migration

- **Feature branch:** `development`
- **Commits audited:** `ecde1573`, `480355db`, `8b657335`, `29b53e46`, `5158f19d` (range `ecde1573~1..5158f19d`)
- **Plan:** `docs/plans/current_spec.md`
- **Prior gate:** Supervisor pass — APPROVED WITH FOLLOW-UPS (non-blocking)
- **Date:** 2026-09-16
- **Verdict:** **SAFE TO MERGE.** No blocking issues found. Zero secrets/internal-content leakage. Zero high/critical dependency or CodeQL findings.

---

## 0. Scope confirmation

This is a docs-tooling-only change: a new `docs-site/` Docusaurus (TypeScript) static site, a git-ignored build-time sync of an allowlisted subset of `docs/`, a new `docs-deploy.yml` GitHub Pages workflow replacing a retired `marked`-based pipeline, and one line added to `scripts/charon_dep_update.sh`.

```
git diff ecde1573~1..5158f19d --stat -- backend/   → empty
git diff ecde1573~1..5158f19d --stat -- frontend/  → empty
```

Confirmed: **no Go changes, no `internal/models` changes, no API/route changes, no database changes, no frontend app changes.** This narrows the Definition of Done to the gates that actually apply.

---

## 1. Definition of Done — gate-by-gate

| # | Gate | Result | Notes |
|---|------|--------|-------|
| 1 | GORM security scan | **N/A (verified)** | Empty `backend/` diff (see §0). Skipped per instructions — no models/GORM/migrations touched. |
| 2 | `npm run audit:ci` (docs-site) | **PASS** | 0 critical, 0 high, 0 low, 18 moderate (moderate allowed; `audit-ci.json` gates on `high: true` with empty allowlist — no CVE exceptions hiding real issues). Re-verified clean in this pass. |
| 3 | CodeQL JS scan, scoped to `docs-site/src/**` | **PASS** | Local `codeql` CLI (v2.26.4) available in this sandbox. Built a scoped database (`--source-root=docs-site`, `--build-mode=none`) and ran `javascript-security-and-quality.qls`. **0 results** across the 4 JS/TS files under `docs-site/src/`. Note: CI's repo-wide `codeql.yml` (`javascript-typescript` matrix job) is *not* restricted to `frontend/` — its `paths-ignore` only excludes `frontend/coverage`, `frontend/dist`, `playwright-report`, `test-results`, `coverage` — so `docs-site/src/**` is already in CI's regular scan scope, not just this one-off local pass. |
| 4 | `docs-deploy.yml` workflow security review | **PASS** | See §2 for full breakdown: permissions scoped to exactly `contents: read, pages: write, id-token: write`; all 3 actions pinned by full commit SHA with version comments; no `pull_request` trigger (only `push: branches: [main]` and `workflow_dispatch`); deploy job additionally gated `if: github.ref == 'refs/heads/main'`; no secrets referenced. |
| 5 | Local Patch Coverage Preflight | **RAN, judged not applicable to this scope** | `bash scripts/local-patch-report.sh` executed and produced both artifacts (`test-results/local-patch-report.md`, `.json`). It reports `Backend patch coverage 73.9%` below the 85% gate — but that finding is **entirely attributable to pre-existing, unrelated changes already on `development` from before this migration** (`backend/internal/services/notification_service.go`, `backend/internal/api/routes/routes.go` — the Web Push notification work, commits `ba3ea15e`/`5523d0d4`/`c2a83721` etc., predating `ecde1573`), because the script diffs `origin/main...HEAD` (the whole branch), not just the 5 docs-site commits. The docs-site migration itself changes zero backend/frontend application lines, so there is no application logic in this feature for a line-coverage gate to measure — docs-site's own build/type-check/audit are its correctness gates, per the task's framing. **This warning is out of scope for this audit and must not block this merge**, but it should be flagged separately since it indicates the `notification_service.go`/`routes.go` coverage gap predates and is independent of this work. |
| 6 | Frontend/backend build+test | **SKIPPED — confirmed out of scope** | Empty diffs under `backend/` and `frontend/` (§0); nothing to validate. |
| 7 | `docs-site` build (`npm run build`) | **PASS** | Succeeds — `[SUCCESS] Generated static files in "build"`. Produces non-fatal `onBrokenLinks: 'warn'` output (deliberate, documented deviation in `docusaurus.config.ts` — synced Markdown retains GitHub-relative links into intentionally-excluded internal dirs; `'throw'` would permanently break every build). This is a known, already-flagged non-blocking follow-up, not a new finding. |
| 8 | `docs-site` type-check (`tsc --noEmit`) | **PASS** | Zero errors. Re-verified clean in this pass. |
| 9 | Staticcheck / Go lint | **N/A — confirmed** | No Go files changed. |
| 10 | Debug/cleanup scan (`docs-site/src/**`) | **PASS** | `grep -rn "console\.\|debugger\|TODO\|FIXME" docs-site/src/` → no matches. No leftover scaffolding debug code. |

---

## 2. `docs-deploy.yml` workflow review (detail)

File: `.github/workflows/docs-deploy.yml`

- **Triggers**: `push: branches: [main]` (paths: `docs-site/**`, `docs/**`, the workflow file itself) and `workflow_dispatch`. **No `pull_request` trigger** — correct, since this job holds `pages: write`/`id-token: write` and must never run against untrusted PR input.
- **Permissions**: top-level block is exactly
  ```yaml
  permissions:
    contents: read
    pages: write
    id-token: write
  ```
  No broader scope (no `contents: write`, no `actions: write`, etc.). Matches the minimum needed for `actions/deploy-pages`.
- **Action pinning**: all three actions pinned to full commit SHA with a trailing `# vX` comment for readability — `actions/checkout@3d3c42e...` (v7), `actions/setup-node@820762786...` (v7), `actions/upload-pages-artifact@fc324d3...` (v5), `actions/deploy-pages@368f8252...` (v5.0.1). No floating tags.
- **`deploy` job**: additionally gated `if: github.ref == 'refs/heads/main'` (belt-and-suspenders on top of the trigger's own branch filter) and uses `needs: build`, so it only ever deploys what `build` just produced from a trusted ref, never a fork's build artifact.
- **Concurrency**: `group: "pages-${{ github.ref }}"`, `cancel-in-progress: false` — avoids two deploys racing without silently dropping either.
- **Secrets**: none referenced anywhere in the file.
- **Timeouts**: `build` 10 min, `deploy` 5 min — bounded, reasonable.

No findings. This workflow is correctly scoped and hardened.

---

## 3. Secrets / internal-content leakage check

**Method**: cross-referenced `docs-site/scripts/docs-manifest.json`'s allowlist against `docs/`'s full top-level listing, then built the site and grepped both the git-ignored sync target (`docs-site/docs/`) and the built output (`docs-site/build/`) for anything from the explicitly-excluded internal-only directories.

- `docs-manifest.json` allowlists 14 individual files and 5 directories (`features`, `configuration`, `guides`, `troubleshooting`, `api`). It does **not** list `security` (dir), `plans`, `reports`, `decisions`, `runbooks`, `implementation`, `reviews`, `patches`, `issues`, `analysis`, `stats_feature_warmup.md`, `SECURITY_PRACTICES.md`, `superpowers`, `testing`, `maintenance`, `performance`, `ci`, `actions`, `development`, `i18n-examples.md`, or `github-setup.md` — all of which exist under repo-root `docs/` and are correctly withheld.
- `docs-site/docs/` (the synced, git-ignored copy produced by `npm run build`'s `prebuild` hook) was inspected directly: its contents match the manifest exactly — no `security/`, `plans/`, `reports/`, `decisions/`, or `runbooks/` directories present.
- `docs-site/build/` (the final built static site) was grepped for known-sensitive filenames/strings (`ghsa`, `vulnerability-analysis`, `break_glass_protocol_redesign`, `emergency-token-rotation`, `emergency-lockout-recovery`) — **zero hits**. The only matches for the string `security` in `build/` are the legitimately-allowlisted `docs/features/security.md`, `docs/features/security-headers.md`, `docs/security.md` (the single top-level public security-features doc — distinct from the excluded `docs/security/` *directory* of internal vulnerability analyses), and `docs/configuration/emergency-setup.md` (a public, feature-scoped config doc, also allowlisted). The broken-link warnings in the build log (`§1` item 7) reference excluded paths as literal unresolvable link text/href — this is a UX defect (404 links), not a content leak: the linked files themselves are never copied or embedded.
- `docs-site/docs/` and `docs-site/build/` are both confirmed git-ignored (`git check-ignore -v` matches `.gitignore:51:docs-site/docs/` and `docs-site/.gitignore:5:/build`), so neither the synced intermediate nor the built output can be accidentally committed.
- Scanned the allowlisted content itself (`docs/features`, `docs/configuration`, `docs/guides`, `docs/troubleshooting`, `docs/api`) for real-looking secret patterns (GitHub PAT prefixes, AWS access-key prefixes, PEM private-key headers, `gotify://` URLs with embedded tokens). The only hits were AWS's own documented placeholder (`AKIAIOSFODNN7EXAMPLE`, used verbatim in AWS's official examples) and a redacted/templated `-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----` template value in a DNS-provider setup guide — both are intentional, non-functional example values, not real credentials.

**No secrets, credentials, or internal-only content leak into the synced or built docs-site output.**

---

## 4. Summary of findings

**Blocking issues: none.**

**Non-blocking / informational (carried over or newly observed, none require action before merge):**

1. `onBrokenLinks: 'warn'` in `docusaurus.config.ts` surfaces a real backlog of dead links pointing at intentionally-excluded internal docs and a few pre-existing dead links unrelated to this migration — already documented in-line by the implementer as a deliberate, scoped deviation from the original plan (§3.7), with the rationale that `'throw'` would permanently break every future build rather than just until a one-time cleanup. Recommend a follow-up docs-writer pass to either rewrite these as plain-text/GitHub-absolute links or promote the referenced files into the public manifest where appropriate — not a merge blocker.
2. The `local-patch-report.sh` backend patch-coverage warning (73.9% vs 85%) is real but belongs to prior unrelated work already on `development` (Web Push notification service/routes changes), not to this docs-site migration, which touches zero backend lines. Flagging so it isn't lost, but it is out of scope for this audit and does not gate this merge.

---

## 5. Environment notes

- CodeQL CLI v2.26.4 was available and used directly in this sandbox (via the `gh-codeql` shim per `CLAUDE.md` troubleshooting notes) — no environment limitation to report for this gate.
- `npm ci` in `docs-site/` reported two blocked postinstall scripts (`@swc/core`, `core-js`) under npm's `allowScripts` policy — expected/benign for these packages (native binary fetch / polyfill detection respectively), did not affect the build, type-check, or audit results.

---

## Verdict

**PASS — safe to merge.** All applicable Definition of Done gates pass: dependency audit clean (0 high/critical), CodeQL JS scan clean (0 findings) on the new surface, workflow permissions correctly minimally scoped with no PR-trigger exposure and fully SHA-pinned actions, build and type-check succeed, no debug/scaffolding code left behind, and — most importantly for a new public-facing static site — no secrets or internal-only documentation leak into either the synced intermediate (`docs-site/docs/`) or the final built output (`docs-site/build/`). The two informational items above are pre-existing or already-acknowledged and do not block merge.
