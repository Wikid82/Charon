# PR 18: A WAF rule set must have rules

**Branch:** `fix/waf-rule-sets-need-rules`, cut from `development` @ `41b4a0c6b`
**Type:** plain `fix:` (no new feature surface, no `(security)` scope, see Public Text)
**Source:** local spec `docs/plans/crowdsec_hardening_batch5_spec.md` (D4, 3x.14 R2/R3/R4, "PR 18 commit table", Q-X9). The prior contents of this file (the PR 17 plan) shipped as #1836.
**Anchors:** every `file:line` below was re-read on 2026-11-04 against `41b4a0c6b`. Where the batch 5 spec drifted, the "Drift" table says so.

## 1. Introduction

A WAF rule set can be saved today with no rules in it: `UpsertRuleSet` only checks the name and a 2 MB cap, the web form accepts "a source URL and no content", and the default "OWASP Core Rule Set" preset in the form is exactly that (empty content plus a tarball URL that nothing fetches). `Manager.ApplyConfig` then writes the file, prepends `SecRuleEngine On` and `SecRequestBodyAccess On`, and `GenerateConfig` emits a WAF handler that loads a file with no detection rules. The Security page reports the WAF as applied while it blocks nothing.

Goal: a rule set must contain at least one rule line to be saved or to be applied, and the user is told passively when a stored rule set has none.

### Entities, actions, scenarios (EARS)

* When an admin saves a rule set whose content has no rule line, the API shall refuse it with HTTP 400 and `error_code: ruleset_empty`, and shall store nothing.
* When the proxy config is applied and a stored rule set has no rule line, the manager shall skip it (no file, no path), and log one Warn naming the rule set.
* While the selected rule set has no rule line and the WAF is on, the enforcement report shall give the reason `ruleset_empty` and the Security page shall show one passive line. No banner, no toast, no migration.
* When the user opens the WAF form, the form shall require rule content and shall not offer a preset that has none.
* Rows that are already empty are kept. They stop activating and explain why.

## 2. Research Findings

### Verified anchors (current development)

| Area | Anchor | Fact |
| --- | --- | --- |
| Model | `backend/internal/models/security_ruleset.go:8` | `SecurityRuleSet`, no validation, `Content` is `type:text`. New predicate goes here (imported by `services` and `caddy` without a cycle). |
| Save | `backend/internal/services/security_service.go:415-445` | `UpsertRuleSet`: nil returns nil, name required, `len(Content) > 2 MiB` refused, then create or update. `ErrRuleSetInUse` sentinel lives at `:448-449`. New sentinel goes beside it. |
| Handler | `backend/internal/api/handlers/security_handler.go:430-460` | Any service error becomes a bare 500 `failed to upsert ruleset` (`:445-448`). Needs an `errors.Is` branch. |
| Coded errors | `backend/internal/api/handlers/security_module_writers.go:16-25`, delete path `security_handler.go:488-489` | `codeXxx`/`msgXxx` const pairs answered with `respondCoded(c, status, code, msg, nil)`. New pair: `codeRulesetEmpty = "ruleset_empty"`. |
| Manager | `backend/internal/caddy/manager.go:390-451` | `rulesetPaths` map built in `for _, rs := range rulesets` at `:397`; the engine-directive prepend is at `:411-428`; path recorded at `:443`; stale-file cleanup follows (`:448+`) and removes files not in `rulesetPaths`, so a skipped set's old file is cleaned up for free. |
| Reasons | `backend/internal/caddy/enforcement_report.go:6-14` | `ReasonNoRuleset`, `ReasonRulesetNotSelected`, `ReasonRulesetUnavailable` (`:11`), `ReasonThresholdsUnset`. |
| Classification | `backend/internal/caddy/config.go:1272-1296` | `buildWAFHandlerWithReason` returns `nil, unresolvedRulesetReason(rulesets, selected)` when `directives == ""` (`:1276-1279`). `unresolvedRulesetReason` is at `:1289-1302`: `selected != nil` gives `ReasonRulesetUnavailable`. |
| Existing reason test | `backend/internal/caddy/security_module_resolution_test.go:319-343` | `owasp := []SecurityRuleSet{{Name: "owasp-crs"}}` (empty content) with empty paths expects `ReasonRulesetUnavailable`. This fixture will now classify as `ruleset_empty` and must get a rule line. |
| Frontend note | `frontend/src/components/SecurityEnforcementNote.tsx:10-13` | `KNOWN_REASONS.waf = ['no_ruleset','ruleset_not_selected','ruleset_unavailable']`; key `security.enforcement.waf.<reason>`. Test table at `frontend/src/components/__tests__/SecurityEnforcementNote.test.tsx:25`. |
| Preset | `frontend/src/pages/WafConfig.tsx:24-30` | `WAF_PRESETS[0]` is `OWASP Core Rule Set`, `content: ''`, tarball `url`. `handlePresetChange` (`:135-145`) copies `name`, `url`, `content`. |
| Form validity | `frontend/src/pages/WafConfig.tsx:157` | `isValid` is true when the name is set and either content or a source URL is set. Submit sends `source_url` and `content` (`:149-152`). Source URL field at `:234-238`; content label asterisk at `:244` is hidden when a URL is set. |
| Error mapping | `frontend/src/api/errorBody.ts:77-84` | `WAF_CODE_MESSAGE_KEYS` with three entries (`waf_rules_source_unknown`, `waf_ruleset_in_use`, `invalid_waf_mode`). |
| Locales | `frontend/src/locales/*/translation.json` | 29 locale files. `wafConfig.sourceUrlHelper` exists in all 29 (en `:690`). The coded `wafConfig.errors.*` keys (en `:721`) and `security.enforcement.waf.*` (en `:604`) exist only in **en, de, es, fr, zh**; the rest fall back to English. Parity test: `frontend/src/__tests__/i18n.test.ts:68`. |
| Docs | `docs/api.md:116` (reason list), `:262-271` (POST body `"content": "# raw rules"`), `:285` (delete 409 text); `docs/cerberus.md:168-174` (`SourceURL // Optional URL for rule updates`); `docs/features/waf.md:33-37` | |

### Drift from the batch 5 spec

| Spec says | Reality at `41b4a0c6b` |
| --- | --- |
| Classification at `caddy/config.go:1297-1299` | `unresolvedRulesetReason`, `config.go:1289-1302`. |
| Locale keys "in all locale files (parity test)" | New `wafConfig.errors.ruleset_empty` and `security.enforcement.waf.ruleset_empty` go in the 5 locales that carry the sibling keys (en, de, es, fr, zh), following the existing precedent. Only the reworded `sourceUrlHelper` touches all 29 (see Q2). |
| `tests/security/waf-config.spec.ts` "any step using the OWASP preset or URL-only create" | No step uses the preset or a URL-only create (the file only has a loose `/ruleset\|rules\|owasp\|crs/i` text check at `:95-97`). The E2E work is therefore new coverage, not a fix. |
| `docs/cerberus.md:173` | Anchor re-checked: still `:173` (`SourceURL  string ... // Optional URL for rule updates`). No drift. |
| Many fixtures are not empty but have no rule line, and many are empty | Rule instead of a hand list: grep `SecurityRuleSet{`, `UpsertRuleSet` and `security/rulesets` across `backend/`, and fix each hit that goes through a save or apply path (service, handler, manager) or that expects `ruleset_unavailable` for empty content. Known hits, not exhaustive. caddy: `config_waf_test.go` (about 15 empty `owasp-crs` fixtures), `config_waf_security_test.go`, `config_test.go`, `config_generate_test.go`, `config_generate_additional_test.go`, `config_extra_test.go`, `config_bouncer_key_path_test.go`, `manager_test.go`, `manager_additional_test.go` (`"rules"`, `"test-rule-content"`, the `ruleContent` fixtures), `manager_patch_coverage_test.go`, `manager_enforcement_test.go`, `security_module_resolution_test.go` (fixture at `:154`, table case at `:185` expecting `ReasonRulesetUnavailable` for empty content, and `:319-343`). handlers: `security_handler_authz_test.go`, `security_handler_coverage_test.go`, `security_settings_handling_test.go`, `security_module_writers_test.go`, `additional_coverage_test.go`, `benchmark_test.go`, `perf_assert_test.go`, `proxy_apply_detached_test.go`, `crowdsec_stop_apply_test.go`, `security_handler_rules_decisions_test.go` (`"content":"test"` at `:91,:186`), `security_handler_audit_test.go` (`:386`). services: `security_service_test.go` (`"rule: 1"`, `:1034`), `coverage_boost_test.go:169`, `security_module_writers_test.go`. Also `api/routes/routes_test.go` and `cmd/api/main_test.go`. Tests that call `GenerateConfig` or `buildWAFHandler` directly with a hand-made `rulesetPaths` do not go through the manager skip, but the selection change (tiers 2-5, section 3.2) affects them when the set is empty, so they are in scope too. |

### Design decisions (carried from 3x.14, unchanged)

* **One predicate** `HasRuleLines(content string) bool` in `models`: true when some line, after trimming whitespace, is not a `#` comment and its first word, lowercased, is exactly `secrule`, `secaction` or `include` (word boundary: the first word ends at a space, tab or end of line, so `SecRuleEngine`, `SecRuleRemoveById 1`, `SecRuleUpdateTargetById`, `SecDefaultAction`, `SecMarker`, `Includes foo` are not rule lines). Decisions: (a) a leading UTF-8 BOM is stripped before the first line is read; (b) a comment line is a comment even if it ends in a backslash, so a `\`-continued comment does not swallow the next line: `# note \` followed by `SecRule ...` counts as a rule (the continuation rule applies only to non-comment lines, where a `\`-continued line counts as its first line); (c) an indented comment is a comment; (d) `\r\n` and `\n` both split; (e) **no per-line size limit**: use `strings.Split`/`strings.FieldsFunc` or a scanner with a raised buffer, never the default 64 KiB scanner (legacy rows may exceed the save cap). Note: `Include` lets an admin include arbitrary files; that is pre-existing behavior and out of scope here.
* **Skip rather than delete.** Existing empty rows stay in the database.
* **Selection skips empty sets in the fallback tiers.** In `buildWAFHandlerWithReason` (`config.go:1206+`) tiers 2 to 5 (host `ruleset_name` match, application match, `owaspFallback`, the lone-ruleset fallback) only consider sets for which `HasRuleLines` is true, so a legacy empty row never hides a valid set. Tier 1 (the explicit `waf_rules_source`) stays strict: if the chosen set is empty, no handler is built and the reason is `ruleset_empty`. The lone-ruleset case with one empty set therefore yields `ruleset_empty` (selected stays nil but all sets are empty): decide the classification in code so that "every candidate is empty" reports `ruleset_empty`, not `ruleset_not_selected`.
* **`advanced_config.ruleset_name` path** (`config.go:634-636` and `:652-654`): when the named set was skipped (no path), the handler is left untouched, exactly as today when a path is missing: no directives-less handler is appended. Add a test.
* **`ApplyConfig` skipping never errors.** The skip logs one Warn and continues; it never returns an error and never fails the apply. The Warn is the only apply-time signal; the passive reason is the user-facing one. Test: a legacy empty row plus a valid set still applies and writes the valid file.
* **Reason, not a new JSON field.** `ruleset_empty` rides the existing enforcement `reason` string. No schema change, no migration, no new response field.
* **The classification tests the content, not the file.** With the manager skip, a selected empty rule set has no path, so without the new branch it would be reported as `ruleset_unavailable`.
* **Remove the placeholder preset** rather than make it fetch anything (a real CRS import needs download, verification and includes: a `feat`, out of scope).

## 3. Technical Specifications

### 3.1 API contract

`POST /api/v1/security/rulesets` (admin only, unchanged route `routes.go:990`).

| Case | Status | Body |
| --- | --- | --- |
| `content` has at least one rule line | 200 | `{"ruleset": {...}}` (unchanged) |
| `content` empty, whitespace, comments only, only engine directives, or only a `source_url` | 400 | `{"error": "A rule set needs at least one rule. Add a SecRule line.", "error_code": "ruleset_empty"}` |
| name missing | 400 | unchanged (`name required`) |
| content over 2 MiB | unchanged | generic error today; not part of this PR |

The check runs in the service (single source of truth) and the handler maps `errors.Is(err, services.ErrRuleSetEmpty)` to `respondCoded(c, http.StatusBadRequest, codeRulesetEmpty, msgRulesetEmpty, nil)`. The check is placed after the name check and the size check, and before the database read, so nothing is written. An update that blanks the content of an existing set is refused the same way (the stored content is not overwritten).

Message wording stays neutral: describe the behavior only.

### 3.2 Backend signatures

* `models.HasRuleLines(content string) bool` in a new file `backend/internal/models/security_ruleset_rules.go` (or beside the struct). Pure function, no allocation beyond `strings.Split`/`bufio.Scanner`. Use a scanner with an enlarged buffer or `strings.Split` (content is capped at 2 MiB at save time but legacy rows may be larger, so do not rely on `bufio.Scanner` defaults).
* `services.ErrRuleSetEmpty = errors.New("rule set has no rules")` in `security_service.go` beside `ErrRuleSetInUse`.
* `SecurityService.UpsertRuleSet`: add `if !models.HasRuleLines(r.Content) { return ErrRuleSetEmpty }` after the size check.
* `handlers`: const pair `codeRulesetEmpty`/`msgRulesetEmpty` in `security_module_writers.go`; in `security_handler.go:445` add `errors.Is(err, services.ErrRuleSetEmpty)` -> `respondCoded(... 400 ...)` **before** the generic 500 branch. A nil payload stays a no-op (`UpsertRuleSet(nil)` returns nil, keep the existing assertion) and gets an explicit test.
* `caddy.Manager.ApplyConfig` (`manager.go:397`): at the top of the loop, `if !models.HasRuleLines(rs.Content) { logger.Log().WithField("ruleset", rs.Name).Warn("skipping WAF rule set that has no rules"); continue }`. Exactly one Warn per skipped rule set per apply. The skip must come before the engine-directive prepend, which is what makes an otherwise empty file look non-empty.
* `caddy.ReasonRulesetEmpty = "ruleset_empty"` in `enforcement_report.go`.
* `unresolvedRulesetReason`: in the `selected != nil` branch, return `ReasonRulesetEmpty` when `!models.HasRuleLines(selected.Content)`, otherwise `ReasonRulesetUnavailable`; when `selected == nil` and every ruleset is empty, return `ReasonRulesetEmpty`.

### 3.3 Frontend

* `WafConfig.tsx`: delete the `OWASP Core Rule Set` entry from `WAF_PRESETS`; **drop the `url` property from every preset and the `setSourceUrl(preset.url)` call in `handlePresetChange`** (no dead code; selecting a preset leaves the source URL field untouched); `isValid` becomes `name.trim().length > 0 && content.trim().length > 0`; the content label asterisk is always shown; `source_url` stays as an optional reference field. In `WafConfig.spec.tsx` the `source_url` cases (`:139`, `:250-254`, `:334`) are **replaced, not removed**: keep the link-display and payload assertions using content-bearing fixtures, and add a test that a URL-only form keeps submit disabled.
* `errorBody.ts`: add `ruleset_empty: 'wafConfig.errors.ruleset_empty'` to `WAF_CODE_MESSAGE_KEYS`.
* `SecurityEnforcementNote.tsx`: add `'ruleset_empty'` to `KNOWN_REASONS.waf`.
* Locales: `wafConfig.errors.ruleset_empty` and `security.enforcement.waf.ruleset_empty` in en, de, es, fr, zh (the precedent). `wafConfig.sourceUrlHelper` is reworded properly in en, de, es, fr, zh, and the **English text is copied into the other 24 locales by an uncommitted one-off node script** (no Python). The PR body and QA report say the 24 are English placeholders, not translations, and make no claim about translation quality.

Suggested English text (neutral, behavior only):

| Key | Text |
| --- | --- |
| `wafConfig.errors.ruleset_empty` | "A rule set needs at least one rule. Add a SecRule line." |
| `security.enforcement.waf.ruleset_empty` | "The WAF is on, but the selected rule set has no rules. Add rules on the WAF page." |
| `wafConfig.sourceUrlHelper` | "Optional. The address is stored for reference and is not downloaded." |

### 3.4 Data flow

Entry: WAF form submit -> `POST /security/rulesets` -> handler -> `SecurityService.UpsertRuleSet` (predicate, 400) -> DB -> `applyProxyConfigDetached` -> `Manager.ApplyConfig` (predicate again, skip + Warn, so a legacy empty row never produces a file) -> `GenerateConfig` (no path for the set; `unresolvedRulesetReason` names `ruleset_empty`) -> enforcement report -> `/security/status` -> `SecurityEnforcementNote`.

### 3.5 Edge cases

* Legacy empty row selected as `waf_rules_source`: stays selected, WAF handler is not generated for hosts that resolve to it, reason `ruleset_empty`. Deleting it still hits the existing 409 `waf_ruleset_in_use`; the user fixes it by editing the content (allowed, because the new content has rules) or choosing another source.
* Content that is only `SecRuleEngine On` or `SecRequestBodyAccess On`: refused (not rule lines).
* `Include` of a file that does not exist: counts as a rule line by design (the predicate cannot know). Out of scope.
* Two hosts, one with an empty set via `advanced_config.ruleset_name`: the report keeps the first unprotected host's reason, existing behavior.
* Name with only an empty-content duplicate of an OWASP-named set: `owaspFallback` selection picks by name, then the content check reports `ruleset_empty`. Covered by the manager/report test.
* The 400 happens before `applyProxyConfigDetached`, so no Caddy reload on a refused save.

## 4. Priorities & Ordering

| Order | Item | Why |
| --- | --- | --- |
| 1 | This PR: WAF rule sets need rules (Aikido finding 782255504). | Our own finding, fix (not `feat`), reachable from the main WAF screen in two clicks; already the sequenced PR 18 after #1836. |
| 2 | Neutral enhancement issue: real OWASP CRS import (download, verification, includes). | Out of scope for this PR (a `feat`); filed as a GitHub issue per CLAUDE.md, scheduled after the user's WAF work. Wording must not mention the finding. |
| 3 | Findings from the next CrowdSec/WAF PR | Merge into the hardening spec ranked by importance (project convention). |

No critical or high out-of-scope finding is known that must precede this PR. The Aikido reply text for 504 / 782255504 is local only, for the user's console: it never goes in a commit, comment, issue, PR or doc.

## 5. Implementation Plan

### Commit Slicing Strategy

Decision: **one PR, six ordered logical commits**. Q-X9 (Option B) already made this its own PR. Dev agents run scoped tests only and leave changes in the working tree. Only the main session commits, pushes and opens the PR. All commands run in the foreground and block until they finish.

| # | Subject | Scope | Files | Depends on | Scoped validation gate |
| --- | --- | --- | --- | --- | --- |
| 1 | `test: pin the rule set fixtures that carry real rules` | Move every existing test fixture that has no rule line (list in Drift table; grep `SecurityRuleSet{`, `UpsertRuleSet(`, JSON `"content"` in handler tests) onto a real `SecRule` line. Add no new behavior. Green before and after. | `services/security_service_test.go`, `services/coverage_boost_test.go`, `handlers/security_handler_rules_decisions_test.go`, `handlers/security_handler_audit_test.go`, `caddy/manager_additional_test.go`, `caddy/config_generate_additional_test.go`, `caddy/security_module_resolution_test.go` (only the fixtures that must stay "unavailable" keep a rule line plus empty paths) | none | Gate G1 (below), whole packages, green |
| 2 | `fix: refuse a WAF rule set without rules` | `HasRuleLines` + table test; `ErrRuleSetEmpty`; `UpsertRuleSet` check; handler const pair + 400 branch; manager skip with one Warn. Tests: predicate table (blank, spaces, comment-only, indented comment, comment ending in a backslash followed by `SecRule`, `\r\n`, UTF-8 BOM before `SecRule`, `SecRuleEngine On`, `SecRuleRemoveById 1`, `SecRuleUpdateTargetById` (all not rule lines), `SecRule`/`secrule` mixed case, `SecAction`, `SecAction<tab>`, `Include<tab>`, `Includes foo` (not), continued line, leading whitespace, a single line over 1 MiB), selection (legacy empty row plus a valid unselected set -> the valid one is used; explicit empty `waf_rules_source` -> `ruleset_empty`), `advanced_config.ruleset_name` naming a skipped set leaves the handler untouched, ApplyConfig with an empty row plus a valid set still applies the valid file with no error, handler nil-payload no-op, service refuses blank/comment/URL-only and an update that blanks content (stored row unchanged), handler 400 `error_code: ruleset_empty` with nothing stored and no Caddy apply, manager skips an empty row (no file, absent from `ruleset_paths`, one Warn, stale file removed) while a real set still gets its file. | `backend/internal/models/security_ruleset.go` (or new file) + `_test.go`, `services/security_service.go` + test, `handlers/security_module_writers.go`, `handlers/security_handler.go` + test, `caddy/manager.go` + test | 1 | Gate G2 (below), whole packages |
| 3 | `fix: explain a selected WAF rule set that has no rules` | `ReasonRulesetEmpty`; `unresolvedRulesetReason` branch (content, not file); update the `:319` reason test (empty content -> `ruleset_empty`, rule line + empty paths -> `ruleset_unavailable`); a manager-level test that a seeded empty selected row yields `ruleset_empty` in the enforcement report and `/security/status`; frontend `KNOWN_REASONS`, locale keys (5 locales), Vitest row in `SecurityEnforcementNote.test.tsx`; `docs/api.md:116` reason list entry. | `caddy/enforcement_report.go`, `caddy/config.go`, `caddy/security_module_resolution_test.go`, `caddy/manager_enforcement_test.go`, `handlers` status test if one asserts reasons, `frontend/src/components/SecurityEnforcementNote.tsx`, its test, `frontend/src/locales/{en,de,es,fr,zh}/translation.json`, `docs/api.md` | 2 | Gate G3 (below) |
| 4 | `fix: remove the WAF preset that has no rules and require rules in the form` | `WafConfig.tsx` preset removal, `isValid`, asterisk, hint wording; `errorBody.ts` mapping; `wafConfig.errors.ruleset_empty` (5 locales) and reworded `sourceUrlHelper` (see Q2); Vitest updates and additions; Playwright. | `frontend/src/pages/WafConfig.tsx`, `frontend/src/api/errorBody.ts` + its test, `frontend/src/pages/__tests__/WafConfig.spec.tsx` (the preset test at `:493-498` switches to a remaining preset; the URL-only cases at `:139`, `:250-254`, `:334` are revisited), `WafConfig.pending.test.tsx`, `WafConfig.rulesSource.test.tsx` (only if fixtures break), locale files, `tests/security/waf-config.spec.ts` | 3 | Gate G4 (below) |
| 5 | `docs: say that a WAF rule set needs rules` | `docs/api.md` POST example uses a real `SecRule` line and gets a 400 `ruleset_empty` row; `docs/cerberus.md:171` comment becomes "stored for reference, not fetched"; `docs/features/waf.md` adds that a rule set needs rules, that existing empty sets are kept but inactive (they are not deleted and can be edited), and what the Security page says when it has none; `docs/features.md` only if it describes presets. Neutral wording. Run markdownlint on the touched files. | `docs/api.md`, `docs/cerberus.md`, `docs/features/waf.md`, possibly `docs/features.md` | 4 | `npx markdownlint-cli2 <touched files>` (or the repo's lefthook markdownlint hook); docs-site sync check only if a new nav entry were added (none is) |
| 6 | `docs: add the QA report for the WAF rule set fix` | `docs/reports/qa_report.md` written by `qa-security`. Not a dev-agent commit. | `docs/reports/qa_report.md` | 5 | the container gates below |

The suggested sequence (tests first, foundation, backend, frontend, docs) is kept, with the predicate living in commit 2 because it has no standalone behavior.

Per-commit scoped gates (foreground, scoped tests only; the regexes are passed to `-run`):

* G1 (commit 1): `cd backend && go test ./internal/models/ ./internal/services/ ./internal/caddy/ ./internal/api/handlers/ ./internal/api/routes/ ./cmd/api/ -count=1`. Whole affected packages, no `-run` filter: the fixture sweep touches tests all over those packages.
* G2 (commit 2): the same command as G1.
* G3: `cd backend && go test ./internal/caddy/ ./internal/api/handlers/ -count=1 -run 'Enforcement|Status|Unresolved'`, then `cd frontend && npx vitest run src/components/__tests__/SecurityEnforcementNote.test.tsx src/api/__tests__/errorBody.test.ts src/__tests__/i18n.test.ts`.
* G4: `cd frontend && npx vitest run src/pages/__tests__/WafConfig src/components/__tests__/SecurityEnforcementNote.test.tsx src/api src/__tests__/i18n.test.ts`, `npm run type-check`, then `cd /projects/Charon && npx playwright test tests/security/waf-config.spec.ts --project=firefox`. The `errorBody` test lives under `src/api` (locate it with `ls`; adjust the path if it sits elsewhere).

Playwright additions in commit 4 (`tests/security/waf-config.spec.ts`, Firefox only, seeded via the API fixtures already in that file): (a) creating with only a URL is blocked client-side (submit disabled) and the preset list has no "OWASP Core Rule Set"; (b) a direct API `POST` with a comment-only body returns 400 `ruleset_empty` (via `request`); (c) a seeded empty row, selected as the WAF source with the WAF on, shows the `waf-enforcement-note` containing the new text. Item (c) needs the seed to bypass the new check (write through the test DB seeding or via a legacy path); if no such seed exists in the E2E harness, cover (c) in Vitest and the Go report test only and say so.

### Test inventory (new code must be covered)

* Go: `HasRuleLines` table; service refusal; handler 400 body and "nothing stored, no apply"; manager skip, one Warn (assert via a captured logger hook), stale file cleanup; `unresolvedRulesetReason` both branches; status endpoint reason passthrough.
* Frontend, with the **thin branch coverage margin (85.1%, floor 85%)** in mind: every new or changed branch needs a test or coverage can slip below the gate. Branches to cover: `isValid` with content only, URL only (now false), blank content; the label asterisk (now unconditional, which removes a branch, net positive); preset change with the remaining presets; `errorBody` `ruleset_empty` mapping with and without an `error_code`; `KNOWN_REASONS` `ruleset_empty` row and the unknown-reason fallback. Deleting the preset removes branches, so net coverage should not drop, but the dev agent must not delete tests without replacement.
* Playwright as above.

### Container gates (run once, by `qa-security` or the main session, not by dev agents)

1. Rebuild the E2E container (`.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`), then `npx playwright test tests/security/waf-config.spec.ts --project=firefox` (targeted, Firefox only; full-suite and cross-browser runs are CI-only).
2. GORM scan: no model field or migration change, but the predicate lives in `models/`, so run `./scripts/scan-gorm-security.sh --check` (zero CRITICAL/HIGH).
3. `bash scripts/local-patch-report.sh`, then `scripts/go-test-coverage.sh` (>= 85%) and `scripts/frontend-test-coverage.sh` (>= 85%). If coverage falls short, `qa-security` returns the uncovered files to the dev agent.
4. `cd backend && go build ./...`, `cd frontend && npm run type-check && npm run build`.
5. `lefthook run pre-commit` (staticcheck is blocking). Local CodeQL/Trivy are deferred to CI because this is a `fix:` with no new endpoint or component.
6. Live check on the rebuilt test container (:8787): save a comment-only rule set via the UI and the API (both refused), then confirm a real `SecRule` set still returns 403 on an attack request, and that a legacy empty row shows the passive line and produces one Warn per apply.

### Public Text

* Plain `fix:`/`test:`/`docs:` subjects as in the table. No `(security)` scope: this is a validation fix whose subject adds nothing the changelog should flag, and the batch 5 spec reserves the one vague `(security)` subject for the import hardening in PR 16. No breaking-change markers (project is v0).
* Code comments, test names, error strings, docs and the PR body describe behavior only ("a rule set needs rules"). No mention of Aikido, a pentest or the finding id, a scanner, bypass or exploitation wording.
* PR body: summary of the five behaviors in section 1 plus the validation commands. No session id, no attribution footer.

## 6. Risks and Mitigations

| Id | Risk | Mitigation |
| --- | --- | --- |
| R1 | Predicate false positives or negatives (`SecRuleEngine` matching `secrule`; indented or continued lines) | Word-boundary match, table test with those exact cases; same function on both paths so they cannot disagree. |
| R2 | Large fixture churn hides a real regression | Commit 1 is test-only and green on its own; the Drift table lists the known fixtures; the dev agent greps rather than trusting the list. |
| R3 | A user relied on an empty "placeholder" row (the batch spec's X-R5) | The row is kept, only stops activating; passive reason on the Security page; the 400 text says what to add. |
| R4 | Existing install has the empty OWASP preset selected as the WAF source and sees the WAF go from "applied" to "not applied" | Intended and honest: it never blocked anything. Passive line only; documented in `waf.md`. |
| R5 | A user may legitimately want a source-URL-only row for later | It was never fetched; the hint now says so. A real import is the filed enhancement issue. |
| R6 | Thin frontend branch coverage margin | Test list above; QA runs the coverage script and bounces uncovered files back. |
| R7 | Locale parity test fails on the partial 5-locale precedent | Add the keys to the same 5 locales as the sibling keys; run `i18n.test.ts` in commit 3 and 4 gates. If the parity test demands all 29, add English text to the rest via a one-off node script (not committed; no Python). |
| R8 | Wording leaks the finding | Public Text rules above; supervisor checks strings and docs in review. |

## 7. Open Questions (with recommendations)

* **Q1. Placeholder preset: remove it, or keep it as a one-click starter that fills in real rules?** Recommend **remove** (the batch spec's decision). A real CRS import is a `feat`; file a neutral enhancement issue.
* **Q2. Source URL hint.** `sourceUrlHelper` exists in all 29 locales and now says something false. Recommend (decided with the Supervisor): reword properly in en, de, es, fr, zh; copy the English into the other 24 with an uncommitted node script and say so in the PR body and QA report, without claiming translation quality.
* **Q3. Keep `SourceURL` in the form at all?** Recommend keep (optional reference field, no API change), revisit when the import feature is designed.
* **Q4. Should an update that blanks an existing set's content be refused, or allowed so users can "clear" a set?** Recommend **refused** (same rule on create and update); deleting the set is the way to remove it.
* **Q5. E2E seed for a legacy empty row.** If the Playwright harness cannot seed past the new check, is Vitest plus the Go report test enough for the passive line? Recommend yes, and state it in the QA report.
* **Q6. Should the Warn also fire when the 400 is returned on save?** Recommend no: the 400 is the feedback; Warn is for apply-time skips only.
* **Q7. Keep `include` as a rule directive?** Recommend yes (an include pulls rules in), accepting that an include of a missing file is not detected here.

## 8. Acceptance Criteria

* Saving a rule set with blank, whitespace, comment-only, engine-directive-only or URL-only content returns 400 `error_code: ruleset_empty`; nothing is stored and no Caddy apply runs.
* A rule set with a `SecRule`, `SecAction` or `Include` line saves and is enforced as before (an attack request still returns 403).
* `ApplyConfig` skips a stored rule set without rule lines, writes no file for it, removes any stale file, and logs one Warn per skipped set per apply.
* The enforcement report and the Security page show the passive `ruleset_empty` line for a selected empty set; no banner, toast or badge.
* The WAF form has no "OWASP Core Rule Set" preset, requires rule content, and shows the translated refusal if the server still refuses.
* `docs/api.md`, `docs/cerberus.md` and `docs/features/waf.md` say a rule set needs rules; markdownlint is clean.
* Scoped gates pass per commit; the QA container gates pass; frontend and backend coverage stay at or above 85%.

## 9. Rollback and Contingency

* The change is additive validation plus a skip: there is no schema change, migration or new response field, so a revert of the PR restores the previous behavior with no data fix-up. Rows are never deleted or modified by this PR.
* If commit 3 (reason plumbing) proves too wide in review, drop it: commits 2 and 4 still close the finding, and the note falls back to the existing `ruleset_unavailable` text.
* If the locale parity test blocks the 5-locale precedent, widen to all 29 (Q2) rather than relax the test.
* If a fixture sweep in commit 1 reveals a test that depends on content without rule lines for a reason other than convenience, stop and report; never `.skip` or delete a test to get green.

## 10. Handoff

After approval, hand to `supervisor` for plan review, then implement commit by commit with `backend-dev` (commits 1-3 backend parts) and `frontend-dev` (commits 3-4 frontend parts, with `playwright-dev` for the spec if preferred), `docs-writer` for commit 5, and `qa-security` last. Dispatch prompts must say: run all commands in the foreground and block until done, scoped tests only, do not commit.
