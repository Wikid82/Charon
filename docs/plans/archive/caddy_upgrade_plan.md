# Caddy v2.11.0-beta.2 Upgrade Plan

**Created:** 2026-01-06
**Risk Level:** LOW
**Estimated Duration:** 30-45 minutes

## Overview

Upgrade Caddy from v2.10.2 to v2.11.0-beta.2 to gain:

- Built-in quic-go v0.58.0 (removes need for CVE patch)
- Built-in smallstep/certificates v0.29.0 (removes need for manual patch)
- Various bug fixes and enhancements

---

## Phase 1: Dockerfile Changes

**File:** `/projects/Charon/Dockerfile`

### 1.1 Update Caddy Version

Change line ~17:

```dockerfile
# FROM:
ARG CADDY_VERSION=2.10.2

# TO:
ARG CADDY_VERSION=2.11.0-beta.2
```

### 1.2 Remove Obsolete Dependency Patches

In the Caddy builder stage (~line 108-115), remove these patches that are now included upstream:

```dockerfile
# REMOVE these lines:
# renovate: datasource=go depName=github.com/quic-go/quic-go
go get github.com/quic-go/quic-go@v0.57.1; \
# renovate: datasource=go depName=github.com/smallstep/certificates
go get github.com/smallstep/certificates@v0.29.0; \
```

**KEEP this patch** (still required):

```dockerfile
# renovate: datasource=go depName=github.com/expr-lang/expr
go get github.com/expr-lang/expr@v1.17.7; \
```

### 1.3 Update Comments

Update the version comment block (~lines 9-17) to reflect the beta version.

---

## Phase 2: Build Verification

### 2.1 Build Docker Image

```bash
docker build --no-cache -t charon:caddy-upgrade-test .
```

### 2.2 Verify Caddy Starts

```bash
docker run --rm charon:caddy-upgrade-test caddy version
```

Expected output should show `v2.11.0-beta.2`.

### 2.3 Verify Plugins Load

```bash
docker run --rm charon:caddy-upgrade-test caddy list-modules | grep -E "security|coraza|crowdsec|maxmind|rate"
```

Expected plugins:

- `http.handlers.crowdsec`
- `http.handlers.waf` (coraza)
- `http.matchers.maxminddb`
- `http.handlers.rate_limit`
- `security` (caddy-security)

---

## Phase 3: Testing

### 3.1 Backend Unit Tests

```bash
# Using existing task
# Task: "Test: Backend Unit Tests"
cd backend && go test ./... -v
```

### 3.2 Integration Tests

```bash
# Start the container
docker compose -f .docker/compose/docker-compose.local.yml up -d

# Run Coraza WAF tests
# Task: "Integration: Coraza WAF"

# Run CrowdSec tests
# Task: "Integration: CrowdSec"
```

### 3.3 Manual Verification Checklist

- [ ] Caddy health endpoint responds: `curl http://localhost:2019/config/`
- [ ] Config reload works: `curl -X POST http://localhost:2019/load -H "Content-Type: application/json" -d @test-config.json`
- [ ] HTTPS/certificate automation works (if applicable)
- [ ] WAF rules trigger correctly
- [ ] CrowdSec bouncer integration works

---

## Phase 4: Documentation

### 4.1 Update CHANGELOG.md

Add entry under next release:

```markdown
### Changed
- Upgraded Caddy from v2.10.2 to v2.11.0-beta.2
- Removed manual quic-go and smallstep/certificates patches (now included upstream)
```

### 4.2 Update Version References

Search and update any version references:

```bash
grep -r "2.10.2" docs/
```

---

## Rollback Plan

If issues are encountered:

1. Revert `ARG CADDY_VERSION` to `2.10.2`
2. Restore the removed dependency patches
3. Rebuild the image

---

## Post-Upgrade Monitoring

After deployment:

- Monitor Caddy logs for errors: `docker logs -f <container> 2>&1 | grep -i caddy`
- Check certificate renewal works
- Verify no performance regressions
