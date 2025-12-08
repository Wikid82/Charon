# Cerberus Rebrand & CrowdSec UX Simplification Plan

## Intent
Rebrand the security surface from “Security” to “Cerberus,” streamline CrowdSec controls, and add export/preset affordances that keep novice users in flow while reducing duplicated toggles.

## Phase 0 — Recon & Guardrails
- Inventory the navigation and overview copy in [frontend/src/components/Layout.tsx](frontend/src/components/Layout.tsx) and [frontend/src/pages/Security.tsx](frontend/src/pages/Security.tsx) to rename labels to “Cerberus” (keep route paths unchanged unless routing requires). Update tests that assert “Security” strings (e.g., [frontend/src/components/__tests__/Layout.test.tsx](frontend/src/components/__tests__/Layout.test.tsx), [frontend/src/pages/__tests__/SystemSettings.test.tsx](frontend/src/pages/__tests__/SystemSettings.test.tsx)).
- Map CrowdSec UX touchpoints: overview card actions in [frontend/src/pages/Security.tsx](frontend/src/pages/Security.tsx) (toggle, start/stop, export), detail page flows in [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx), and import-only view in [frontend/src/pages/ImportCrowdSec.tsx](frontend/src/pages/ImportCrowdSec.tsx). Note supporting hooks/api in [frontend/src/hooks/useSecurity.ts](frontend/src/hooks/useSecurity.ts), [frontend/src/api/security.ts](frontend/src/api/security.ts), and [frontend/src/api/crowdsec.ts](frontend/src/api/crowdsec.ts).
- Confirm copy/feature-flag alignment with Cerberus flag (`feature.cerberus.enabled`) and ensure the header banner in [frontend/src/pages/Security.tsx](frontend/src/pages/Security.tsx) narrates the new name.

## Phase 1 — Cerberus Naming & Navigation
- Sidebar: Rename “Security” group to “Cerberus” and child “Overview” to “Dashboard” in [frontend/src/components/Layout.tsx](frontend/src/components/Layout.tsx); keep emoji or refresh iconography to match three-head guardian motif. Ensure mobile/desktop nav tests cover the renamed labels.
- Dashboard page: Retitle h1 and hero banner strings to “Cerberus Dashboard” (e.g., `Cerberus Dashboard`, `Cerberus Disabled`) in [frontend/src/pages/Security.tsx](frontend/src/pages/Security.tsx). Update toast and overlay copy to keep the lore tone (“Three heads turn…” etc.) but mention Cerberus explicitly where it helps recognition.
- Docs links: Verify the external link buttons still point to https://wikid82.github.io/charon/security; if a Cerberus-specific page exists, swap URLs accordingly.

## Phase 2 — CrowdSec Controls (Overview Card)
- Remove redundant start/stop buttons when a master toggle exists. Convert the CrowdSec card action cluster in [frontend/src/pages/Security.tsx](frontend/src/pages/Security.tsx) to a single toggle that: (a) calls `startCrowdsec()` when switching on, (b) calls `stopCrowdsec()` when switching off, (c) reflects `status.crowdsec.enabled` and `statusCrowdsec()` state. Keep Logs/Config/Export buttons.
- Eliminate the local “enabled” switch and start/stop duplication so the UI shows one clear state. Disable controls when Cerberus is off.
- Keep export action but surface a naming prompt (see Phase 4) so downloads are intentional.

## Phase 3 — CrowdSec Config Page Simplification
- Remove the “Mode” select block from [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx); replace with a binary toggle or pill (“Disabled” vs “Local”) bound to `updateSetting('security.crowdsec.mode', ...)` so users don’t see redundant enable/disable alongside the overview toggle.
- Drop the “Mode” heading; elevate status microcopy near the toggle (“CrowdSec runs locally; disable to pause decisions”).
- Keep ban/unban workflows and file editor intact; ensure decision queries are gated on mode !== disabled after refactor.

## Phase 4 — Import/Export Experience
- Rename “Import Configuration” section in [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx) to “Configuration Packages” with side-by-side Import and Export controls. (Recommended canonical section name: **Configuration Packages**.)
- Add export capability on the config page (currently only on overview) using `exportCrowdsecConfig()` with a filename prompt that proposes a default from a lightweight “Planning agent” helper (stub: derive `crowdsec-export-${new Date().toISOString()}.tar.gz`, allow override before download). Reuse the same helper in the overview card export to keep naming consistent.
- Update [frontend/src/pages/ImportCrowdSec.tsx](frontend/src/pages/ImportCrowdSec.tsx) to reuse the shared import/export helper UI if feasible, or clearly label it as a tasks-only entry point. Ensure backup creation messaging stays intact.

## Phase 5 — Presets for CrowdSec
- Introduce a presets catalog file (e.g., [frontend/src/data/crowdsecPresets.ts](frontend/src/data/crowdsecPresets.ts)) mirroring the style of [frontend/src/data/securityPresets.ts](frontend/src/data/securityPresets.ts), with curated baseline parsers/collections (e.g., “Honeypot Friendly Defaults”, “Bot Mitigation Essentials”, “Geolocation Aware”).
- Surface preset chooser in [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx) above file editor: selecting a preset should prefill a preview and require explicit “Apply” to write via `writeCrowdsecFile()` (with `createBackup()` first). Add small-print warnings for aggressive presets.
- Consider quick chips for “Local only” vs “Community decisions” if backend supports; otherwise, hide or disable with tooltip.

## Research: CrowdSec Hub / Presets (summary & recommended approach)
- Official Hub: CrowdSec maintains a canonical Hub repository for parsers, scenarios, collections, and blockers at https://github.com/crowdsecurity/hub and an online Hub UI (https://hub.crowdsec.net/ or https://app.crowdsec.net/hub/). This is the same repo used as `cscli` source-of-truth.
- Distribution Methods & UX:
	- `cscli` CLI: operators typically use `cscli hub pull` to fetch items from the Hub; Charon currently calls `cscli` for decisions/listing in the backend via `CommandExecutor`.
	- Index / API: the Hub publishes an index (e.g., `.index.json`) and the repo can be parsed to list available presets; the Hub UI uses this index.
	- Single-file or tarball packages: Hub items are directories that can be packaged and applied to a CrowdSec runtime.

- Integration options for Charon (recommended):
	1. Curated Presets (default): Ship a small set of curated presets with Charon (frontend `crowdsecPresets.ts`, backend `charon_presets.go`). This is the safest, offline-first, and support-friendly route.
	2. Live Hub Sync (optional advanced): Provide an admin-only backend endpoint to pull from the official Hub or via `cscli`. Cache and validate fetched presets; admin must explicitly Apply.
	3. Hybrid: Default to curated shipped presets with opt-in live sync that fetches additional or replacement presets from the Hub.

- Security & UX: validate remote presets before applying, create backups, and track origin/etag for auditability. If fetching from Hub, prefer a `pull` endpoint that runs in a server-side sanitized environment and returns a preview; require explicit `apply` to change the active config.

## Implementation Recommendations (High-level file/function list)
**Frontend files & functions (UI changes):**
- `frontend/src/components/Layout.tsx` — rename nav `Security` to `Cerberus` and `Overview` to `Dashboard`.
- `frontend/src/pages/Security.tsx` — change header to `Cerberus Dashboard` and update hero/help copy to reference Cerberus. Update label `Security Suite Disabled` to `Cerberus Disabled` and UI text `Enable Cerberus` where applicable.
- `frontend/src/pages/CrowdSecConfig.tsx` — replace the `Mode` select with a binary toggle (or pill) and rename `Import Configuration` to `Configuration Packages` for the import/export section. Add `PresetChooser` component to preview and apply presets.
- `frontend/src/pages/ImportCrowdSec.tsx` — update copy to `Configuration Packages` if the import page is still used.
- `frontend/src/data/crowdsecPresets.ts` — add curated default presets for Charon.
- `frontend/src/hooks/useCrowdsecPresets.ts` — new hook for presets queries/mutations (`useQuery` for `getCrowdsecPresets`, `useMutation` for pull/apply).
- `frontend/src/api/presets.ts` — typed API client functions: `getCrowdsecPresets`, `pullCrowdsecPreset`, `applyCrowdsecPreset`.

**Back-end files & functions (API & runtime behavior):**
- `backend/internal/api/handlers/crowdsec_handler.go` — extend or add routes: `GET /admin/crowdsec/presets`, `POST /admin/crowdsec/presets/pull`, `POST /admin/crowdsec/presets/apply`, `POST /admin/crowdsec/presets/import` to mirror existing import flow.
- `backend/internal/crowdsec/hub.go` — new backend helper to fetch `index.json` from Hub, download tarball, and validate (with `hublint`/`cshub` if available). Expose a `FetchPreset` function to return a `tar.gz` blob or parsed files for preview.
- `backend/internal/crowdsec/presets.go` — curated presets and caching/validation.
- `backend/internal/api/handlers/crowdsec_exec.go` — consider exposing a `ExecuteCscli()` helper wrapper for secure `cscli` usage if the backend runs on the same host where `cscli` is installed (current code provides an executor abstraction already; reuse it for hub operations).

## Unit test updates (explicit suggestions)
- `frontend/src/components/__tests__/Layout.test.tsx` — assert `Cerberus` and `Dashboard` display labels and preserve route paths. Add test for collapsed/expanded state showing the new Dashboard name.
- `frontend/src/pages/__tests__/Security.test.tsx` — update expectations for `Cerberus Dashboard` and hero copy `Cerberus Disabled` when `feature.cerberus.enabled` is false.
- `frontend/src/pages/__tests__/CrowdSecConfig.test.tsx` — remove assertions for `Mode` select (or replace with binary toggle tests) and add tests for `PresetChooser` interactions: preview, pull, and apply. Add tests for `Configuration Packages` header presence.
- `frontend/src/pages/__tests__/ImportCrowdSec.test.tsx` — update to assert `Configuration Packages` and to reuse import/export helper test cases.
- `backend/internal/api/handlers/crowdsec_handler_coverage_test.go` — add tests for the new `presets` endpoints: `GET /admin/crowdsec/presets` and `POST /admin/crowdsec/presets/pull` with mocked hub fetch and error conditions.

## Live Hub vs Curated Presets (decision note)
- Charon's default should be curated presets shipped with the app for stability and easier support.
- Provide optional Live Hub Sync as an opt-in admin feature. Live Hub Sync should:
	- Fetch a list from the Hub index and display a preview in UI
	- Validate content and run a `trailing` or `linter` before presenting an operator with a safe apply
	- Cache fetched presets server-side and only apply them when the admin clicks "Apply" (with automatic pre-apply backup)
	- Allow rollback via stored backups


## Phase 6 — Testing & Copy Polish
- Update unit/UI tests that assert strings or button counts in [frontend/src/components/__tests__/Layout.test.tsx](frontend/src/components/__tests__/Layout.test.tsx), [frontend/src/pages/__tests__/SystemSettings.test.tsx](frontend/src/pages/__tests__/SystemSettings.test.tsx), and any CrowdSec page tests to reflect renamed labels and removed start/stop buttons.
- Add targeted tests for the new export naming prompt and the toggle-driven start/stop behavior (mock `startCrowdsec`/`stopCrowdsec` in [frontend/src/api/crowdsec.ts](frontend/src/api/crowdsec.ts)).
- Light copy edit to keep “Cerberus” consistent and reassure users when controls are disabled by global flag.

## Phase 7 — Docs & Housekeeping
- Update [docs/features.md](docs/features.md) and [docs/security.md](docs/security.md) with Cerberus naming and simplified CrowdSec flows; include screenshots after UI update.
- No `.gitignore`, `.dockerignore`, `.codecov.yml`, or `Dockerfile` changes needed based on current scope (new files stay under tracked `frontend/src/data` and existing build ignores already cover docs/plan artifacts). Re-evaluate if new binary assets or build args appear.

## Success Criteria
- Sidebar, overview headings, and toasts consistently say “Cerberus.”
- CrowdSec overview shows one toggle-driven control with non-conflicting actions; config page has no standalone Mode select.
- Import/Export flows include a user-visible filename choice; presets available and gated by backup + apply flow.
- Tests and docs updated; builds and lint/tests green via existing tasks.
