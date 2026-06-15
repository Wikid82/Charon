# Technical Specification: Enhanced Dashboard with Statistics (Issue #25)

**Version:** 1.1
**Date:** 2026-06-14
**Branch:** `feature/stats`
**Status:** Approved (Conditions Resolved — see Supervisor Review section)

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Architecture Overview](#2-architecture-overview)
3. [Database Schema](#3-database-schema)
4. [Backend API Contracts](#4-backend-api-contracts)
5. [WebSocket Design](#5-websocket-design)
6. [Certificate Expiry](#6-certificate-expiry)
7. [Frontend Component Design](#7-frontend-component-design)
8. [Log Capture Strategy](#8-log-capture-strategy)
9. [Performance Considerations](#9-performance-considerations)
10. [Testing Strategy](#10-testing-strategy)
11. [Commit Slicing Strategy](#11-commit-slicing-strategy)
12. [Definition of Done Checklist](#12-definition-of-done-checklist)
13. [Risk Register](#13-risk-register)
14. [Supervisor Review](#14-supervisor-review)
15. [Files to Create / Modify](#15-files-to-create--modify)

---

## 1. Executive Summary

Issue #25 upgrades the Charon dashboard from a static entity-count panel into a data-rich observability surface. The goal is to give administrators an at-a-glance view of traffic patterns, host health, certificate expiry warnings, and real-time request metrics — all without requiring any external monitoring stack.

Key deliverables:

- A `RequestLog` SQLite table that persists every request proxied through Caddy with host, status code, bytes, method, and response time
- Seven REST endpoints under `/api/v1/stats/` providing summary, time-bucketed counts, top-host ranking, status-code distribution, traffic volume, certificate expiry warnings, and service health
- A WebSocket channel (`/api/v1/stats/live`) that pushes incremental metric updates to every connected dashboard tab
- A rebuilt `Dashboard.tsx` that uses Recharts (already in `package.json` as `recharts@^3.8.1`) to render bar, line, and pie charts in a responsive Tailwind grid
- React Query for polling fallback + WebSocket overlay for real-time updates

The feature is architecturally contained: it introduces one new model, one new service, one new handler file, and two new frontend files. It does not change any existing proxy, certificate, or Caddy configuration logic.

---

## 2. Architecture Overview

### 2.1 Data Flow

```
Caddy Access Log (/var/log/caddy/access.log)
        |  (existing LogWatcher tails this file)
        v
services.LogWatcher.ParseLogEntry()
        |  (already parses CaddyAccessLog -> SecurityLogEntry)
        |  NEW: also fan-out to StatsIngester
        v
services.StatsIngester (NEW)
        |  in-memory ring buffer, batch flush every 5 s
        v
models.RequestLog table (SQLite, WAL)
        |
        +-> services.StatsService.GetSummary()
        +-> services.StatsService.GetRequestCounts()
        +-> services.StatsService.GetTopHosts()
        +-> services.StatsService.GetStatusDistribution()
        +-> services.StatsService.GetTrafficVolume()
        +-> handlers.StatsWSHub (NEW, goroutine)
                |  pushes StatsPush JSON every 10 s
                v
        WS /api/v1/stats/live -> frontend
```

### 2.2 Packages Touched

| Package | Change Type | Reason |
|---|---|---|
| `backend/internal/models/` | CREATE `request_log.go` | New persistence model |
| `backend/internal/services/` | CREATE `stats_service.go`, `stats_ingester.go`, `stats_types.go` | Business logic + shared WS types |
| `backend/internal/api/handlers/` | CREATE `stats_handler.go` | HTTP + WS handlers |
| `backend/internal/api/routes/routes.go` | MODIFY | Register AutoMigrate + routes |
| `backend/internal/services/log_watcher.go` | MODIFY | Fan-out to `StatsIngester` |
| `frontend/src/api/stats.ts` | CREATE | Typed API client |
| `frontend/src/hooks/useStats.ts` | CREATE | React Query hooks |
| `frontend/src/hooks/useStatsWebSocket.ts` | CREATE | WebSocket state hook |
| `frontend/src/components/stats/` | CREATE (6 files) | Chart widgets |
| `frontend/src/pages/Dashboard.tsx` | MODIFY | Integrate new widgets |
| `ARCHITECTURE.md` | MODIFY | Document new stats subsystem |
| `docs/features.md` | MODIFY | Mention dashboard stats |

### 2.3 No New External Dependencies

- Chart library: `recharts@^3.8.1` — already installed
- WebSocket: `gorilla/websocket` — already used by `logs_ws.go`
- Log parsing: existing `LogWatcher` + `CaddyAccessLog` + `SecurityLogEntry` models are reused

---

## 3. Database Schema

### 3.1 New Model: `RequestLog`

**File:** `backend/internal/models/request_log.go`

```go
package models

import "time"

// RequestLog persists a single HTTP request proxied through Caddy.
// Written by StatsIngester in batches; queried by StatsService for dashboard aggregations.
type RequestLog struct {
    ID             uint      `json:"-"                gorm:"primaryKey"`
    Timestamp      time.Time `json:"timestamp"        gorm:"not null;index:idx_request_log_ts"`
    Host           string    `json:"host"             gorm:"not null;index:idx_request_log_host;size:253"`
    Method         string    `json:"method"           gorm:"not null;size:16"`
    StatusCode     int       `json:"status_code"      gorm:"not null;index:idx_request_log_status"`
    BytesSent      int64     `json:"bytes_sent"       gorm:"not null;default:0"`
    ResponseTimeMS float64   `json:"response_time_ms" gorm:"not null;default:0"`
    ClientIP       string    `json:"client_ip"        gorm:"size:45"`
    Blocked        bool      `json:"blocked"          gorm:"default:false"`
}
```

**Privacy note (M3 — GDPR / enterprise security):** `ClientIP` is stored as a SHA-256 hash (first 16 bytes, hex-encoded, 32 characters) by default rather than as the raw IP address. This makes the value non-personally-identifiable while still enabling abuse-pattern detection across sessions. Raw IP storage is opt-in via the `Setting` key `stats.store_raw_client_ip` (default: `"false"`). The `StatsIngester.Ingest()` method applies the hash transformation before enqueuing the entry unless that setting is `"true"`. The `size:45` tag on `ClientIP` is retained to accommodate both the 32-character hash and raw IPv6 addresses for operators who opt in.

The retention policy is controlled by the `Setting` key `stats.retention_days` (default: `"30"`).

**Index strategy:**

| Index Name | Columns | Purpose |
|---|---|---|
| `idx_request_log_ts` | `timestamp` | All time-range WHERE clauses |
| `idx_request_log_host` | `host` | Host GROUP BY and top-N queries |
| `idx_request_log_status` | `status_code` | Status distribution aggregation |
| `idx_request_log_ts_host` | `timestamp, host` | Compound: per-host time-range queries |

The compound index `idx_request_log_ts_host` is not expressible via simple GORM struct tags with two-column compound syntax in SQLite. It must be created via a post-AutoMigrate `db.Exec`:

```go
db.Exec("CREATE INDEX IF NOT EXISTS idx_request_log_ts_host ON request_logs (timestamp, host)")
```

This `db.Exec` call is added to `routes.go` immediately after the `AutoMigrate` call.

### 3.2 Storage Estimation

| Scenario | Req/day | Row size (est.) | Rows/day | GB/year |
|---|---|---|---|---|
| Light home use | 10,000 | ~200 bytes | 10,000 | 0.7 GB |
| Small team | 200,000 | ~200 bytes | 200,000 | 14 GB |
| Heavy use | 1,000,000 | ~200 bytes | 1,000,000 | 70 GB |

**Retention policy recommendation:** Default 30-day rolling retention enforced by a nightly cleanup goroutine in `StatsIngester`. Configurable via a `Setting` key `stats.retention_days` (default: `"30"`).

### 3.3 No Separate Rollup Table (Initial Version)

For the initial release, pre-aggregated rollup tables are not warranted. Query performance for 30 days x 200,000 req/day = 6 million rows is adequate with the compound index and SQLite WAL mode. A rollup table can be added in a follow-up if query times exceed 500 ms under production load.

### 3.4 AutoMigrate Registration

In `backend/internal/api/routes/routes.go`, add `&models.RequestLog{}` to the `db.AutoMigrate(...)` call immediately after `&models.OrthrusAgent{}`:

```go
&models.RequestLog{},   // Issue #25: Request statistics
```

Then immediately after the `AutoMigrate` block, add:

```go
if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_request_log_ts_host ON request_logs (timestamp, host)").Error; err != nil {
    logger.Log().WithError(err).Warn("Failed to create compound stats index")
}
```

---

## 4. Backend API Contracts

All stats endpoints live under the authenticated management group (`management` group, inside `protected.Group("/")`). They require a valid JWT session and management role.

### 4.1 `GET /api/v1/stats/summary`

**Handler:** `StatsHandler.GetSummary`
**Auth:** Required (management role)
**Description:** Returns top-level KPIs for the dashboard header row.

**Response:**
```json
{
  "total_requests_24h": 14823,
  "total_requests_7d": 98241,
  "active_hosts": 7,
  "blocked_requests_24h": 142,
  "avg_response_time_ms": 38.4,
  "certs_expiring_soon": 2,
  "certs_expired": 0
}
```

**Response Go type:** `StatsSummaryResponse` (defined in `stats_handler.go`)

### 4.2 `GET /api/v1/stats/requests`

**Handler:** `StatsHandler.GetRequests`
**Auth:** Required (management role)
**Query params:**

| Param | Type | Default | Description |
|---|---|---|---|
| `period` | string | `24h` | `24h`, `7d`, or `30d` |
| `bucket` | string | `1h` | Bucket size: `1h`, `6h`, `1d`. Validated against allowlist before passing to service. |
| `host` | string | (all) | Filter to single host. Validated: max 253 characters (matches `gorm:"size:253"` column constraint). Requests with `host` longer than 253 characters return HTTP 400. |

**Response:**
```json
{
  "period": "7d",
  "bucket": "1d",
  "buckets": [
    { "timestamp": "2026-06-08T00:00:00Z", "count": 12004, "blocked": 88 },
    { "timestamp": "2026-06-09T00:00:00Z", "count": 13441, "blocked": 101 }
  ]
}
```

**Response Go type:** `StatsRequestsResponse`

### 4.3 `GET /api/v1/stats/top-hosts`

**Handler:** `StatsHandler.GetTopHosts`
**Auth:** Required (management role)
**Query params:**

| Param | Type | Default | Description |
|---|---|---|---|
| `period` | string | `24h` | `24h`, `7d`, or `30d` |
| `limit` | int | `10` | Max results (1-50) |

**Response:**
```json
{
  "period": "24h",
  "hosts": [
    { "host": "app.example.com", "count": 5421, "bytes_sent": 182304512, "avg_response_ms": 42.1 },
    { "host": "api.example.com", "count": 3211, "bytes_sent": 98124800, "avg_response_ms": 28.7 }
  ]
}
```

**Response Go type:** `StatsTopHostsResponse`

### 4.4 `GET /api/v1/stats/status-distribution`

**Handler:** `StatsHandler.GetStatusDistribution`
**Auth:** Required (management role)
**Query params:**

| Param | Type | Default | Description |
|---|---|---|---|
| `period` | string | `24h` | `24h`, `7d`, or `30d` |
| `host` | string | (all) | Filter to single host. Validated: max 253 characters (matches `gorm:"size:253"` column constraint). Requests with `host` longer than 253 characters return HTTP 400. |

**Response:**
```json
{
  "period": "24h",
  "distribution": [
    { "class": "2xx", "count": 13200, "pct": 89.1 },
    { "class": "3xx", "count": 800,   "pct": 5.4  },
    { "class": "4xx", "count": 700,   "pct": 4.7  },
    { "class": "5xx", "count": 123,   "pct": 0.8  }
  ]
}
```

**Response Go type:** `StatsStatusDistributionResponse`

### 4.5 `GET /api/v1/stats/traffic-volume`

**Handler:** `StatsHandler.GetTrafficVolume`
**Auth:** Required (management role)
**Query params:**

| Param | Type | Default | Description |
|---|---|---|---|
| `period` | string | `24h` | `24h`, `7d`, or `30d` |
| `bucket` | string | `1h` | `1h`, `6h`, `1d` |

**Response:**
```json
{
  "period": "24h",
  "bucket": "1h",
  "buckets": [
    { "timestamp": "2026-06-14T00:00:00Z", "bytes_sent": 14829304, "request_count": 823 },
    { "timestamp": "2026-06-14T01:00:00Z", "bytes_sent": 9182731,  "request_count": 511 }
  ]
}
```

**Response Go type:** `StatsTrafficVolumeResponse`

### 4.6 `GET /api/v1/stats/cert-expiry`

**Handler:** `StatsHandler.GetCertExpiry`
**Auth:** Required (management role)
**Query params:**

| Param | Type | Default | Description |
|---|---|---|---|
| `within_days` | int | `30` | Return certs expiring within N days. Valid range: **1–365**. Values outside this range return HTTP 400. |

**Validation (C2):** The handler rejects requests where `within_days < 1` or `within_days > 365` with HTTP 400 and error body `{"error": "within_days must be between 1 and 365"}`. This prevents full table scans triggered by extreme values.

**Response:**
```json
{
  "expiring": [
    {
      "uuid": "abc-123",
      "name": "app.example.com",
      "domains": "app.example.com,*.example.com",
      "expires_at": "2026-07-01T00:00:00Z",
      "days_remaining": 17,
      "provider": "letsencrypt"
    }
  ],
  "expired": []
}
```

**Notes:** Queries the existing `SSLCertificate` model. The `ExpiresAt *time.Time` field already exists. No new DB columns needed.

**Response Go type:** `StatsCertExpiryResponse`

### 4.7 `GET /api/v1/stats/health`

**Handler:** `StatsHandler.GetServiceHealth`
**Auth:** Required (management role)

**Response:**
```json
{
  "caddy": { "status": "ok", "message": "" },
  "database": { "status": "ok", "message": "" },
  "log_ingestion": { "status": "ok", "message": "", "last_entry_at": "2026-06-14T12:34:56Z" },
  "stats_ingester": { "status": "ok", "queue_depth": 0, "dropped_count": 0 }
}
```

**Response Go type:** `StatsHealthResponse`

### 4.8 `WS /api/v1/stats/live`

**Handler:** `StatsHandler.LiveWebSocket`
**Auth:** JWT validated before WebSocket upgrade (same middleware chain as all management routes)
**Protocol:** WebSocket, JSON messages
**Description:** Server pushes `StatsPushMessage` every 10 seconds.

---

## 5. WebSocket Design

### 5.1 Message Schema

All server-to-client messages follow this envelope:

```json
{
  "type": "stats_update",
  "ts": "2026-06-14T12:34:56Z",
  "data": {
    "requests_last_minute": 47,
    "blocked_last_minute": 2,
    "active_hosts": 7,
    "top_host_now": "app.example.com",
    "avg_response_ms": 38.4,
    "status_counts_last_minute": {
      "2xx": 44,
      "3xx": 1,
      "4xx": 2,
      "5xx": 0
    }
  }
}
```

### 5.2 Broadcast Mechanism

A `StatsWSHub` struct (in `backend/internal/api/handlers/stats_handler.go`) uses the hub pattern already established by the existing WebSocket handlers:

```
StatsWSHub
|-- clients    map[string]*StatsWSClient
|-- register   chan *StatsWSClient
|-- unregister chan string
+-- broadcast  chan StatsPushMessage

StatsIngester.flushLoop() -> StatsWSHub.Broadcast() channel
StatsWSHub.Run() goroutine -> fans out to all registered clients (non-blocking per client)
```

Each `StatsWSClient` has a buffered `send` channel (capacity 10). Non-blocking send in the hub's broadcast loop drops the message for a slow client rather than blocking others — same pattern as `LogWatcher.broadcast`.

### 5.3 Pipeline: Log Event to Stats Push

```
LogWatcher.readLoop()
    -> ParseLogEntry(line) -> SecurityLogEntry
    -> (existing) broadcast to Cerberus log subscribers
    -> (NEW)      StatsIngester.Ingest(entry)     [non-blocking, drops if ch full]
                      -> buffered channel (capacity 1000)

StatsIngester.flushLoop() (goroutine, ticker 5 s)
    -> drain channel into []RequestLog slice
    -> db.CreateInBatches(rows, 500)
    -> update rolling 1-min atomic counters
    -> StatsWSHub.Broadcast(StatsPushMessage{...})
```

### 5.4 Heartbeat / Ping-Pong

Server sends `websocket.PingMessage` every 30 seconds (matching the pattern in `logs_ws.go`). Failure to receive a Pong within 60 seconds closes the connection. The frontend `useStatsWebSocket.ts` reconnects after 5 seconds on close.

### 5.5 Auth on WebSocket Upgrade

The `/api/v1/stats/live` route is registered inside the `management` group which already has `authMiddleware` applied. Gin middleware runs before `upgrader.Upgrade()` is called — the same pattern used by `/api/v1/cerberus/logs/ws` (see `cerberus_logs_ws.go`).

---

## 6. Certificate Expiry

### 6.1 Existing Model

`backend/internal/models/ssl_certificate.go` already has:

```go
ExpiresAt *time.Time `json:"expires_at,omitempty" gorm:"index"`
```

No new columns are needed on `SSLCertificate`.

### 6.2 Query in StatsService

```go
// GetCertExpiry in stats_service.go
func (s *StatsService) GetCertExpiry(ctx context.Context, withinDays int) (*StatsCertExpiryResponse, error) {
    now := time.Now()
    threshold := now.Add(time.Duration(withinDays) * 24 * time.Hour)

    var expiring []models.SSLCertificate
    if err := s.db.WithContext(ctx).
        Where("expires_at IS NOT NULL AND expires_at > ? AND expires_at <= ?", now, threshold).
        Order("expires_at ASC").
        Find(&expiring).Error; err != nil {
        return nil, fmt.Errorf("query expiring certs: %w", err)
    }

    var expired []models.SSLCertificate
    if err := s.db.WithContext(ctx).
        Where("expires_at IS NOT NULL AND expires_at <= ?", now).
        Order("expires_at DESC").
        Find(&expired).Error; err != nil {
        return nil, fmt.Errorf("query expired certs: %w", err)
    }
    // ... map to response DTO
}
```

### 6.3 Frontend Widget

`CertExpiryWidget` renders:
- Count badge on the dashboard summary: derived from `StatsSummaryResponse.certs_expiring_soon`
- Expandable detail list from `GET /api/v1/stats/cert-expiry`
- Color coding: > 30 days = green (no warning shown), 8-30 days = amber badge, <= 7 days = red badge

---

## 7. Frontend Component Design

### 7.1 Dashboard Page Restructure

**File:** `frontend/src/pages/Dashboard.tsx` (MODIFY)

The updated layout adds two new sections below the existing stats cards row:

```
[Existing Stats Cards Row: Proxy Hosts | Certs | Remote Servers | ACLs | System Status]

[NEW: StatsSummaryBanner — live KPI row: req/min, blocked/min, avg response time]

[PeriodSelector: 24h | 7d | 30d]

[Chart Grid 2-col on md+]
  [RequestTrendChart — line]   [StatusDistributionChart — pie]
  [TopHostsChart — horiz bar]  [TrafficVolumeChart — area]

[System Grid 2-col on md+]
  [CertExpiryWidget]           [ServiceHealthWidget]

[Existing UptimeWidget]
```

`PeriodSelector` state is held in `Dashboard.tsx` as `const [period, setPeriod] = useState<Period>('24h')` and passed as a prop to each chart component.

`useStatsWebSocket()` is called once in `Dashboard.tsx`; the returned `livePayload` is passed to `StatsSummaryBanner` for real-time updates.

### 7.2 New Components

All new components live under `frontend/src/components/stats/`.

| File | Component | Chart Type | Data Source |
|---|---|---|---|
| `StatsSummaryBanner.tsx` | `StatsSummaryBanner` | KPI row | `useStatsSummary()` + `livePayload` prop |
| `PeriodSelector.tsx` | `PeriodSelector` | Tab group | Props only |
| `RequestTrendChart.tsx` | `RequestTrendChart` | `LineChart` | `useStatsRequests(period)` |
| `StatusDistributionChart.tsx` | `StatusDistributionChart` | `PieChart` | `useStatsStatusDistribution(period)` |
| `TopHostsChart.tsx` | `TopHostsChart` | `BarChart` (layout="vertical") | `useStatsTopHosts(period)` |
| `TrafficVolumeChart.tsx` | `TrafficVolumeChart` | `AreaChart` | `useStatsTrafficVolume(period)` |
| `CertExpiryWidget.tsx` | `CertExpiryWidget` | List + badges | `useStatsCertExpiry()` |
| `ServiceHealthWidget.tsx` | `ServiceHealthWidget` | Status dots | `useStatsHealth()` |
| `index.ts` | Re-exports all above | — | — |

### 7.3 Component Props Interfaces

```typescript
// frontend/src/components/stats/PeriodSelector.tsx
export type Period = '24h' | '7d' | '30d'
interface PeriodSelectorProps {
  value: Period
  onChange: (period: Period) => void
}

// frontend/src/components/stats/StatsSummaryBanner.tsx
interface StatsSummaryBannerProps {
  livePayload?: StatsPushPayload | null
}

// frontend/src/components/stats/RequestTrendChart.tsx
interface RequestTrendChartProps {
  period: Period
}

// frontend/src/components/stats/StatusDistributionChart.tsx
interface StatusDistributionChartProps {
  period: Period
  host?: string
}

// frontend/src/components/stats/TopHostsChart.tsx
interface TopHostsChartProps {
  period: Period
  limit?: number  // default 10
}

// frontend/src/components/stats/TrafficVolumeChart.tsx
interface TrafficVolumeChartProps {
  period: Period
}

// frontend/src/components/stats/CertExpiryWidget.tsx
interface CertExpiryWidgetProps {
  withinDays?: number  // default 30
}

// frontend/src/components/stats/ServiceHealthWidget.tsx
// No props — reads from useStatsHealth() internally
```

### 7.4 API Client

**File:** `frontend/src/api/stats.ts` (CREATE — full contents)

```typescript
import client from './client'

export type Period = '24h' | '7d' | '30d'
export type Bucket = '1h' | '6h' | '1d'

export interface StatsSummary {
  total_requests_24h: number
  total_requests_7d: number
  active_hosts: number
  blocked_requests_24h: number
  avg_response_time_ms: number
  certs_expiring_soon: number
  certs_expired: number
}

export interface RequestBucket {
  timestamp: string
  count: number
  blocked: number
}

export interface StatsRequestsResponse {
  period: Period
  bucket: Bucket
  buckets: RequestBucket[]
}

export interface TopHost {
  host: string
  count: number
  bytes_sent: number
  avg_response_ms: number
}

export interface StatsTopHostsResponse {
  period: Period
  hosts: TopHost[]
}

export interface StatusClass {
  class: string
  count: number
  pct: number
}

export interface StatsStatusDistributionResponse {
  period: Period
  distribution: StatusClass[]
}

export interface TrafficBucket {
  timestamp: string
  bytes_sent: number
  request_count: number
}

export interface StatsTrafficVolumeResponse {
  period: Period
  bucket: Bucket
  buckets: TrafficBucket[]
}

export interface CertExpiry {
  uuid: string
  name: string
  domains: string
  expires_at: string
  days_remaining: number
  provider: string
}

export interface StatsCertExpiryResponse {
  expiring: CertExpiry[]
  expired: CertExpiry[]
}

export interface ServiceStatus {
  status: 'ok' | 'degraded' | 'error'
  message: string
}

export interface StatsHealthResponse {
  caddy: ServiceStatus
  database: ServiceStatus
  log_ingestion: ServiceStatus & { last_entry_at?: string }
  stats_ingester: ServiceStatus & { queue_depth: number }
}

export const getStatsSummary = async (): Promise<StatsSummary> => {
  const res = await client.get<StatsSummary>('/stats/summary')
  return res.data
}

export const getStatsRequests = async (
  period: Period,
  bucket: Bucket = '1h',
  host?: string
): Promise<StatsRequestsResponse> => {
  const res = await client.get<StatsRequestsResponse>('/stats/requests', {
    params: { period, bucket, ...(host ? { host } : {}) },
  })
  return res.data
}

export const getStatsTopHosts = async (
  period: Period,
  limit = 10
): Promise<StatsTopHostsResponse> => {
  const res = await client.get<StatsTopHostsResponse>('/stats/top-hosts', {
    params: { period, limit },
  })
  return res.data
}

export const getStatsStatusDistribution = async (
  period: Period,
  host?: string
): Promise<StatsStatusDistributionResponse> => {
  const res = await client.get<StatsStatusDistributionResponse>('/stats/status-distribution', {
    params: { period, ...(host ? { host } : {}) },
  })
  return res.data
}

export const getStatsTrafficVolume = async (
  period: Period,
  bucket: Bucket = '1h'
): Promise<StatsTrafficVolumeResponse> => {
  const res = await client.get<StatsTrafficVolumeResponse>('/stats/traffic-volume', {
    params: { period, bucket },
  })
  return res.data
}

export const getStatsCertExpiry = async (
  withinDays = 30
): Promise<StatsCertExpiryResponse> => {
  const res = await client.get<StatsCertExpiryResponse>('/stats/cert-expiry', {
    params: { within_days: withinDays },
  })
  return res.data
}

export const getStatsHealth = async (): Promise<StatsHealthResponse> => {
  const res = await client.get<StatsHealthResponse>('/stats/health')
  return res.data
}
```

### 7.5 React Query Hooks

**File:** `frontend/src/hooks/useStats.ts` (CREATE)

Exports: `useStatsSummary`, `useStatsRequests`, `useStatsTopHosts`, `useStatsStatusDistribution`, `useStatsTrafficVolume`, `useStatsCertExpiry`, `useStatsHealth`, and the `STATS_QUERY_KEYS` constant object.

Key configuration for all stat hooks:
- `staleTime: 30_000` — data considered fresh for 30 s
- `refetchInterval: 60_000` — polling fallback every 60 s (used when WebSocket is not connected)
- Exception: `useStatsCertExpiry` uses `refetchInterval: 300_000` (5 min) since certs change rarely
- Exception: `useStatsHealth` uses `staleTime: 10_000`, `refetchInterval: 30_000`

**M4 — Polling suppression when WebSocket is active:** Hooks that receive real-time updates via WebSocket must not fire redundant polling. `useStatsSummary` accepts an optional `connected` boolean parameter: `useStatsSummary(connected?: boolean)`. When `connected` is `true`, the hook sets `refetchInterval: false` to suppress polling. `Dashboard.tsx` calls `useStatsWebSocket()` once at the top level, extracts the `connected` boolean, and passes it into `useStatsSummary(connected)`. Other hooks (`useStatsRequests`, `useStatsTopHosts`, etc.) do not receive live WS pushes directly and continue polling at their configured interval.

Pattern in `Dashboard.tsx`:
```typescript
const { livePayload, connected } = useStatsWebSocket()
const summary = useStatsSummary(connected)
// chart hooks always poll (no direct WS feed):
const requests = useStatsRequests(period)
```

**File:** `frontend/src/hooks/useStatsWebSocket.ts` (CREATE)

```typescript
export interface StatsPushPayload {
  requests_last_minute: number
  blocked_last_minute: number
  active_hosts: number
  top_host_now: string
  avg_response_ms: number
  status_counts_last_minute: {
    '2xx': number
    '3xx': number
    '4xx': number
    '5xx': number
  }
}

export function useStatsWebSocket(): {
  livePayload: StatsPushPayload | null
  connected: boolean
}
```

Behavior:
- Connects to `ws[s]://<host>/api/v1/stats/live` on mount
- On `stats_update` message: updates `livePayload` state and calls `queryClient.invalidateQueries({ queryKey: STATS_QUERY_KEYS.summary })`
- On close: schedules reconnect after 5 s
- Cleans up WebSocket on component unmount

### 7.6 Responsive Grid Layout

```
// Stats charts section
<div className="grid grid-cols-1 md:grid-cols-2 gap-6">
  <RequestTrendChart period={period} />
  <StatusDistributionChart period={period} />
  <TopHostsChart period={period} />
  <TrafficVolumeChart period={period} />
</div>

// System section
<div className="grid grid-cols-1 md:grid-cols-2 gap-6">
  <CertExpiryWidget />
  <ServiceHealthWidget />
</div>
```

Each chart card uses: `className="rounded-xl border border-border bg-surface-elevated p-6"`
Each chart uses `<ResponsiveContainer width="100%" height={220}>` from Recharts.

---

## 8. Log Capture Strategy

### 8.1 Decision: Option C — In-Memory Channel + Batch Writer

**Selected approach:** Tap into the existing `LogWatcher` tail loop, fan-out parsed `SecurityLogEntry` structs to a new `StatsIngester` via a buffered channel, and batch-insert into SQLite every 5 seconds.

**Why not Option A (parse log file separately)?**
Would require opening and tailing the same file twice, creating file handle contention and duplicated parsing logic. `LogWatcher` already does this perfectly.

**Why not Option B (Gin middleware)?**
The management interface on port 8080 uses Gin; actual proxied traffic flows through Caddy on ports 80/443. A Gin middleware captures only management API calls — not user traffic. We need Caddy log data.

**Why Option C wins:**
- Zero file I/O duplication — reuses existing `LogWatcher` parsing
- Decoupled: `StatsIngester` never blocks `LogWatcher`'s broadcast loop (buffered channel + non-blocking send)
- Batch writes reduce SQLite write frequency from per-request to per-5-seconds
- The buffered channel (capacity 1000) absorbs burst traffic without blocking

### 8.2 Integration Point in `LogWatcher`

**File:** `backend/internal/services/log_watcher.go` (MODIFY)

Add to the `LogWatcher` struct:
```go
statsIngester *StatsIngester  // optional; nil = disabled
```

Add method:
```go
// SetStatsIngester wires a StatsIngester to receive all parsed log entries.
// Must be called before Start().
func (w *LogWatcher) SetStatsIngester(si *StatsIngester) {
    w.mu.Lock()
    defer w.mu.Unlock()
    w.statsIngester = si
}
```

In `readLoop`, after the existing `w.broadcast(*entry)` call, add:
```go
if w.statsIngester != nil {
    w.statsIngester.Ingest(*entry)
}
```

### 8.3 `StatsIngester` Design

**File:** `backend/internal/services/stats_ingester.go` (CREATE)

```go
// StatsIngester receives SecurityLogEntry values from LogWatcher,
// buffers them in memory, and batch-inserts them into request_logs every flushInterval.
type StatsIngester struct {
    db            *gorm.DB
    hub           BroadcastHub  // interface defined in services package — no import cycle
    ch            chan models.SecurityLogEntry
    flushInterval time.Duration
    ctx           context.Context
    cancel        context.CancelFunc
    mu            sync.Mutex
    lastIngestAt  time.Time
    // Rolling 1-minute counters (reset each broadcast)
    recentCount   atomic.Int64
    recentBlocked atomic.Int64
    // C3: tracks entries dropped due to full buffer
    droppedCount  atomic.Int64
}

func NewStatsIngester(db *gorm.DB) *StatsIngester

// Ingest enqueues a log entry. Non-blocking: drops entry and increments droppedCount if buffer is full.
// ClientIP is SHA-256 hashed (first 16 bytes, hex) unless stats.store_raw_client_ip setting is "true".
func (si *StatsIngester) Ingest(entry models.SecurityLogEntry)

// SetHub sets the WebSocket hub for post-flush broadcasts. Must be set before Start().
func (si *StatsIngester) SetHub(hub BroadcastHub)

// Start launches the flush goroutine and cleanup goroutine. The provided ctx is the server's root context.
func (si *StatsIngester) Start(ctx context.Context) error

// Stop cancels the internal context and blocks until the flush goroutine completes,
// draining the channel into one final batch insert before returning.
// Called during graceful server shutdown to respect server.Run(ctx) lifecycle (M1).
func (si *StatsIngester) Stop()

// LastIngestAt returns the time of the most recently flushed batch (for health checks).
func (si *StatsIngester) LastIngestAt() time.Time

// QueueDepth returns the current number of entries waiting to be flushed.
func (si *StatsIngester) QueueDepth() int

// DroppedCount returns the cumulative number of entries dropped due to a full buffer (C3).
func (si *StatsIngester) DroppedCount() int64
```

**C1 — Type ownership and import cycle avoidance:** `StatsPushMessage` and `StatsPushData` types are defined in `backend/internal/services/stats_types.go` (CREATE), NOT in `handlers`. The `BroadcastHub` interface is also defined there. This ensures `handlers` imports `services`, never the reverse:

```go
// backend/internal/services/stats_types.go

// BroadcastHub abstracts the WebSocket hub for stats broadcasts.
// Implemented by StatsWSHub in the handlers package.
type BroadcastHub interface {
    Broadcast(msg StatsPushMessage)
}

// StatsPushMessage is the JSON envelope pushed to WebSocket clients every 10 s.
type StatsPushMessage struct {
    Type string        `json:"type"`  // always "stats_update"
    Ts   string        `json:"ts"`
    Data StatsPushData `json:"data"`
}

// StatsPushData holds the incremental metric snapshot for a single push interval.
type StatsPushData struct {
    RequestsLastMinute  int64            `json:"requests_last_minute"`
    BlockedLastMinute   int64            `json:"blocked_last_minute"`
    ActiveHosts         int              `json:"active_hosts"`
    TopHostNow          string           `json:"top_host_now"`
    AvgResponseMS       float64          `json:"avg_response_ms"`
    StatusCountsLastMin map[string]int64 `json:"status_counts_last_minute"`
}
```

Batch insert uses `db.WithContext(ctx).CreateInBatches(rows, 500)`.

### 8.4 `StatsService` Design

**File:** `backend/internal/services/stats_service.go` (CREATE)

```go
type StatsService struct {
    db       *gorm.DB
    ingester *StatsIngester
    cache    *statsCache
}

func NewStatsService(db *gorm.DB, ingester *StatsIngester) *StatsService

// Public query methods (all accept ctx context.Context, return (*T, error))
func (s *StatsService) GetSummary(ctx context.Context) (*StatsSummaryResult, error)
func (s *StatsService) GetRequestCounts(ctx context.Context, period, bucket, host string) (*StatsRequestsResult, error)
func (s *StatsService) GetTopHosts(ctx context.Context, period string, limit int) (*StatsTopHostsResult, error)
func (s *StatsService) GetStatusDistribution(ctx context.Context, period, host string) (*StatsStatusResult, error)
func (s *StatsService) GetTrafficVolume(ctx context.Context, period, bucket string) (*StatsTrafficResult, error)
func (s *StatsService) GetCertExpiry(ctx context.Context, withinDays int) (*StatsCertExpiryResult, error)
func (s *StatsService) GetServiceHealth(ctx context.Context) (*StatsHealthResult, error)

// Private helpers
func periodStart(period string) time.Time   // returns time.Now().Add(-duration)
func bucketSQL(bucket string) string         // returns SQLite strftime expression
```

Result types are defined in `stats_service.go` (not exported to handler via separate file, to keep the package cohesive). The handler file maps these result types to `gin.H` JSON responses.

**M2 — `bucket` allowlist validation order:** The handler validates `bucket` against the allowlist `["1h", "6h", "1d"]` BEFORE calling any service function. `bucketSQL()` is a private helper that panics on unrecognised input (defensive programming); it must never be called with user-supplied input that has not been validated. The validation in the handler returns HTTP 400 with `{"error": "bucket must be one of: 1h, 6h, 1d"}` for any other value.

SQLite bucket expression for 6h grouping:
```go
case "6h":
    return "strftime('%Y-%m-%d ', timestamp) || printf('%02d', (CAST(strftime('%H', timestamp) AS INT) / 6) * 6) || ':00:00'"
```

### 8.5 `StatsHandler` Design

**File:** `backend/internal/api/handlers/stats_handler.go` (CREATE)

```go
// StatsHandler handles HTTP and WebSocket requests for dashboard statistics.
type StatsHandler struct {
    statsService *services.StatsService
    hub          *StatsWSHub
    wsTracker    *services.WebSocketTracker
}

func NewStatsHandler(
    statsService *services.StatsService,
    hub *StatsWSHub,
    wsTracker *services.WebSocketTracker,
) *StatsHandler

// HTTP handlers
func (h *StatsHandler) GetSummary(c *gin.Context)
func (h *StatsHandler) GetRequests(c *gin.Context)
func (h *StatsHandler) GetTopHosts(c *gin.Context)
func (h *StatsHandler) GetStatusDistribution(c *gin.Context)
func (h *StatsHandler) GetTrafficVolume(c *gin.Context)
func (h *StatsHandler) GetCertExpiry(c *gin.Context)
func (h *StatsHandler) GetServiceHealth(c *gin.Context)

// WebSocket
func (h *StatsHandler) LiveWebSocket(c *gin.Context)

// Route registration
func (h *StatsHandler) RegisterRoutes(rg *gin.RouterGroup)
```

`RegisterRoutes` registers:
```go
rg.GET("/stats/summary",             h.GetSummary)
rg.GET("/stats/requests",            h.GetRequests)
rg.GET("/stats/top-hosts",           h.GetTopHosts)
rg.GET("/stats/status-distribution", h.GetStatusDistribution)
rg.GET("/stats/traffic-volume",      h.GetTrafficVolume)
rg.GET("/stats/cert-expiry",         h.GetCertExpiry)
rg.GET("/stats/health",              h.GetServiceHealth)
rg.GET("/stats/live",                h.LiveWebSocket)
```

**Handler input validation (C2, M2, M5):** Each handler validates its inputs before calling any service method:

- `GetRequests` / `GetTrafficVolume`: `bucket` validated against allowlist `["1h", "6h", "1d"]` first; HTTP 400 if invalid (M2). `host` validated ≤ 253 characters; HTTP 400 if too long (M5).
- `GetStatusDistribution`: `host` validated ≤ 253 characters; HTTP 400 if too long (M5).
- `GetCertExpiry`: `within_days` validated 1–365; HTTP 400 if out of range (C2).

**StatsWSHub (same file):**

Note: `StatsPushMessage` and `StatsPushData` are imported from the `services` package (defined in `backend/internal/services/stats_types.go`). They are NOT redefined here.

```go
type StatsWSClient struct {
    id   string
    conn *websocket.Conn
    send chan services.StatsPushMessage
}

// StatsWSHub manages WebSocket connections for stats live-push.
// Implements services.BroadcastHub.
type StatsWSHub struct {
    clients    map[string]*StatsWSClient
    register   chan *StatsWSClient
    unregister chan string
    broadcast  chan services.StatsPushMessage
}

func NewStatsWSHub() *StatsWSHub

// Run starts the hub event loop. Must be called in a goroutine.
// Exits cleanly when ctx is cancelled (M1 — respects server.Run(ctx) lifecycle):
// closes all client connections gracefully before returning.
func (h *StatsWSHub) Run(ctx context.Context)

// Broadcast sends a message to all connected clients (non-blocking per client).
// Implements services.BroadcastHub.
func (h *StatsWSHub) Broadcast(msg services.StatsPushMessage)
```

---

## 9. Performance Considerations

### 9.1 SQLite Write Throughput

SQLite WAL mode handles approximately 1,000-5,000 writes/second on commodity hardware. Batch inserts of 500 rows every 5 seconds result in at most 100 write operations/second — well within limits even under heavy proxy load.

At 10,000 req/s (extreme), the ingester channel (capacity 1000) would fill in 0.1 seconds. Mitigation: `Ingest` uses a non-blocking select that drops entries when the buffer is full, incrementing a dropped-count counter for monitoring. The actual dashboard impact is negligible (1-min stats are still accurate from the entries that did flush).

### 9.2 Query Optimization

- All aggregation queries filter `WHERE timestamp >= :start` which is an index-range scan on `idx_request_log_ts`
- `GROUP BY host` queries benefit from `idx_request_log_host`
- The compound index `idx_request_log_ts_host` covers the most expensive pattern: per-host time-range queries
- `LIMIT 10` on top-hosts prevents the result set from growing unbounded

### 9.3 Caching Strategy

`StatsService` maintains a simple in-memory TTL cache keyed by `"endpoint:params"` string:

```go
type statsCache struct {
    mu      sync.RWMutex
    entries map[string]statsCacheEntry
}

type statsCacheEntry struct {
    data      any
    expiresAt time.Time
}
```

Cache key format: `"summary"`, `"requests:24h:1h:"`, `"top-hosts:24h:10"`, etc.

Cache TTLs per endpoint:

| Endpoint | TTL |
|---|---|
| summary | 15 s |
| requests | 30 s |
| top-hosts | 30 s |
| status-distribution | 30 s |
| traffic-volume | 30 s |
| cert-expiry | 5 min |
| health | 10 s |

Cache is invalidated after each `flushLoop` by calling `statsCache.InvalidatePattern("requests")` and `statsCache.InvalidatePattern("summary")`.

### 9.4 WebSocket Broadcast Throttling

`StatsWSHub` sends at most one broadcast per 10 seconds. The hub uses a `time.Ticker(10 * time.Second)` to pace broadcasts rather than broadcasting on every flush signal from `StatsIngester`. This prevents flooding frontends when many requests arrive in short bursts.

---

## 10. Testing Strategy

### 10.1 Backend Unit Tests

**File:** `backend/internal/services/stats_service_test.go` (CREATE)

Uses `testdb.go` pattern (existing SQLite in-memory helper).

| Test Function | Cases |
|---|---|
| `TestGetSummary` | empty DB returns zero-value response; seeded 24h data returns correct counts |
| `TestGetRequestCounts_Period` | period=24h, 7d, 30d each return correct bucket count |
| `TestGetRequestCounts_Bucket` | bucket=1h, 6h, 1d — verify correct SQL grouping |
| `TestGetRequestCounts_HostFilter` | host param filters rows correctly |
| `TestGetTopHosts_Limit` | limit clamped to 50 max; tie-breaking by count desc |
| `TestGetStatusDistribution` | 2xx/3xx/4xx/5xx counts correct; pct sums to 100.0 |
| `TestGetTrafficVolume` | bytes_sent summed correctly per bucket |
| `TestGetCertExpiry` | expired certs separate from expiring; sorted correctly |
| `TestGetServiceHealth_Nominal` | all status fields "ok" on healthy system |
| `TestStatsCache_TTL` | cached result returned; expires after TTL; cache miss hits DB |

**File:** `backend/internal/services/stats_ingester_test.go` (CREATE)

| Test Function | Cases |
|---|---|
| `TestIngest_NonBlocking` | full buffer does not block caller |
| `TestFlush_BatchInsert` | 10 entries produce 10 rows in DB after flush |
| `TestFlush_AtomicCounters` | recentCount and recentBlocked reset after broadcast |
| `TestCleanup_OldRows` | rows older than retention_days are deleted |
| `TestLastIngestAt_UpdatedAfterFlush` | timestamp advances after each flush |
| `TestQueueDepth_Accuracy` | QueueDepth() returns len(ch) |
| `TestDroppedCount_IncrementOnFullBuffer` | droppedCount increments when buffer is at capacity; DroppedCount() returns correct value (C3) |

**File:** `backend/internal/api/handlers/stats_handler_test.go` (CREATE)

| Test Function | HTTP Case |
|---|---|
| `TestGetSummaryHandler_OK` | 200 with correct JSON structure |
| `TestGetSummaryHandler_Unauthorized` | 401 without JWT |
| `TestGetRequestsHandler_InvalidPeriod` | 400 Bad Request for period="99d" |
| `TestGetRequestsHandler_ValidPeriods` | 200 for 24h, 7d, 30d |
| `TestGetRequestsHandler_InvalidBucket` | 400 Bad Request for bucket="2h"; allowlist enforced before service call (M2) |
| `TestGetRequestsHandler_HostTooLong` | 400 Bad Request when host exceeds 253 characters (M5) |
| `TestGetTopHostsHandler_LimitClamping` | limit=100 clamped to 50 |
| `TestGetCertExpiryHandler_Default` | within_days defaults to 30 |
| `TestGetCertExpiryHandler_InvalidWithinDays` | 400 for within_days=0, within_days=366, within_days=-1 (C2) |
| `TestGetStatusDistributionHandler_HostTooLong` | 400 Bad Request when host exceeds 253 characters (M5) |
| `TestLiveWebSocketHandler_Upgrade` | 101 Switching Protocols; ping received |

### 10.2 Frontend Unit Tests

**File:** `frontend/src/api/__tests__/stats.test.ts` (CREATE)

Uses `vi.mock('axios')` to mock `client`. Asserts correct URL and params for each function.

**Component tests under `frontend/src/components/stats/__tests__/`:**

| Test File | Key Assertions |
|---|---|
| `RequestTrendChart.test.tsx` | Shows skeleton while loading; renders SVG `<line>` elements with data |
| `StatusDistributionChart.test.tsx` | Renders pie segments matching status class data |
| `TopHostsChart.test.tsx` | Renders bar chart; top host appears first |
| `CertExpiryWidget.test.tsx` | No badge when empty; amber badge for expiring; red badge for <= 7 days |
| `ServiceHealthWidget.test.tsx` | Green dot for "ok"; amber for "degraded"; red for "error" |
| `PeriodSelector.test.tsx` | Clicking "7d" calls `onChange('7d')` |
| `StatsSummaryBanner.test.tsx` | Shows livePayload.requests_last_minute when provided |

Mock pattern:
```typescript
vi.mock('../../../hooks/useStats', () => ({
  useStatsSummary: vi.fn(() => ({ data: mockSummary, isLoading: false })),
  // ...
}))
```

### 10.3 Integration Tests

**File:** `backend/integration/stats_test.go` (CREATE)

End-to-end: spin up Gin router with real SQLite in-memory DB, seed `RequestLog` rows via direct GORM insert, call each HTTP endpoint, assert JSON response shape and values. Uses the existing `integration/` test helper patterns.

Scenarios:
- Request counts for seeded data match expectations
- Empty database returns zero-value responses (not 500 errors)
- Invalid query params return 400 errors
- Auth required: 401 without token

### 10.4 Playwright E2E Tests

**File:** `tests/stats.spec.ts` (CREATE)

```
Scenario: Dashboard shows statistics section
  Given the user is logged in as admin
  When navigating to /
  Then "Request Statistics" heading is visible
  And PeriodSelector tabs are visible (24h, 7d, 30d)

Scenario: Period selector drives chart data
  When clicking the "7d" tab
  Then URL or aria-selected attribute reflects 7d selection

Scenario: Empty state renders without errors
  Given no RequestLog rows exist
  When viewing the dashboard
  Then charts render an empty state message
  And no console errors are thrown

Scenario: Certificate expiry widget
  When viewing the dashboard
  Then CertExpiryWidget section is visible
  And it shows either a count of expiring certs or "No expiring certificates"

Scenario: Service health widget shows status
  When viewing the dashboard
  Then ServiceHealthWidget shows at least one health status indicator
```

---

## 11. Commit Slicing Strategy

**PR:** `feat: enhanced dashboard with real-time statistics (#25)`
**Branch:** `feature/stats`
**Base branch:** `main`
**Strategy:** Single PR with 10 ordered logical commits. Each commit is independently buildable. Validation gates must pass before the next commit is authored.

---

### Commit 1: `feat(models): add RequestLog model and AutoMigrate registration`

**Scope:**
- CREATE `backend/internal/models/request_log.go`
- MODIFY `backend/internal/api/routes/routes.go` — add `&models.RequestLog{}` to AutoMigrate and compound index creation

**Dependencies:** None (foundation commit)

**Validation gate:**
- `cd backend && go build ./...` succeeds
- `go test ./internal/models/...` passes
- `./scripts/scan-gorm-security.sh --check` passes

---

### Commit 2: `feat(services): add StatsIngester for log fan-out and batch DB writes`

**Scope:**
- CREATE `backend/internal/services/stats_types.go` — `BroadcastHub` interface, `StatsPushMessage`, `StatsPushData` (C1: types live here to avoid import cycle with `handlers`)
- CREATE `backend/internal/services/stats_ingester.go`
- CREATE `backend/internal/services/stats_ingester_test.go`
- MODIFY `backend/internal/services/log_watcher.go` — add `statsIngester` field, `SetStatsIngester` method, fan-out in `readLoop`
- MODIFY `backend/internal/services/log_watcher_test.go` — cover new fan-out path

**Dependencies:** Commit 1

**Validation gate:**
- `go test ./internal/services/...` passes
- `go test -race ./internal/services/...` passes (no data races)

---

### Commit 3: `feat(services): add StatsService with aggregation queries and TTL cache`

**Scope:**
- CREATE `backend/internal/services/stats_service.go`
- CREATE `backend/internal/services/stats_service_test.go`

**Dependencies:** Commits 1, 2

**Validation gate:**
- `go test ./internal/services/...` passes (all table-driven tests green)

---

### Commit 4: `feat(api): add stats handlers, WebSocket hub, and route registration`

**Scope:**
- CREATE `backend/internal/api/handlers/stats_handler.go`
- CREATE `backend/internal/api/handlers/stats_handler_test.go`
- MODIFY `backend/internal/api/routes/routes.go` — instantiate StatsIngester, StatsWSHub, StatsService, StatsHandler; call `logWatcher.SetStatsIngester(ingester)`; call `statsHandler.RegisterRoutes(management)`

**Lifecycle note (M1):** `routes.go` passes the server's root context (from `server.Run(ctx)`) to both `ingester.Start(ctx)` and `go hub.Run(ctx)`. `ingester.Stop()` is deferred/called on shutdown to drain the final batch. This ensures both long-running goroutines respect context cancellation and clean up gracefully.

**Dependencies:** Commits 1, 2, 3

**Validation gate:**
- `go test ./internal/api/...` passes
- `lefthook run pre-commit` passes
- Manual: `curl` against running server returns JSON from `/api/v1/stats/summary`

---

### Commit 5: `feat(frontend/api): add stats API client and TypeScript type definitions`

**Scope:**
- CREATE `frontend/src/api/stats.ts`
- CREATE `frontend/src/api/__tests__/stats.test.ts`

**Dependencies:** Commit 4 (endpoints must exist for integration validation)

**Validation gate:**
- `cd frontend && npm run type-check` passes
- `npm run test` passes

---

### Commit 6: `feat(frontend/hooks): add useStats and useStatsWebSocket hooks`

**Scope:**
- CREATE `frontend/src/hooks/useStats.ts`
- CREATE `frontend/src/hooks/useStatsWebSocket.ts`
- CREATE `frontend/src/hooks/__tests__/useStats.test.ts`
- CREATE `frontend/src/hooks/__tests__/useStatsWebSocket.test.ts`

**Dependencies:** Commit 5

**Validation gate:**
- `npm run type-check` passes
- `npm run test` passes

---

### Commit 7: `feat(frontend/components): add stats chart and widget components`

**Scope:**
- CREATE all files under `frontend/src/components/stats/` (8 component files + index.ts + 6 test files)

**Dependencies:** Commit 6

**Validation gate:**
- `npm run type-check` passes
- `npm run test` passes
- `npm run build` succeeds

---

### Commit 8: `feat(frontend/dashboard): integrate stats sections into Dashboard page`

**Scope:**
- MODIFY `frontend/src/pages/Dashboard.tsx`

**Dependencies:** Commit 7

**Validation gate:**
- `npm run build` succeeds
- `npm run type-check` passes
- Manual browser check: dashboard renders charts without console errors

---

### Commit 9: `test(e2e): add Playwright tests for enhanced dashboard statistics`

**Scope:**
- CREATE `tests/stats.spec.ts`
- CREATE `backend/integration/stats_test.go`

**Dependencies:** Commit 8

**Validation gate:**
- `npx playwright test --project=firefox tests/stats.spec.ts` passes

---

### Commit 10: `docs: update ARCHITECTURE.md and features.md for stats subsystem`

**Scope:**
- MODIFY `ARCHITECTURE.md` — add "Stats Subsystem" section under Core Components
- MODIFY `docs/features.md` — add enhanced dashboard entry

**Dependencies:** All prior commits

**Validation gate (full DoD sweep):**
- `bash scripts/local-patch-report.sh` passes
- `scripts/go-test-coverage.sh` reports >= 85%
- `scripts/frontend-test-coverage.sh` reports >= 85%
- `lefthook run pre-commit` passes

---

### Rollback Notes

- Commits 1-4 are backend-only; reverting them does not affect the frontend
- The `RequestLog` migration is additive (no column drops on existing tables); the table can be dropped manually if rolled back
- The `LogWatcher` modification (Commit 2) is backward-compatible: `statsIngester` is nil by default and the fan-out is behind a nil check
- Commits 5-8 are frontend-only; reverting them leaves backend APIs active but simply unused

---

## 12. Definition of Done Checklist

- [ ] `npx playwright test --project=firefox` — all E2E tests pass (including `tests/stats.spec.ts`)
- [ ] `./scripts/scan-gorm-security.sh --check` — zero CRITICAL/HIGH findings (new model added)
- [ ] `bash scripts/local-patch-report.sh` — report generated at `test-results/local-patch-report.md`
- [ ] `lefthook run pre-commit` — zero errors
- [ ] `make lint-staticcheck-only` — zero SA/S1xxx findings in new files
- [ ] `scripts/go-test-coverage.sh` — backend coverage >= 85%
- [ ] `scripts/frontend-test-coverage.sh` — frontend coverage >= 85%
- [ ] `cd frontend && npm run type-check` — zero TypeScript errors
- [ ] `cd backend && go build ./...` — compiles cleanly
- [ ] `cd frontend && npm run build` — Vite build succeeds
- [ ] `go test -race ./...` — no data races in ingester or hub goroutines
- [ ] `ARCHITECTURE.md` updated — stats subsystem documented
- [ ] `docs/features.md` updated — enhanced dashboard entry added
- [ ] Zero `console.log`, `fmt.Println`, unused imports in committed files
- [ ] All Go source files have package doc comments
- [ ] All handler tests use the existing `testdb.go` helper for consistency

---

## 13. Risk Register

### Risk 1: SQLite Write Contention Under High Load

**Probability:** Low (home/small-team use case)
**Impact:** Medium — ingestion lag; stats fall behind real-time by more than one flush interval
**Mitigation:**
- Non-blocking `Ingest` prevents blocking `LogWatcher` broadcast path
- Batch insert (up to 500 rows per DB call) minimizes write frequency
- WAL mode allows concurrent reads during writes
- If contention is detected: increase flush interval from 5 s to 10 s via `stats.flush_interval_secs` setting

### Risk 2: LogWatcher Fan-Out Breaks Existing Security Log Streaming

**Probability:** Low (change is additive with nil-guard)
**Impact:** High — security log WebSocket would stop delivering entries
**Mitigation:**
- `statsIngester` field is nil by default; fan-out is a single `if w.statsIngester != nil` check
- Existing `log_watcher_test.go` tests must continue to pass (enforced in Commit 2 validation gate)
- Integration test in Commit 9 explicitly verifies security log subscribers still receive events when `StatsIngester` is wired

### Risk 3: Import Cycle Between `handlers` and `services`

**Probability:** Medium (handlers need StatsIngester; StatsIngester needs to call hub Broadcast)
**Impact:** Medium — compile failure
**Mitigation:**
- Define `BroadcastHub` interface in `services` package
- `StatsWSHub` (in `handlers`) implements `BroadcastHub`
- `StatsIngester` holds a `BroadcastHub` interface value, not a concrete `*StatsWSHub`
- This breaks the cycle: `handlers` imports `services`, not the reverse

### Risk 4: Empty-State Rendering Errors on Fresh Install

**Probability:** High (fresh installs have zero `RequestLog` rows)
**Impact:** Low — chart components may panic if buckets array is nil instead of empty
**Mitigation:**
- Backend always returns `"buckets": []` (never null) when there are no rows
- Frontend chart components check `if (!data || data.buckets.length === 0)` and render an empty state message
- Playwright E2E test validates empty state explicitly (Scenario 3)

### Risk 5: WebSocket Connection Storms With Multiple Tabs

**Probability:** Medium (power users open multiple tabs)
**Impact:** Low-Medium — hub must handle concurrent clients without deadlock
**Mitigation:**
- `StatsWSHub` uses a select-based event loop (single goroutine) with buffered per-client channels
- Non-blocking send in `Broadcast` drops messages for slow/lagging clients
- `WebSocketTracker` (existing) tracks and exposes stats WS connections for monitoring
- Tested via `go test -race` on hub goroutine

---

## 14. Supervisor Review

**Review date:** 2026-06-14
**Reviewer:** Supervisor agent (Code Review Lead)
**Outcome:** APPROVED WITH CONDITIONS

### Conditions Raised

| ID | Severity | Title | Resolution |
|---|---|---|---|
| C1 | Critical | Import cycle — `StatsPushMessage` type ownership | Moved `StatsPushMessage`, `StatsPushData`, and `BroadcastHub` interface into new `backend/internal/services/stats_types.go`. Sections 2.2, 8.3, 8.5, and 15 (FILES) updated. |
| C2 | Critical | `within_days` upper-bound validation missing | Section 4.6 updated: valid range 1–365, HTTP 400 for out-of-range values. `TestGetCertExpiryHandler_InvalidWithinDays` added to Section 10.1 handler test table. |
| C3 | Critical | `droppedCount` counter missing from `StatsIngester` | `droppedCount atomic.Int64` field and `DroppedCount() int64` method added to `StatsIngester` in Section 8.3. Section 4.7 health response updated to include `"dropped_count": 0`. `TestDroppedCount_IncrementOnFullBuffer` added to Section 10.1 ingester test table. |
| M1 | Major | `StatsIngester` and `StatsWSHub` lack documented `Stop()`/shutdown lifecycle | `Stop()` method added to `StatsIngester` in Section 8.3 (drains final batch on cancellation). `StatsWSHub.Run(ctx)` documented to close all clients when ctx is cancelled. Commit 4 lifecycle note added to Section 11. |
| M2 | Major | `bucket` allowlist validation order not specified | Section 8.4 updated: handler validates `bucket` against `["1h", "6h", "1d"]` BEFORE passing to service. Section 8.5 validation summary added. `TestGetRequestsHandler_InvalidBucket` added to Section 10.1. |
| M3 | Major | `ClientIP` privacy / GDPR note missing | Section 3.1 updated: `ClientIP` stored as SHA-256 hash (first 16 bytes, hex) by default. `stats.store_raw_client_ip` Setting key documented (default: `"false"`). `StatsIngester.Ingest()` description updated. |
| M4 | Major | Polling fires even when WebSocket is connected | Section 7.5 updated: `useStatsSummary(connected?: boolean)` suppresses polling when `connected=true`. `Dashboard.tsx` pattern documented — WS `connected` boolean threaded into summary hook. |
| M5 | Major | `host` query parameter unbounded string length | Sections 4.2 and 4.4 updated: `host` validated ≤ 253 characters; HTTP 400 if exceeded. `TestGetRequestsHandler_HostTooLong` and `TestGetStatusDistributionHandler_HostTooLong` added to Section 10.1. |

All critical issues and major concerns have been resolved in version 1.1 of this specification.

---

## 15. Files to Create / Modify

### Backend — CREATE

| File | Purpose |
|---|---|
| `backend/internal/models/request_log.go` | `RequestLog` GORM model + index declarations |
| `backend/internal/services/stats_types.go` | `BroadcastHub` interface, `StatsPushMessage`, `StatsPushData` (C1: owned here to break import cycle) |
| `backend/internal/services/stats_ingester.go` | Log fan-out receiver, buffer, batch writer, WS broadcaster |
| `backend/internal/services/stats_ingester_test.go` | Unit tests for ingester |
| `backend/internal/services/stats_service.go` | Aggregation queries, TTL cache, cert expiry, health check |
| `backend/internal/services/stats_service_test.go` | Unit tests for service (table-driven) |
| `backend/internal/api/handlers/stats_handler.go` | HTTP handlers + StatsWSHub (imports services.StatsPushMessage) |
| `backend/internal/api/handlers/stats_handler_test.go` | Unit tests for all handlers |
| `backend/integration/stats_test.go` | Integration tests for all stats endpoints |

### Backend — MODIFY

| File | Change |
|---|---|
| `backend/internal/api/routes/routes.go` | Add `&models.RequestLog{}` to AutoMigrate; add compound index `db.Exec`; instantiate and wire `StatsIngester`, `StatsWSHub`, `StatsService`, `StatsHandler`; call `RegisterRoutes` on management group |
| `backend/internal/services/log_watcher.go` | Add `statsIngester *StatsIngester` field; add `SetStatsIngester()` method; add fan-out in `readLoop` after existing `broadcast` call |
| `backend/internal/services/log_watcher_test.go` | Add test coverage for `SetStatsIngester` fan-out path |

### Frontend — CREATE

| File | Purpose |
|---|---|
| `frontend/src/api/stats.ts` | All stats API functions + TypeScript types |
| `frontend/src/api/__tests__/stats.test.ts` | Unit tests for API client |
| `frontend/src/hooks/useStats.ts` | React Query hooks for all stats endpoints + STATS_QUERY_KEYS |
| `frontend/src/hooks/useStatsWebSocket.ts` | WebSocket connection hook + StatsPushPayload type |
| `frontend/src/hooks/__tests__/useStats.test.ts` | Hook unit tests |
| `frontend/src/hooks/__tests__/useStatsWebSocket.test.ts` | WS hook unit tests |
| `frontend/src/components/stats/StatsSummaryBanner.tsx` | Live KPI row (req/min, blocked, avg response) |
| `frontend/src/components/stats/PeriodSelector.tsx` | 24h / 7d / 30d tab selector |
| `frontend/src/components/stats/RequestTrendChart.tsx` | Recharts LineChart: request counts over time |
| `frontend/src/components/stats/StatusDistributionChart.tsx` | Recharts PieChart: 2xx/3xx/4xx/5xx |
| `frontend/src/components/stats/TopHostsChart.tsx` | Recharts BarChart (vertical layout): top hosts |
| `frontend/src/components/stats/TrafficVolumeChart.tsx` | Recharts AreaChart: bytes sent over time |
| `frontend/src/components/stats/CertExpiryWidget.tsx` | Cert expiry warnings with color-coded badges |
| `frontend/src/components/stats/ServiceHealthWidget.tsx` | Service health status indicators |
| `frontend/src/components/stats/index.ts` | Re-exports all stats components |
| `frontend/src/components/stats/__tests__/RequestTrendChart.test.tsx` | Component unit test |
| `frontend/src/components/stats/__tests__/StatusDistributionChart.test.tsx` | Component unit test |
| `frontend/src/components/stats/__tests__/TopHostsChart.test.tsx` | Component unit test |
| `frontend/src/components/stats/__tests__/CertExpiryWidget.test.tsx` | Component unit test |
| `frontend/src/components/stats/__tests__/ServiceHealthWidget.test.tsx` | Component unit test |
| `frontend/src/components/stats/__tests__/PeriodSelector.test.tsx` | Component unit test |
| `frontend/src/components/stats/__tests__/StatsSummaryBanner.test.tsx` | Component unit test |
| `tests/stats.spec.ts` | Playwright E2E tests for dashboard statistics |

### Frontend — MODIFY

| File | Change |
|---|---|
| `frontend/src/pages/Dashboard.tsx` | Add `period` state + `PeriodSelector`; add `useStatsWebSocket()` call; add `StatsSummaryBanner`, chart grid (4 charts), system grid (cert expiry + health) |

### Documentation — MODIFY

| File | Change |
|---|---|
| `ARCHITECTURE.md` | Add "Stats Subsystem" subsection under Core Components: describes `StatsIngester` -> `RequestLog` -> `StatsService` -> REST/WS API flow |
| `docs/features.md` | Add line: "Enhanced Dashboard — real-time request statistics, top hosts, traffic volume graphs, and certificate expiry warnings" |
