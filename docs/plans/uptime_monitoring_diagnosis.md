# Uptime Monitoring Diagnosis: Wizarr Host False "Down" Status

## Summary

**Issue**: Newly created Wizarr Proxy Host shows as "down" in uptime monitoring, despite the domain working correctly when accessed by users.

**Root Cause**: Port mismatch in host-level TCP connectivity check. The `checkHost()` function extracts the port from the public URL (443 for HTTPS) but should be checking the actual backend `forward_port` (5690 for Wizarr).

**Status**: Identified - Fix Required

## Detailed Analysis

### 1. Code Location

**Primary Issue**: `backend/internal/services/uptime_service.go`

- **Function**: `checkHost()` (lines 359-402)
- **Logic Flow**: `checkAllHosts()` → `checkHost()` → `CheckAll()` → `checkMonitor()`

### 2. How Uptime Monitoring Works

#### Two-Level Check System

1. **Host-Level Pre-Check** (TCP connectivity)
   - Runs first via `checkAllHosts()` → `checkHost()`
   - Groups services by their backend `forward_host` (e.g., `172.20.0.11`)
   - Attempts TCP connection to determine if host is reachable
   - If host is DOWN, marks all monitors on that host as down **without** checking individual services

2. **Service-Level Check** (HTTP/HTTPS)
   - Only runs if host-level check passes
   - Performs actual HTTP GET to public URL
   - Accepts 2xx, 3xx, 401, 403 as "up"
   - Correctly handles redirects (302)

### 3. The Bug

In `checkHost()` at line 375:

```go
for _, monitor := range monitors {
    port := extractPort(monitor.URL)  // Gets port from public URL
    if port == "" {
        continue
    }

    // Tries to connect using extracted port
    addr := net.JoinHostPort(host.Host, port)  // 172.20.0.11:443
    conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
```

**Problem**:

- `monitor.URL` is the **public URL**: `https://wizarr.hatfieldhosted.com`
- `extractPort()` returns `443` (HTTPS default)
- But Wizarr backend actually runs on `172.20.0.11:5690`
- TCP connection to `172.20.0.11:443` **fails** (no service listening)
- Host marked as "down"
- All monitors on that host marked "down" without individual checks

### 4. Evidence from Logs and Database

#### Heartbeat Records (Most Recent First)

```
down|Host unreachable|0|2025-12-22 21:29:05
up|HTTP 200|64|2025-12-22 21:29:04
down|Host unreachable|0|2025-12-22 21:01:26
up|HTTP 200|47|2025-12-22 21:00:19
```

**Pattern**: Alternating between successful HTTP checks and host-level failures.

#### Database State

```sql
-- uptime_monitors
name: Wizarr
url: https://wizarr.hatfieldhosted.com
status: down
failure_count: 3
max_retries: 3

-- uptime_hosts
id: 0c764438-35ff-451f-822a-7297f39f39d4
name: Wizarr
host: 172.20.0.11
status: down  ← This is causing the problem

-- proxy_hosts
name: Wizarr
domain_names: wizarr.hatfieldhosted.com
forward_host: 172.20.0.11
forward_port: 5690  ← This is the actual port!
```

#### Caddy Access Logs

Uptime check succeeds at HTTP level:

```
172.20.0.1 → GET / → 302 → /admin
172.20.0.1 → GET /admin → 302 → /login
172.20.0.1 → GET /login → 200 OK (16905 bytes)
```

### 5. Why Other Hosts Don't Have This Issue

Checking working hosts (using Radarr as example):

```sql
-- Radarr (working)
forward_host: 100.99.23.57
forward_port: 7878
url: https://radarr.hatfieldhosted.com

-- 302 redirect logic works correctly:
GET / → 302 → /login
```

**Why it works**: For services that redirect on root path, the HTTP check succeeds with 200-399 status codes. The port mismatch issue exists for all hosts, but:

1. **If the forward_port happens to be a standard port** (80, 443, 8080) that the extractPort() function returns, it may work by coincidence
2. **If the host IP doesn't respond on that port**, the TCP check fails
3. **Wizarr uses port 5690** - a non-standard port that extractPort() will never return

### 6. Additional Context

The uptime monitoring feature was recently enhanced with host-level grouping to:

- Reduce check overhead for multiple services on same host
- Provide consolidated DOWN notifications
- Avoid individual checks when host is unreachable

This is a good architectural decision, but the port extraction logic has a bug.

## Root Cause Summary

**The `checkHost()` function extracts the port from the monitor's public URL instead of using the actual backend forward_port from the proxy host configuration.**

### Why This Happens

1. `UptimeMonitor` stores the public URL (e.g., `https://wizarr.hatfieldhosted.com`)
2. `UptimeHost` only stores the `forward_host` IP, not the port
3. `checkHost()` tries to extract port from monitor URLs
4. For HTTPS URLs, it extracts 443
5. Wizarr backend is on 172.20.0.11:5690, not :443
6. TCP connection fails → host marked down → monitor marked down

## Proposed Fixes

### Option 1: Store Forward Port in UptimeHost (Recommended)

**Changes Required**:

1. Add `Ports` field to `UptimeHost` model:

   ```go
   type UptimeHost struct {
       // ... existing fields
       Ports []int `json:"ports" gorm:"-"` // Not stored, computed on the fly
   }
   ```

2. Modify `checkHost()` to try all ports associated with monitors on that host:

   ```go
   // Collect unique ports from all monitors for this host
   portSet := make(map[int]bool)
   for _, monitor := range monitors {
       if monitor.ProxyHostID != nil {
           var proxyHost models.ProxyHost
           if err := s.DB.First(&proxyHost, *monitor.ProxyHostID).Error; err == nil {
               portSet[proxyHost.ForwardPort] = true
           }
       }
   }

   // Try connecting to any of the ports
   success := false
   for port := range portSet {
       addr := net.JoinHostPort(host.Host, strconv.Itoa(port))
       conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
       // ... rest of logic
   }
   ```

**Pros**:

- Checks actual backend ports
- More accurate for non-standard ports
- Minimal schema changes

**Cons**:

- Requires database queries in check loop
- More complex logic

### Option 2: Store ForwardPort Reference in UptimeMonitor

**Changes Required**:

1. Add `ForwardPort` field to `UptimeMonitor`:

   ```go
   type UptimeMonitor struct {
       // ... existing fields
       ForwardPort int `json:"forward_port"`
   }
   ```

2. Update `SyncMonitors()` to populate it:

   ```go
   monitor = models.UptimeMonitor{
       // ... existing fields
       ForwardPort: host.ForwardPort,
   }
   ```

3. Update `checkHost()` to use stored forward port:

   ```go
   for _, monitor := range monitors {
       port := monitor.ForwardPort
       if port == 0 {
           continue
       }
       addr := net.JoinHostPort(host.Host, strconv.Itoa(port))
       // ... rest of logic
   }
   ```

**Pros**:

- Simple, no extra DB queries
- Forward port readily available

**Cons**:

- Schema migration required
- Duplication of data (port stored in both ProxyHost and UptimeMonitor)

### Option 3: Skip Host-Level Check for Non-Standard Ports

**Temporary workaround** - not recommended for production.

Only perform host-level checks for monitors on standard ports (80, 443, 8080).

### Option 4: Use ProxyHost Forward Port Directly (Simplest)

**Changes Required**:

Modify `checkHost()` to query the proxy host for each monitor to get the actual forward port:

```go
// In checkHost(), replace the port extraction:
for _, monitor := range monitors {
    var port int

    if monitor.ProxyHostID != nil {
        var proxyHost models.ProxyHost
        if err := s.DB.First(&proxyHost, *monitor.ProxyHostID).Error; err == nil {
            port = proxyHost.ForwardPort
        }
    } else {
        // Fallback to URL extraction for non-proxy monitors
        portStr := extractPort(monitor.URL)
        if portStr != "" {
            port, _ = strconv.Atoi(portStr)
        }
    }

    if port == 0 {
        continue
    }

    addr := net.JoinHostPort(host.Host, strconv.Itoa(port))
    conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
    // ... rest of check
}
```

**Pros**:

- No schema changes
- Works immediately
- Handles both proxy hosts and standalone monitors

**Cons**:

- Database query in check loop (but monitors are already cached)
- Slight performance overhead

## Recommended Solution

**Option 4** (Use ProxyHost Forward Port Directly) is recommended because:

1. No schema migration required
2. Simple fix, easy to test
3. Minimal performance impact (monitors already queried)
4. Can be deployed immediately
5. Handles edge cases (standalone monitors)

## Testing Plan

1. **Unit Test**: Add test case for non-standard port host check
2. **Integration Test**:
   - Create proxy host with non-standard forward port
   - Verify host-level check uses correct port
   - Verify monitor status updates correctly
3. **Manual Test**:
   - Apply fix
   - Wait for next uptime check cycle (60 seconds)
   - Verify Wizarr shows as "up"
   - Verify no other monitors affected

## Debugging Commands

```bash
# Check Wizarr monitor status
docker compose -f docker-compose.test.yml exec charon sh -c \
  "sqlite3 /app/data/charon.db \"SELECT name, status, failure_count, url FROM uptime_monitors WHERE name = 'Wizarr';\""

# Check Wizarr host status
docker compose -f docker-compose.test.yml exec charon sh -c \
  "sqlite3 /app/data/charon.db \"SELECT name, host, status FROM uptime_hosts WHERE name = 'Wizarr';\""

# Check recent heartbeats
docker compose -f docker-compose.test.yml exec charon sh -c \
  "sqlite3 /app/data/charon.db \"SELECT status, message, created_at FROM uptime_heartbeats WHERE monitor_id = 'eed56336-e646-4cf5-a3fc-ac4d2dd8760e' ORDER BY created_at DESC LIMIT 5;\""

# Check Wizarr proxy host config
docker compose -f docker-compose.test.yml exec charon sh -c \
  "sqlite3 /app/data/charon.db \"SELECT name, forward_host, forward_port FROM proxy_hosts WHERE name = 'Wizarr';\""

# Monitor real-time uptime checks in logs
docker compose -f docker-compose.test.yml logs -f charon | grep -i "wizarr\|uptime"
```

## Related Files

- `backend/internal/services/uptime_service.go` - Main uptime service
- `backend/internal/models/uptime.go` - UptimeMonitor model
- `backend/internal/models/uptime_host.go` - UptimeHost model
- `backend/internal/services/uptime_service_test.go` - Unit tests

## References

- Issue created: 2025-12-23
- Related feature: Host-level uptime grouping
- Related PR: [Reference to ACL/permission changes if applicable]

---

**Next Steps**: Implement Option 4 fix and add test coverage for non-standard port scenarios.
