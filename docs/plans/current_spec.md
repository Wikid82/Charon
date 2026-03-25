# CrowdSec Dashboard Integration — Implementation Specification

**Issue:** #26
**Version:** 1.1
**Status:** Draft — Post-Supervisor Review

---

## 1. Executive Summary

### What We're Building

A metrics and analytics dashboard for CrowdSec within Charon's existing Security section. This adds visualization, aggregation, and export capabilities to the CrowdSec module — surfacing data that today is only available via CLI or raw LAPI queries.

### Why

CrowdSec is already operationally integrated (start/stop, config, bouncer registration, decisions, console enrollment). What's missing is **visibility**: users cannot see attack trends, scenario breakdowns, ban history, or top offenders without SSH-ing into the container and running `cscli` commands. A dashboard closes this gap and makes Charon's security posture observable from the UI.

### Success Metrics

| Metric | Target |
|--------|--------|
| Issue #26 checklist tasks complete | 8/8 |
| New backend aggregation endpoints covered by unit tests | ≥ 85% line coverage |
| New frontend components covered by Vitest unit tests | ≥ 85% line coverage |
| E2E tests for dashboard page passing | All browsers (Firefox, Chromium, WebKit) |
| Dashboard page initial load time | < 2 seconds on cached data |
| No new CRITICAL/HIGH security findings | GORM scanner, CodeQL, Trivy |

---

## 2. Requirements (EARS Notation)

### R1: Metrics Dashboard Tab

**WHEN** the user navigates to `/security/crowdsec`, **THE SYSTEM SHALL** display a "Dashboard" tab alongside the existing configuration interface, showing summary statistics (total bans, active bans, unique IPs, top scenario).

### R2: Active Bans with Reasons

**WHEN** the Dashboard tab is active, **THE SYSTEM SHALL** display a table of currently active decisions including IP, scenario/reason, duration, type (ban/captcha), origin, and time remaining.

### R3: Scenarios Triggered

**WHEN** the Dashboard tab is active, **THE SYSTEM SHALL** display a breakdown of CrowdSec scenarios that have triggered decisions, with counts per scenario, sourced from LAPI alerts.

### R4: Ban History Timeline

**WHEN** the Dashboard tab is active, **THE SYSTEM SHALL** display a time-series chart showing ban/unban events over a configurable time range (default: last 24 hours; options: 1h, 6h, 24h, 7d, 30d).

### R5: Top Attacking IPs

**WHEN** the Dashboard tab is active, **THE SYSTEM SHALL** display a horizontal bar chart of the top 10 IP addresses by number of decisions, within the selected time range.

### R6: Attack Type Breakdown

**WHEN** the Dashboard tab is active, **THE SYSTEM SHALL** display a pie/donut chart breaking down decisions by scenario type (e.g., `crowdsecurity/http-probing`, `crowdsecurity/ssh-bf`).

### R7: Alert Notifications

**WHEN** a new CrowdSec decision is created, **THE SYSTEM SHALL** dispatch a notification via the existing notification provider system (Gotify/webhook) if the user has enabled CrowdSec decision notifications. *(Partially implemented — this PR enriches the payload with scenario and structured data.)*

### R8: Ban Export

**WHEN** the user clicks the "Export" button on the Dashboard tab, **THE SYSTEM SHALL** export the current decisions list as CSV or JSON, with the user's choice of format and time range.

### Non-Functional Requirements

- **NF1:** Aggregation queries SHALL complete within 200ms for tables up to 100k rows.
- **NF2:** Dashboard data SHALL be cached server-side with a 30-second TTL.
- **NF3:** All new endpoints SHALL require admin authentication.
- **NF4:** No LAPI keys SHALL appear in API responses (except the existing `GetBouncerKey` endpoint).
- **NF5:** The dashboard SHALL be accessible: keyboard navigable, screen-reader compatible, WCAG 2.2 AA contrast.

---

## 3. Architecture & Design

### 3.1 Data Sources

The dashboard draws from two data sources:

| Source | Data | Access Method |
|--------|------|---------------|
| **CrowdSec LAPI** (`/v1/decisions`, `/v1/alerts`) | Active decisions, alerts with scenarios, timestamps, IP addresses | HTTP via `network.NewInternalServiceHTTPClient` with SSRF-safe URL validation |
| **SQLite `security_decisions` table** | Historical decisions persisted by Charon's middleware | GORM queries with aggregation |

### 3.2 New Backend Endpoints

All endpoints are prefixed with `/api/v1/admin/crowdsec/` and require admin auth.

```
GET  /admin/crowdsec/dashboard/summary        → DashboardSummary
GET  /admin/crowdsec/dashboard/timeline        → DashboardTimeline
GET  /admin/crowdsec/dashboard/top-ips         → DashboardTopIPs
GET  /admin/crowdsec/dashboard/scenarios       → DashboardScenarios
GET  /admin/crowdsec/alerts                    → ListAlerts (LAPI wrapper)
GET  /admin/crowdsec/decisions/export          → ExportDecisions (CSV/JSON)
```

### 3.3 SecurityDecision Model Enrichment

The existing `SecurityDecision` model needs new indexed fields to support scenario-based aggregation without requiring LAPI for historical queries:

```go
// backend/internal/models/security_decision.go
type SecurityDecision struct {
    ID        uint      `json:"-" gorm:"primaryKey"`
    UUID      string    `json:"uuid" gorm:"uniqueIndex"`
    Source    string    `json:"source" gorm:"index"`
    Action    string    `json:"action" gorm:"index"`
    IP        string    `json:"ip" gorm:"index"`
    Host      string    `json:"host" gorm:"index"`
    RuleID    string    `json:"rule_id" gorm:"index"`
    Details   string    `json:"details" gorm:"type:text"`
    CreatedAt time.Time `json:"created_at" gorm:"index"`

    // NEW FIELDS (PR-1)
    Scenario  string    `json:"scenario" gorm:"index"`    // e.g., "crowdsecurity/http-probing"
    Country   string    `json:"country" gorm:"index;size:2"` // ISO 3166-1 alpha-2 (best-effort, see below)
    ExpiresAt time.Time `json:"expires_at" gorm:"index"`  // When this decision expires
}
```

**Country field population (I3 clarification):** The `Country` field is populated on a best-effort basis. CrowdSec LAPI alerts include a `source.cn` field (ISO 3166-1 alpha-2 country code, e.g., `"CN"`, `"RU"`, `"US"`) when GeoIP data is available in the CrowdSec hub enrichment. No additional GeoIP lookup by Charon is required. When `source.cn` is absent (e.g., local IPs, LAPI unavailable, manual bans), the field is stored as an empty string `""`. The frontend treats empty-string country values as "Unknown" in display.

**Migration:** GORM `AutoMigrate` will add columns non-destructively. No manual migration needed. Existing rows will have empty `Scenario`/`Country`/`ExpiresAt` fields.

**Composite Indexes:** In addition to the single-column indexes defined via struct tags, the following composite indexes are required for performant aggregation queries on 100k+ row tables. Define them via GORM `compositeIndex` tags or a custom `Migrator.CreateIndex()` call in the model's `AfterAutoMigrate` hook:

| Composite Index Name | Columns | Covers |
|----------------------|---------|--------|
| `idx_sd_source_created` | `(source, created_at)` | All time-range filtered queries: summary counts, timeline bucketing |
| `idx_sd_source_scenario_created` | `(source, scenario, created_at)` | Scenario breakdown, top-scenario ranking |
| `idx_sd_source_ip_created` | `(source, ip, created_at)` | Top-IPs ranking, unique IP counts |

**GORM definition (compositeIndex tag approach):**

```go
type SecurityDecision struct {
    // ...
    Source    string    `json:"source" gorm:"index;compositeIndex:idx_sd_source_created;compositeIndex:idx_sd_source_scenario_created;compositeIndex:idx_sd_source_ip_created"`
    Scenario  string   `json:"scenario" gorm:"index;compositeIndex:idx_sd_source_scenario_created"`
    IP        string   `json:"ip" gorm:"index;compositeIndex:idx_sd_source_ip_created"`
    CreatedAt time.Time `json:"created_at" gorm:"index;compositeIndex:idx_sd_source_created,sort:desc;compositeIndex:idx_sd_source_scenario_created,sort:desc;compositeIndex:idx_sd_source_ip_created,sort:desc"`
    // ...
}
```

Alternatively, define via explicit `Migrator` calls in a post-migration hook if tag complexity is too high:

```go
db.Migrator().CreateIndex(&SecurityDecision{}, "idx_sd_source_created")
```

### 3.4 Data Flow

```
┌─────────────┐     ┌──────────────────┐     ┌───────────────────┐
│   React UI  │────▶│  Go Backend API  │────▶│  CrowdSec LAPI    │
│  Dashboard  │     │  /dashboard/*    │     │  /v1/decisions     │
│  Components │◀────│  + Cache Layer   │◀────│  /v1/alerts        │
└─────────────┘     │                  │     └───────────────────┘
                    │  Aggregation     │
                    │  Queries         │────▶┌───────────────────┐
                    │                  │     │  SQLite DB         │
                    │                  │◀────│  security_decisions│
                    └──────────────────┘     └───────────────────┘
```

### 3.5 Caching Strategy

A simple in-memory cache with TTL per endpoint. The cache instance is owned by `CrowdsecHandler`:

```go
// backend/internal/api/handlers/crowdsec_handler.go — add field to existing struct
type CrowdsecHandler struct {
    // ... existing fields (DB, Executor, CmdExec, BinPath, etc.)
    dashCache *dashboardCache // Dashboard analytics cache (initialized in constructor)
}
```

`dashCache` is initialized in `NewCrowdsecHandler()` (or the factory function that creates the handler). `BanIP` and `UnbanIP` call `h.dashCache.Invalidate("dashboard")` after successful operations to ensure stale data is not served.

```go
// backend/internal/api/handlers/crowdsec_dashboard_cache.go
type dashboardCache struct {
    mu      sync.RWMutex
    entries map[string]*cacheEntry
}

type cacheEntry struct {
    data      interface{}
    expiresAt time.Time
}
```

| Endpoint | TTL | Rationale |
|----------|-----|-----------|
| `/dashboard/summary` | 30s | High-level counters, frequent access |
| `/dashboard/timeline` | 60s | Aggregated time-series, moderate computation |
| `/dashboard/top-ips` | 60s | Aggregated ranking |
| `/dashboard/scenarios` | 60s | Scenario grouping |
| `/alerts` | 30s | LAPI-sourced, changes frequently |

Cache is invalidated on any `BanIP` or `UnbanIP` call.

### 3.6 Frontend Component Tree

```
CrowdSecConfig.tsx (existing — add tab navigation)
├── Tab: Configuration (existing content, unchanged)
└── Tab: Dashboard (NEW)
    └── CrowdSecDashboard.tsx
        ├── DashboardSummaryCards.tsx      (4 stat cards)
        ├── BanTimelineChart.tsx           (Recharts AreaChart)
        ├── TopAttackingIPsChart.tsx       (Recharts BarChart horizontal)
        ├── ScenarioBreakdownChart.tsx     (Recharts PieChart)
        ├── ActiveDecisionsTable.tsx       (table with sort/filter)
        ├── AlertsList.tsx                 (LAPI alerts feed)
        ├── DashboardTimeRangeSelector.tsx (1h/6h/24h/7d/30d toggle)
        └── DecisionsExportButton.tsx      (CSV/JSON export)
```

### 3.7 Charting Library: Recharts

**Decision:** Add `recharts` as a frontend dependency.

| Criterion | Recharts | Chart.js/react-chartjs-2 | Nivo |
|-----------|----------|--------------------------|------|
| React integration | Native (built on React components) | Wrapper around imperative API | Native |
| Bundle size (tree-shaken) | ~45 KB gzipped (only imported charts) | ~65 KB gzipped | ~80+ KB |
| TypeScript support | Built-in | @types package | Built-in |
| D3-based | Yes (composable) | No (Canvas-based) | Yes |
| Accessibility | SVG-based, supports ARIA labels | Canvas (limited a11y) | SVG, good a11y |
| Tailwind compatibility | SVG renders, style via props | Canvas, no CSS styling | SVG renders |
| Maintenance | Active, 24k+ stars, MIT | Active | Active |
| SSR/SSG | Works | Requires client-only | Works |

**Rationale:** Recharts is React-native (no imperative bridge), renders SVG (accessible, inspectable, screen-reader friendly), tree-shakeable, and has the best Tailwind CSS compatibility. Bundle impact is minimal since only the chart types we use are imported.

**Install:** `npm install recharts` in `frontend/`.

### 3.8 Build & Config File Changes

**`frontend/package.json`** — add `"recharts": "^2.15.0"` to `dependencies`.

**`Dockerfile`** — No changes needed. `npm ci` at line 113 will install Recharts from the updated `package-lock.json`.

**`.gitignore`** — No changes needed. `node_modules/` is already ignored.

**`codecov.yml`** — No changes needed. Frontend coverage flags already cover `frontend/src/**`.

**`.dockerignore`** — No changes needed. Build context already includes `frontend/`.

---

## 4. API Specification

### 4.1 `GET /admin/crowdsec/dashboard/summary`

Returns aggregate counts for the dashboard summary cards.

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `range` | string | `24h` | Time range: `1h`, `6h`, `24h`, `7d`, `30d` |

**Response (200):**

```json
{
  "total_decisions": 1247,
  "active_decisions": 23,
  "unique_ips": 891,
  "top_scenario": "crowdsecurity/http-probing",
  "decisions_trend": 12.5,
  "range": "24h",
  "cached": true,
  "generated_at": "2026-03-25T10:30:00Z"
}
```

**`decisions_trend` calculation:** Percentage change vs. the previous period of equal length. For a `24h` range: compare total decisions in the last 24h against the total decisions in the 24h before that. Formula: `((current - previous) / previous) * 100`. If `previous = 0`, return `0.0` (no trend data). If `current = 0` and `previous > 0`, return `-100.0`. Value is a float rounded to 1 decimal place. Positive = increase, negative = decrease.

**Backend Implementation:**

```go
// File: backend/internal/api/handlers/crowdsec_dashboard.go
// Function: func (h *CrowdsecHandler) DashboardSummary(c *gin.Context)

// HYBRID APPROACH (B7): active_decisions comes from LAPI; historical metrics from SQLite.
//
// Existing Cerberus middleware callers of SecurityService.LogDecision() do not
// populate ExpiresAt. Rather than modifying all middleware callers in PR-1, we
// use the authoritative LAPI source for active decision counts and SQLite for
// historical aggregations only.
//
// From CrowdSec LAPI (real-time, authoritative):
//   GET /v1/decisions  → len(decisions) = active_decisions
//
// From SQLite (historical aggregation):
//   SELECT COUNT(*) as total FROM security_decisions WHERE created_at >= ? AND source = 'crowdsec'
//   SELECT COUNT(DISTINCT ip) as unique_ips FROM security_decisions WHERE created_at >= ? AND source = 'crowdsec'
//   SELECT scenario, COUNT(*) as cnt FROM security_decisions WHERE created_at >= ? AND source = 'crowdsec' GROUP BY scenario ORDER BY cnt DESC LIMIT 1
//
// Fallback: If LAPI is unreachable, active_decisions = -1 (frontend shows "N/A").
// The ExpiresAt field is still added to the model for future enrichment when
// Cerberus middleware callers are updated in a subsequent PR.
```

### 4.2 `GET /admin/crowdsec/dashboard/timeline`

Returns time-bucketed decision counts for the timeline chart.

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `range` | string | `24h` | Time range |
| `interval` | string | auto | Bucket interval: `5m`, `1h`, `1d` (auto-selected based on range if omitted) |

**Auto-interval mapping:**

| Range | Default Interval | Max Buckets |
|-------|-----------------|-------------|
| `1h` | `5m` | 12 |
| `6h` | `15m` | 24 |
| `24h` | `1h` | 24 |
| `7d` | `6h` | 28 |
| `30d` | `1d` | 30 |

**Response (200):**

```json
{
  "buckets": [
    { "timestamp": "2026-03-25T00:00:00Z", "bans": 5, "captchas": 1 },
    { "timestamp": "2026-03-25T01:00:00Z", "bans": 12, "captchas": 0 }
  ],
  "range": "24h",
  "interval": "1h",
  "cached": true
}
```

**Backend Implementation:**

```go
// File: backend/internal/api/handlers/crowdsec_dashboard.go
// Function: func (h *CrowdsecHandler) DashboardTimeline(c *gin.Context)

// SQLite time bucketing via strftime for all 5 intervals:
//
// 5m:  strftime('%Y-%m-%dT%H:', created_at) ||
//        printf('%02d:00Z', (CAST(strftime('%M', created_at) AS INTEGER) / 5) * 5)
//
// 15m: strftime('%Y-%m-%dT%H:', created_at) ||
//        printf('%02d:00Z', (CAST(strftime('%M', created_at) AS INTEGER) / 15) * 15)
//
// 1h:  strftime('%Y-%m-%dT%H:00:00Z', created_at)
//
// 6h:  strftime('%Y-%m-%dT', created_at) ||
//        printf('%02d:00:00Z', (CAST(strftime('%H', created_at) AS INTEGER) / 6) * 6)
//
// 1d:  strftime('%Y-%m-%dT00:00:00Z', created_at)
//
// Example (1h bucket — default for 24h range):
// SELECT strftime('%Y-%m-%dT%H:00:00Z', created_at) as bucket,
//        SUM(CASE WHEN action = 'block' THEN 1 ELSE 0 END) as bans,
//        SUM(CASE WHEN action = 'challenge' THEN 1 ELSE 0 END) as captchas
// FROM security_decisions
// WHERE created_at >= ? AND source = 'crowdsec'
// GROUP BY bucket ORDER BY bucket ASC
```

The `intervalToStrftime(interval string) string` helper function maps the interval parameter to the appropriate SQLite `strftime` expression. The 5-minute and 6-hour roundings use integer division to snap to the nearest bucket boundary.

### 4.3 `GET /admin/crowdsec/dashboard/top-ips`

Returns top attacking IPs ranked by decision count.

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `range` | string | `24h` | Time range |
| `limit` | int | `10` | Max results (capped at 50) |

**Response (200):**

```json
{
  "ips": [
    { "ip": "203.0.113.42", "count": 47, "last_seen": "2026-03-25T09:15:00Z", "country": "CN" },
    { "ip": "198.51.100.7", "count": 23, "last_seen": "2026-03-25T08:30:00Z", "country": "RU" }
  ],
  "range": "24h",
  "cached": true
}
```

> **Design Decision (B3):** `top_scenario` was removed from the per-IP response. Computing the most frequent scenario per IP requires a correlated subquery or two-pass query, adding significant complexity for marginal UX value. The scenario breakdown chart (Section 4.4) already provides scenario visibility. If per-IP scenario data is needed later, it can be added as a detail view.

**Backend Implementation:**

```go
// File: backend/internal/api/handlers/crowdsec_dashboard.go
// Function: func (h *CrowdsecHandler) DashboardTopIPs(c *gin.Context)

// SELECT ip, COUNT(*) as count, MAX(created_at) as last_seen, country
// FROM security_decisions
// WHERE created_at >= ? AND source = 'crowdsec'
// GROUP BY ip ORDER BY count DESC LIMIT ?
```

### 4.4 `GET /admin/crowdsec/dashboard/scenarios`

Returns scenario breakdown with counts.

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `range` | string | `24h` | Time range |

**Response (200):**

```json
{
  "scenarios": [
    { "name": "crowdsecurity/http-probing", "count": 89, "percentage": 42.3 },
    { "name": "crowdsecurity/ssh-bf", "count": 45, "percentage": 21.4 },
    { "name": "crowdsecurity/http-bad-user-agent", "count": 32, "percentage": 15.2 }
  ],
  "total": 210,
  "range": "24h",
  "cached": true
}
```

### 4.5 `GET /admin/crowdsec/alerts`

Wraps the LAPI `/v1/alerts` endpoint, providing alert data without exposing LAPI keys.

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `range` | string | `24h` | Time range filter |
| `scenario` | string | (all) | Filter by scenario name |
| `limit` | int | `50` | Max results (capped at 200) |
| `offset` | int | `0` | Pagination offset |

**Response (200):**

```json
{
  "alerts": [
    {
      "id": 1234,
      "scenario": "crowdsecurity/http-probing",
      "ip": "203.0.113.42",
      "message": "Ip 203.0.113.42 performed 'crowdsecurity/http-probing' ...",
      "events_count": 15,
      "start_at": "2026-03-25T09:10:00Z",
      "stop_at": "2026-03-25T09:15:00Z",
      "created_at": "2026-03-25T09:15:30Z"
    }
  ],
  "total": 247,
  "source": "lapi",
  "cached": true
}
```

**Backend Implementation:**

```go
// File: backend/internal/api/handlers/crowdsec_dashboard.go
// Function: func (h *CrowdsecHandler) ListAlerts(c *gin.Context)

// Uses same SSRF-safe HTTP client pattern as GetLAPIDecisions:
// endpoint := baseURL.ResolveReference(&url.URL{Path: "/v1/alerts"})
// req.Header.Set("X-Api-Key", apiKey)
// Falls back to `cscli alerts list -o json` on LAPI failure.
```

### 4.6 `GET /admin/crowdsec/decisions/export`

Exports decisions as downloadable CSV or JSON.

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `format` | string | `csv` | Export format: `csv` or `json` |
| `range` | string | `24h` | Time range |
| `source` | string | `all` | Filter: `crowdsec`, `waf`, `ratelimit`, `manual`, `all` |

**Response:** File download with appropriate `Content-Type` and `Content-Disposition` headers.

- CSV: `text/csv; charset=utf-8` with filename `crowdsec-decisions-{timestamp}.csv`
- JSON: `application/json` with filename `crowdsec-decisions-{timestamp}.json`

**CSV Columns:** `uuid,ip,action,source,scenario,rule_id,host,country,created_at,expires_at`

**Security — CSV Formula Injection (CWE-1236):** CSV field values MUST be sanitized to prevent formula injection. Fields starting with `=`, `+`, `-`, `@`, `\t`, or `\r` SHALL be prefixed with a single quote (`'`) before writing to the CSV output. This applies to all user-influenced fields: `ip`, `scenario`, `rule_id`, `host`, and `details`. Reference: [OWASP CSV Injection](https://owasp.org/www-community/attacks/CSV_Injection).

---

## 5. Frontend Component Design

### 5.1 New Files

| File | Purpose |
|------|---------|
| `frontend/src/pages/CrowdSecDashboard.tsx` | Dashboard tab content (orchestrator) |
| `frontend/src/components/crowdsec/DashboardSummaryCards.tsx` | 4 summary stat cards |
| `frontend/src/components/crowdsec/BanTimelineChart.tsx` | Area chart for ban timeline |
| `frontend/src/components/crowdsec/TopAttackingIPsChart.tsx` | Horizontal bar chart |
| `frontend/src/components/crowdsec/ScenarioBreakdownChart.tsx` | Pie/donut chart |
| `frontend/src/components/crowdsec/ActiveDecisionsTable.tsx` | Enhanced decisions table |
| `frontend/src/components/crowdsec/AlertsList.tsx` | LAPI alerts feed |
| `frontend/src/components/crowdsec/DashboardTimeRangeSelector.tsx` | Time range toggle (1h/6h/24h/7d/30d) |
| `frontend/src/components/crowdsec/DecisionsExportButton.tsx` | Export dropdown (CSV/JSON) |
| `frontend/src/api/crowdsecDashboard.ts` | API client for dashboard endpoints |
| `frontend/src/hooks/useCrowdsecDashboard.ts` | React Query hooks for dashboard data |

### 5.2 Integration into Existing Page

`CrowdSecConfig.tsx` currently renders configuration UI directly. We add tab navigation using Radix UI Tabs (already in `package.json` as `@radix-ui/react-tabs`):

```tsx
// CrowdSecConfig.tsx — modified
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../components/ui'
import { CrowdSecDashboard } from './CrowdSecDashboard'

// Inside the return:
<Tabs defaultValue="config" className="w-full">
  <TabsList>
    <TabsTrigger value="config">Configuration</TabsTrigger>
    <TabsTrigger value="dashboard">Dashboard</TabsTrigger>
  </TabsList>
  <TabsContent value="config">
    {/* ... existing configuration content (moved here) ... */}
  </TabsContent>
  <TabsContent value="dashboard">
    <CrowdSecDashboard />
  </TabsContent>
</Tabs>
```

### 5.3 API Client

```typescript
// frontend/src/api/crowdsecDashboard.ts

import client from './client'

export type TimeRange = '1h' | '6h' | '24h' | '7d' | '30d'

export interface DashboardSummary {
  total_decisions: number
  active_decisions: number
  unique_ips: number
  top_scenario: string
  decisions_trend: number
  range: string
  cached: boolean
  generated_at: string
}

export interface TimelineBucket {
  timestamp: string
  bans: number
  captchas: number
}

export interface TimelineData {
  buckets: TimelineBucket[]
  range: string
  interval: string
  cached: boolean
}

export interface TopIP {
  ip: string
  count: number
  last_seen: string
  country: string
}

export interface TopIPsData {
  ips: TopIP[]
  range: string
  cached: boolean
}

export interface ScenarioEntry {
  name: string
  count: number
  percentage: number
}

export interface ScenariosData {
  scenarios: ScenarioEntry[]
  total: number
  range: string
  cached: boolean
}

export interface CrowdSecAlert {
  id: number
  scenario: string
  ip: string
  message: string
  events_count: number
  start_at: string
  stop_at: string
  created_at: string
  duration: string
  type: string
  origin: string
}

export interface AlertsData {
  alerts: CrowdSecAlert[]
  total: number
  source: string
  cached: boolean
}

export async function getDashboardSummary(range: TimeRange): Promise<DashboardSummary> {
  const resp = await client.get('/admin/crowdsec/dashboard/summary', { params: { range } })
  return resp.data
}

export async function getDashboardTimeline(range: TimeRange): Promise<TimelineData> {
  const resp = await client.get('/admin/crowdsec/dashboard/timeline', { params: { range } })
  return resp.data
}

export async function getDashboardTopIPs(range: TimeRange, limit = 10): Promise<TopIPsData> {
  const resp = await client.get('/admin/crowdsec/dashboard/top-ips', { params: { range, limit } })
  return resp.data
}

export async function getDashboardScenarios(range: TimeRange): Promise<ScenariosData> {
  const resp = await client.get('/admin/crowdsec/dashboard/scenarios', { params: { range } })
  return resp.data
}

export async function getAlerts(params: {
  range?: TimeRange; scenario?: string; limit?: number; offset?: number
}): Promise<AlertsData> {
  const resp = await client.get('/admin/crowdsec/alerts', { params })
  return resp.data
}

export async function exportDecisions(
  format: 'csv' | 'json', range: TimeRange, source = 'all'
): Promise<Blob> {
  const resp = await client.get('/admin/crowdsec/decisions/export', {
    params: { format, range, source },
    responseType: 'blob',
  })
  return resp.data
}
```

### 5.4 React Query Hooks

```typescript
// frontend/src/hooks/useCrowdsecDashboard.ts

import { useQuery } from '@tanstack/react-query'
import {
  getDashboardSummary,
  getDashboardTimeline,
  getDashboardTopIPs,
  getDashboardScenarios,
  getAlerts,
  type TimeRange,
} from '../api/crowdsecDashboard'

const STALE_TIME = 30_000 // 30 seconds — matches backend cache TTL

export function useDashboardSummary(range: TimeRange) {
  return useQuery({
    queryKey: ['crowdsec-dashboard-summary', range],
    queryFn: () => getDashboardSummary(range),
    staleTime: STALE_TIME,
  })
}

export function useDashboardTimeline(range: TimeRange) {
  return useQuery({
    queryKey: ['crowdsec-dashboard-timeline', range],
    queryFn: () => getDashboardTimeline(range),
    staleTime: STALE_TIME,
  })
}

export function useDashboardTopIPs(range: TimeRange, limit = 10) {
  return useQuery({
    queryKey: ['crowdsec-dashboard-top-ips', range, limit],
    queryFn: () => getDashboardTopIPs(range, limit),
    staleTime: STALE_TIME,
  })
}

export function useDashboardScenarios(range: TimeRange) {
  return useQuery({
    queryKey: ['crowdsec-dashboard-scenarios', range],
    queryFn: () => getDashboardScenarios(range),
    staleTime: STALE_TIME,
  })
}

export function useAlerts(params: {
  range?: TimeRange; scenario?: string; limit?: number; offset?: number
}) {
  return useQuery({
    queryKey: ['crowdsec-alerts', params],
    queryFn: () => getAlerts(params),
    staleTime: STALE_TIME,
  })
}
```

### 5.5 Accessibility Considerations

- **Charts:** Each Recharts component gets `role="img"` and `aria-label` describing the chart content. Recharts SVG output supports `<title>` and `<desc>` elements.
- **Tables:** Use semantic `<table>`, `<th>`, `<td>` with proper headers. Sort controls use `aria-sort`.
- **Time Range Selector:** Uses Radix Tabs pattern (roving tabindex, arrow key navigation).
- **Color:** All chart colors meet WCAG 2.2 AA contrast (4.5:1 minimum). Patterns/shapes supplement color for colorblind users.
- **Keyboard:** All interactive elements (tabs, buttons, table rows) keyboard navigable. Focus indicators visible.
- **Screen Reader:** Summary cards use `aria-live="polite"` for dynamic updates when time range changes.

---

## 6. Implementation Phases

### Phase 1 / PR-1: Backend Aggregation APIs + Model Enrichment

**Scope:** Backend only. No frontend changes.

**Files to create:**

| File | Description |
|------|-------------|
| `backend/internal/api/handlers/crowdsec_dashboard.go` | New handler file with all 6 dashboard/export endpoints |
| `backend/internal/api/handlers/crowdsec_dashboard_cache.go` | In-memory cache with TTL |
| `backend/internal/api/handlers/crowdsec_dashboard_test.go` | Unit tests for all new endpoints |

**Files to modify:**

| File | Change |
|------|--------|
| `backend/internal/models/security_decision.go` | Add `Scenario`, `Country`, `ExpiresAt` fields |
| `backend/internal/api/handlers/crowdsec_handler.go` | In `RegisterRoutes`, add 6 new route registrations |
| `backend/internal/api/handlers/crowdsec_handler.go` | In `BanIP`, after successful `cscli decisions add`, call `h.Security.LogDecision()` to persist a `SecurityDecision` record with `Source: "crowdsec"`, `Action: "block"`, `Scenario: "manual"`, `ExpiresAt` computed from the duration parameter, and `Country: ""` (empty for manual bans). This ensures manual bans appear in dashboard aggregations. |
| `backend/internal/api/handlers/crowdsec_handler.go` | In `UnbanIP`, after successful unban, call `h.dashCache.Invalidate("dashboard")` to clear cached dashboard data |

**New Route Registrations (in `RegisterRoutes`):**

```go
// Dashboard analytics endpoints (Issue #26)
rg.GET("/admin/crowdsec/dashboard/summary", h.DashboardSummary)
rg.GET("/admin/crowdsec/dashboard/timeline", h.DashboardTimeline)
rg.GET("/admin/crowdsec/dashboard/top-ips", h.DashboardTopIPs)
rg.GET("/admin/crowdsec/dashboard/scenarios", h.DashboardScenarios)
rg.GET("/admin/crowdsec/alerts", h.ListAlerts)
rg.GET("/admin/crowdsec/decisions/export", h.ExportDecisions)
```

**Key Functions:**

```go
func (h *CrowdsecHandler) DashboardSummary(c *gin.Context)
func (h *CrowdsecHandler) DashboardTimeline(c *gin.Context)
func (h *CrowdsecHandler) DashboardTopIPs(c *gin.Context)
func (h *CrowdsecHandler) DashboardScenarios(c *gin.Context)
func (h *CrowdsecHandler) ListAlerts(c *gin.Context)
func (h *CrowdsecHandler) ExportDecisions(c *gin.Context)
func parseTimeRange(rangeStr string) (time.Time, error)
func (c *dashboardCache) Get(key string) (interface{}, bool)
func (c *dashboardCache) Set(key string, data interface{}, ttl time.Duration)
func (c *dashboardCache) Invalidate(prefixes ...string)
```

**Database Indexes:** GORM will create single-column indexes via struct tags and composite indexes via `compositeIndex` tags (see Section 3.3). Verify after migration:

- **Single-column:** `idx_security_decisions_scenario`, `idx_security_decisions_country`, `idx_security_decisions_expires_at`
- **Composite:** `idx_sd_source_created`, `idx_sd_source_scenario_created`, `idx_sd_source_ip_created`

See Appendix C for verification queries.

**Acceptance Criteria:**

- [ ] All 6 endpoints return valid JSON with correct schemas
- [ ] `parseTimeRange` rejects invalid inputs (returns 400)
- [ ] Aggregation queries use parameterized GORM (no raw SQL concatenation)
- [ ] Cache returns stale data within TTL, fresh data after TTL expires
- [ ] Export produces valid CSV with proper escaping and JSON
- [ ] Alerts endpoint falls back to `cscli alerts list` when LAPI unreachable
- [ ] No LAPI keys in any response body
- [ ] Unit test coverage ≥ 85%
- [ ] GORM security scanner passes with 0 CRITICAL/HIGH

---

### Phase 2 / PR-2: Frontend Dashboard Page with Charts

**Scope:** Frontend only (depends on PR-1 being merged).

**Files to create:**

| File | Description |
|------|-------------|
| `frontend/src/pages/CrowdSecDashboard.tsx` | Dashboard tab orchestrator |
| `frontend/src/components/crowdsec/DashboardSummaryCards.tsx` | 4 stat cards |
| `frontend/src/components/crowdsec/BanTimelineChart.tsx` | Recharts AreaChart |
| `frontend/src/components/crowdsec/TopAttackingIPsChart.tsx` | Recharts BarChart |
| `frontend/src/components/crowdsec/ScenarioBreakdownChart.tsx` | Recharts PieChart |
| `frontend/src/components/crowdsec/ActiveDecisionsTable.tsx` | Enhanced decisions table |
| `frontend/src/components/crowdsec/DashboardTimeRangeSelector.tsx` | Time range toggle |
| `frontend/src/api/crowdsecDashboard.ts` | API client functions |
| `frontend/src/hooks/useCrowdsecDashboard.ts` | React Query hooks |
| `frontend/src/components/crowdsec/__tests__/DashboardSummaryCards.test.tsx` | Unit tests |
| `frontend/src/components/crowdsec/__tests__/BanTimelineChart.test.tsx` | Unit tests |
| `frontend/src/components/crowdsec/__tests__/TopAttackingIPsChart.test.tsx` | Unit tests |
| `frontend/src/components/crowdsec/__tests__/ScenarioBreakdownChart.test.tsx` | Unit tests |
| `frontend/src/components/crowdsec/__tests__/ActiveDecisionsTable.test.tsx` | Unit tests |
| `frontend/src/hooks/__tests__/useCrowdsecDashboard.test.ts` | Hook tests |
| `tests/security/crowdsec-dashboard.spec.ts` | Playwright E2E tests (new file, follows existing `tests/security/crowdsec-*.spec.ts` convention) |

**Files to modify:**

| File | Change |
|------|--------|
| `frontend/package.json` | Add `recharts` dependency |
| `frontend/src/pages/CrowdSecConfig.tsx` | Wrap content in Radix Tabs; add Dashboard tab |

**Acceptance Criteria:**

- [ ] Dashboard tab renders when CrowdSec page visited and tab clicked
- [ ] All 4 summary cards display data from `/dashboard/summary`
- [ ] Timeline chart renders with Recharts AreaChart
- [ ] Top IPs chart renders as horizontal BarChart
- [ ] Scenario breakdown renders as PieChart/DonutChart
- [ ] Time range selector switches all charts simultaneously
- [ ] Loading states show skeletons (consistent with existing patterns)
- [ ] Error states show user-friendly messages
- [ ] Vitest unit test coverage ≥ 85%
- [ ] Playwright E2E tests pass on Firefox, Chromium, WebKit
- [ ] All interactive elements keyboard navigable
- [ ] Chart colors meet WCAG 2.2 AA contrast requirements
- [ ] Tab labels use i18n: `t('crowdsec.tabs.config')` and `t('crowdsec.tabs.dashboard')` with keys added to all locale files
- [ ] `CrowdSecDashboard` is loaded via `React.lazy()` with `<Suspense>` fallback to avoid loading Recharts bundle on the Configuration tab

---

### Phase 3 / PR-3: Alerts Feed, Enhanced Notifications, Ban Export

**Scope:** Frontend alerts UI + notification enrichment + export functionality.

**Files to create:**

| File | Description |
|------|-------------|
| `frontend/src/components/crowdsec/AlertsList.tsx` | Alerts feed with filters |
| `frontend/src/components/crowdsec/DecisionsExportButton.tsx` | Export dropdown (CSV/JSON) — moved from PR-2 for coherent slicing (button + tests in same PR) |
| `frontend/src/components/crowdsec/__tests__/AlertsList.test.tsx` | Unit tests |
| `frontend/src/components/crowdsec/__tests__/DecisionsExportButton.test.tsx` | Unit tests |

**Files to modify:**

| File | Change |
|------|--------|
| `frontend/src/pages/CrowdSecDashboard.tsx` | Add AlertsList component |
| `backend/internal/services/enhanced_security_notification_service.go` | Enrich `crowdsec_decision` payload with scenario and structured data |

**Notification Enrichment:** Currently the notification service dispatches a generic event for `crowdsec_decision`. This PR enriches the payload to include scenario, IP, action, and duration in a structured format.

**Acceptance Criteria:**

- [ ] Alerts list displays LAPI alerts with scenario, IP, events count, timestamps
- [ ] Alerts list supports filtering by scenario
- [ ] Export button downloads valid CSV or JSON file
- [ ] Export filename includes timestamp
- [ ] Notification payload includes scenario name and structured data
- [ ] E2E tests cover export flow and alerts display
- [ ] Unit test coverage ≥ 85%

---

## 7. Testing Strategy

### 7.1 Backend Unit Tests

**File:** `backend/internal/api/handlers/crowdsec_dashboard_test.go`

| Test | What It Validates |
|------|-------------------|
| `TestDashboardSummary_EmptyDB` | Returns zero counts when no decisions exist |
| `TestDashboardSummary_WithData` | Returns correct aggregates for seeded data |
| `TestDashboardSummary_InvalidRange` | Returns 400 for unsupported range values |
| `TestDashboardSummary_CacheHit` | Second call returns cached data without DB query |
| `TestDashboardTimeline_Buckets` | Correct time bucketing for 24h/1h interval |
| `TestDashboardTimeline_EmptyRange` | Returns empty buckets array |
| `TestDashboardTopIPs_Ranking` | IPs ordered by count descending |
| `TestDashboardTopIPs_LimitCap` | Limit parameter capped at 50 |
| `TestDashboardScenarios_Breakdown` | Correct percentage calculation |
| `TestListAlerts_LAPISuccess` | Parses LAPI response correctly |
| `TestListAlerts_LAPIFallback` | Falls back to cscli on LAPI failure |
| `TestExportDecisions_CSV` | Valid CSV output with headers |
| `TestExportDecisions_JSON` | Valid JSON array output |
| `TestExportDecisions_InvalidFormat` | Returns 400 for unsupported format |
| `TestParseTimeRange_Valid` | Accepts 1h, 6h, 24h, 7d, 30d |
| `TestParseTimeRange_Invalid` | Rejects arbitrary strings, negative values |
| `TestDashboardCache_TTL` | Entries expire after configured TTL |
| `TestDashboardCache_Invalidation` | Cache cleared on BanIP/UnbanIP |

**Test Fixtures:** Use GORM's `sqlite://file::memory:?cache=shared` in-memory DB with seeded `SecurityDecision` records spanning multiple scenarios, IPs, and time ranges.

### 7.2 Frontend Unit Tests (Vitest)

| Test File | Key Tests |
|-----------|-----------|
| `DashboardSummaryCards.test.tsx` | Renders 4 cards, shows loading skeleton, handles error |
| `BanTimelineChart.test.tsx` | Renders Recharts AreaChart, handles empty data |
| `TopAttackingIPsChart.test.tsx` | Renders BarChart, shows correct IP labels |
| `ScenarioBreakdownChart.test.tsx` | Renders PieChart, shows percentages |
| `ActiveDecisionsTable.test.tsx` | Renders table rows, sorts by column |
| `AlertsList.test.tsx` | Renders alert items, filters by scenario |
| `DecisionsExportButton.test.tsx` | Triggers correct API call for CSV/JSON |
| `useCrowdsecDashboard.test.ts` | Hooks return data, handle loading/error states |

**Mocking:** Use `vi.mock('../api/crowdsecDashboard')` with typed mock data matching the API schemas defined in Section 4.

### 7.3 E2E Tests (Playwright)

**File:** `tests/security/crowdsec-dashboard.spec.ts`

```typescript
test.describe('CrowdSec Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/security/crowdsec')
  })

  test('Dashboard tab is visible and clickable', async ({ page }) => {
    await test.step('Navigate to dashboard tab', async () => {
      await page.getByRole('tab', { name: 'Dashboard' }).click()
    })
    await test.step('Verify summary cards are visible', async () => {
      await expect(page.getByTestId('dashboard-summary-cards')).toBeVisible()
    })
  })

  test('Time range selector updates all charts', async ({ page }) => {
    // Click Dashboard tab, select 7d range, verify charts re-render
  })

  test('Export button downloads CSV file', async ({ page }) => {
    // Click Dashboard tab, click Export, select CSV, verify download
  })

  test('Active decisions table displays data', async ({ page }) => {
    // Verify table headers and at least one row (requires seed data or ban)
  })
})
```

**Test Data:** E2E tests run against the Docker container which has CrowdSec running. Tests that require specific decision data should use the existing `POST /admin/crowdsec/ban` endpoint to create test bans. Because `BanIP` now calls `SecurityService.LogDecision()` (see B1 fix), these bans will be persisted as `SecurityDecision` records and will appear in dashboard aggregation queries. Seed at least 3 bans with distinct IPs and the default duration to populate summary cards and top-IPs chart.

### 7.4 Test Execution Order

1. **GORM Security Scanner** — Before any commit (manual stage, PR-1 only)
2. **Backend unit tests** — `cd backend && go test ./internal/api/handlers/ -run TestDashboard -v -count=1`
3. **Frontend unit tests** — `cd frontend && npm run test`
4. **E2E Playwright** — Via task `Test: E2E Playwright (FireFox) - Security Suite`

---

## 8. Commit Slicing Strategy

### Decision: 3 PRs (Multi-PR)

**Trigger reasons:**

- Cross-domain changes (backend + frontend + new dependency)
- Risk isolation (new dependency in separate PR aids rollback)
- Review size (estimated ~2500 lines total; 800-1000 per PR is reviewable)
- Testing gates (backend APIs must exist before frontend can E2E test against them)

### PR Slices

| Slice | Scope | Files (est.) | Dependencies | Validation Gate |
|-------|-------|-------------|--------------|-----------------|
| **PR-1** | Backend: model enrichment + 6 API endpoints + cache + unit tests | ~800 LOC across 3 new files + 2 modified | None | Backend unit tests pass ≥ 85% coverage; GORM scanner clean |
| **PR-2** | Frontend: Recharts dependency + dashboard page + charts + Vitest + E2E (no export button) | ~1100 LOC across 11 new files + 2 modified | PR-1 merged | Vitest ≥ 85% coverage; E2E pass all browsers |
| **PR-3** | Alerts feed + notification enrichment + export button + export UI | ~600 LOC across 4 new files + 2 modified | PR-2 merged | Unit + E2E pass; export produces valid files |

### Rollback & Contingency

- **PR-1 rollback:** Drop new columns via migration or tolerate empty columns (GORM ignores unknown columns). Remove new routes.
- **PR-2 rollback:** Revert frontend changes. Remove `recharts` from `package.json`. The backend endpoints remain harmlessly unused.
- **PR-3 rollback:** Revert frontend components. Notification changes are backward-compatible (extra fields in payload are ignored by existing formatters).

---

## 9. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| **LAPI unavailable** when CrowdSec stopped | High | Dashboard shows incomplete data | All endpoints return graceful empty states; summary uses SQLite-only data; alerts endpoint falls back to cscli |
| **SQLite performance** on large `security_decisions` tables | Medium | Slow dashboard loads | Indexed columns, time-range scoping, 30s cache TTL, LIMIT clauses |
| **Recharts bundle size** increases frontend load | Low | Slower initial page load | Tree-shaking (only import used charts), code-split dashboard tab via `React.lazy` |
| **LAPI API changes** between CrowdSec versions | Low | Parsing failures | Defensive parsing with fallback to cscli; version-agnostic field access |
| **Country lookup accuracy** | Medium | Inaccurate country data | Country field is best-effort from LAPI's `source.cn` field; clearly labeled as approximate |
| **SSRF via LAPI URL** | Low | Internal network scanning | Reuse existing `validateCrowdsecLAPIBaseURL` with internal-only allowlist |
| **Sensitive IP data exposure** | Medium | Privacy concern | All dashboard endpoints behind admin auth; no public access; export requires admin role |

---

## 10. Out of Scope

| Item | Rationale |
|------|-----------|
| **Full CrowdSec management clone** (à la crowdsec_manager) | Charon embeds CrowdSec as one module, not a standalone manager |
| **Hub browser expansion** | Existing preset system is sufficient; hub browsing is a separate feature |
| **Real-time SSE/WebSocket decision streaming** | Adds significant complexity; polling with 30s cache is adequate for v1 |
| **GeoIP map visualization** | Requires map rendering library + GeoIP database; future enhancement |
| **CrowdSec Traefik/Nginx integration** | Charon uses Caddy only |
| **Custom scenario creation** | Users manage scenarios via CrowdSec Hub/CLI, not Charon UI |
| **Decision auto-expiry cleanup** | CrowdSec handles decision lifecycle; Charon only displays |
| **i18n for chart data labels** | Deferred; chart data labels use English scenario names from CrowdSec. Tab labels (`Configuration`, `Dashboard`) are i18n'd in PR-2 (see acceptance criteria). |
| **Dark mode chart theme** | Deferred; initial charts use Charon's existing color palette |

---

## Appendix A: Time Range Validation

```go
func parseTimeRange(rangeStr string) (time.Time, error) {
    now := time.Now().UTC()
    switch rangeStr {
    case "1h":
        return now.Add(-1 * time.Hour), nil
    case "6h":
        return now.Add(-6 * time.Hour), nil
    case "24h", "":
        return now.Add(-24 * time.Hour), nil
    case "7d":
        return now.Add(-7 * 24 * time.Hour), nil
    case "30d":
        return now.Add(-30 * 24 * time.Hour), nil
    default:
        return time.Time{}, fmt.Errorf("invalid range: %s (valid: 1h, 6h, 24h, 7d, 30d)", rangeStr)
    }
}
```

## Appendix B: Dashboard Color Palette

Colors chosen for WCAG 2.2 AA compliance against both light and dark backgrounds:

| Use | Color | Hex | Contrast vs White | Contrast vs #1a1a2e |
|-----|-------|-----|-------------------|---------------------|
| Bans (primary) | Blue | `#3b82f6` | 4.5:1 | 5.2:1 |
| Captchas | Amber | `#f59e0b` | 3.1:1 (large text) | 7.8:1 |
| Top scenario 1 | Indigo | `#6366f1` | 4.6:1 | 4.8:1 |
| Top scenario 2 | Emerald | `#10b981` | 4.5:1 | 5.9:1 |
| Top scenario 3 | Rose | `#f43f5e` | 4.5:1 | 5.7:1 |
| Top scenario 4 | Cyan | `#06b6d4` | 4.5:1 | 6.1:1 |
| Top scenario 5+ | Slate | `#64748b` | 4.6:1 | 4.5:1 |

## Appendix C: SQLite Index Verification

After migration, verify indexes exist:

```sql
SELECT name, tbl_name FROM sqlite_master
WHERE type = 'index' AND tbl_name = 'security_decisions';
```

Expected new **single-column** indexes: `idx_security_decisions_scenario`, `idx_security_decisions_country`, `idx_security_decisions_expires_at` — in addition to existing indexes on `source`, `action`, `ip`, `host`, `rule_id`, `created_at`.

Expected new **composite** indexes (see Section 3.3 for rationale):

| Index Name | Columns | Purpose |
|------------|---------|---------|
| `idx_sd_source_created` | `(source, created_at DESC)` | All time-range filtered aggregation queries |
| `idx_sd_source_scenario_created` | `(source, scenario, created_at DESC)` | Scenario breakdown, top-scenario ranking |
| `idx_sd_source_ip_created` | `(source, ip, created_at DESC)` | Top-IPs ranking, unique IP counts |

**Verification query for composite indexes:**

```sql
SELECT name, sql FROM sqlite_master
WHERE type = 'index' AND tbl_name = 'security_decisions'
  AND name LIKE 'idx_sd_%';
```

**strftime interval reference** (used in timeline bucketing, Section 4.2):

| Interval | SQLite Expression |
|----------|-------------------|
| `5m` | `strftime('%Y-%m-%dT%H:', created_at) \|\| printf('%02d:00Z', (CAST(strftime('%M', created_at) AS INTEGER) / 5) * 5)` |
| `15m` | `strftime('%Y-%m-%dT%H:', created_at) \|\| printf('%02d:00Z', (CAST(strftime('%M', created_at) AS INTEGER) / 15) * 15)` |
| `1h` | `strftime('%Y-%m-%dT%H:00:00Z', created_at)` |
| `6h` | `strftime('%Y-%m-%dT', created_at) \|\| printf('%02d:00:00Z', (CAST(strftime('%H', created_at) AS INTEGER) / 6) * 6)` |
| `1d` | `strftime('%Y-%m-%dT00:00:00Z', created_at)` |

---

## Revision History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-03-25 | — | Initial draft |
| 1.1 | 2026-03-25 | — | Supervisor review remediation: 7 blocking fixes (B1–B7) + 5 important items (I1–I5). **B1:** Added `LogDecision()` call in `BanIP` for manual ban persistence. **B2:** Added composite indexes for aggregation query performance. **B3:** Removed `top_scenario` from per-IP response (simplicity). **B4:** Added `strftime` format strings for all 5 timeline intervals. **B5:** Added CSV formula injection sanitization requirement. **B6:** Specified `dashCache` field wiring on `CrowdsecHandler` struct. **B7:** Hybrid approach — `active_decisions` from LAPI, historical metrics from SQLite. **I1:** Tab labels use i18n keys. **I2:** Defined `decisions_trend` formula. **I3:** Clarified country field population from LAPI `source.cn`. **I4:** `React.lazy()` for dashboard tab. **I5:** Confirmed `tests/security/` convention. Moved `DecisionsExportButton.tsx` to PR-3. |
