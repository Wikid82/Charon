# WAF Integration Test Fix Plan

## Status: Pending Implementation

## Problem Summary

The WAF integration test (`scripts/coraza_integration.sh`) fails with HTTP 401 because the proxy host creation endpoint requires authentication, but the script attempts to create the proxy host **before** registering and logging in.

## Root Cause Analysis

### Current Flow (Broken)

Looking at the script execution order:

1. **Lines 175-200**: Creates proxy host without authentication
   - `curl -s -X POST ... http://localhost:8080/api/v1/proxy-hosts` (no cookie)
   - Returns HTTP 401 Unauthorized

2. **Lines 202-210**: Registers user and logs in (too late)
   - Creates `TMP_COOKIE` file
   - Successfully authenticates

3. **Lines 217-227**: Creates WAF ruleset (correctly uses cookie)
   - Uses `-b ${TMP_COOKIE}` ✓

### Evidence from CI Logs

```
{"client":"172.18.0.1","latency":"433.811µs","level":"info","method":"POST","msg":"handled request","path":"/api/v1/proxy-hosts","request_id":"26716960-4547-496b-8271-2acdcdda9872","status":401}
```

The 401 status confirms the proxy host endpoint now requires authentication.

## Required Changes

### 1. Move Authentication Before Proxy Host Creation

The user registration and login block (currently lines 207-210) must be moved **before** the proxy host creation (currently lines 175-200).

### 2. Add Cookie to Proxy Host Creation

The `CREATE_RESP` curl command on line 188 needs `-b ${TMP_COOKIE}` added.

### 3. Add Cookie to Fallback Update Command

The fallback `curl -s -X PUT` command on line 195 needs `-b ${TMP_COOKIE}` added.

### 4. Add Cookie to Unauthenticated Proxy Host List

The `curl -s http://localhost:8080/api/v1/proxy-hosts` on line 191 needs `-b ${TMP_COOKIE}` added.

## Detailed Line Changes

### Step 1: Add Authentication Block After API Ready Check (After Line 146)

Insert the following after the API ready check loop and **before** the proxy host creation:

```bash
echo "Registering admin user and logging in to retrieve session cookie..."
TMP_COOKIE=$(mktemp)
curl -s -X POST -H "Content-Type: application/json" -d '{"email":"integration@example.local","password":"password123","name":"Integration Tester"}' http://localhost:8080/api/v1/auth/register >/dev/null || true
curl -s -X POST -H "Content-Type: application/json" -d '{"email":"integration@example.local","password":"password123"}' -c ${TMP_COOKIE} http://localhost:8080/api/v1/auth/login >/dev/null
```

### Step 2: Remove Duplicate Authentication Block (Lines 207-210)

Delete or comment out the existing authentication block that appears after proxy host creation:

```bash
# REMOVE THESE LINES:
echo "Registering admin user and logging in to retrieve session cookie..."
TMP_COOKIE=$(mktemp)
curl -s -X POST -H "Content-Type: application/json" -d '{"email":"integration@example.local","password":"password123","name":"Integration Tester"}' http://localhost:8080/api/v1/auth/register >/dev/null || true
curl -s -X POST -H "Content-Type: application/json" -d '{"email":"integration@example.local","password":"password123"}' -c ${TMP_COOKIE} http://localhost:8080/api/v1/auth/login >/dev/null
```

### Step 3: Add Cookie to Proxy Host Creation (Line 188)

Change:

```bash
CREATE_RESP=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" -d "${PROXY_HOST_PAYLOAD}" http://localhost:8080/api/v1/proxy-hosts)
```

To:

```bash
CREATE_RESP=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" -d "${PROXY_HOST_PAYLOAD}" -b ${TMP_COOKIE} http://localhost:8080/api/v1/proxy-hosts)
```

### Step 4: Add Cookie to Proxy Host List (Line 191)

Change:

```bash
EXISTING_UUID=$(curl -s http://localhost:8080/api/v1/proxy-hosts | grep -o '{[^}]*"domain_names":"integration.local"[^}]*}' | head -n1 | grep -o '"uuid":"[^"]*"' | sed 's/"uuid":"\([^"]*\)"/\1/')
```

To:

```bash
EXISTING_UUID=$(curl -s -b ${TMP_COOKIE} http://localhost:8080/api/v1/proxy-hosts | grep -o '{[^}]*"domain_names":"integration.local"[^}]*}' | head -n1 | grep -o '"uuid":"[^"]*"' | sed 's/"uuid":"\([^"]*\)"/\1/')
```

### Step 5: Add Cookie to Proxy Host Update (Line 195)

Change:

```bash
curl -s -X PUT -H "Content-Type: application/json" -d "${PROXY_HOST_PAYLOAD}" http://localhost:8080/api/v1/proxy-hosts/$EXISTING_UUID
```

To:

```bash
curl -s -X PUT -H "Content-Type: application/json" -d "${PROXY_HOST_PAYLOAD}" -b ${TMP_COOKIE} http://localhost:8080/api/v1/proxy-hosts/$EXISTING_UUID
```

## Corrected Flow

After the fix, the script will execute in this order:

1. Build/start containers
2. Wait for API ready
3. **Register user and login** (create TMP_COOKIE)
4. Start httpbin backend container
5. **Create proxy host WITH cookie**
6. Create WAF ruleset with cookie
7. Enable WAF globally with cookie
8. Run WAF tests
9. Cleanup

## Verification

After implementing the fix, the test should:

1. Return HTTP 201 (or 200 for update) for proxy host creation
2. Proceed to WAF ruleset creation successfully
3. Complete the full BLOCK mode and MONITOR mode tests

## Related Files

- `scripts/coraza_integration.sh` - Main integration test script
- `.github/skills/scripts/skill-runner.sh` - Skill runner that invokes the test

## Notes

- The script already correctly uses authentication for:
  - WAF ruleset creation (line 218)
  - Security config updates (lines 223, 274)
  - Proxy host deletion in cleanup (line 294)
- Only the proxy host creation and related fallback commands were missing authentication
