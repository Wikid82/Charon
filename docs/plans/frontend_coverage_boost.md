# Frontend Coverage Boost Plan (>=85%)

Current (QA): statements 84.54%, branches 75.85%, functions 78.97%.
Goal: reach >=85% with the smallest number of high-yield tests.

## Targeted Tests (minimal set with maximum lift)
- **API units (fast, high gap)**
  - [src/api/notifications.ts](frontend/src/api/notifications.ts): cover payload branches in `previewProvider` (with/without `data`) and `previewExternalTemplate` (id vs inline template vs both), plus happy-path CRUD wrappers to verify endpoint URLs.
  - [src/api/logs.ts](frontend/src/api/logs.ts): assert `getLogContent` query param building (search/host/status/level/sort), `downloadLog` sets `window.location.href`, and `connectLiveLogs` callbacks for `onOpen`, `onMessage` (valid JSON), parse error branch, `onError`, and `onClose` (closing when readyState OPEN/CONNECTING).
  - [src/api/users.ts](frontend/src/api/users.ts): cover invite, permissions update, validate/accept invite paths; assert returned shapes and URL composition (e.g., `/users/${id}/permissions`).

- **Component tests (few, branch-heavy)**
  - [src/pages/SMTPSettings.tsx](frontend/src/pages/SMTPSettings.tsx): component test with React Testing Library (RTL).
    - Ensure initial render waits for query then hydrates host/port/encryption (flaky area); verify loading spinner disappears.
    - Save success vs error toast branches; `Test Connection` success/error; `Send Test Email` success clears input and error path shows toast.
    - Button disables: test connection disabled when `host` or `fromAddress` empty; send test disabled when `testEmail` empty.
  - [src/components/LiveLogViewer.tsx](frontend/src/components/LiveLogViewer.tsx): component test with mocked `WebSocket` and `connectLiveLogs`.
    - Verify pause/resume toggles, log trimming to `maxLogs`, filter by text/level, parse-error branch (bad JSON), and disconnect cleanup invokes returned close fn.
  - [src/pages/UsersPage.tsx](frontend/src/pages/UsersPage.tsx): component test.
    - Invite modal success when `email_sent` false shows manual link copy branch; toggle permission mode text for allow_all vs deny_all; checkbox host toggle logic.
    - Permissions modal seeds state from selected user and saves via `updateUserPermissions` mutation.
    - Delete confirm branch (stub `confirm`), enabled Switch disabled for admins, enabled toggles for non-admin users.

- **Security & CrowdSec flows**
  - [src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx): component test (can mock queries/mutations).
    - Cover hub unavailable (503) -> `preset-hub-unavailable`, cached preview fallback via `getCrowdsecPresetCache`, validation error (400) -> `preset-validation-error`, and apply fallback when backend returns 501 to hit local apply path and `preset-apply-info` rendering.
    - Import flow with file set + disabled state; mode toggle (`crowdsec-mode-toggle`) updates via `updateSetting`; ensure decisions table renders "No banned IPs" vs list.
  - [src/pages/Security.tsx](frontend/src/pages/Security.tsx): component test.
    - Banner when `cerberus.enabled` is false; toggles `toggle-crowdsec`/`toggle-acl`/`toggle-waf`/`toggle-rate-limit` call mutations and optimistic cache rollback on error.
    - LiveLogViewer renders only when Cerberus enabled; whitelist input saves via `useUpdateSecurityConfig` and break-glass button triggers mutation.

- **Shell/UI overview**
  - [src/pages/Dashboard.tsx](frontend/src/pages/Dashboard.tsx): component test to cover health states (ok, error, undefined) and counts computed from hooks.
  - [src/components/Layout.tsx](frontend/src/components/Layout.tsx): component test.
    - Feature-flag filtering (hide Uptime/Cerberus when flags false), sidebar collapse persistence (localStorage), mobile toggle (`data-testid="mobile-menu-toggle"`), nested menu expand/collapse, logout button click, and version/git commit rendering.

- **Missing/low names from QA list**
  - `Summary.tsx`, `FeatureFlagProvider.tsx`, `useFeatureFlags.ts`, `LiveLogViewerRow.tsx`: confirm current paths (may have been renamed). Add light RTL/unit tests mirroring above patterns if still present (e.g., summary widget rendering counts, provider supplying default flags).

## SMTPSettings Deflake Strategy
- Wait for data: use `await screen.findByText('Email (SMTP) Settings')` and `await waitFor(() => expect(hostInput).toHaveValue('...'))` after mocking `getSMTPConfig` to resolve once.
- Avoid racing mutations: wrap `vi.useFakeTimers()` only if timers are used; otherwise keep real timers and `await act(async () => ...)` on mutations.
- Reset query cache per test (`queryClient.clear()` or `QueryClientProvider` fresh instance) and isolate toast spies.
- Prefer role/label queries (`getByLabelText('SMTP Host')`) over brittle text selectors; ensure `toast` mocks are flushed before assertions.

## Ordered Phases (minimal steps to >=85%)
- Phase 1 (API unit bursts) — expected +0.30 to statements: notifications.ts, logs.ts, users.ts.
- Phase 2 (UI quick wins) — expected +0.50: SMTPSettings, LiveLogViewer, UsersPage.
- Phase 3 (Security shell) — expected +0.40: CrowdSecConfig, Security page.
- Phase 4 (Shell polish) — expected +0.20: Dashboard, Layout, any remaining Summary/feature-flag provider files if present.

Total projected lift: ~+1.4% (buffered) with 8–10 focused tests. Stop after Phase 3 if coverage already surpasses 85%; Phase 4 only if buffer needed.
