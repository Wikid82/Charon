# Frontend Coverage Boost — CrowdSecConfig to 100%
**Date**: December 10, 2025
**Goal**: Drive frontend coverage past the target with zero dead branches, prioritizing a full sweep of [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx) while honoring the broader [frontend_coverage_boost](docs/plans/frontend_coverage_boost.md) roadmap.

---

## Mission and Targets
- Elevate overall frontend coverage (statements/branches/functions) by executing the existing coverage boost plan, with CrowdSec flows as the flagship effort.
- Achieve **100% statement/branch coverage** for CrowdSecConfig and lock in regression-proof harnesses for presets, imports/exports, mode toggles, file edits, and ban/unban flows.
- Keep test count lean by maximizing branch coverage per test; prefer RTL plus mocked API clients over heavy integration scaffolding.

## Surface Map for CrowdSecConfig (must-cover branches)
- **Data gating**: loading, `error`, missing `status`, missing `status.crowdsec`, disabled mode banner, and local mode rendering.
- **Mode toggle**: `handleModeToggle` success vs mutation error (toast path) with `data-testid="crowdsec-mode-toggle"` disabled state while pending.
- **Import/Export**: `handleExport` success/failure (toast), `handleImport` with/without file, backup mutation errors surfaced via `importMutation.onError`.
- **Preset lifecycle**: initial `useEffect` slug selection, `pullPresetMutation` success (preview/meta set), 400 validation (`preset-validation-error`), 503 hub offline (`preset-hub-unavailable` plus cached preview button), generic failure message, `getCrowdsecPresetCache` fallback path.
- **Apply paths**: backend apply success (sets `preset-apply-info`), 501 fallback to `applyPresetLocally` (sets status and local toast), 400 validation error, 503 hub unavailable, missing cache error (`Preset must be pulled...`), generic failure with backup in payload, disabled button logic (`presetActionDisabled`).
- **Local apply helper**: missing preset, missing target file, empty preview/content, success path that writes via `writeCrowdsecFile` and refreshes file list.
- **Preset preview UI**: meta display, warning, cached preview button, source/etag fields, render when catalog empty.
- **File editor**: `handleReadFile` sets `selectedPath` and loads content, `handleSaveFile` with backup and write success, close button resets state, textarea `onChange` updates state.
- **Banned IPs**: disabled mode message, loading, error, empty state, populated table rendering, `Ban IP` modal open/submit success/error, `Unban` confirmation flow success/error.
- **Status overlay messaging**: `getMessage()` branches for each pending mutation (pull/apply/import/write/mode/ban/unban) to assert correct `ConfigReloadOverlay` messaging.

## Phases (minimize request count)
### Phase 1 — Harness and fixtures
- Add a focused RTL harness for CrowdSec pages (e.g., [frontend/src/pages/__tests__/CrowdSecConfig.test.tsx](frontend/src/pages/__tests__/CrowdSecConfig.test.tsx)) with a reusable `renderWithQueryClient` helper to isolate cache per test.
- Mock API layers (`getSecurityStatus`, `listCrowdsecPresets`, `pullCrowdsecPreset`, `applyCrowdsecPreset`, `getCrowdsecPresetCache`, `listCrowdsecFiles`, `readCrowdsecFile`, `writeCrowdsecFile`, `listCrowdsecDecisions`, `banIP`, `unbanIP`, `exportCrowdsecConfig`, `importCrowdsecConfig`, `createBackup`, `updateSetting`) via vi.fn/MSW to drive branches deterministically.
- Create lightweight fixture data: security status (disabled/local), preset catalogs (hub available/unavailable, cached), decisions list (empty/populated), file lists, preset previews.

### Phase 2 — CrowdSecConfig 100% coverage
Execute targeted tests hitting every branch listed in the surface map:
- **Gatekeeper states**: render loading, error, no status, missing `crowdsec`, disabled mode messaging, local mode happy path base render.
- **Mode toggle**: assert success toast and invalidation, and error toast path (simulate thrown error) with switch disabled while pending.
- **Import/Export**: success export download invocation; import with file triggers backup plus import mutations; no-file guard; import error toast from `onError`.
- **Presets**: initial preset selection when `selectedPresetSlug` is empty; pull success populates preview/meta; hub 503 shows `preset-hub-unavailable` and cached preview button; validation 400 sets `preset-validation-error`; generic failure sets `preset-status`; cached preview load path toggles `hubUnavailable` false.
- **Apply**: backend success populates `preset-apply-info` (backup/reload/usedCscli fields); backend 501 falls back to `applyPresetLocally` and sets status/local toast; backend 400 validation error path; backend 503 hub unavailable path; missing cache error path setting validation message; generic failure with backup path; button disabled when hub offline plus preset requires hub.
- **Local apply helper**: guard when no preset selected; guard when no target file; guard when preview missing; success writes file, updates `applyInfo` with cacheKey, refreshes list, sets `selectedPath` and `fileContent`.
- **File editor**: list select loads content; save triggers backup plus write success; close resets state; textarea change updates state.
- **Banned IPs**: disabled mode message; loading spinner; error rendering; empty state; populated table row render (IP/Reason/Duration/Created/Source/Actions); unban confirm modal flows to success; ban modal opens, disables submit until IP entered, success toast path.
- **Overlay messaging**: drive each mutation pending flag (pull/apply/import/write/mode/ban/unban) to assert `ConfigReloadOverlay` message/submessage selections.

- Add the high-yield tests from the roadmap to lift overall coverage: [frontend/src/api/notifications.ts](frontend/src/api/notifications.ts), [frontend/src/api/logs.ts](frontend/src/api/logs.ts), [frontend/src/api/users.ts](frontend/src/api/users.ts), [frontend/src/pages/SMTPSettings.tsx](frontend/src/pages/SMTPSettings.tsx), [frontend/src/components/LiveLogViewer.tsx](frontend/src/components/LiveLogViewer.tsx), [frontend/src/pages/UsersPage.tsx](frontend/src/pages/UsersPage.tsx), [frontend/src/pages/Security.tsx](frontend/src/pages/Security.tsx), [frontend/src/pages/Dashboard.tsx](frontend/src/pages/Dashboard.tsx), [frontend/src/components/Layout.tsx](frontend/src/components/Layout.tsx) (plus any remaining Summary/FeatureFlagProvider items if present).
- Apply the deflake strategies noted for SMTP and ensure React Query caches are reset between tests.

## Test Data and Techniques
- Favor MSW or vi.fn stubs with per-test response shaping to toggle status codes (200/400/501/503) and payloads for presets/decisions/files.
- Use `await screen.findBy...` to avoid race conditions with async queries; keep real timers unless code relies on timers.
- Spy on `toast.success/error/info` to assert side effects without leaking state across tests.
- For downloads, mock `downloadCrowdsecExport` and `promptCrowdsecFilename` to avoid touching the filesystem while still asserting call arguments.

## Commands and Checks
- `cd frontend && npm test -- --runInBand --watch=false` for focused iterations on new specs.
- `cd frontend && npm run coverage` (or `vitest run --coverage`) to verify 100% on CrowdSecConfig and >=85% overall before merging.
- `cd frontend && npm run type-check` to ensure new test utils respect types.

## File Hygiene Notes
- [.gitignore](.gitignore): already excludes [frontend/coverage](frontend/coverage) and [frontend/test-results](frontend/test-results); no change needed for the new specs or fixtures.
- [.dockerignore](.dockerignore): keeps docs and tests out of the image; safe to leave as-is for this plan.
- [.codecov.yml](.codecov.yml): coverage target at 75% is looser than our goal but fine; ignore patterns keep tests out of reports without harming source coverage—no update required.
- [Dockerfile](Dockerfile): no frontend testing impact; no adjustments needed for this coverage work.

## Risks and Mitigations
- **Async flakiness**: mitigate with `findBy` queries and isolated QueryClient per test.
- **Mutation overlap**: ensure one mutation pending flag is exercised per test to avoid ambiguous overlay assertions.
- **Fixture drift**: store preset/file/decision fixtures near tests to keep intent visible; update when API shapes evolve.

## Definition of Done (for this effort)
- All CrowdSecConfig branches covered (100% statements/branches/functions) with deterministic RTL tests.
- Remaining coverage boost items from the roadmap implemented or queued with clear owners.
- Frontend test suite passes locally; coverage report confirms lift; no ignores or Docker/git hygiene regressions introduced.
go test -coverprofile=handlers_full.cover ./internal/api/handlers -v
go tool cover -func=handlers_full.cover | grep total

# HTML report
go tool cover -html=handlers_full.cover -o handlers_coverage.html

# Pre-commit (includes all checks)
cd /projects/Charon
.venv/bin/pre-commit run --all-files
```

---

## Part 7: Risk Assessment

### Technical Risks

**Risk 1: WebSocket Testing Complexity**
- Impact: High
- Probability: Medium
- Mitigation: Use httptest.Server + real WebSocket library (proven approach)
- Fallback: Simplify tests, accept lower coverage

**Risk 2: Timing-Dependent Tests**
- Impact: Medium (flaky tests)
- Probability: Medium
- Mitigation: Use reduced ticker intervals for testing or mock time
- Fallback: Accept longer test execution time

**Risk 3: Goroutine Leaks**
- Impact: Medium
- Probability: Low
- Mitigation: Proper defer cleanup, verify with runtime.NumGoroutine()
- Fallback: Use goleak library if needed

**Risk 4: Coverage Target Not Met**
- Impact: Medium
- Probability: Low
- Mitigation: Secondary tests (Part 2) already planned
- Fallback: Adjust target or add more test cases

### Configuration Review

**Files Checked**:
- `.codecov.yml`: Target 75% (below our 85% goal) - No changes needed ✓
- `.gitignore`: Excludes `*.cover`, `*.html`, test artifacts - No changes needed ✓
- `.dockerignore`: Excludes test files properly - No changes needed ✓

**Conclusion**: No configuration changes required.

---

## Part 8: Success Criteria

### Required
- [ ] LogsWebSocketHandler coverage ≥ 85%
- [ ] Overall handler coverage ≥ 85%
- [ ] All tests pass consistently
- [ ] No goroutine leaks
- [ ] Pre-commit checks pass
- [ ] CI/CD pipeline passes

### Optional
- [ ] Secondary handler coverage improved
- [ ] HTML coverage report generated
- [ ] Documentation updated
- [ ] Code review approved

---

## Part 9: File Reference

### Files to Create
1. `backend/internal/api/handlers/logs_ws_test_utils.go` - WebSocket testing utilities
2. `backend/internal/api/handlers/logs_ws_comprehensive_test.go` - Main test suite (18 tests)
3. `backend/internal/api/handlers/settings_handler_smtp_test.go` - SMTP config tests (optional)

### Files to Modify
1. `backend/internal/api/handlers/security_notifications_test.go` - Add error path tests (optional)
2. `backend/internal/api/handlers/logs_handler_test.go` or create `logs_handler_coverage_test.go` (optional)

### Files to Reference
1. `backend/internal/api/handlers/logs_ws.go` - Implementation under test
2. `backend/internal/logger/logger.go` - BroadcastHook dependency
3. `backend/internal/logger/logger_test.go` - Logger testing patterns
4. `backend/internal/api/handlers/proxy_host_handler_test.go` - Test setup patterns

---

## Appendix: Test Case Summary

| # | Test Name | Category | Est. Coverage | Lines |
|---|-----------|----------|---------------|-------|
| 1 | SuccessfulConnection | Happy Path | 5% | 10 |
| 2 | ReceiveLogEntries | Happy Path | 10% | 15 |
| 3 | PingKeepalive | Happy Path | 5% | 8 |
| 4 | LevelFilter | Filters | 8% | 12 |
| 5 | SourceFilter | Filters | 8% | 12 |
| 6 | CombinedFilters | Filters | 5% | 8 |
| 7 | CaseInsensitiveFilters | Filters | 3% | 5 |
| 8 | UpgradeFailure | Error Paths | 5% | 8 |
| 9 | ClientDisconnect | Error Paths | 8% | 12 |
| 10 | WriteJSONFailure | Error Paths | 6% | 10 |
| 11 | ChannelClosed | Error Paths | 5% | 8 |
| 12 | PingWriteFailure | Error Paths | 4% | 6 |
| 13 | MultipleConnections | Concurrency | 3% | 5 |
| 14 | HighVolumeLogging | Concurrency | 3% | 5 |
| 15 | EmptyLogFields | Edge Cases | 3% | 5 |
| 16 | SubscriberIDUniqueness | Edge Cases | 2% | 3 |
| 17 | WithRealLogger | Integration | 4% | 6 |
| 18 | ConnectionLifecycle | Integration | 3% | 5 |
| **Total** | | | **~85%** | **~90** |

---

## Document Metadata

**Author**: GitHub Copilot
**Date**: December 10, 2025
**Version**: 1.0
**Status**: Ready for Implementation
**Estimated Effort**: 2-3 days
**Expected Coverage Gain**: +1.5% to +2.0%
**Target Achievement Probability**: 95%+
