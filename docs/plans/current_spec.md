# Uptime Feature Trace Analysis - Bug Investigation

**Issue:** 6 out of 14 proxy hosts show "No History Available" in uptime heartbeat graphs
**Date:** December 17, 2025
**Status:** 🔴 ROOT CAUSE IDENTIFIED - SQLite Database Corruption

---

## Executive Summary

**This is NOT a logic bug.** The root cause is **SQLite database corruption** affecting specific records in the `uptime_heartbeats` table. The error `database disk image is malformed` is consistently returned when querying heartbeat history for exactly 6 specific monitor IDs.

---

## 1. Evidence from Container Logs

### Error Pattern Observed

```log
2025/12/17 07:44:04 /app/backend/internal/services/uptime_service.go:877 database disk image is malformed
[8.185ms] [rows:0] SELECT * FROM `uptime_heartbeats` WHERE monitor_id = "2b8cea58-b8f9-43fc-abe0-f6a0baba2351" ORDER BY created_at desc LIMIT 60
```

### Affected Monitor IDs (6 total)

| Monitor UUID | Status Code | Error |
|--------------|-------------|-------|
| `2b8cea58-b8f9-43fc-abe0-f6a0baba2351` | 500 | database disk image is malformed |
| `5523d6b3-e2bf-4727-a071-6546f58e8839` | 500 | database disk image is malformed |
| `264fb47b-9814-479a-bb40-0397f21026fe` | 500 | database disk image is malformed |
| `97ecc308-ca86-41f9-ba59-5444409dee8e` | 500 | database disk image is malformed |
| `cad93a3d-6ad4-4cba-a95c-5bb9b46168cd` | 500 | database disk image is malformed |
| `cdc4d769-8703-4881-8202-4b2493bccf58` | 500 | database disk image is malformed |

### Working Monitor IDs (8 total - return HTTP 200)

- `fdbc17bd-a00a-4bde-b2f9-e6db69a55c0a`
- `869aee1a-37f0-437c-b151-72074629af3e`
- `dc254e9c-28b5-4b59-ae9a-3c0378420a5a`
- `33371a73-09a2-4c50-b327-69fab5324728`
- `412f9c0b-8498-4045-97c9-021d6fc2ed7e`
- `bef3866b-dbde-4159-9c40-1fb002ed0396`
- `84329e2b-7f7e-4c8b-a1a6-ca52d3b7e565`
- `edd36d10-0e5b-496c-acea-4e4cf7103369`
- `0b426c10-82b8-4cc4-af0e-2dd5f1082fb2`

---

## 2. Complete File Map - Uptime Feature

### Frontend Layer (`frontend/src/`)

| File | Purpose |
|------|---------|
| [pages/Uptime.tsx](frontend/src/pages/Uptime.tsx) | Main Uptime page component, displays MonitorCard grid |
| [api/uptime.ts](frontend/src/api/uptime.ts) | API client functions: `getMonitors()`, `getMonitorHistory()`, `updateMonitor()`, `deleteMonitor()`, `checkMonitor()` |
| [components/UptimeWidget.tsx](frontend/src/components/UptimeWidget.tsx) | Dashboard widget showing uptime summary |
| No dedicated hook | Uses inline `useQuery` in components |

### Backend Layer (`backend/internal/`)

| File | Purpose |
|------|---------|
| [api/routes/routes.go](backend/internal/api/routes/routes.go#L230-L240) | Route registration for `/uptime/*` endpoints |
| [api/handlers/uptime_handler.go](backend/internal/api/handlers/uptime_handler.go) | HTTP handlers: `List()`, `GetHistory()`, `Update()`, `Delete()`, `Sync()`, `CheckMonitor()` |
| [services/uptime_service.go](backend/internal/services/uptime_service.go) | Business logic: monitor checking, notification batching, history retrieval |
| [models/uptime.go](backend/internal/models/uptime.go) | GORM models: `UptimeMonitor`, `UptimeHeartbeat` |
| [models/uptime_host.go](backend/internal/models/uptime_host.go) | GORM models: `UptimeHost`, `UptimeNotificationEvent` |

---

## 3. Data Flow Analysis

### Request Flow: UI → API → DB → Response

```text
┌─────────────────────────────────────────────────────────────────────────┐
│ FRONTEND                                                                │
├─────────────────────────────────────────────────────────────────────────┤
│ 1. Uptime.tsx loads → useQuery(['monitors'], getMonitors)               │
│ 2. For each monitor, MonitorCard renders                                │
│ 3. MonitorCard calls useQuery(['uptimeHistory', monitor.id],            │
│    () => getMonitorHistory(monitor.id, 60))                             │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ API CLIENT (frontend/src/api/uptime.ts)                                 │
├─────────────────────────────────────────────────────────────────────────┤
│ getMonitorHistory(id: string, limit: number = 50):                      │
│   client.get<UptimeHeartbeat[]>                                         │
│     (`/uptime/monitors/${id}/history?limit=${limit}`)                   │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ BACKEND ROUTES (backend/internal/api/routes/routes.go)                  │
├─────────────────────────────────────────────────────────────────────────┤
│ protected.GET("/uptime/monitors/:id/history", uptimeHandler.GetHistory) │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ HANDLER (backend/internal/api/handlers/uptime_handler.go)               │
├─────────────────────────────────────────────────────────────────────────┤
│ func (h *UptimeHandler) GetHistory(c *gin.Context) {                    │
│     id := c.Param("id")                                                 │
│     limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))             │
│     history, err := h.service.GetMonitorHistory(id, limit)              │
│     if err != nil {                                                     │
│         c.JSON(500, gin.H{"error": "Failed to get history"}) ◄─ ERROR   │
│         return                                                          │
│     }                                                                   │
│     c.JSON(200, history)                                                │
│ }                                                                       │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ SERVICE (backend/internal/services/uptime_service.go:875-879)           │
├─────────────────────────────────────────────────────────────────────────┤
│ func (s *UptimeService) GetMonitorHistory(id string, limit int)         │
│     ([]models.UptimeHeartbeat, error) {                                 │
│     var heartbeats []models.UptimeHeartbeat                             │
│     result := s.DB.Where("monitor_id = ?", id)                          │
│                   .Order("created_at desc")                             │
│                   .Limit(limit)                                         │
│                   .Find(&heartbeats)     ◄─ GORM QUERY                  │
│     return heartbeats, result.Error      ◄─ ERROR RETURNED HERE         │
│ }                                                                       │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ DATABASE (SQLite via GORM)                                              │
├─────────────────────────────────────────────────────────────────────────┤
│ SELECT * FROM uptime_heartbeats                                         │
│ WHERE monitor_id = "..."                                                │
│ ORDER BY created_at desc                                                │
│ LIMIT 60                                                                │
│                                                                         │
│ ERROR: "database disk image is malformed"                               │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Database Schema

### UptimeMonitor Table

```go
type UptimeMonitor struct {
    ID             string    `gorm:"primaryKey" json:"id"`      // UUID
    ProxyHostID    *uint     `json:"proxy_host_id"`             // Optional FK
    RemoteServerID *uint     `json:"remote_server_id"`          // Optional FK
    UptimeHostID   *string   `json:"uptime_host_id"`            // FK to UptimeHost
    Name           string    `json:"name"`
    Type           string    `json:"type"`                      // http, tcp, ping
    URL            string    `json:"url"`
    UpstreamHost   string    `json:"upstream_host"`
    Interval       int       `json:"interval"`                  // seconds
    Enabled        bool      `json:"enabled"`
    Status         string    `json:"status"`                    // up, down, pending
    LastCheck      time.Time `json:"last_check"`
    Latency        int64     `json:"latency"`                   // ms
    FailureCount   int       `json:"failure_count"`
    MaxRetries     int       `json:"max_retries"`
    // ... timestamps
}
```

### UptimeHeartbeat Table (where corruption exists)

```go
type UptimeHeartbeat struct {
    ID        uint      `gorm:"primaryKey" json:"id"`          // Auto-increment
    MonitorID string    `json:"monitor_id" gorm:"index"`       // UUID FK
    Status    string    `json:"status"`                        // up, down
    Latency   int64     `json:"latency"`
    Message   string    `json:"message"`
    CreatedAt time.Time `json:"created_at" gorm:"index"`
}
```

---

## 5. Root Cause Identification

### Primary Issue: SQLite Database Corruption

The error `database disk image is malformed` is a SQLite-specific error indicating:

- Corruption in the database file's B-tree structure
- Possible causes:
  1. **Disk I/O errors** during write operations
  2. **Unexpected container shutdown** mid-transaction
  3. **File system issues** in Docker volume
  4. **Database file written by multiple processes** (concurrent access without WAL)
  5. **Full disk** causing incomplete writes

### Why Only Some Monitors Are Affected

The corruption appears to be **localized to specific B-tree pages** that contain
the heartbeat records for those 6 monitors. SQLite's error occurs when:

- The query touches corrupted pages
- The index on `monitor_id` or `created_at` has corruption
- The data pages for those specific rows are damaged

### Evidence Supporting This Conclusion

1. **Consistent 500 errors** for the same 6 monitor IDs
2. **Other queries succeed** (listing monitors returns 200)
3. **Error occurs at the GORM layer** (service.go:877)
4. **Query itself is correct** (same pattern works for 8 other monitors)
5. **No ID mismatch** - UUIDs are correctly passed from frontend to backend

---

## 6. Recommended Actions

### Immediate Actions

1. **Stop the container gracefully** to prevent further corruption:

   ```bash
   docker stop charon
   ```

2. **Backup the current database** before any repair:

   ```bash
   docker cp charon:/app/data/charon.db ./charon.db.backup.$(date +%Y%m%d)
   ```

3. **Check database integrity** from within container:

   ```bash
   docker exec -it charon sqlite3 /app/data/charon.db "PRAGMA integrity_check;"
   ```

4. **Attempt database recovery**:

   ```bash
   # Export all data that can be read
   sqlite3 /app/data/charon.db ".dump" > dump.sql
   # Create new database
   sqlite3 /app/data/charon_new.db < dump.sql
   # Replace original
   mv /app/data/charon_new.db /app/data/charon.db
   ```

### If Recovery Fails

5. **Delete corrupted heartbeat records** (lossy but restores functionality):

   ```sql
   DELETE FROM uptime_heartbeats WHERE monitor_id IN (
       '2b8cea58-b8f9-43fc-abe0-f6a0baba2351',
       '5523d6b3-e2bf-4727-a071-6546f58e8839',
       '264fb47b-9814-479a-bb40-0397f21026fe',
       '97ecc308-ca86-41f9-ba59-5444409dee8e',
       'cad93a3d-6ad4-4cba-a95c-5bb9b46168cd',
       'cdc4d769-8703-4881-8202-4b2493bccf58'
   );
   VACUUM;
   ```

### Long-Term Prevention

6. **Enable WAL mode** for better crash resilience (in DB initialization):

   ```go
   db.Exec("PRAGMA journal_mode=WAL;")
   ```

7. **Add periodic VACUUM** to compact database and rebuild indexes

8. **Consider heartbeat table rotation** - archive old heartbeats to prevent
   unbounded growth

---

## 7. Code Quality Notes

### No Logic Bugs Found

After tracing the complete data flow:

- ✅ Frontend correctly passes monitor UUID
- ✅ API route correctly extracts `:id` param
- ✅ Handler correctly calls service with UUID
- ✅ Service correctly queries by `monitor_id`
- ✅ GORM model has correct field types and indexes

### Potential Improvement: Error Handling

The handler currently returns generic "Failed to get history" for all errors:

```go
// Current (hides root cause)
if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get history"})
    return
}

// Better (exposes root cause in logs, generic to user)
if err != nil {
    logger.Log().WithError(err).WithField("monitor_id", id).Error("GetHistory failed")
    c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get history"})
    return
}
```

---

## 8. Summary

| Question | Answer |
|----------|--------|
| Is this a frontend bug? | ❌ No |
| Is this a backend logic bug? | ❌ No |
| Is this an ID mismatch? | ❌ No (UUIDs are consistent) |
| Is this a timing issue? | ❌ No |
| **Is this database corruption?** | ✅ **YES** |
| Affected component | SQLite `uptime_heartbeats` table |
| Root cause | Disk image malformed (B-tree corruption) |
| Immediate fix | Database recovery/rebuild |
| Permanent fix | Enable WAL mode, graceful shutdowns |

---

*Investigation completed: December 17, 2025*
*Investigator: GitHub Copilot*
