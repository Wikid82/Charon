<!--
  Sample Orchestration Plan used by the Management agent when invoking subagents.
  Keep this file small and precise. Subagents will read the file and act according to the Handoff Contract.
-->

# Plan: Aggregated Host Statuses Endpoint + Dashboard Widget

## 1) Title

Implement `/api/v1/host_statuses` backend endpoint and the `CharonStatusWidget` frontend component.

## 2) Overview

This feature provides an aggregated view of the number of proxy hosts and the number of hosts that are up/down. The backend exposes an endpoint returning aggregated counts, and the frontend consumes the endpoint and presents a dashboard widget.

## 3) Handoff Contract (Example)

**GET** /api/v1/stats/host_statuses

Response (200):

```json
{
  "total_proxy_hosts": 12,
  "hosts_up": 10,
  "hosts_down": 2
}
```

## 4) Backend Requirements

- Add a new read-only route `GET /api/v1/stats/host_statuses` under `internal/api/handlers/`.
- Implement the handler to use existing models/services and return the aggregated counts in JSON.
- Add unit tests under `backend/internal/services` and the handler's folder.

## 5) Frontend Requirements

- Add `frontend/src/components/CharonStatusWidget.tsx` to render the widget using the endpoint or existing monitors if no endpoint is present.
- Add a hook and update the API client if necessary: `frontend/src/api/stats.ts` with `getHostStatuses()`.
- Add unit tests: vitest for the component and the hook.

## 6) Acceptance Criteria

- Backend: `go test ./...` passes.
- Frontend: `npm run type-check` and `npm run build` pass.
- All unit tests pass and new coverage for added code is included.

## 7) Artifacts

- `docs/plans/current_spec.md` (the plan file)
- `backend` changed files including handler and tests
- `frontend` changed files including component and tests
