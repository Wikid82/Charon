# PRs 19 and 20: Security settings checks, and the CrowdSec-labelled leftovers

**Base:** `development` @ `d36ded796` (the prior contents of this file, the PR 18 plan, shipped as #1838).
**Source:** issues #1761, #1756, #1770, #1773, #1774, #1741, #1701 (read with `gh issue view`), plus the local batch 5 spec (`docs/plans/crowdsec_hardening_batch5_spec.md`, untracked, read-only; sections 3o/3p and the C4/U1 rows).
**Authority:** the code at `d36ded796`. Every anchor below was re-read on 2026-11-05; the issue texts and the batch 5 spec were checked against it and the drift is listed in section 2.

## 1. Introduction and decision

Seven small findings remain from the CrowdSec hardening batches. They are all low severity, none is user-submitted, and none is critical or high. Two of them change backend behavior on a security surface (#1761 validation, #1756 the on/off decision); one narrows a rollback (#1770); four are tests, docs and locale text.

**Decision: two PRs, merged in this order.** This is a deliberate exception to "one feature = one PR": these are independent bug fixes, not one feature, and they have different risk, different validation gates and different reviewers' attention.

| PR | Branch (from `development` @ `d36ded796`) | Issues | Why it is its own PR |
| --- | --- | --- | --- |
| **PR 19** `fix: check security settings values and keep a refused change from overwriting newer ones` | `fix/security-settings-checks` | #1761, #1770, #1756 (reader behavior unchanged; see Q1) | Backend only, security surface, needs the container gate and the GORM scan (it touches `internal/models`). No frontend, no locale keys. |
| **PR 20** `fix: tidy security test helpers, translations and docs` | `fix/security-settings-leftovers` (exists, clean at `d36ded796`) | #1773, #1741, #1701, #1774 | Test, JSON and Markdown only plus one test-script fix. Cheap gates; must not wait on, or be blocked by, the riskier PR. |

Merging PR 20 first is also fine (disjoint files except `docs/api.md`, see risks); the recommended order is PR 19 then PR 20 so the PR 20 docs commit can describe the validated ranges as shipped.

Not in either PR:

* **#1743** (1358 frontend lint warnings): its own sweep PR together with #1716 and #1820. It is a mechanical, wide-diff change and would bury these fixes.
* **#1776**: no longer reproduces. Nothing to do; close it with a short comment when convenient.
* **#1840** (status counts a host with its own WAF handler as unprotected): it does **not** fall out of #1756. #1756 is about the Cerberus on/off switch; #1840 is in the WAF eligibility count (`backend/internal/caddy/config.go:546-548`, `report.WAF.record(wafH != nil, reason)` fed by `buildWAFHandlerWithReason`, plus `enforcement_report.go:34`). Different code, different tests, a different container reproduction. Leave it open and schedule it right after PR 20 as its own small `fix:` (see section 3).

### Entities, actions, scenarios (EARS)

* When an admin saves the security config with a value outside its allowed range, the API shall answer 400 with a plain message and shall store nothing (no row column, no module key).
* When a save carries a value equal to the one already stored, the API shall accept it even if it is outside the new range, so an old stored value never blocks an unrelated save (same rule `checkRulesSource` uses).
* When the proxy refuses a change and the writer restores, the restore shall write back only the settings keys and row columns the writer itself changed, and only where the current value is still the one the writer wrote; anything a concurrent admin write changed is kept.
* When an admin presses Disable (the break-glass `/security/disable`), the switch shall be off for the proxy, the status and the management middleware alike, not only for the middleware (PR 19, write side). No reader's rule changes, so no existing instance's enforcement changes on upgrade.

## 2. Research findings (re-verified at `d36ded796`)

### 2.1 #1761 validation gap

* `backend/internal/services/security_config_update.go:56-70` `validate()` checks only `admin_whitelist` (`ValidateAdminWhitelist`), `crowdsec_mode` (`ValidateCrowdSecMode`) and `waf_mode` (`validWAFModes`, `ErrInvalidWAFMode`). **`waf_mode` is already validated** (shipped in #1767); the issue's `waf_mode` item is done.
* Not checked, and written straight to the row by `applyTo` (lines 179-228): `waf_paranoia_level`, `rate_limit_requests`, `rate_limit_burst`, `rate_limit_window_sec`, `rate_limit_bypass_list`, `crowdsec_api_url`. `rate_limit_mode` is also unchecked (`writeRateLimitModeKey` treats any non-empty value other than `enabled` as off).
* The consumers already ignore bad values quietly: `caddy/config.go:1387-1388` falls back to paranoia 1 outside 1..4; `parseBypassCIDRs` (`config.go:1511-`) silently drops invalid entries and accepts a plain IP; `buildRateLimitHandler` returns nil while a proxy threshold is `<= 0` (reported as `thresholds_unset`). So **zero means "not set" and must stay valid**; the harm is that a typo is stored and silently does nothing, and a huge number is stored.
* The handler path: `security_handler.go:302` `UpdateConfig` -> `h.svc.ApplyUpdate`; errors go through `respondConfigUpdateError` (`security_module_writers.go:70`), where any `*services.InvalidConfigError` becomes 400 `{"error": <message>}`.
* The Rate Limiting page already limits its inputs to requests 1..1000, burst 1..100, window 1..3600 (`frontend/src/pages/RateLimiting.tsx:208-236`), so the backend ceilings below are looser than the UI and no UI change is needed.
* Existing tests that pin today's acceptance and must change: `backend/internal/api/handlers/security_handler_audit_test.go:454-473` (`negative_burst` and `huge_values` expect 200 "Backend accepts"). The commit must grep for any other case that sends an out-of-range value (`security_config_partial_update_test.go`, `security_config_save_pins_test.go`, `security_settings_handling_test.go`).
* Other writers of the same columns (`SecurityService.Upsert`, used by `Disable`, `settings_handler.go:782` and the legacy handlers) copy a row that was read from the database, so they introduce no new values; they stay out of scope.

### 2.2 #1770 restore broader than the change

* `models.CaptureModuleState` (`models/security_module_state.go:272-313`) images **every** module and Cerberus key plus ten row columns. `RestoreSettings`/`RestoreRow` (lines 316-358) write all of them back unconditionally.
* `restoreModuleState` (`handlers/security_module_writers.go:54`) runs after the apply, outside the writer's transaction, in two separate write-locked transactions. A concurrent admin write that landed between the writer's commit and the restore is overwritten, while that admin got a 200.
* Four writers use it: `security_handler.go:327` (config save) and `:1210` (module toggle), `settings_handler.go:442` and `:611`.
* The older pair is separate: `snapshotSettings`/`restoreSettings`/`snapshotDefaultSecurityConfigState`/`restoreDefaultSecurityConfigState` (`security_handler.go:1226-1300`), used only by the CrowdSec Start revert (`crowdsec_handler.go:820-866`, `crowdsec_lifecycle.go:260`). That revert has its own rule: a row Start created is kept with seed values (`crowd_sec_mode` disabled, `enabled` false) instead of being deleted, and it restores `crowd_sec_mode`, which the newer snapshot does not image.

### 2.3 #1756 the four readers (verified; they differ more than the issue says)

| Reader | Where | Order | Accepted spellings | Nothing stored |
| --- | --- | --- | --- | --- |
| Proxy (reference) | `caddy/cerberus_decision.go:19` `CerberusEnabledForProxy`; also boot via `services.ProxyCerberusCheck` | static, then default row `Enabled`, then `feature.cerberus.enabled`, then legacy `security.cerberus.enabled` (only when the feature key is absent) | `true`, any case; an empty or other value is "off" | the row or static value |
| Status | `handlers/security_handler.go:112-145` | static, row `Enabled`, feature key, legacy key | `true`, any case; an **empty value counts as absent** | the row or static value |
| Management middleware | `cerberus/cerberus.go:114` `IsEnabled` | **row `Enabled == false` is a hard off**, then feature key, legacy key, then static, then static heuristics (`CrowdSecMode == local`, any static module on, all-zero static config means on) | `true`, any case | the static heuristics |
| Presets and console gate | `handlers/crowdsec_handler.go:670` `isCerberusEnabled` (5 call sites in `crowdsec_preset_handler.go`, `crowdsec_apply_jobs_handler.go`) | feature key only, then env `FEATURE_CERBERUS_ENABLED`, then env `CERBERUS_ENABLED`, then **default true** | `true`, `1`, `yes`, trimmed, any case | **on** |

Three facts decide the design:

0. **Key-only enablement is real.** `POST /security/cerberus/enable|disable` (`toggleSecurityModule` -> `writeModuleToggle`, `security_module_writers.go:31-47`) and the Cerberus feature toggle in `frontend/src/pages/SystemSettings.tsx:309` write only `feature.cerberus.enabled` and `security.cerberus.enabled`, never the row. A fresh install seeds the row with `Enabled: false` (`models/seed.go:14-18`). So an instance that turned Cerberus on this way is **today** ON for the proxy and the status (key beats row) and OFF for the management middleware (row hard off). Any change that makes the readers follow the row would silently turn off enforcement on such instances at upgrade; that is certain, not a possibility.
1. **Break-glass.** `Disable` / `persistDisabled` (`security_handler.go:585-620`) writes only the row (`Enabled = false`); it does not touch the keys. The emergency reset (`emergency_handler.go:262-269`) deliberately leaves the Cerberus keys alone. The "row false is a hard off" rule in `IsEnabled` is therefore the fail-safe that makes `/security/disable` work on the management API, and the comment at `cerberus.go:115-117` says so. The gap is on the write side: because `Disable` leaves the keys alone, a stale `feature.cerberus.enabled=true` keeps the proxy and the status ON after a break-glass disable. Making the middleware follow the proxy rule instead would leave the middleware on too.
2. **The presets gate is a different question.** A fresh install seeds the default row with `Enabled: false` (`models/seed.go:14-18`) and no key, so `isCerberusEnabled` answers on (default true) while the proxy rule answers off. Presets and the console work before the suite is switched on; folding this reader into the proxy rule would make those endpoints refuse on a fresh install.

### 2.4 #1773

`tests/utils/security-helpers.ts:160-215` `captureSecurityState` records effective booleans from `/security/status`; `restoreSecurityState` (lines 179-215) writes a concrete `true`/`false` key through `setSecurityModuleEnabled` when the effective value differs, so a key that was absent before can remain after. `tests/utils/security-settings-api.ts` (#1767) already has the right helpers: `snapshotSecurityState` (261), `restoreSecurityState` (274, row first, switches via `restoreModuleSwitches`), `captureModuleSwitches` (137), `restoreModuleSwitches` (157), `expectModuleSwitchesRestored` (174). Importers of the old pair: `waf-`, `acl-`, `combined-`, `rate-limit-` and `crowdsec-enforcement.spec.ts` (each calls `captureSecurityState` in `beforeAll` and `restoreSecurityState` in `afterAll`), `zzzz-break-glass-recovery.spec.ts` and `tests/crowdsec-whitelist.spec.ts` import other names from the same file. `security-settings-api.ts` does not import `security-helpers.ts`, so there is no import cycle.

### 2.5 #1774 docs (each statement verified against code)

| Statement | Finding |
| --- | --- |
| `docs/cerberus.md:874` roadmap row 1 "WAF placeholder + metrics" | Stale: the WAF is the Coraza engine with rule sets (the page says so at line 161). Reword. |
| `docs/cerberus.md:878` row 5 "CrowdSec local agent: Planned" | Stale: the local engine ships and is GUI-controlled (lines 144, 273). Mark complete. |
| `docs/cerberus.md:195-203` Prometheus section | Only `IncWAFRequest` has a caller (`cerberus/cerberus.go:186`, inside the management API middleware), so `charon_waf_requests_total` counts requests to Charon's own API, not proxied traffic. `IncWAFBlocked` and `IncWAFMonitored` (`metrics/metrics.go:45-49`) have **no callers**, so the blocked and monitored series stay at 0. The section and its "Total requests evaluated" text are wrong. |
| `docs/api.md:1460` "when rate limiting is implemented" | Stale: rate limiting is implemented. Verify what the live-log websocket route is actually subject to (the management API limiter, `internal/ratelimit/http.go`) and reword. |
| Burst row in the rate-limiting table | `docs/features/rate-limiting.md:18` and `:45` already say the burst is for the management interface. `docs/cerberus.md:231` says "a burst of 20" without that qualifier. Verify and add the qualifier if missing; no change if the table is already correct. |
| Rule set files with their own `SecRuleEngine` line | Confirmed in `caddy/manager.go:420-436`: the engine line (and `SecRequestBodyAccess`) is prepended only when the content has no `secruleengine` (case-insensitive), so a rule set carrying its own line keeps it whatever its mode or the saved WAF mode. `docs/cerberus.md:163,193` describe the mode as always governing. |
| 429 response, and where detection hits are logged | Unverified in code (the rate limiter is a Caddy plugin; the log classification is `services/log_watcher.go:254`). Verified by the container gate below, not by reading. |

### 2.6 #1741

`proxyHosts.source.local`, `.localUnavailable`, `.localUnavailableHint` exist in `en` and are used by `frontend/src/components/ProxyHostForm.tsx:777,778,792`. They are missing in `de`, `es`, `fr`, `zh` (a script comparing the full `proxyHosts` subtree against `en` finds exactly these three keys in each, nothing else, and no extra keys). The parity test `frontend/src/__tests__/i18n.test.ts:68-98` has a root list (`it.each([...])`) that does not include `proxyHosts`; adding that one root closes the gap for the whole subtree.

### 2.7 #1701

`TestRealCommandExecutor_Execute_OutputLimit` (`backend/internal/api/handlers/crowdsec_cscli_contract_test.go:754`): the wall-clock bound was already widened from 4 s to 15 s in `1519c8c37`, so the flake the issue describes is mitigated but still a timing assertion. More importantly `largeOutputScript` (line 738) ends in `; fx sleep 5`. `fx` is not a shell command (`which fx` finds nothing), so the script exits with "not found" right after the output instead of idling, and the test cannot show that the executor's own limit (rather than the end of the command) stopped it, which is the stated intent of the helper comment. It looks like a typo for `exec sleep`.

### 2.8 Drift against the batch 5 spec and the issues

* #1761: `waf_mode` already validated (2.1). #1756: "a raw query where an empty value means absent" is the status handler (`security_handler.go:131`), and `cerberus_decision.go` exists as the issue says. #1741 and #1773 match. #1701's bound is already 15 s. Batch 5 C4/U1 rows describe the same four readers; its note (3n, M6) that the four reads are "not unified" is exactly this work.

## 3. Priorities & Ordering

No finding here is critical or high, and none is user-submitted, so the CLAUDE.md rules give: bug fixes before any `feat` (there is no `feat` in this work), medium/low items scheduled after, highest value first.

| Order | Item | Why this position |
| --- | --- | --- |
| 1 | **#1761** value checks | The only real validation gap: an admin typo or a script is stored and silently does nothing (or stores 999999999). Smallest blast radius, clearest fix. |
| 2 | **#1770** narrow the restore | A silent lost update: a concurrent admin write is overwritten while that admin saw 200. Rare (needs a refused apply plus a concurrent write) but data loss without any signal. |
| 3 | **#1756** the suite switch | Pins, a shared value parser and the write-side Disable fix; no reader rule changes. The only behavior change is for an admin pressing Disable. |
| 4 | **#1741** translations | User-visible (English fallback for four maintained locales) but cosmetic. |
| 5 | **#1773** helper restore | Test hygiene; can leave a concrete key behind and make later specs order-dependent. |
| 6 | **#1701** timing assertion and `fx` typo | Flake and a vacuous check. |
| 7 | **#1774** docs | Last, because it describes the shipped code (including the PR 19 ranges). |
| After | **#1840** WAF unprotected count | Low, WAF, a separate path; next small `fix:` after PR 20. |
| Parked | #1743 (lint sweep with #1716, #1820); #1776 (does not reproduce) | Excluded as instructed. |

PR 19 carries 1-3, PR 20 carries 4-7. #1756's commits (4 to 6) are separable from the rest of PR 19; nothing else waits on them.

**Findings made while planning (not rolled into these PRs):**

* Unused metrics `IncWAFBlocked`/`IncWAFMonitored` and the two series they feed (2.5): low, file a GitHub issue after PR 20 (dead code vs documenting; Q3). PR 20 documents the truth without touching the metric surface.
* `/security/disable` leaves the Cerberus keys untouched, so a stale key `true` keeps the proxy and the status on after a break-glass disable. This is an admin-only completeness gap with no external leverage (Supervisor judgment), fixed in PR 19 commit 6; no advisory.

## 4. Technical specification

### 4.1 PR 19, #1761: value checks

All in `backend/internal/services/security_config_update.go` plus a small helper move in `security_config_validation.go`.

* Exported limits (so the tests and the docs quote one source): `MaxWAFParanoiaLevel = 4`, `MaxRateLimitRequests = 100000`, `MaxRateLimitBurst = 10000`, `MaxRateLimitWindowSec = 86400`. Minimum for the three rate-limit numbers is 0 (0 = not set, unchanged meaning); `waf_paranoia_level` is 1..4 (the model comment `models/security_config.go:32` and `caddy/config.go:1387` already say so). Ceilings are deliberately above the UI's (1000/100/3600) so an API or script user is not boxed in.
* `rate_limit_bypass_list`: every comma-separated entry, trimmed, blank entries skipped, must be an IP or a CIDR. Extract the loop in `ValidateAdminWhitelist` into `validateAddressList(list string, errOnBad error) error` (DRY, second use) and add `ErrInvalidRateLimitBypass` ("rate_limit_bypass_list must be a comma-separated list of IP addresses or CIDR ranges"). The empty string stays valid and clears the list.
* `crowdsec_api_url`: empty, or an absolute http/https URL with a host (reuse `isHTTPURL`, `security_config_update.go:98`). `ErrInvalidCrowdSecAPIURL`. Checked whether the value feeds outbound requests: it does not. The row column is only stored (`models/security_config.go:28`, copied in `Upsert`); the static `config.CrowdSecAPIURL` is documented "ignored: CrowdSec readers always use the local engine address" (`config/config.go:82`) and every CrowdSec client uses `crowdsec.LocalLAPIURL`. So the SSRF validator (`security/url_validator.go`) is not needed here, and `isHTTPURL` is the right size check (stored-value hygiene). A test pins that no code path reads the column for a request (grep result recorded in the commit message body, not a runtime test).
* `rate_limit_mode` (decided, Q4): exactly the lowercase values `""`, `"disabled"`, `"enabled"`. The proxy compares with `==` (`caddy/manager.go:733`) while the resolver uses `EqualFold` (`models/security_modules.go:105`), so `Enabled` would be on in one and off in the other; accepting only lowercase removes that. The check runs on the incoming `RateLimitMode` **whenever it is present, whatever `rate_limit_enable` carries**: `applyTo` (lines 197-206) passes any non-empty mode through `ResolveRateLimitMode` when `rate_limit_enable` is false, so `{rate_limit_enable: false, rate_limit_mode: "garbage"}` would otherwise store garbage. Test that exact payload.
* The new numeric/URL/list checks run in a `checkValues(stored models.SecurityConfig)` method called next to `u.checkRulesSource(tx, before)` (line ~283), inside the write transaction and before any column is written, so a refused save stores nothing. For each field: skip the check when the incoming value equals the stored value (the echo rule), so a row that already holds an old out-of-range value never makes an unrelated or echoed save fail. **New row rule:** when the row does not exist (`!exists`, `before = newSecurityConfigRow(name)`, paranoia 1, everything else zero), the stored values are those defaults, so a first save of `waf_paranoia_level: 0` is refused (0 is not 1, and not equal to the default 1), while `rate_limit_*: 0` equal the zero defaults and pass. Test both on a missing row.
* **Second writer check (done):** `settings_handler.go` (~372 and ~577) writes only `admin_whitelist`, through `syncAdminWhitelist` -> `SecurityService.Upsert` on a row it just read, so it cannot introduce a rate-limit, paranoia, bypass-list or URL value. `ApplyUpdate` is the only writer of those columns that takes new values.
* Errors are wrapped in `*InvalidConfigError`, so `respondConfigUpdateError` already answers 400 `{"error": "<message>"}`; no new response codes, no frontend change, no locale keys (this is why PR 19 has no frontend or translation work and the frontend coverage margin is untouched).

### 4.2 PR 19, #1770: restore only what the writer touched

Models change (`backend/internal/models/security_module_state.go`):

* Add `func (s *ModuleStateSnapshot) Seal(tx *gorm.DB) error`: called by each writer inside its transaction after its writes, it reads the current value of every imaged key and the row, stores that as the after-image, and records which keys/columns differ from the before-image (the touched set).
* `RestoreSettings` and `RestoreRow` become compare-and-restore: for each touched key, restore the before-image only when the key's current value still equals the after-image (a key that did not exist before is deleted only if it still holds the after-image); for the row, update only the touched columns whose current value still equals the after-image. A skipped key/column logs one Warn naming the key (log-injection safe, use `logValue`).
  * **Compare unit:** for a settings key, `Value` and `Category` only (not `ID`, `UpdatedAt` or `Type`, which a rewrite of the same value would change); for a row column, the column value.
  * **Accepted limits, stated in the code comment and the PR body:** (1) a concurrent write that stores *the same value* as the after-image is indistinguishable from the writer's own and is overwritten by the restore; (2) the two restore transactions (settings, then row) are separate, so a concurrent write can win on the settings and lose on the row or the reverse (a mixed outcome); (3) a concurrent admin write's own proxy apply may already have applied the failed writer's values to the proxy, and the restore does not re-apply (the next apply corrects it). A test covers (2) explicitly: a concurrent write to a touched key lands between the two restore transactions and the final state is the documented mixed result.
  * **No writer touches `crowd_sec_mode`:** `ApplyUpdate` validates `crowdsec_mode` but never stores it (`applyTo` has no such column, `security_config_update.go:14-17`), the module toggles and the settings writers do not write it, and only the CrowdSec Start path changes it. So no touched set in commits 2 contains it; commit 3 adds it to the image only for the Start revert.
  * **Query-count pin:** I found no query-count pin for the writers. The nearest pin on a path `Seal` could affect is the apply-path default-row read count in `backend/internal/caddy/manager_cerberus_decision_test.go`; `Seal` runs in the writers' transactions, not in the apply, so it must leave that file passing unchanged. Commit 2 adds its own test that `Seal` issues one read per imaged key plus one for the row. Both still run under `services.WithWriteLock` as today, so check and write are atomic.
* Fold the two restore transactions in `restoreModuleState` only if the existing tests allow; the comment at `security_module_writers.go:51-53` says they are separate so one failure does not undo the other, keep that.
* Call `Seal` at the four writer sites (`security_config_update.go` end of `ApplyUpdate`'s transaction, `security_handler.go:1195` module toggle, `settings_handler.go:423` and `:535` sites).

Unifying the two helper pairs: replace `snapshotCrowdsecStart`/`revertCrowdsecStart` internals with `CaptureModuleState` (extra key `models.CrowdSecEnabledSettingKey`) and the same compare-and-restore, keep Start's special rule (a row that Start created is kept with seed values, not deleted) as an explicit option on the restore, and image `crowd_sec_mode` (add it to `securityConfigColumns`; `ApplyUpdate` never changes it, so it is never in a config-save touched set). Delete `snapshotSettings`, `restoreSettings`, `snapshotDefaultSecurityConfigState`, `restoreDefaultSecurityConfigState`, `defaultSecurityConfigSnapshot`, `settingSnapshot` when unused (CLAUDE.md: delete dead code). This is the riskiest sub-step (Start is an engine lifecycle path); it is its own commit so it can be dropped without losing the rest (Q5).

### 4.3 PR 19, #1756: shared parser, pin matrix, and the write-side fix (Q1)

**Decision (Q1, revised after review): reader behavior does not change.** The four readers keep their orders. Three things ship instead:

1. **Pin matrix (no behavior change).** A table-driven test that sets up each stored state and records the answer of all four readers in one place: static on/off, no row, row on/off; feature key absent/`true`/`false`/`TRUE`/`1`/`yes`/empty/garbage; legacy key likewise; both keys together. The matrix also includes the **key-only paths**: the state after `EnableCerberus` (`POST /security/cerberus/enable`, both keys true, row untouched, so a fresh install has row `Enabled=false`) and after the Cerberus feature-flag toggle that `SystemSettings.tsx:309` drives, and the same for `DisableCerberus`. Whatever it shows is committed as current behavior, including the disagreements, and the matrix documents that key-only instances are proxy ON, status ON, middleware OFF today.
2. **Shared value parser.** One function in `models` (`parseModuleSetting` is the existing recognizer; export a Cerberus-key variant that trims and matches `true` case-insensitively) used by the proxy, status and `IsEnabled` readers for the key value. Orders, defaults and the presets gate's `1`/`yes`/env/default-on behavior stay exactly as pinned; if the shared parser would change an answer in the matrix, the parser adapts, not the reader.
3. **Write-side completeness fix.** `Disable` / `persistDisabled` (`security_handler.go:585-640`) also writes `feature.cerberus.enabled` and `security.cerberus.enabled` to `false` (reuse `writeModuleToggle(tx, models.CerberusFeatureSettingKey, false, actor)`, the same writes `POST /security/cerberus/disable` already makes) in the same write-locked transaction as the row save, with the `CaptureModuleState`/`Seal` snapshot and `restoreModuleState` if the proxy apply fails (so it benefits from #1770). Test: stale key `true` plus Disable leaves both keys `false`, proxy decision off, status off, middleware off; a rejected apply restores keys and row. Also the localhost-without-token path and the token path.

**Effect, stated plainly.** The only behavior change is for an admin pressing Disable: it now really disables the suite instead of leaving the proxy and the status on when a key says on. No reader rule changes, so **no existing instance's enforcement turns off on upgrade**, including key-only-enabled instances. An instance that already has a stale key `true` after an earlier Disable stays as it is until its admin presses Disable again.

**Rejected options, kept for the record.**

* (A) Row `Enabled=false` is a hard off in every reader: **rejected**, because it is a certain silent downgrade for instances enabled through the key-only paths (proxy and status go from on to off at upgrade, with no admin action). The fail-safe it was meant to give is delivered by the write-side fix instead.
* (B) The proxy rule everywhere (key beats row): **rejected**, because it can leave the management middleware on after a break-glass disable when a stale key says on, which defeats the lockout protection `cerberus.go:115-117` documents, and it turns the middleware ON for key-only instances at upgrade.
* The presets/console gate (`isCerberusEnabled`) stays as is (Q2): keep the name, fix its comment, pin it, share only the parser.
* Optional, per the issue: the settings API accepts only `true`/`false` (any case) for the two Cerberus keys. Check `settings_handler.go` first; add it only if it does not already, and only as a refusal of new garbage (existing stored values keep their pinned meaning).

### 4.4 PR 20

* **#1773**: in `tests/utils/security-helpers.ts` make `captureSecurityState` return the `SecurityStateSnapshot` from `security-settings-api.ts` (`snapshotSecurityState`) and delete the local `restoreSecurityState`, re-exporting the shared one so the five specs keep their imports; `withSecurityEnabled` restores through it. Remove `CapturedSecurityState` if unused. Keep `setSecurityModuleEnabled` (still used to turn modules on). Add a short spec (or an assertion in `waf-enforcement.spec.ts` `afterAll`) that starts with a module key absent, runs enable then cleanup, and calls `expectModuleSwitchesRestored` to prove the key is gone again.
* **#1741**: add the three keys to `frontend/src/locales/{de,es,fr,zh}/translation.json` under `proxyHosts.source` with real translations (the English hint contains an arrow, "Hecate -> Agent", keep the product names untranslated and the arrow); add `'proxyHosts'` to the `it.each` root list in `frontend/src/__tests__/i18n.test.ts:80`.
* **#1701**: in `crowdsec_cscli_contract_test.go` fix `fx sleep 5` to `exec sleep 60` (exec so the kill reaches the sleeper) and keep the 30 s test context. Drop the `time.Since(start)` assertion and the unused `start`. Add `require.NoError(t, ctx.Err())` right after `Execute`: if the executor's own limit did not kill the command, the sleeper outlives the 30 s context and `ctx.Err()` is `DeadlineExceeded`, so the test fails deterministically with no wall-clock bound. Note that the error text cannot tell the two cases apart: `CombinedOutputLimited` returns `ErrOutputLimit` whenever the buffer overflowed (`crowdsec/cscli_exec.go:87-102`), even if the context also expired, which is why the `ctx.Err()` check is the assertion that matters. Keep the error-message and `len(out) < 40_000_000` assertions.
* **#1774**: edit `docs/cerberus.md` (roadmap rows 1 and 5, Prometheus section, burst qualifier at 231 if missing, the `SecRuleEngine` note at 163/193), `docs/api.md:1460`, and `docs/api.md` near `POST /security/config` (line 131) to state the PR 19 ranges. The 429 and log statements are written from what the container gate observes. Edits are in existing files, so `docs-site` needs no manifest or nav change; never edit `docs-site/docs/`.

## 5. Implementation plan and Commit Slicing Strategy

Each PR is a merge commit (not squash). Commit subjects are plain; no breaking markers, no ids of advisories or findings. Dev agents run scoped tests only and in the foreground; "whole package" means the full `go test` of that package, not the repo. Cross-cutting changes (a function signature in `models`, a handler used in several files) run every package that imports the changed one (the PR 17 lesson).

### PR 19 commits

| # | Subject | Scope and files | Depends on | Scoped gate (foreground) |
| --- | --- | --- | --- | --- |
| 1 | `fix: reject out-of-range security settings values` | `services/security_config_update.go`, `security_config_validation.go`, new errors; tests in `services/security_config_update_test.go`, `handlers/security_config_partial_update_test.go`; update `handlers/security_handler_audit_test.go:454-473` and any other pin found by grep. Echo-rule cases, boundary cases (0, max, max+1, -1), bypass list (plain IP, CIDR, blank entry, bad entry), URL (empty, http, https, ftp, no host), nothing stored on refusal. | none | `go test ./internal/services/... ./internal/api/handlers/...` (whole packages: `ApplyUpdate` is used by several handler test files) |
| 2 | `fix: restore only the settings a refused change touched` | `models/security_module_state.go` (`Seal`, compare-and-restore), four writer sites, `handlers/security_module_writers.go`; tests: touched-only restore, concurrent write to an untouched key and to a touched key kept, absent key restored by deletion, row columns, Warn logged. | 1 (same files in `ApplyUpdate`) | `go test ./internal/models/... ./internal/services/... ./internal/api/handlers/...` and `./scripts/scan-gorm-security.sh --check` (models changed) |
| 3 | `refactor: use one snapshot for the CrowdSec start revert` | `handlers/crowdsec_handler.go`, `crowdsec_lifecycle.go`, `security_handler.go` (delete the old pair), `models` (`crowd_sec_mode` in the image, keep-created-row option). Drop if Q5 says so. | 2 | whole packages `./internal/api/handlers/... ./internal/models/... ./internal/services/... ./internal/crowdsec/...` (the Start revert is exercised from the lifecycle and the startup reconcile in `services`); the existing Start-failure and revert tests must pass unchanged |
| 4 | `test: pin how each reader decides the security suite is on` | the matrix test (4.3 item 1) in `internal/api/handlers` (or a small helper package if the import graph needs it), including the `EnableCerberus`/`DisableCerberus` key-only paths and the feature-flag toggle; no production change | none (can precede 1) | the new test; no behavior change, so the other packages need not rerun |
| 5 | `refactor: read the security suite switch value one way` | `models/security_modules.go` (exported Cerberus-key value parser), `caddy/cerberus_decision.go`, `handlers/security_handler.go` status, `cerberus/cerberus.go`, comment on `crowdsec_handler.go:669`. Orders and defaults unchanged; the commit 4 matrix passes unmodified. | 4 | `go test ./internal/... ./cmd/...` (`Cerberus.IsEnabled` and `ProxyCerberusCheck` are used from `cmd/api/main.go:249`, `services/crowdsec_startup.go` and the handlers, so every importer runs) |
| 6 | `fix: make the break-glass disable switch the security suite off everywhere` | `handlers/security_handler.go` (`Disable`, `persistDisabled`: also write the two Cerberus keys false in the same write-locked transaction, snapshot and restore on a failed apply); tests per 4.3 item 3; the matrix gains the post-Disable rows. | 2, 4, 5 | `go test ./internal/... ./cmd/...` (same reason as commit 5) |
| 7 | `docs: describe the allowed security settings values` | `docs/api.md` ranges; `docs/cerberus.md` one sentence that Disable now also switches the Cerberus keys off | 1, 6 | `npm run lint:md` on the touched files |

Suggested order of execution: 1, 2, 3, 4, 5, 6, 7. Commit 4 is independent and may go first if the matrix should be reviewed before any change. Commit 3 is droppable (Q5); if it is dropped, nothing after it changes.

**PR 19 definition-of-done and container gate (run once by `qa-security`, not by the dev agents):** full backend suite and `scripts/go-test-coverage.sh` (85%), `scripts/local-patch-report.sh`, GORM scan (models changed), lefthook, `go build ./...`. The change adds validation but no new endpoint or UI, so the local CodeQL/Trivy scans are deferred to CI (`fix:` scope, no new feature surface). Container gate on a throwaway container built from the branch image (unique container and network names, never the live `charon`, own data volume): (a) `POST /security/config` with `waf_paranoia_level` 0, 5, 99, `rate_limit_requests` -1 and 100001, `rate_limit_burst` -1, `rate_limit_window_sec` 86401, `rate_limit_bypass_list` `not-an-ip`, `crowdsec_api_url` `ftp://x` -> each 400, row unchanged (read with `docker exec ... sqlite3 -readonly`); (b) boundary and ordinary values 200; the Rate Limiting page save at 1000/100/3600 still works in the UI; (c) a save that echoes the row read from `GET /security/config` returns 200; (d) failed-apply restore, with the proxy rejection **forced explicitly**: create a proxy host whose `advanced_config` carries a handler name Caddy's config load refuses (for example `[{"handler":"charon_no_such_handler"}]`), after setting it through the API if the API accepts it; if the API validates that field, fall back to making the Caddy admin endpoint unreachable by stopping the Caddy process inside the throwaway container just before the save (restart it afterwards). Use whichever the PR 17 container gate used (see `docs/reports/qa_report.md` history) and record the method in the gate output. Then change one module through `POST /security/config` and confirm the response is the apply failure (500) and that only that module's key (and only its touched row columns) returned to the prior state, while a second write made through a parallel request before the restore is kept; (e) if commit 3 is in: CrowdSec Start with the engine binary unable to start reverts the row and keys as before (compare against the base image run); (f) upgrade scenarios, run on the base image and on the branch image with the same data volume and compare: (i) fresh install; (ii) suite enabled through the key-only paths (`POST /security/cerberus/enable`, and the Cerberus feature toggle on the System Settings page), which leaves the row `Enabled=false`: the status and a proxied request (an ACL or WAF block) must be identical on both images (proxy ON, status ON, management middleware OFF today), proving no enforcement turns off on upgrade; (iii) suite enabled through the Security page (row and keys true); (iv) stale key `true` with row `Enabled=false`, then `/security/disable` from localhost: on the base image the proxy keeps enforcing, on the branch image the proxy stops enforcing, the status shows off and the keys read `false`; (v) the same Disable with a rejected apply (method from (d)): keys and row return to the prior state; (vi) emergency reset: unchanged on both images.

### PR 20 commits

| # | Subject | Scope and files | Depends on | Scoped gate (foreground) |
| --- | --- | --- | --- | --- |
| 1 | `test: stop a timing check and a typo from weakening the output limit test` | `crowdsec_cscli_contract_test.go` (4.4) | none | `go test ./internal/api/handlers -run 'TestRealCommandExecutor_Execute_OutputLimit' -count=5`, then the whole package once |
| 2 | `fix: add the missing local-host labels to the German, Spanish, French and Chinese translations` | four `translation.json` files, `i18n.test.ts` (add `proxyHosts` root) | none | `cd frontend && npx vitest run src/__tests__/i18n.test.ts`; the test fails before the JSON change, passes after; `npm run type-check` |
| 3 | `test: restore security module keys exactly after enforcement specs` | `tests/utils/security-helpers.ts`, the five enforcement specs' imports only if needed, the absent-key assertion | none | targeted, firefox only: `npx playwright test tests/security-enforcement/waf-enforcement.spec.ts --project=firefox`, then `rate-limit-enforcement.spec.ts`, then `acl-enforcement.spec.ts` (one spec per command; against a rebuilt e2e container via the `docker-rebuild-e2e` skill); no full-suite or multi-browser run locally |
| 4 | `docs: correct the rate limiting, metrics and WAF notes` | the doc edits in 4.4, each verified as below | PR 19 merged (for the ranges) | `npm run lint:md`; `bash docs-site/scripts/sync-docs.sh && cd docs-site && zensical build --strict` when Zensical is installed locally, otherwise left to CI |

**PR 20 verification for the docs commit (container, throwaway, from the PR 19 image):** (a) set rate limit 2 requests per 60 s on a proxied host, send 3: record the status of the third (expected 429) and its headers, and find where it is logged (Caddy access log, and whether `log_watcher` classifies it as `ratelimit`); (b) a rule set in Blocking mode whose content starts with its own `SecRuleEngine DetectionOnly`: send the attack request and record whether it is blocked, to confirm "keeps its own line"; (c) scrape `/metrics` after a WAF block on a proxied host and confirm `charon_waf_requests_total` does not move (only API requests count) and the blocked/monitored series stay 0. Write the docs from those observations; if an observation contradicts section 2.5, the observation wins.

**PR 20 definition of done (qa-security):** frontend coverage script run once; expected unchanged because only JSON and a test file change, but the branch margin is thin (85.1% against a floor of 85%), so the QA run must report the actual number and the PR must not merge below 85. Full `vitest` run (not just the i18n file) is part of that single QA run. Backend: the handlers package once. Markdownlint clean on every touched `.md`.

### Rollback and contingency

* Each PR is a merge commit and each commit builds on its own, so a revert of the PR is clean. PR 19 commits 1, 2 and 6 are independent in behavior: reverting 6 restores the old Disable, reverting 3 restores the old CrowdSec revert helpers, reverting 1 relaxes validation.
* If commit 3 shows any difference in the Start-failure behavior that the existing tests do not pin, drop it from the PR and file it as an issue instead of widening the PR.
* Commits 5 and 6 are independent of 1-3: reverting 6 restores the old Disable, reverting 5 restores the old per-reader parsing. If Q1's recommendation is not accepted, ship 1-4 and 7 without 5 and 6.

## 6. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Validation rejects a value a real instance already stores (for example paranoia 0 from an old seed) and bricks saves | Echo rule: a value equal to the stored one is accepted; only changed values are checked. Test with a stored out-of-range row. Ceilings are above the UI's. |
| Rate-limit zeros are valid (not set) | Minimum is 0 for the three numbers; a test pins that 0 is accepted and that `thresholds_unset` behavior is untouched. |
| `Seal` adds queries to every module write | Two small reads per writer inside the existing transaction; no new transaction. A test pins the query count if the existing query-count pin tests exist (batch 5 "r7" mentions one for the apply path). |
| Compare-and-restore skips a restore the user expected (concurrent write won) | Intended and logged at Warn; the response is still the apply failure, and the stored state is the newer admin write, which the issue asks for. |
| Replacing the CrowdSec Start revert helpers changes engine lifecycle behavior | Own commit, existing revert tests unchanged, container gate (e), droppable. |
| Changing a reader's rule would silently turn off enforcement on key-only-enabled instances (certain, not a possibility) | No reader rule changes (Q1). The matrix pins the key-only states; container gate (f)(ii) compares base and branch on the same data volume. |
| Disable now writes the keys, so a later admin sees the suite off in more places | Intended: it is what Disable means. Only an admin pressing Disable is affected; no upgrade-time change. Documented in `docs/cerberus.md`. |
| Folding the presets gate into the proxy rule would refuse presets on a fresh install | Not folded (Q2). |
| `docs/api.md` is edited by PR 19 commit 6 and PR 20 commit 4 | Different sections; merge PR 19 first and rebase PR 20. |
| Frontend branch coverage margin is thin (85.1% against 85%) | PR 19 touches no frontend. PR 20 touches no instrumented source. QA reports the real number; no frontend source change is planned, and any that creeps in must come with tests. |
| New locale text is machine-quality | Short labels; ask a native-speaker review in the PR description is not required, but the strings reuse terms already in each locale file (for example the translation of "Docker", "Agent"). |
| The Playwright helper change breaks the other importers | Keep the exported names; run the three specs listed, and let CI run the full suite. |

## 7. Open questions (with recommendations)

* **Q1 (decided in review, confirm). #1756 without changing any reader.** The row `Enabled=false` hard off exists only in the management middleware, and instances enabled through key-only paths (`POST /security/cerberus/enable`, the System Settings toggle) have the row off today, so making readers follow the row (A) or the key (B) is a certain silent change in enforcement at upgrade. **Recommendation: change no reader.** Ship the pin matrix, the shared value parser, and make `/security/disable` also write the two Cerberus keys false so the break-glass reaches the proxy and the status. Only an admin pressing Disable sees a change (it now really disables); no existing instance's enforcement turns off on upgrade. A and B are documented as rejected in 4.3.
* **Q2. Is the presets/console gate part of the unification?** **Recommendation: no.** It answers a different question (the suite is not explicitly switched off) and defaults to on, so presets and console work on a fresh install before the suite is on. Keep it, fix its comment, pin it, share only the parser. Tell me if you want it folded; the cost is that presets refuse until the suite is on.
* **Q3. The never-incremented `IncWAFBlocked` / `IncWAFMonitored` series.** **Recommendation:** document that they stay at 0 in PR 20 and file a low issue to remove or really wire them; do not change the metric surface in these PRs.
* **Q4 (decided). `rate_limit_mode`:** exact lowercase empty/`disabled`/`enabled`, validated whenever the field is present (including with `rate_limit_enable: false`). Listed so the decision is visible; no action needed from you.
* **Q5. Fold the CrowdSec Start revert onto the shared snapshot (PR 19 commit 3)?** The issue asks for it and it deletes ~80 lines of duplicate code, but it touches engine lifecycle. **Recommendation: yes, as its own droppable commit** with the container check; drop it if anything differs.
* **Q6. Backend ceilings (100000 requests, 10000 burst, 86400 s window).** **Recommendation:** keep them generous and above the UI; tell me if you prefer them equal to the UI's 1000/100/3600.
* **Q7. Two PRs or one?** **Recommendation: two** (section 1). One PR would put a security-surface change and eight low-risk test/doc/locale edits behind the same container gate and review.

## 8. Public Text

Neutral, plain subjects; no breaking markers; no advisory or finding ids; nothing that describes a weakness.

**PR 19 title:** `fix: check security settings values and keep a refused change from overwriting newer ones`
**PR 19 body:** Admin-saved security settings now have allowed ranges (WAF paranoia level, rate limit numbers, the bypass list, the CrowdSec API address). A value already stored is still accepted when it is saved back unchanged. When the proxy refuses a security change, only the settings that change touched are put back, and a newer change made in the meantime is kept. Pressing Disable now switches the security suite off for the proxy and the status as well as the management interface. Includes tests and the container checks listed in the plan.

Commit subjects (section 5): `fix: reject out-of-range security settings values`; `fix: restore only the settings a refused change touched`; `refactor: use one snapshot for the CrowdSec start revert`; `test: pin how each reader decides the security suite is on`; `refactor: read the security suite switch value one way`; `fix: make the break-glass disable switch the security suite off everywhere`; `docs: describe the allowed security settings values`.

**PR 20 title:** `fix: tidy security test helpers, translations and docs`
**PR 20 body:** Adds the missing local-host labels to the German, Spanish, French and Chinese translations and extends the translation parity test; makes the security end-to-end helpers restore settings exactly; removes a timing assertion and a typo from a command-output test; corrects the rate limiting, metrics and WAF notes in the docs.

Commit subjects (section 5): `test: stop a timing check and a typo from weakening the output limit test`; `fix: add the missing local-host labels to the German, Spanish, French and Chinese translations`; `test: restore security module keys exactly after enforcement specs`; `docs: correct the rate limiting, metrics and WAF notes`.

**`(security)` scope: not used, and no private advisory.** The Supervisor judged all of this admin-only with no external leverage: the validation (#1761) is input hygiene, the restore (#1770) is a consistency fix, and #1756's Disable fix completes an admin action rather than closing a hole an outside party can use. None is a vulnerability fix or a new protective mechanism, so by CLAUDE.md they stay plain `fix:`/`refactor:`. The subject of commit 6 is deliberately plain and says what Disable now does; keep the PR body to that and do not add language about lockouts or about the readers disagreeing.

## 9. Acceptance criteria

* PR 19: every out-of-range value in 4.1 returns 400 with nothing stored; boundary and echoed values return 200; a failed apply restores only touched keys/columns and keeps a concurrent write (tested); the CrowdSec Start failure revert behaves as before; the pin matrix passes unmodified after the parser refactor, key-only-enabled instances are identical on the base and branch images, and Disable with a stale key `true` stops the proxy enforcing; backend coverage at least 85%; GORM scan clean; container gate (a)-(f) recorded.
* PR 20: the i18n parity test covers `proxyHosts` and passes with the three new keys in all four locales; the three enforcement specs pass on firefox and leave module keys exactly as found; the output-limit test no longer measures wall-clock time, runs `exec sleep 60` and asserts `ctx.Err()` is nil; each changed doc statement has a code or container observation behind it; frontend coverage still at least 85% in the single QA run; `npm run lint:md` clean.
* Both: no commit contains a session id or link; Definition of Done from CLAUDE.md run once by `qa-security`.
