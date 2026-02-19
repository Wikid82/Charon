# Phase 2 E2E Failure Fix Plan

## 1. Introduction

This plan analyzes Phase 2 E2E failures from the remediation checklist and
prioritizes fixes that unblock the most tests. It focuses on shared root
causes, dependency clusters, and ownership for targeted remediation.

## 2. Research Findings

### 2.1 Source of Truth

Primary input: [E2E_REMEDIATION_CHECKLIST.md](../../E2E_REMEDIATION_CHECKLIST.md)
(Phase 2A, 2B, 2C failures).

### 2.2 Failure Clusters

- Core UI Docker integration: 2 failures on missing/blocked connection source
  control.
- Settings notifications: 7 failures with timeouts or page context closure.
- Settings strict-mode collisions: 5 failures from over-broad selectors.
- Tasks log viewing: 12 timeouts waiting for log responses.
- Caddy import sessions: 3 failures (import results and missing session banner).
- Monitoring real-time logs: 19 failures with WebSocket status stuck at
  Disconnected.
- Wait-helpers: 1 failure waiting for URL string match.

## 3. Root Cause Categorization

### 3.1 Failure Buckets (54 total)

| Bucket | Count | Examples |
| --- | --- | --- |
| Backend API issues | 24 | Notifications CRUD/timeouts, system settings save, Caddy import results, log viewing API timeouts |
| Frontend UI issues | 3 | Docker integration control missing, certificate email validation state |
| WebSocket issues | 19 | Real-time logs never connect (Disconnected state persists) |
| Test infrastructure issues | 6 | Strict-mode collisions (selectors), wait-helpers URL timeout |
| Admin access/permissions issues | 2 | Guest visibility of backup button, permissions uncheck disabled |

### 3.2 Root Cause Patterns

- Logs viewing failures (12) all timeout on `page.waitForResponse`, indicating
  a shared logs API endpoint not returning or blocked in Docker mode.
- Real-time logs failures (19) all show Disconnected, indicating WebSocket
  handshake or server-side streaming not established for `/api/v1/logs`.
- Caddy import failures cluster on missing import session artifacts (no banner
  and zero parsed imports), suggesting a shared import-session persistence or
  retrieval issue.
- Settings notifications failures cluster on timeouts and context closure,
  suggesting API routes or navigation errors when provider lists/templates
  are queried or mutated.
- Strict-mode collisions in settings and monitoring point to test selectors
  resolving multiple nodes, indicating test infra refinement needed.
- Admin access failures show inconsistent RBAC enforcement between UI
  visibility and server-side enforcement.

## 4. Technical Specifications

### 4.1 Priority Ranking (Max Impact First)

1. WebSocket connection failures for real-time logs (19 tests blocked)
2. Logs API timeouts for static log viewing (12 tests blocked)
3. Notifications settings API timeouts/context closure (7 tests blocked)
4. Caddy import session persistence/results (3 tests blocked)
5. Docker integration UI controls missing (2 tests blocked)
6. Strict-mode collisions and wait-helpers (6 tests blocked)
7. Admin access/permissions mismatches (2 tests blocked)

### 4.2 Fix Batches

#### Critical Fixes (Block multiple suites)

- WebSocket connection / event delivery
  - Affected tests: 19 (monitoring/real-time-logs)
  - Root cause: WebSocket never reaches Connected; likely backend
    upgrade/streaming path or proxy config issue.
  - Recommendation: Backend Dev

- Logs API timeouts
  - Affected tests: 12 (tasks/logs-viewing)
  - Root cause: log listing endpoints timing out or blocked in container mode.
  - Recommendation: Backend Dev

- Notifications settings API timeouts
  - Affected tests: 7 (settings/notifications)
  - Root cause: provider/template APIs not responding or UI navigation error
    closing the page context.
  - Recommendation: Backend Dev with Frontend Dev support

- Caddy import session persistence
  - Affected tests: 3 (tasks/caddy-import-*)
  - Root cause: import sessions not persisted or banner data not returned.
  - Recommendation: Backend Dev

#### Secondary Fixes (Quick wins or infra)

- Docker integration UI controls
  - Affected tests: 2 (core/proxy-hosts Docker integration)
  - Root cause: missing/hidden form control for connection source.
  - Recommendation: Frontend Dev

- Strict-mode collisions and wait helpers
  - Affected tests: 6 (settings + monitoring + wait-helpers)
  - Root cause: selectors match multiple elements or URL helper too strict.
  - Recommendation: Playwright Dev

- Admin access/permissions mismatches
  - Affected tests: 2 (tasks/backups guest UI, settings permission uncheck)
  - Root cause: UI visibility vs RBAC mismatch or disabled inputs.
  - Recommendation: Backend Dev with Frontend Dev support

## 5. Effort and Impact Estimates

| Category | Effort | Impact | Notes |
| --- | --- | --- | --- |
| WebSocket connection | L | Very High | Unblocks 19 monitoring tests |
| Logs API timeouts | M | High | Unblocks 12 task tests |
| Notifications API timeouts | M | High | Unblocks 7 settings tests |
| Caddy import sessions | M | Medium | Unblocks 3 task tests |
| Docker integration UI | S | Medium | Unblocks 2 core tests |
| Strict-mode + wait helpers | S | Medium | Unblocks 6 tests |
| Admin access mismatches | S | Low | Unblocks 2 tests |

## 6. Implementation Plan

### Phase 1: WebSocket and Logs APIs

1. Verify `/api/v1/logs` WebSocket handshake and server-side stream starts.
2. Validate static logs API endpoints and response time in Docker mode.
3. Confirm UI connects to correct WebSocket endpoint for app/security modes.

### Phase 2: Notifications and Caddy Import Sessions

1. Validate notification providers CRUD endpoints and template endpoints.
2. Ensure notification routes do not crash the page context.
3. Validate import-session persistence and banner retrieval endpoints.

### Phase 3: UI and Test Infrastructure Quick Wins

1. Restore Docker integration connection source control visibility.
2. Tighten selectors in strict-mode failures (system status, user management,
   uptime monitor).
3. Adjust wait-helpers URL matching to handle expected navigation timing.

### Phase 4: RBAC Consistency

1. Ensure guest users cannot see Create Backup UI controls.
2. Ensure permission management inputs reflect actual capability and are
   enabled for admin flows.

## 7. Acceptance Criteria (EARS)

- WHEN the real-time logs page loads, THE SYSTEM SHALL establish a WebSocket
  connection and report Connected status within the test timeout.
- WHEN static logs are requested, THE SYSTEM SHALL return log data within
  the test timeout for pagination, filtering, and download flows.
- WHEN notification providers/templates are managed, THE SYSTEM SHALL respond
  to CRUD requests without page context closure or timeouts.
- WHEN a Caddy import session exists, THE SYSTEM SHALL return the session
  banner and import results for review flows.
- WHEN a guest user accesses backups, THE SYSTEM SHALL hide Create Backup
  controls and enforce server-side RBAC.
- WHEN strict-mode selectors are used, THE SYSTEM SHALL present a unique
  element for each targeted control in settings and monitoring pages.

## 8. Delegation Recommendations

- Backend Dev
  - WebSocket connection and streaming
  - Logs API timeouts
  - Notifications APIs
  - Caddy import session persistence
  - RBAC enforcement for backups and permissions

- Frontend Dev
  - Docker integration UI control visibility
  - UI state handling for notifications if backend responses are valid

- Playwright Dev
  - Strict-mode selector refinements
  - wait-helpers URL matching reliability

## 9. Confidence Score

Confidence: 78 percent

Rationale: Failure clusters are clear and repeated across suites, but root
causes still require endpoint-level confirmation in backend logs and
WebSocket diagnostics.
