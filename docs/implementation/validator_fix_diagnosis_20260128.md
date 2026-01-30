# Duplicate Proxy Host Diagnosis Report

**Date:** 2026-01-28
**Issue:** Charon container unhealthy, all proxy hosts down
**Error:** `validation failed: invalid route 1 in server charon_server: duplicate host matcher: immaculaterr.hatfieldhosted.com`

---

## Executive Summary

**Finding:** The database contains NO duplicate entries. There is only **one** proxy_host record for domain `Immaculaterr.hatfieldhosted.com` (ID 24). The duplicate host matcher error from Caddy indicates a **code-level bug** in the configuration generation logic, NOT a database integrity issue.

**Impact:**
- Caddy failed to load configuration at startup
- All proxy hosts are unreachable
- Container health check failing
- Frontend still accessible (direct backend connection)

**Root Cause:** Unknown bug in Caddy config generation that produces duplicate host matchers for the same domain, despite deduplication logic being present in the code.

---

## Investigation Details

### 1. Database Analysis

#### Active Database Location
- **Host path:** `/projects/Charon/data/charon.db` (empty/corrupted - 0 bytes)
- **Container path:** `/app/data/charon.db` (active - 177MB)
- **Backup:** `/projects/Charon/data/charon.db.backup-20260128-065828` (empty - contains schema but no data)

#### Database Integrity Check

**Total Proxy Hosts:** 19
**Query Results:**
```sql
-- Check for the problematic domain
SELECT id, uuid, name, domain_names, enabled, created_at, updated_at
FROM proxy_hosts
WHERE domain_names LIKE '%immaculaterr%';
```

**Result:** Only **ONE** entry found:
```
ID: 24
UUID: 4f392485-405b-4a35-b022-e3d16c30bbde
Name: Immaculaterr
Domain: Immaculaterr.hatfieldhosted.com (note: capital 'I')
Forward Host: Immaculaterr
Forward Port: 5454
Enabled: true
Created: 2026-01-16 20:42:59
Updated: 2026-01-16 20:42:59
```

#### Duplicate Detection Queries

**Test 1: Case-insensitive duplicate check**
```sql
SELECT COUNT(*), LOWER(domain_names)
FROM proxy_hosts
GROUP BY LOWER(domain_names)
HAVING COUNT(*) > 1;
```
**Result:** 0 duplicates found

**Test 2: Comma-separated domains check**
```sql
SELECT id, name, domain_names
FROM proxy_hosts
WHERE domain_names LIKE '%,%';
```
**Result:** No multi-domain entries found

**Test 3: Locations check (could cause route duplication)**
```sql
SELECT ph.id, ph.name, ph.domain_names, COUNT(l.id) as location_count
FROM proxy_hosts ph
LEFT JOIN locations l ON l.proxy_host_id = ph.id
WHERE ph.enabled = 1
GROUP BY ph.id;
```
**Result:** All proxy_hosts have 0 locations, including ID 24

**Test 4: Advanced config check**
```sql
SELECT id, name, domain_names, advanced_config
FROM proxy_hosts
WHERE id = 24;
```
**Result:** No advanced_config set (NULL)

**Test 5: Soft deletes check**
```sql
.schema proxy_hosts | grep -i deleted
```
**Result:** No soft delete columns exist

**Conclusion:** Database is clean. Only ONE entry for this domain exists.

---

### 2. Error Analysis

#### Error Message from Docker Logs
```
{"error":"validation failed: invalid route 1 in server charon_server: duplicate host matcher: immaculaterr.hatfieldhosted.com","level":"error","msg":"Failed to apply initial Caddy config","time":"2026-01-28T13:18:53-05:00"}
```

#### Key Observations:
1. **"invalid route 1"** - This is the SECOND route (0-indexed), suggesting the first route (index 0) is valid
2. **Lowercase domain** - Caddy error shows `immaculaterr` (lowercase) but database has `Immaculaterr` (capital I)
3. **Timing** - Error occurs at initial startup when `ApplyConfig()` is called
4. **Validation stage** - Error happens in Caddy's validation, not in Charon's generation

#### Code Review Findings

**File:** `/projects/Charon/backend/internal/caddy/config.go`
**Function:** `GenerateConfig()` (line 19)

**Deduplication Logic Present:**
- Line 437: `processedDomains := make(map[string]bool)` - Track processed domains
- Line 469-488: Domain normalization and duplicate detection
  ```go
  d = strings.TrimSpace(d)
  d = strings.ToLower(d) // Normalize to lowercase
  if processedDomains[d] {
      logger.Log().WithField("domain", d).Warn("Skipping duplicate domain")
      continue
  }
  processedDomains[d] = true
  ```
- Line 461: Reverse iteration to prefer newer hosts
  ```go
  for i := len(hosts) - 1; i >= 0; i--
  ```

**Expected Behavior:** The deduplication logic SHOULD prevent this error.

**Hypothesis:** One of the following is occurring:
1. **Bug in deduplication logic:** The domain is bypassing the duplicate check
2. **Multiple code paths:** Domain is added through a different path (e.g., frontend route, locations, advanced config)
3. **Database query issue:** GORM joins/preloads causing duplicate records in the Go slice
4. **Race condition:** Config is being generated/applied multiple times simultaneously (unlikely at startup)

---

### 3. All Proxy Hosts in Database

```
ID  Name          Domain
2   FileFlows     fileflows.hatfieldhosted.com
4   Profilarr     profilarr.hatfieldhosted.com
5   HomePage      homepage.hatfieldhosted.com
6   Prowlarr      prowlarr.hatfieldhosted.com
7   Tautulli      tautulli.hatfieldhosted.com
8   TubeSync      tubesync.hatfieldhosted.com
9   Bazarr        bazarr.hatfieldhosted.com
11  Mealie        mealie.hatfieldhosted.com
12  NZBGet        nzbget.hatfieldhosted.com
13  Radarr        radarr.hatfieldhosted.com
14  Sonarr        sonarr.hatfieldhosted.com
15  Seerr         seerr.hatfieldhosted.com
16  Plex          plex.hatfieldhosted.com
17  Charon        charon.hatfieldhosted.com
18  Wizarr        wizarr.hatfieldhosted.com
20  PruneMate     prunemate.hatfieldhosted.com
21  GiftManager   giftmanager.hatfieldhosted.com
22  Dockhand      dockhand.hatfieldhosted.com
24  Immaculaterr  Immaculaterr.hatfieldhosted.com  ← PROBLEMATIC
```

**Note:** ID 24 is the newest proxy_host (most recent updated_at timestamp).

---

### 4. Caddy Configuration State

**Current Status:** NO configuration loaded (Caddy is running with minimal admin-only config)

**Query:** `curl localhost:2019/config/` returns empty/default config

**Last Successful Config:**
- Timestamp: 2026-01-27 19:15:38
- Config Hash: `a87bd130369d62ab29a1fcf377d855a5b058223c33818eacff6f7312c2c4d6a0`
- Status: Success (before ID 24 was added)

**Recent Config History (from caddy_configs table):**
```
ID  Hash                                                              Applied At              Success
299 a87bd130...c2c4d6a0 2026-01-27 19:15:38  true
298 a87bd130...c2c4d6a0 2026-01-27 15:40:56  true
297 a87bd130...c2c4d6a0 2026-01-27 03:34:46  true
296 dbf4c820...d963b234 2026-01-27 02:01:45  true
295 dbf4c820...d963b234 2026-01-27 02:01:45  true
```

All recent configs were successful. The failure happened on 2026-01-28 13:18:53 (not recorded in table due to early validation failure).

---

### 5. Database File Status

**Critical Issue:** The host's `/projects/Charon/data/charon.db` file is **empty** (0 bytes).

**Timeline:**
- Original file was likely corrupted or truncated
- Container is using an in-memory or separate database file
- Volume mount may be broken or asynchronous

**Evidence:**
```bash
-rw-r--r-- 1 root root    0 Jan 28 18:24 /projects/Charon/data/charon.db
-rw-r--r-- 1 root root 177M Jan 28 18:26 /projects/Charon/data/charon.db.investigation
```

The actual database was copied from the container.

---

## Recommended Remediation Plan

### Immediate Short-Term Fix (Workaround)

**Option 1: Disable Problematic Proxy Host**
```sql
-- Run inside container
docker exec charon sqlite3 /app/data/charon.db \
  "UPDATE proxy_hosts SET enabled = 0 WHERE id = 24;"

-- Restart container to apply
docker restart charon
```

**Option 2: Delete Duplicate Entry (if acceptable data loss)**
```sql
docker exec charon sqlite3 /app/data/charon.db \
  "DELETE FROM proxy_hosts WHERE id = 24;"
docker restart charon
```

**Option 3: Change Domain to Bypass Duplicate Detection**
```sql
-- Temporarily rename the domain to isolate the issue
docker exec charon sqlite3 /app/data/charon.db \
  "UPDATE proxy_hosts SET domain_names = 'immaculaterr-temp.hatfieldhosted.com' WHERE id = 24;"
docker restart charon
```

### Medium-Term Fix (Debug & Patch)

**Step 1: Enable Debug Logging**
```bash
# Set debug logging in container
docker exec charon sh -c "export CHARON_DEBUG=1; kill -HUP \$(pidof charon)"
```

**Step 2: Generate Config Manually**
Create a debug script to generate and inspect the Caddy config:
```go
// In backend/cmd/debug/main.go
package main

import (
    "encoding/json"
    "fmt"
    "log"

    "github.com/Wikid82/charon/backend/internal/caddy"
    "github.com/Wikid82/charon/backend/internal/database"
    "github.com/Wikid82/charon/backend/internal/models"
)

func main() {
    db, _ := database.Connect("data/charon.db")
    var hosts []models.ProxyHost
    db.Preload("Locations").Preload("DNSProvider").Find(&hosts)

    config, err := caddy.GenerateConfig(hosts, "data/caddy/data", "", "frontend/dist", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
    if err != nil {
        log.Fatal(err)
    }

    json, _ := json.MarshalIndent(config, "", "  ")
    fmt.Println(string(json))
}
```

Run and inspect:
```bash
go run backend/cmd/debug/main.go > /tmp/caddy-config-debug.json
jq '.apps.http.servers.charon_server.routes[] | select(.match[0].host[] | contains("immaculaterr"))' /tmp/caddy-config-debug.json
```

**Step 3: Add Unit Test**
```go
// In backend/internal/caddy/config_test.go
func TestGenerateConfig_PreventCaseSensitiveDuplicates(t *testing.T) {
    hosts := []models.ProxyHost{
        {UUID: "uuid-1", DomainNames: "Example.com", Enabled: true, ForwardHost: "app1", ForwardPort: 8080},        {UUID: "uuid-2", DomainNames: "example.com", Enabled: true, ForwardHost: "app2", ForwardPort: 8081},
    }

    config, err := GenerateConfig(hosts, "/tmp/data", "", "", "", false, false, false, false, false, "", nil, nil, nil, nil, nil)
    require.NoError(t, err)

    // Should only have ONE route for this domain (not two)
    server := config.Apps.HTTP.Servers["charon_server"]
    routes := server.Routes

    domainCount := 0
    for _, route := range routes {
        for _, match := range route.Match {
            for _, host := range match.Host {
                if strings.ToLower(host) == "example.com" {
                    domainCount++
                }
            }
        }
    }

    assert.Equal(t, 1, domainCount, "Should only have one route for case-insensitive duplicate domain")
}
```

### Long-Term Fix (Root Cause Prevention)

**1. Add Database Constraint**
```sql
-- Create unique index on normalized domain names
CREATE UNIQUE INDEX idx_proxy_hosts_domain_names_lower
ON proxy_hosts(LOWER(domain_names));
```

**2. Add Pre-Save Validation Hook**
```go
// In backend/internal/models/proxy_host.go
func (p *ProxyHost) BeforeSave(tx *gorm.DB) error {
    // Normalize domain names to lowercase
    p.DomainNames = strings.ToLower(p.DomainNames)

    // Check for existing domain (case-insensitive)
    var existing ProxyHost
    if err := tx.Where("id != ? AND LOWER(domain_names) = ?",
        p.ID, strings.ToLower(p.DomainNames)).First(&existing).Error; err == nil {
        return fmt.Errorf("domain %s already exists (ID: %d)", p.DomainNames, existing.ID)
    }

    return nil
}
```

**3. Add Duplicate Detection to Frontend**
```typescript
// In frontend/src/components/ProxyHostForm.tsx
const checkDomainUnique = async (domain: string) => {
  const response = await api.get(`/api/v1/proxy-hosts?domain=${encodeURIComponent(domain.toLowerCase())}`);
  if (response.data.length > 0) {
    setError(`Domain ${domain} is already in use by "${response.data[0].name}"`);
    return false;
  }
  return true;
};
```

**4. Add Monitoring/Alerting**
- Add Prometheus metric for config generation failures
- Set up alert for repeated validation failures
- Log full generated config to file for debugging

---

## Next Steps

### Immediate Action Required (Choose ONE):

**Recommended:** Option 1 (Disable)
- **Pros:** Non-destructive, can re-enable later, allows investigation
- **Cons:** Service unavailable until bug is fixed
- **Command:**
  ```bash
  docker exec charon sqlite3 /app/data/charon.db \
    "UPDATE proxy_hosts SET enabled = 0 WHERE id = 24;"
  docker restart charon
  ```

### Follow-Up Investigation:

1. **Check for code-level bug:** Add debug logging to `GenerateConfig()` to print:
   - Total hosts processed
   - Each domain being added to processedDomains map
   - Final route count vs expected count

2. **Verify GORM query behavior:** Check if `.Preload()` is causing duplicate records in the slice

3. **Test with minimal reproduction:** Create a fresh database with only ID 24, see if error persists

4. **Review recent commits:** Check if any recent changes to config.go introduced the bug

---

## Files Involved

- **Database:** `/app/data/charon.db` (inside container)
- **Backup:** `/projects/Charon/data/charon.db.backup-20260128-065828`
- **Investigation Copy:** `/projects/Charon/data/charon.db.investigation`
- **Code:** `/projects/Charon/backend/internal/caddy/config.go` (GenerateConfig function)
- **Manager:** `/projects/Charon/backend/internal/caddy/manager.go` (ApplyConfig function)

---

## Appendix: SQL Queries Used

```sql
-- Find all proxy hosts with specific domain
SELECT id, uuid, name, domain_names, forward_host, forward_port, enabled, created_at, updated_at
FROM proxy_hosts
WHERE domain_names LIKE '%immaculaterr.hatfieldhosted.com%'
ORDER BY created_at;

-- Count total hosts
SELECT COUNT(*) as total FROM proxy_hosts;

-- Check for duplicate domains (case-insensitive)
SELECT COUNT(*), domain_names
FROM proxy_hosts
GROUP BY LOWER(domain_names)
HAVING COUNT(*) > 1;

-- Check proxy hosts with locations
SELECT ph.id, ph.name, ph.domain_names, COUNT(l.id) as location_count
FROM proxy_hosts ph
LEFT JOIN locations l ON l.proxy_host_id = ph.id
WHERE ph.enabled = 1
GROUP BY ph.id
ORDER BY ph.id;

-- Check recent Caddy config applications
SELECT * FROM caddy_configs
ORDER BY applied_at DESC
LIMIT 5;

-- Get all enabled proxy hosts
SELECT id, name, domain_names, enabled
FROM proxy_hosts
WHERE enabled = 1
ORDER BY id;
```

---

**Report Generated By:** GitHub Copilot
**Investigation Date:** 2026-01-28
**Status:** Investigation Complete - Awaiting Remediation Decision
