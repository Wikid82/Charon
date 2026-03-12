# QA Report — ESLint v10 Upgrade (stacked on TypeScript 6.0)

**Date**: 2026-03-11
**Branch**: Current working branch (ESLint v10 + TypeScript 6.0)
**Scope**: Dev tooling upgrade — ESLint `^9.39.3 <10.0.0` → `^10.0.0`, @eslint/js same, 3 npm overrides for peer dep compatibility (react-hooks, jsx-a11y, promise). No source code changes.

---

## Check Results

| # | Check | Status | Details |
|---|-------|--------|---------|
| 1 | Frontend Lint | **PASS** | 0 errors, 857 warnings (all pre-existing, exit 0) |
| 2 | Type Safety (`tsc --noEmit`) | **PASS** | Clean, no type errors |
| 3 | Frontend Unit Tests (Vitest) | **PASS** | 993 passed, 84 skipped, 0 failed (40 test files passed, 5 skipped) |
| 4 | Frontend Build (`vite build`) | **PASS** | 2455 modules transformed, built in 7.85s |
| 5 | Pre-commit Hooks (lefthook) | **PASS** | 6/6 applicable hooks passed (6 skipped — no matching staged files) |
| 6 | npm audit (`--omit=dev`) | **PASS** | 0 vulnerabilities |
| 7 | ESLint Version | **PASS** | v10.0.3 confirmed |

---

## Warnings Detail (Check #1)

857 warnings across 22 rules — all pre-existing, none introduced by the upgrade:

| Count | Rule | Category |
|------:|------|----------|
| 567 | `testing-library/no-node-access` | Test quality |
| 82 | `testing-library/prefer-find-by` | Test quality |
| 54 | `jsx-a11y/label-has-associated-control` | Accessibility |
| 37 | `unicorn/no-useless-undefined` | Code style |
| 29 | `testing-library/no-container` | Test quality |
| 18 | `jsx-a11y/click-events-have-key-events` | Accessibility |
| 18 | `jsx-a11y/no-static-element-interactions` | Accessibility |
| 13 | `testing-library/no-unnecessary-act` | Test quality |
| 12 | `vitest/no-disabled-tests` | Test quality |
| 4 | `vitest/expect-expect` | Test quality |
| 3 | `react-compiler/react-compiler` | React |
| 3 | `security/detect-non-literal-regexp` | Security |
| 2 | `security/detect-unsafe-regex` | Security |
| 2 | `sonarjs/no-identical-functions` | Code quality |
| 2 | `promise/always-return` | Async |
| 2 | `jsx-a11y/role-has-required-aria-props` | Accessibility |
| 2 | `jsx-a11y/heading-has-content` | Accessibility |
| 2 | `jsx-a11y/no-autofocus` | Accessibility |
| 2 | `testing-library/no-manual-cleanup` | Test quality |
| 1 | `unicorn/no-array-for-each` | Code style |
| 1 | `testing-library/prefer-screen-queries` | Test quality |
| 1 | `testing-library/prefer-presence-queries` | Test quality |

These warnings are pre-existing and unrelated to the ESLint v10 upgrade.

---

## Skipped Scans (per task scope)

- **GORM Security Scan** — No backend model changes
- **CodeQL Go** — No Go code changed
- **Docker Image Security** — Dev tooling only, not deployed

---

## Overall Verdict: **PASS**

All 7 verification checks passed. The ESLint v10 upgrade is clean — zero regressions detected. The npm overrides for peer dep compatibility introduce no production vulnerabilities.
