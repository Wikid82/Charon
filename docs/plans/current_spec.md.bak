<!--
  This file is a placeholder for the current plan. The `Planning` agent must write the detailed plan here (see docs/plans/sample_orchestration_plan.md for a sample).
  Subagents will read this file as the single source of truth for the feature implementation.
-->

<!--
    CURRENT SPEC: Aggregated Host Statuses (Uptime) — Endpoint + Dashboard Widget
    - Replace this file with the feature spec and Handoff JSON contract for implementing
        'Aggregated Host Statuses': an API endpoint grouping uptime monitors by host and
        a dashboard widget that shows aggregated host-level health and quick drill-down.
    - This document should be used as the single source of truth for developers and handoff.
-->

# Current Plan: Aggregated Host Statuses

This feature adds a backend endpoint that returns aggregated health information for upstream hosts
and a frontend Dashboard widget to display the aggregated view. The goal is to provide host-level
health at-a-glance to help identify server-wide outages and quickly navigate to affected services.

## Summary
- Endpoint: `GET /api/v1/uptime/hosts/aggregated` (authenticated)
- Backend: Service method + handler + route + GORM query, small in-memory cache, server-side filters
- Frontend: API client, custom React Query hook, `HostStatusesWidget` in Dashboard, demo/test pages
- Acceptance: Auth respects accessible hosts, accurate counts, performance (fast aggregate queries)

## HandOff JSON contract (Truth)
Request: `GET /api/v1/uptime/hosts/aggregated`
- Query Params (optional):
    - `status` (string): filter results by host status: up|down|pending|maintenance
    - `q` (string): search text (host or name)
    - `sort_by` (string): `monitor_count|down_count|avg_latency|last_check` (default: `down_count`)
    - `order` (string): `asc|desc` (default: `desc`)
    - `page` (int): pagination page (default 1)
    - `per_page` (int): items per page (default 50)

Response: 200 JSON
```json
{
    "aggregated_hosts": [
        {
            "id": "uuid",
            "host": "10.0.0.12",
            "name": "web-01",
            "status": "down",
            "monitor_count": 3,
            "counts": { "up": 1, "down": 2, "pending": 0, "maintenance": 0 },
            "avg_latency_ms": 257,
            "last_check": "2025-12-05T09:54:54Z",
            "last_status_change": "2025-12-05T09:53:44Z",
            "affected_monitors": [
                { "id": "mon-1", "name": "example-api", "status": "down", "last_check": "2025-12-05T09:54:54Z" },
                { "id": "mon-2", "name": "webapp", "status": "down", "last_check": "2025-12-05T09:52:14Z" }
            ],
            "uptime_24h": 99.3
        }
    ],
    "meta": { "page": 1, "per_page": 50, "total": 1 }
}
```

Notes:
- All timestamps are ISO 8601 UTC.
- Field names use snake_case (server -> frontend contract per project guidelines).
- Only accessible hosts are returned to the authenticated caller (utilize existing auth handlers).

## Backend Requirements
1. Database
     - Ensure index on `uptime_monitors(uptime_host_id)`, `uptime_monitors(status)`, and `uptime_monitors(last_check)`.
     - No model changes required for `UptimeHost` or `UptimeMonitor` unless we want an `avg_latency` column cached (optional).

2. Service (in `internal/services/uptime_service.go`)
     - Add method: `GetAggregatedHostStatuses(filters AggregationFilter) ([]AggregatedHost, error)`.
     - Implementation detail:
         - Query should join `uptime_hosts` and `uptime_monitors` and run a `GROUP BY uptime_host_id`.
         - Use a SELECT that computes: monitor_count, up_count, down_count, pending_count, maintenance_count, avg_latency, last_check (MAX), last_status_change (MAX).
         - Provide a parameter to include a limited list of affected monitors (eg. top N by last_check) and optional `uptime_24h` calculation where a heartbeat history exists.
         - Return GORM structs matching the `AggregatedHost` DTO.

3. Handler (in `internal/api/handlers/uptime_handler.go`)
     - Add `func (h *UptimeHandler) AggregatedHosts(c *gin.Context)` that:
         - Binds query params; validates and normalizes them.
         - Calls `service.GetAggregatedHostStatuses(filters)`.
         - Filters the results using `authMiddleware` (maintain accessible hosts list or `authHandler.GetAccessibleHosts` logic).
         - Caches the result for `CHARON_UPTIME_AGGREGATION_TTL` (default 30s). Cache strategy: package global in `services` with simple `sync.Map` + TTL.
         - Produces a 200 JSON with the contract above.
     - Add unit tests and integration tests verifying results and auth scoping.

4. Routes
     - Register under protected group in `internal/api/routes/routes.go`:
         - `protected.GET('/uptime/hosts/aggregated', uptimeHandler.AggregatedHosts)`

5. Observability
     - Add a Prometheus counter/metric: `charon_uptime_aggregated_requests_total` (labels: status, cache_hit true/false).
     - Add logs for aggregation errors.

6. Security
     - Ensure only authenticated users can access aggregated endpoint.
     - Respect `authHandler.GetAccessibleHosts` (or similar) to filter hosts the user should see.

7. Tests
     - Unit tests for service logic calculating aggregates (mock DB / in-memory DB fixtures).
     - Handler integration tests using the testdb and router that verify JSON response structure, pagination, filters, and auth filtering.
     - Perf tests: basic benchmark to ensure aggregation query completes within acceptable time for 10k monitors (e.g. < 200ms unless run on dev env; document specifics).

## Frontend Requirements
1. API client changes (`frontend/src/api/uptime.ts`)
     - Add `export const getAggregatedHosts = async (params?: AggregationQueryParams) => client.get<AggregatedHost[]>('/uptime/hosts/aggregated', { params }).then(r => r.data)`
     - Add new TypeScript types for `AggregatedHost`, `AggregatedHostCounts`, `AffectedMonitor`.

2. React Query Hook (`frontend/src/hooks/useAggregatedHosts.ts`)
     - `useAggregatedHosts` should accept params similar to query params (filters), and accept `enabled` flag.
     - Use TanStack Query with `refetchInterval: 30_000` and `staleTime: 30_000` to match backend TTL.

3. Dashboard Widget (`frontend/src/components/Dashboard/HostStatusesWidget.tsx`)
     - Shows high-level summary: total hosts, down_count, up_count, pending.
     - Clickable host rows navigate to the uptime or host detail page.
     - Visuals: small status badge, host name, counts, avg latency, last check time.
     - Accessible: all interactive elements keyboard and screen-reader navigable.
     - Fallback: if the aggregated endpoint is not found or returns 403, display a short explanatory message with a link to uptime page.

4. Dashboard Page Update (`frontend/src/pages/Dashboard.tsx`)
     - Add `HostStatusesWidget` to the Dashboard layout (prefer 2nd column near `UptimeWidget`).

5. Tests
     - Unit tests for `HostStatusesWidget` rendering different states.
     - Mock API responses for `useAggregatedHosts` using the existing test utilities.
     - Add Storybook story if used in repo (optional).

6. Styling
     - Keep styling consistent with `UptimeWidget` (dark-card, status badges, mini bars).

## Acceptance Criteria
1. API
     - `GET /api/v1/uptime/hosts/aggregated` returns aggregated host objects in the correct format.
     - Query params `status`, `q`, `sort_by`, `order`, `page`, `per_page` work as expected.
     - The endpoint respects user-specific host access permissions.
     - Endpoint adheres to TTL caching; cache invalidation occurs after TTL or when underlying monitor status change triggers invalidation.

2. Backend Tests
     - Unit tests cover all aggregation branches and logic (e.g. zero-monitor host, mixed statuses, all down host).
     - Integration tests validate auth-scoped responses.

3. Frontend UI
     - Widget displays host-level counts and shows a list of top N hosts with status badges.
     - Clicking a host navigates to the uptime or host detail page.
     - Widget refreshes according to TTL and reacts to manual refreshes.
     - UI has automated tests covering rendering with typical API responses, filtering and pagination UI behavior.

4. Performance
     - Aggregation query responds within acceptable time for typical deployments (document target; e.g. < 200ms for 5k monitors), or we add a follow-up plan to add precomputation.

## Example API Contract (Sample Request + Response)
Request:
```http
GET /api/v1/uptime/hosts/aggregated?sort_by=down_count&order=desc&page=1&per_page=20
Authorization: Bearer <token>
```

Response:
```json
{
    "aggregated_hosts": [
        {
            "id": "39b6f7c2-2a5c-47d7-9c9d-1d7f1977dabc",
            "host": "10.0.10.12",
            "name": "production-web-1",
            "status": "down",
            "monitor_count": 3,
            "counts": {"up": 1, "down": 2, "pending": 0, "maintenance": 0},
            "avg_latency_ms": 257,
            "last_check": "2025-12-05T09:54:54Z",
            "last_status_change": "2025-12-05T09:53:44Z",
            "affected_monitors": [
                {"id":"m-01","name":"api.example","status":"down","last_check":"2025-12-05T09:54:54Z","latency":105},
                {"id":"m-02","name":"www.example","status":"down","last_check":"2025-12-05T09:52:14Z","latency":401}
            ],
            "uptime_24h": 98.77
        }
    ],
    "meta": {"page":1,"per_page":20,"total":1}
}
```

## Error cases
- 401 Unauthorized — Invalid or missing token.
- 403 Forbidden — Caller lacks host access.
- 500 Internal Server Error — DB / aggregation error.

## Observability & Operational Notes
- Metrics: `charon_uptime_aggregated_requests_total`, `charon_uptime_aggregated_cache_hits_total`.
- Cache TTL: default 30s via `CHARON_UPTIME_AGGREGATION_TTL` env var.
- Logging: Rate-limited errors and aggregation durations logged to the general logger.

## Follow-ups & Optional Enhancements
1. Add an endpoint-level `since` parameter that returns delta/trend information (e.g. change in down_count in last 24 hours).
2. Background precompute task (materialized aggregated table) for very large installations.
3. Add a configuration to show `affected_monitors` collapsed/expanded per host for faster page loads.

## Short List of Files To Change
- Backend:
    - backend/internal/services/uptime_service.go (add aggregation method)
    - backend/internal/api/handlers/uptime_handler.go (add handler method)
    - backend/internal/api/routes/routes.go (register new route)
    - backend/internal/services/uptime_service_test.go (add tests)
    - backend/internal/api/handlers/uptime_handler_test.go (add handler tests)
    - backend/internal/models/uptime.go / uptime_host.go (index recommendations or small schema updates if needed)

- Frontend:
    - frontend/src/api/uptime.ts (add `getAggregatedHosts`)
    - frontend/src/hooks/useAggregatedHosts.ts (new hook)
    - frontend/src/components/Dashboard/HostStatusesWidget.tsx (new widget)
    - frontend/src/pages/Dashboard.tsx (add widget)
    - frontend/src/components/__tests__/HostStatusesWidget.test.tsx (new tests)

---
If you want, I can now scaffold the backend service method + handler and the frontend API client and widget as a follow-up PR.
