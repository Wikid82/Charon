# SSRF Protection Manual Test Plan

**Issue Tracking**: Manual QA Verification for SSRF Remediation
**Status**: Ready for QA
**Priority**: HIGH
**Related**: [ssrf-protection.md](../security/ssrf-protection.md)

---

## Prerequisites

- Charon instance running (Docker or local)
- Admin credentials
- Access to API endpoints
- cURL or similar HTTP client

---

## Test Cases

### 1. Private IP Blocking (RFC 1918)

| Test ID | Input URL | Expected Result |
|---------|-----------|-----------------|
| SSRF-001 | `http://10.0.0.1/webhook` | ❌ Blocked: "private IP address" |
| SSRF-002 | `http://10.255.255.255/webhook` | ❌ Blocked |
| SSRF-003 | `http://172.16.0.1/webhook` | ❌ Blocked |
| SSRF-004 | `http://172.31.255.255/webhook` | ❌ Blocked |
| SSRF-005 | `http://192.168.0.1/webhook` | ❌ Blocked |
| SSRF-006 | `http://192.168.255.255/webhook` | ❌ Blocked |

**Command**:

```bash
curl -X POST http://localhost:8080/api/v1/settings/test-url \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"url": "http://10.0.0.1/webhook"}'
```

**Expected Response**: HTTP 400 with error containing "private IP"

---

### 2. Localhost Blocking

| Test ID | Input URL | Expected Result |
|---------|-----------|-----------------|
| SSRF-010 | `http://127.0.0.1/admin` | ❌ Blocked: "localhost" |
| SSRF-011 | `http://127.0.0.2/admin` | ❌ Blocked |
| SSRF-012 | `http://localhost/admin` | ❌ Blocked |
| SSRF-013 | `http://localhost:8080/api` | ❌ Blocked |
| SSRF-014 | `http://[::1]/admin` | ❌ Blocked |

**Expected Response**: HTTP 400 with error containing "localhost"

---

### 3. Cloud Metadata Blocking

| Test ID | Input URL | Expected Result |
|---------|-----------|-----------------|
| SSRF-020 | `http://169.254.169.254/` | ❌ Blocked: "private IP" |
| SSRF-021 | `http://169.254.169.254/latest/meta-data/` | ❌ Blocked |
| SSRF-022 | `http://169.254.169.254/latest/meta-data/iam/security-credentials/` | ❌ Blocked |
| SSRF-023 | `http://169.254.0.1/` | ❌ Blocked (link-local range) |

**Command**:

```bash
curl -X POST http://localhost:8080/api/v1/settings/test-url \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"url": "http://169.254.169.254/latest/meta-data/"}'
```

---

### 4. Legitimate External URLs

| Test ID | Input URL | Expected Result |
|---------|-----------|-----------------|
| SSRF-030 | `https://httpbin.org/post` | ✅ Allowed |
| SSRF-031 | `https://hooks.slack.com/services/test` | ✅ Allowed (may 404) |
| SSRF-032 | `https://api.github.com/` | ✅ Allowed |
| SSRF-033 | `https://example.com/webhook` | ✅ Allowed |

**Expected Response**: HTTP 200 with `reachable: true` or network error (not SSRF block)

---

### 5. Protocol Bypass Attempts

| Test ID | Input URL | Expected Result |
|---------|-----------|-----------------|
| SSRF-040 | `file:///etc/passwd` | ❌ Blocked: "HTTP or HTTPS" |
| SSRF-041 | `ftp://internal.server/file` | ❌ Blocked |
| SSRF-042 | `gopher://localhost:25/` | ❌ Blocked |
| SSRF-043 | `data:text/html,<script>` | ❌ Blocked |

---

### 6. IPv6-Mapped IPv4 Blocking

| Test ID | Input URL | Expected Result |
|---------|-----------|-----------------|
| SSRF-050 | `http://[::ffff:127.0.0.1]/` | ❌ Blocked |
| SSRF-051 | `http://[::ffff:10.0.0.1]/` | ❌ Blocked |
| SSRF-052 | `http://[::ffff:192.168.1.1]/` | ❌ Blocked |
| SSRF-053 | `http://[::ffff:169.254.169.254]/` | ❌ Blocked |

---

### 7. Redirect Protection

| Test ID | Scenario | Expected Result |
|---------|----------|-----------------|
| SSRF-060 | URL redirects to 127.0.0.1 | ❌ Blocked at redirect |
| SSRF-061 | URL redirects > 2 times | ❌ Stopped after 2 redirects |
| SSRF-062 | URL redirects to private IP | ❌ Blocked |

**Test Setup**: Use httpbin.org redirect:

```bash
# This should be blocked if final destination is private
curl -X POST http://localhost:8080/api/v1/settings/test-url \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"url": "https://httpbin.org/redirect-to?url=http://127.0.0.1/"}'
```

---

### 8. Webhook Configuration Endpoints

| Test ID | Endpoint | Payload | Expected |
|---------|----------|---------|----------|
| SSRF-070 | `POST /api/v1/settings/security/webhook` | `{"webhook_url": "http://10.0.0.1/"}` | ❌ 400 |
| SSRF-071 | `POST /api/v1/notifications/custom-webhook` | `{"webhook_url": "http://192.168.1.1/"}` | ❌ 400 |
| SSRF-072 | `POST /api/v1/settings/security/webhook` | `{"webhook_url": "https://hooks.slack.com/test"}` | ✅ 200 |

---

## Verification Checklist

- [ ] All private IP ranges blocked (10.x, 172.16-31.x, 192.168.x)
- [ ] Localhost/loopback blocked (127.x, ::1)
- [ ] Cloud metadata blocked (169.254.169.254)
- [ ] Link-local blocked (169.254.x.x, fe80::)
- [ ] Invalid schemes blocked (file, ftp, gopher, data)
- [ ] IPv6-mapped IPv4 blocked
- [ ] Redirect to private IP blocked
- [ ] Legitimate external URLs allowed
- [ ] Error messages don't leak internal details

---

## Pass Criteria

- All SSRF-0xx tests marked "Blocked" return HTTP 400
- All SSRF-0xx tests marked "Allowed" return HTTP 200 or non-security error
- No test reveals internal IP addresses or hostnames in error messages

---

**Last Updated**: December 31, 2025
