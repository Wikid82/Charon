# Alpine Base Image Migration Specification

**Version:** 1.0
**Created:** February 4, 2026
**Status:** Planning Phase
**Estimated Effort:** 40-60 hours (2-3 sprints)
**Priority:** High (Security Optimization)

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Research Phase](#research-phase)
3. [Compatibility Analysis](#compatibility-analysis)
4. [Dockerfile Changes](#dockerfile-changes)
5. [Testing Requirements](#testing-requirements)
6. [Rollback Plan](#rollback-plan)
7. [Implementation Phases](#implementation-phases)
8. [Risk Assessment](#risk-assessment)
9. [Success Metrics](#success-metrics)
10. [Post-Migration Monitoring](#post-migration-monitoring)

---

## Executive Summary

### Context

**Current State:**
- Base Image: `debian:trixie-slim` (Debian 13)
- Security Issues: 7 HIGH CVEs in glibc/libtasn1 (no fixes available)
- Image Size: ~350MB final image
- Attack Surface: glibc, apt ecosystem

**Historical Context:**
- Previously migrated from Alpine → Debian due to **CVE-2025-60876** (busybox heap overflow - CRITICAL)
- CVE-2025-60876 status as of Feb 2026: Likely patched (requires verification)
- Debian CVE situation worsening: 7 HIGH CVEs with "no fix available"

**Migration Driver:**
- Reduce attack surface (musl libc vs glibc)
- Smaller base image (~5MB Alpine vs ~120MB Debian base)
- Faster security updates from Alpine Security Team
- User roadmap request (identified as priority)

### Goals

- ✅ Eliminate Debian glibc HIGH CVEs
- ✅ Reduce Docker image size by 30-40%
- ✅ Maintain 100% feature parity
- ✅ Achieve <5% performance variance
- ✅ Pass all E2E and integration tests

### Non-Goals

- ❌ Rewrite Go code for Alpine-specific optimizations
- ❌ Change application architecture
- ❌ Migrate to Distroless (considered but rejected for complexity)

---

## Research Phase

### 1.1 Alpine Security Posture Analysis

#### Historical Critical CVE: CVE-2025-60876

**Original Issue (Debian Migration Trigger):**
- **CVE ID:** CVE-2025-60876
- **Severity:** MEDIUM (originally reported as CRITICAL)
- **Affected:** busybox 1.37.0-r20, busybox-binsh 1.37.0-r20, ssl_client 1.37.0-r20
- **Type:** Heap buffer overflow (CWE-122)
- **Date Discovered:** January 2026

**Current Status (February 2026):**
- ✅ **LIKELY PATCHED** - Alpine Security typically patches within 2-4 weeks for CRITICAL/HIGH
- ⚠️ **VERIFICATION REQUIRED** - Must confirm patch before migration
- 📊 **Verification Method:** Check Alpine Security Advisory page + scan Alpine 3.23.x with Grype
- 🔗 **Source:** https://security.alpinelinux.org/vuln/busybox

**Verification Command:**
```bash
# Test Alpine 3.23 latest security posture
docker run --rm alpine:3.23 /bin/sh -c "apk info busybox"
grype alpine:3.23 --only-fixed --fail-on critical,high
```

**Expected Result:** Zero HIGH/CRITICAL CVEs in busybox packages

#### Current Alpine 3.23 Security State

**Latest Version:** alpine:3.23.3 (as of Feb 2026)

**Known Vulnerabilities (as of January 2026 scan):**
- **Busybox CVE-2025-60876:** MEDIUM (heap overflow) - Status: PENDING VERIFICATION
- **Curl CVE-2025-15079:** MEDIUM (HTTP/2 DoS) - Status: PENDING VERIFICATION
- **Curl CVE-2025-14819:** MEDIUM (TLS validation) - Status: PENDING VERIFICATION

**Alpine vs Debian CVE Comparison:**

| Metric | Alpine 3.23 (Jan 2026) | Debian Trixie (Feb 2026) |
|--------|------------------------|--------------------------|
| CRITICAL CVEs | 0 | 0 |
| HIGH CVEs | 0 (unverified) | **7** (glibc, libtasn1) |
| MEDIUM CVEs | 8 (busybox, curl) | 20 |
| Patch Availability | Pending verification | ❌ No fixes available |
| C Library | musl (immune to glibc CVEs) | glibc (7 HIGH CVEs) |
| Package Manager | apk (smaller, simpler) | apt (complex, larger) |
| Base Image Size | ~7MB | ~120MB |

**Recommendation:** Alpine 3.23.3+ expected to have significantly better security posture than Debian Trixie

#### Alpine Version Selection

**Candidates:**

1. **alpine:3.23.3** (Recommended - Stable)
   - ✅ Latest stable Alpine release
   - ✅ Long-term support through 2026-11
   - ✅ Mature ecosystem, well-tested
   - ✅ Renovate can track minor updates (3.23.x)
   - ⚠️ Must verify busybox CVE is patched

2. **alpine:edge** (Not Recommended - Rolling)
   - ⚠️ Rolling release, unstable
   - ⚠️ Breaking changes without warning
   - ⚠️ Not suitable for production
   - ❌ Rejected for reliability concerns

3. **alpine:3.22** (Not Recommended - Older)
   - ❌ Older packages, higher CVE risk
   - ❌ End-of-life approaching (Nov 2026)
   - ❌ Rejected for security reasons

**Decision:** Use **`alpine:3.23@sha256:...`** with Renovate tracking

#### musl vs glibc Compatibility

**Charon Application Profile:**
- **Language:** go 1.25.7 (static binaries with CGO_ENABLED=1 for SQLite)
- **C Dependencies:** SQLite (libsqlite3-dev)
- **Go Stdlib Features:** Standard library calls only (net, crypto, http)

**musl Compatibility Assessment:**

| Component | Debian (glibc) | Alpine (musl) | Compatibility Risk |
|-----------|---------------|--------------|-------------------|
| Go Runtime | ✅ glibc-friendly | ✅ musl-friendly | 🟢 **LOW** - Go abstracts libc |
| SQLite (CGO) | ✅ Built with glibc | ✅ Built with musl | 🟢 **LOW** - API compatible |
| Caddy Server | ✅ Built with glibc | ✅ Built with musl | 🟢 **LOW** - Go binary, static |
| CrowdSec | ✅ Built with glibc | ✅ Built with musl | 🟢 **LOW** - Go binary, static |
| gosu | ✅ Built from source | ✅ Built from source | 🟢 **LOW** - Go binary |
| DNS Resolution | ✅ glibc NSS | ⚠️ musl resolver | 🟡 **MEDIUM** - See below |

**DNS Resolution Differences:**

**glibc (Debian):**
- Uses Name Service Switch (NSS) from `/etc/nsswitch.conf`
- Supports complex resolution order (DNS, mDNS, LDAP, etc.)
- Go's `net` package uses cgo DNS resolver by default

**musl (Alpine):**
- Simple resolver, reads `/etc/resolv.conf` directly
- No NSS support (no `/etc/nsswitch.conf`)
- Faster, simpler, but less flexible

**Impact on Charon:**
- 🟢 **Minimal** - Charon only does standard DNS queries (A/AAAA records)
- 🟢 **Go DNS Fallback** - Set `GODEBUG=netdns=go` to use pure Go resolver (no cgo)
- ⚠️ **Test Required** - DNS provider integrations (Cloudflare, Route53, etc.) must be re-tested

**Mitigation:**
```dockerfile
# Force Go to use pure Go DNS resolver (no cgo)
ENV GODEBUG=netdns=go
```

**Reference:**
- Go DNS Resolver: https://pkg.go.dev/net#hdr-Name_Resolution
- musl DNS Limitations: https://wiki.musl-libc.org/functional-differences-from-glibc.html

### 1.2 Package Ecosystem Research

**Research Tool:**
```bash
# Analyze Debian packages currently used
docker run --rm debian:trixie-slim dpkg -l | grep ^ii

# Search Alpine equivalents
docker run --rm alpine:3.23 apk search <package>
```

---

## Compatibility Analysis

### 2.1 Package Mapping: Debian apt → Alpine apk

#### Build Stage Packages (gosu-builder)

| Debian Package | Alpine Equivalent | Status | Notes |
|----------------|------------------|--------|-------|
| `git` | `git` | ✅ Direct match | Same package name |
| `clang` | `clang` | ✅ Direct match | LLVM toolchain |
| `lld` | `lld` | ✅ Direct match | LLVM linker |
| `gcc` | `gcc` | ✅ Direct match | GNU Compiler |
| `libc6-dev` | `musl-dev` | ⚠️ Different | musl development headers |

**Build Script Changes:**
```diff
- RUN apt-get update && apt-get install -y --no-install-recommends \
-    git clang lld && \
-    rm -rf /var/lib/apt/lists/*
- RUN xx-apt install -y gcc libc6-dev
+ RUN apk add --no-cache git clang lld
+ RUN xx-apk add gcc musl-dev
```

#### Build Stage Packages (backend-builder)

| Debian Package | Alpine Equivalent | Status | Notes |
|----------------|------------------|--------|-------|
| `clang` | `clang` | ✅ Direct match | |
| `lld` | `lld` | ✅ Direct match | |
| `gcc` | `gcc` | ✅ Direct match | |
| `libc6-dev` | `musl-dev` | ⚠️ Different | musl headers |
| `libsqlite3-dev` | `sqlite-dev` | ✅ Direct match | SQLite development |

**Build Script Changes:**
```diff
- RUN apt-get update && apt-get install -y --no-install-recommends \
-    clang lld && \
-    rm -rf /var/lib/apt/lists/*
- RUN xx-apt install -y gcc libc6-dev libsqlite3-dev
+ RUN apk add --no-cache clang lld
+ RUN xx-apk add gcc musl-dev sqlite-dev
```

#### Build Stage Packages (caddy-builder)

| Debian Package | Alpine Equivalent | Status | Notes |
|----------------|------------------|--------|-------|
| `git` | `git` | ✅ Direct match | xcaddy requires git |

**Build Script Changes:**
```diff
- RUN apt-get update && apt-get install -y --no-install-recommends git && \
-    rm -rf /var/lib/apt/lists/*
+ RUN apk add --no-cache git
```

#### Build Stage Packages (crowdsec-builder)

| Debian Package | Alpine Equivalent | Status | Notes |
|----------------|------------------|--------|-------|
| `git` | `git` | ✅ Direct match | |
| `clang` | `clang` | ✅ Direct match | |
| `lld` | `lld` | ✅ Direct match | |
| `gcc` | `gcc` | ✅ Direct match | |
| `libc6-dev` | `musl-dev` | ⚠️ Different | |

**Build Script Changes:**
```diff
- RUN apt-get update && apt-get install -y --no-install-recommends \
-    git clang lld && \
-    rm -rf /var/lib/apt/lists/*
- RUN xx-apt install -y gcc libc6-dev
+ RUN apk add --no-cache git clang lld
+ RUN xx-apk add gcc musl-dev
```

#### Build Stage Packages (crowdsec-fallback)

| Debian Package | Alpine Equivalent | Status | Notes |
|----------------|------------------|--------|-------|
| `curl` | `curl` | ✅ Direct match | |
| `ca-certificates` | `ca-certificates` | ✅ Direct match | |
| `tar` | `tar` | ✅ Direct match | Alpine has tar built-in (busybox) |

**Build Script Changes:**
```diff
# Note: Debian slim does NOT include tar by default - must be explicitly installed
- RUN apt-get update && apt-get install -y --no-install-recommends \
-    curl ca-certificates tar && \
-    rm -rf /var/lib/apt/lists/*
+ RUN apk add --no-cache curl ca-certificates
# Note: tar is already available in Alpine via busybox
```

#### Runtime Stage Packages (Final Image)

| Debian Package | Alpine Equivalent | Status | Notes |
|----------------|------------------|--------|-------|
| `bash` | `bash` | ✅ Direct match | Maintenance scripts require bash |
| `ca-certificates` | `ca-certificates` | ✅ Direct match | SSL certificates |
| `libsqlite3-0` | `sqlite-libs` | ⚠️ Different | SQLite runtime library |
| `sqlite3` | `sqlite` | ⚠️ Different | SQLite CLI tool |
| `tzdata` | `tzdata` | ✅ Direct match | Timezone database |
| `curl` | `curl` | ✅ Direct match | Healthchecks, scripts |
| `gettext-base` | `gettext` | ⚠️ Different | envsubst for templates |
| `libcap2-bin` | `libcap` | ⚠️ Different | setcap for Caddy ports |
| `libc-ares2` | `c-ares` | ⚠️ Different | DNS resolution library |
| `binutils` | `binutils` | ✅ Direct match | objdump for debug symbol check |

**Runtime Script Changes:**
```diff
- RUN apt-get update && apt-get install -y --no-install-recommends \
-    bash ca-certificates libsqlite3-0 sqlite3 tzdata curl gettext-base libcap2-bin libc-ares2 binutils && \
-    apt-get upgrade -y && \
-    rm -rf /var/lib/apt/lists/*
+ RUN apk add --no-cache \
+    bash ca-certificates sqlite-libs sqlite tzdata curl gettext libcap c-ares binutils
```

### 2.2 Critical Integration Points

#### 1. CGO-Enabled SQLite

**Current Build (Debian):**
```dockerfile
RUN CGO_ENABLED=1 xx-go build \
    -ldflags "-s -w" \
    -o charon ./cmd/api
```

**Alpine Consideration:**
- ✅ **Compatible** - SQLite compiled against musl libc
- ✅ **No Code Changes** - Go's `mattn/go-sqlite3` driver is libc-agnostic
- ⚠️ **Test Required** - Database operations (CRUD, migrations, backups)

**Validation Test:**
```bash
# After Alpine build, verify SQLite functionality
docker exec charon sqlite3 /app/data/charon.db "PRAGMA integrity_check;"
# Expected: ok
```

#### 2. Network Calls (DNS Resolution)

**Current Behavior (Debian):**
- Go's `net` package uses cgo DNS resolver by default
- Queries `/etc/nsswitch.conf` then falls back to `/etc/resolv.conf`
- Supports mDNS, LDAP, custom NSS modules

**Alpine Behavior:**
- musl libc has no NSS support
- DNS queries go directly to `/etc/resolv.conf`
- Simpler, faster, but less flexible

**Impact Assessment:**

| Feature | Risk Level | Test Required |
|---------|-----------|---------------|
| ACME DNS-01 Challenge | 🟡 MEDIUM | ✅ Test all 15 DNS providers |
| Docker Host Resolution | 🟢 LOW | ✅ Test `host.docker.internal` |
| Webhook URLs | 🟢 LOW | ✅ Test external webhook delivery |
| CrowdSec LAPI | 🟢 LOW | ✅ Test `127.0.0.1:8085` connectivity |

**Mitigation Strategy:**
```dockerfile
# Force Go to use pure Go DNS resolver (bypass cgo)
ENV GODEBUG=netdns=go
```

**Reference:** https://pkg.go.dev/net#hdr-Name_Resolution

#### 3. TLS/SSL Certificates

**Current (Debian):**
- Uses glibc's certificate validation
- System certificates: `/etc/ssl/certs/ca-certificates.crt`

**Alpine:**
- Uses musl + OpenSSL/LibreSSL
- System certificates: `/etc/ssl/certs/ca-certificates.crt` (same path)

**Impact:**
- 🟢 **No Changes Required** - Go's `crypto/tls` uses system cert pool via standard path
- ⚠️ **Test Required** - Let's Encrypt cert validation, webhook HTTPS calls

#### 4. Timezone Data

**Current (Debian):**
- Timezone database: `/usr/share/zoneinfo/`
- Package: `tzdata`

**Alpine:**
- Timezone database: `/usr/share/zoneinfo/`
- Package: `tzdata` (same structure)

**Impact:**
- 🟢 **No Changes Required** - Go's `time.LoadLocation()` uses standard paths

#### 5. Caddy Privileged Port Binding

**Current (Debian):**
- Uses `setcap` from `libcap2-bin` package
- Command: `setcap 'cap_net_bind_service=+ep' /usr/bin/caddy`

**Alpine:**
- Uses `setcap` from `libcap` package
- Same command syntax

**Build Script:**
```diff
# Runtime image - set Caddy capabilities
- RUN setcap 'cap_net_bind_service=+ep' /usr/bin/caddy
+ RUN setcap 'cap_net_bind_service=+ep' /usr/bin/caddy
# No change required - same command
```

#### 6. Shell Scripts (docker-entrypoint.sh)

**Current Dependencies:**
- `bash` shell
- `envsubst` (from `gettext-base`)
- `gosu` (privilege dropping)
- `curl` (healthchecks)

**Alpine Changes:**
```diff
- gettext-base  # Debian package name
+ gettext       # Alpine package name (includes envsubst)
```

**Test Required:**
- ✅ Container startup sequence
- ✅ CrowdSec initialization scripts
- ✅ Database migrations

### 2.3 Known Breaking Changes

#### None Identified

Alpine migration for Go applications is typically seamless due to:
1. Go's portable standard library
2. Static binaries (minimize libc surface area)
3. Similar package ecosystem (apk vs apt naming differences only)

**Confidence Level:** 🟢 **HIGH** (95%)

---

## Dockerfile Changes

### 3.1 Current Dockerfile Structure Analysis

**Multi-Stage Build Overview:**
1. **xx** - Cross-compilation helpers (`tonistiigi/xx`)
2. **gosu-builder** - Build gosu from source (Go 1.25)
3. **frontend-builder** - Build React frontend (Node 24)
4. **backend-builder** - Build Go backend (Go 1.25)
5. **caddy-builder** - Build Caddy with plugins (Go 1.25 + xcaddy)
6. **crowdsec-builder** - Build CrowdSec (Go 1.25)
7. **crowdsec-fallback** - Download CrowdSec static binaries (amd64 only)
8. **Final Runtime** - Debian Trixie-slim runtime image

**Total Stages:** 8
**Final Image Size (Current):** ~350MB

### 3.2 Proposed Alpine Dockerfile

**Changes Required:** Stages 2, 4, 5, 6, 7, 8

#### Stage 2: gosu-builder (Debian → Alpine)

**Before (Debian):**
```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.25-trixie AS gosu-builder
RUN apt-get update && apt-get install -y --no-install-recommends \
    git clang lld && \
    rm -rf /var/lib/apt/lists/*
RUN xx-apt install -y gcc libc6-dev
```

**After (Alpine):**
```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS gosu-builder
RUN apk add --no-cache git clang lld
RUN xx-apk add --no-cache gcc musl-dev
```

**Size Impact:** -15MB (Alpine base smaller)

#### Stage 4: backend-builder (Debian → Alpine)

**Before (Debian):**
```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.25-trixie AS backend-builder
RUN apt-get update && apt-get install -y --no-install-recommends \
    clang lld && \
    rm -rf /var/lib/apt/lists/*
RUN xx-apt install -y gcc libc6-dev libsqlite3-dev
```

**After (Alpine):**
```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS backend-builder
RUN apk add --no-cache clang lld
RUN xx-apk add --no-cache gcc musl-dev sqlite-dev
```

**Size Impact:** -10MB

#### Stage 5: caddy-builder (Debian → Alpine)

**Before (Debian):**
```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.25-trixie AS caddy-builder
RUN apt-get update && apt-get install -y --no-install-recommends git && \
    rm -rf /var/lib/apt/lists/*
```

**After (Alpine):**
```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS caddy-builder
RUN apk add --no-cache git
```

**Size Impact:** -8MB

#### Stage 6: crowdsec-builder (Debian → Alpine)

**Before (Debian):**
```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.25.6-trixie AS crowdsec-builder
RUN apt-get update && apt-get install -y --no-install-recommends \
    git clang lld && \
    rm -rf /var/lib/apt/lists/*
RUN xx-apt install -y gcc libc6-dev
```

**After (Alpine):**
```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.25.6-alpine AS crowdsec-builder
RUN apk add --no-cache git clang lld
RUN xx-apk add --no-cache gcc musl-dev
```

**Size Impact:** -12MB

#### Stage 7: crowdsec-fallback (Debian → Alpine)

**Before (Debian):**
```dockerfile
FROM debian:trixie-slim AS crowdsec-fallback
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl ca-certificates tar && \
    rm -rf /var/lib/apt/lists/*
```

**After (Alpine):**
```dockerfile
FROM alpine:3.23 AS crowdsec-fallback
RUN apk add --no-cache curl ca-certificates
# tar is already available via busybox
```

**Size Impact:** -100MB (Debian slim → Alpine base)

#### Stage 8: Final Runtime (Debian → Alpine)

**Before (Debian):**
```dockerfile
FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash ca-certificates libsqlite3-0 sqlite3 tzdata curl gettext-base libcap2-bin libc-ares2 binutils && \
    apt-get upgrade -y && \
    rm -rf /var/lib/apt/lists/*
```

**After (Alpine):**
```dockerfile
FROM alpine:3.23
RUN apk add --no-cache \
    bash ca-certificates sqlite-libs sqlite tzdata curl gettext libcap c-ares binutils
```

**Size Impact:** -100MB (Debian slim → Alpine runtime)

### 3.3 Complete Dockerfile Diff

**Summary of Changes:**
```diff
# Build Stages (golang base images)
- FROM --platform=$BUILDPLATFORM golang:1.25-trixie
+ FROM --platform=$BUILDPLATFORM golang:1.25-alpine

# Fallback Stage
- FROM debian:trixie-slim
+ FROM alpine:3.23

# Final Runtime Stage
- FROM debian:trixie-slim@sha256:...
+ FROM alpine:3.23@sha256:...

# Package Manager Commands
- RUN apt-get update && apt-get install -y --no-install-recommends \
-    <packages> && \
-    rm -rf /var/lib/apt/lists/*
+ RUN apk add --no-cache <packages>

# Cross-Compilation Package Install
- RUN xx-apt install -y gcc libc6-dev
+ RUN xx-apk add --no-cache gcc musl-dev

# Package Name Changes
- libsqlite3-dev → sqlite-dev
- libc6-dev → musl-dev
- gettext-base → gettext
- libsqlite3-0 → sqlite-libs
- libcap2-bin → libcap
- libc-ares2 → c-ares
```

**Lines Changed:** ~50 lines (out of ~450 total Dockerfile)

**Estimated Effort:** 4-6 hours (including testing)

### 3.4 Size Comparison (Estimated)

| Component | Debian Trixie | Alpine 3.23 | Savings |
|-----------|--------------|------------|---------|
| Base Image | 120MB | 7MB | -113MB |
| Build Stages | 850MB (intermediate) | 700MB (intermediate) | -150MB |
| **Final Runtime** | **~350MB** | **~220MB** | **-130MB (-37%)** |

**Note:** Final runtime size savings driven by:
1. Alpine base image (7MB vs 120MB)
2. Smaller runtime packages (musl vs glibc)
3. No apt cache/metadata

---

## Testing Requirements

### 4.1 Pre-Migration Verification Tests

#### Test 1: Alpine CVE Verification

**Objective:** Confirm CVE-2025-60876 (busybox) and related CVEs are patched

**Procedure:**
```bash
# Build test Alpine image with minimal packages
cat > Dockerfile.alpine-test << 'EOF'
FROM alpine:3.23
RUN apk add --no-cache busybox curl ca-certificates
EOF

docker build -t alpine-test:3.23 -f Dockerfile.alpine-test .

# Scan with Grype
grype alpine-test:3.23 --only-fixed --fail-on critical,high --output json \
  > alpine-3.23-scan.json

# Scan with Trivy
trivy image alpine-test:3.23 --severity CRITICAL,HIGH --exit-code 1
```

**Expected Result:**
- Zero CRITICAL or HIGH CVEs in busybox packages
- Grype exit code: 0
- Trivy exit code: 0

**Abort Criteria:** If CVE-2025-60876 still present, delay migration and escalate

**Timeline:** Before starting Phase 1 (blocking)

#### Test 2: Package Availability Check

**Objective:** Verify all required Alpine packages exist

**Procedure:**
```bash
# Check each package from compatibility analysis
docker run --rm alpine:3.23 sh -c "
  apk search bash && \
  apk search ca-certificates && \
  apk search sqlite-libs && \
  apk search sqlite && \
  apk search tzdata && \
  apk search curl && \
  apk search gettext && \
  apk search libcap && \
  apk search c-ares && \
  apk search binutils && \
  apk search gcc && \
  apk search musl-dev && \
  apk search sqlite-dev
"
```

**Expected Result:** All packages found with versions listed

**Abort Criteria:** Any package missing from Alpine repository

**Timeline:** Before Phase 1 (blocking)

### 4.2 Build-Time Testing

#### Test 3: Multi-Architecture Build

**Objective:** Verify Alpine Dockerfile builds successfully on amd64 and arm64

**Procedure:**
```bash
# Build for linux/amd64
docker buildx build --platform linux/amd64 \
  --build-arg VERSION=alpine-test \
  -t charon:alpine-amd64 \
  --load .

# Build for linux/arm64
docker buildx build --platform linux/arm64 \
  --build-arg VERSION=alpine-test \
  -t charon:alpine-arm64 \
  --load .
```

**Validation:**
```bash
# Verify binaries built correctly
docker run --rm charon:alpine-amd64 /app/charon version
docker run --rm charon:alpine-arm64 /app/charon version

# Verify libc linkage (should show musl)
docker run --rm charon:alpine-amd64 ldd /app/charon
# Expected: libc.musl-x86_64.so.1 or "statically linked"
```

**Expected Result:**
- Build succeeds on both architectures
- Binary reports correct version
- No glibc dependencies (musl only)

**Timeline:** Phase 1 - Week 1

#### Test 4: Image Size Verification

**Objective:** Confirm 30-40% size reduction

**Procedure:**
```bash
# Compare image sizes
docker images | grep "charon.*debian"
docker images | grep "charon.*alpine"

# Calculate savings
echo "Debian size: <debian-mb-size> MB"
echo "Alpine size: <alpine-mb-size> MB"
echo "Savings: $(( (<debian> - <alpine>) / <debian> * 100 ))%"
```

**Expected Result:**
- Alpine image 120-150MB smaller than Debian
- 30-40% size reduction achieved

**Timeline:** Phase 1 - Week 1

### 4.3 Runtime Testing (Docker Compose)

#### Test 5: Container Startup Sequence

**Objective:** Verify docker-entrypoint.sh executes successfully

**Procedure:**
```bash
# Start Alpine container with fresh data volume
docker-compose -f .docker/compose/docker-compose.alpine-test.yml up -d

# Watch startup logs
docker logs -f charon-alpine

# Expected log sequence:
# 1. Environment variable expansion
# 2. CrowdSec initialization
# 3. Database migrations
# 4. Backend API startup
# 5. Caddy proxy startup
# 6. Health check success
```

**Validation Checks:**
```bash
# Check all processes running
docker exec charon-alpine ps aux | grep -E "charon|caddy"

# Verify health check
curl http://localhost:8080/api/v1/health
# Expected: {"status":"ok"}

# Check database file permissions
docker exec charon-alpine ls -la /app/data/charon.db
# Expected: charon:charon ownership
```

**Expected Result:** Container starts successfully, all services running, health check passes

**Timeline:** Phase 2 - Week 2

#### Test 6: Database Operations

**Objective:** Verify SQLite CGO binding works with musl libc

**Procedure:**
```bash
# Create test proxy host via API
curl -X POST http://localhost:8080/api/v1/proxy-hosts \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "domain": "alpine-test.local",
    "target": "http://localhost:9000"
  }'

# Query database directly
docker exec charon-alpine sqlite3 /app/data/charon.db \
  "SELECT * FROM proxy_hosts WHERE domain='alpine-test.local';"

# Run database integrity check
docker exec charon-alpine sqlite3 /app/data/charon.db \
  "PRAGMA integrity_check;"
# Expected: ok

# Test migrations
docker exec charon-alpine /app/charon migrate
```

**Expected Result:**
- Proxy host created successfully
- Database queries return correct data
- Integrity check passes
- Migrations run without errors

**Timeline:** Phase 2 - Week 2

#### Test 7: DNS Resolution

**Objective:** Verify DNS queries work with musl libc resolver

**Procedure:**
```bash
# Test external DNS resolution
docker exec charon-alpine nslookup google.com
docker exec charon-alpine ping -c 1 google.com

# Test Docker internal DNS
docker exec charon-alpine nslookup host.docker.internal

# Test within Go application (backend)
curl -X POST http://localhost:8080/api/v1/test/dns \
  -d '{"hostname":"cloudflare.com"}'
```

**Expected Result:**
- External DNS resolves correctly
- Docker internal DNS works
- Go application DNS calls succeed

**Timeline:** Phase 2 - Week 2

### 4.4 E2E Testing (Playwright)

#### Test 8: Full E2E Test Suite

**Objective:** Verify 100% E2E test pass rate with Alpine image

**Procedure:**
```bash
# Start Alpine-based E2E environment
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e-alpine

# Run full Playwright test suite
npx playwright test --project=chromium --project=firefox --project=webkit

# Run with coverage
.github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage-alpine
```

**Test Coverage:**
- ✅ Proxy host CRUD operations (15 DNS provider types)
- ✅ Certificate provisioning (HTTP-01, DNS-01 challenges)
- ✅ Security settings (ACL, WAF, CrowdSec, Rate Limiting)
- ✅ User management (create, edit, delete users)
- ✅ Real-time log streaming (WebSocket)
- ✅ Docker container discovery
- ✅ Backup/restore operations
- ✅ Emergency recovery workflow

**Expected Result:**
- 100% test pass rate (544/544 tests passing)
- Zero timeout errors
- Zero element interaction failures
- Coverage matches Debian baseline (82-85%)

**Timeline:** Phase 3 - Week 2-3

#### Test 9: DNS Provider Integration Tests

**Objective:** Verify all 15 DNS provider plugins work with Alpine

**Providers to Test:**
1. Cloudflare (DNS-01)
2. Route53 (AWS DNS-01)
3. Google Cloud DNS
4. Azure DNS
5. DigitalOcean DNS
6. Linode DNS
7. Vultr DNS
8. Namecheap DNS
9. GoDaddy DNS
10. RFC2136 (BIND DNS)
11. Manual DNS
12. Webhook DNS (HTTP)
13. DuckDNS
14. acme-dns
15. PowerDNS

**Test Procedure (per provider):**
```bash
# Via E2E test
npx playwright test tests/dns-provider-{provider}.spec.ts

# Verification
docker exec charon-alpine curl http://localhost:2019/config/ | \
  jq '.apps.http.servers.srv0.tls_automation_policies[0].dns'
# Expected: Provider-specific configuration JSON
```

**Expected Result:** All 15 DNS provider tests pass

**Timeline:** Phase 3 - Week 2-3

### 4.5 Integration Testing (Go)

#### Test 10: Cerberus Security Suite

**Objective:** Verify security middleware functions correctly

**Procedure:**
```bash
# Run Cerberus integration tests
cd backend/integration
go test -v -tags=integration ./cerberus_integration_test.go

# Test WAF (Coraza)
go test -v -tags=integration ./coraza_integration_test.go

# Test CrowdSec
go test -v -tags=integration ./crowdsec_integration_test.go

# Test Rate Limiting
go test -v -tags=integration ./rate_limit_integration_test.go
```

**Expected Result:**
- All integration tests pass
- WAF blocks SQL injection/XSS payloads
- CrowdSec bans malicious IPs
- Rate limiting enforces thresholds (429 responses)

**Timeline:** Phase 3 - Week 3

#### Test 11: Backend Unit Tests

**Objective:** Ensure 85% code coverage maintained

**Procedure:**
```bash
# Run backend tests with coverage
cd backend
go test -v -cover -coverprofile=coverage.out ./...

# Generate coverage report
go tool cover -html=coverage.out -o coverage.html

# Verify threshold
go tool cover -func=coverage.out | tail -1
# Expected: total coverage >= 85%
```

**Expected Result:** Coverage ≥ 85%, all tests pass

**Timeline:** Phase 3 - Week 3

### 4.6 Performance Testing

#### Test 12: Request Latency Benchmark

**Objective:** Verify <5% performance variance vs Debian

**Procedure:**
```bash
# Debian baseline (existing image)
docker run -d --name charon-debian wikid82/charon:latest

# Alpine candidate
docker run -d --name charon-alpine charon:alpine-test

# Benchmark API endpoints (100 requests each)
for endpoint in /api/v1/proxy-hosts /api/v1/certificates /api/v1/users; do
  echo "Testing $endpoint"

  # Debian
  ab -n 100 -c 10 http://localhost:8080$endpoint > debian-$endpoint.txt

  # Alpine
  ab -n 100 -c 10 http://localhost:8081$endpoint > alpine-$endpoint.txt
done

# Compare results
grep "Time per request" debian-*.txt
grep "Time per request" alpine-*.txt
```

**Expected Result:**
- Alpine latency within 5% of Debian
- No significant regression in throughput (req/sec)

**Acceptable Variance:** ±5%

**Timeline:** Phase 4 - Week 3

#### Test 13: Memory Usage

**Objective:** Compare memory footprint

**Procedure:**
```bash
# Monitor memory usage over 1 hour
docker stats --no-stream charon-debian > debian-memory.txt
sleep 3600
docker stats --no-stream charon-debian >> debian-memory.txt

docker stats --no-stream charon-alpine > alpine-memory.txt
sleep 3600
docker stats --no-stream charon-alpine >> alpine-memory.txt

# Calculate average and peak
awk '{sum+=$2; peak=($2>peak)?$2:peak} END {print "Avg:", sum/NR, "MB | Peak:", peak, "MB"}' \
  debian-memory.txt alpine-memory.txt
```

**Expected Result:**
- Alpine memory usage similar or lower than Debian
- No memory leaks (stable usage over time)

**Timeline:** Phase 4 - Week 3

### 4.7 Security Testing

#### Test 14: CVE Scan (Final Alpine Image)

**Objective:** Confirm zero HIGH/CRITICAL CVEs in final image

**Procedure:**
```bash
# Scan with Grype
grype charon:alpine-test --fail-on critical,high --output sarif \
  > grype-alpine-final.sarif

# Scan with Trivy
trivy image charon:alpine-test --severity CRITICAL,HIGH --exit-code 1 \
  --format sarif > trivy-alpine-final.sarif

# Generate comparison report
diff <(jq -r '.runs[0].results[] | .ruleId' grype-debian.sarif) \
     <(jq -r '.runs[0].results[] | .ruleId' grype-alpine-final.sarif)
```

**Acceptance Criteria:**
- Zero CRITICAL CVEs
- Zero HIGH CVEs (or documented risk acceptance)
- Significant reduction vs Debian (7 HIGH → 0)

**Timeline:** Phase 5 - Week 4

#### Test 15: SBOM Verification

**Objective:** Generate Alpine SBOM and validate no unexpected dependencies

**Procedure:**
```bash
# Generate SBOM with Syft
syft charon:alpine-test -o cyclonedx-json > sbom-alpine.cyclonedx.json

# Compare base OS packages
jq -r '.components[] | select(.type=="operating-system") | .name' \
  sbom-debian.cyclonedx.json sbom-alpine.cyclonedx.json
```

**Expected Result:**
- No unexpected third-party dependencies
- Base OS: Alpine Linux 3.23.x
- All packages from Alpine repository

**Timeline:** Phase 5 - Week 4

### 4.8 Test Pass Criteria

**Blocking Issues (Must Pass):**
- ✅ Alpine CVE verification (Test 1)
- ✅ Multi-architecture build (Test 3)
- ✅ Container startup (Test 5)
- ✅ Database operations (Test 6)
- ✅ E2E test suite 100% pass (Test 8)
- ✅ Security CVE scan (Test 14)

**Non-Blocking Issues (Can Be Mitigated):**
- ⚠️ Performance regression <10% (Test 12) - Acceptable if justified
- ⚠️ DNS resolution edge cases (Test 7) - Can be fixed with `GODEBUG=netdns=go`

---

## Rollback Plan

### 5.1 Rollback Triggers

**When to Roll Back:**
1. **Critical E2E Test Failures:** >10% test failure rate that cannot be fixed within 48 hours
2. **Security Regression:** New CRITICAL CVE introduced in Alpine 3.23
3. **Performance Degradation:** >15% latency regression in production
4. **Data Loss Risk:** Database corruption or migration failures
5. **User-Facing Bug:** Production incident affecting >50% of users

### 5.2 Rollback Procedure

#### Step 1: Immediate Traffic Diversion (5 minutes)

```bash
# Stop Alpine container
docker-compose -f .docker/compose/docker-compose.yml down

# Revert docker-compose.yml to Debian image
git checkout HEAD~1 .docker/compose/docker-compose.yml

# Start Debian container
docker-compose -f .docker/compose/docker-compose.yml up -d
```

#### Step 2: Data Backup Validation (10 minutes)

```bash
# Verify latest backup integrity
docker exec charon-debian sqlite3 /app/data/charon.db "PRAGMA integrity_check;"

# Restore from pre-Alpine backup if needed
docker exec charon-debian /app/scripts/db-recovery.sh \
  /app/data/backups/charon-pre-alpine-migration.db
```

#### Step 3: Health Verification (5 minutes)

```bash
# Check health endpoints
curl http://localhost:8080/api/v1/health

# Verify proxy routing
curl -H "Host: test.example.com" http://localhost

# Check logs for errors
docker logs charon-debian | grep -i error
```

**Total Rollback Time:** < 20 minutes

### 5.3 Post-Rollback Actions

1. **Incident Report:** Document root cause of rollback
2. **User Communication:** Notify users of temporary Debian revert
3. **Issue Creation:** File GitHub issue with rollback details
4. **Root Cause Analysis:** RCA within 48 hours
5. **Fix Timeline:** Define timeline to address Alpine blockers

### 5.4 Rollback Testing (Pre-Migration)

**Pre-Migration Validation:**
```bash
# Practice rollback procedure in staging
docker-compose -f .docker/compose/docker-compose.alpine-staging.yml up -d
sleep 60

# Simulate rollback
docker-compose down
docker-compose -f .docker/compose/docker-compose.yml up -d

# Verify rollback success
curl http://localhost:8080/api/v1/health
```

**Timeline:** Phase 4 - Week 3 (before production deployment)

---

## Implementation Phases

### Phase 1: Research and Spike (Week 1 - 8 hours)

**Deliverables:**
- ✅ Alpine 3.23.3 CVE scan results (Test 1)
- ✅ Package availability verification (Test 2)
- ✅ Alpine test Dockerfile (proof-of-concept)
- ✅ Multi-architecture build validation (Test 3)

**Success Criteria:**
- Zero CRITICAL/HIGH CVEs in Alpine base image
- All required packages available
- PoC Dockerfile builds successfully on amd64 and arm64

**Timeline:** February 5-8, 2026

**Assignee:** DevOps Team

**Risks:**
- 🔴 **HIGH:** CVE-2025-60876 not patched → Delay migration
- 🟡 **MEDIUM:** Missing Alpine packages → Find alternatives
- 🟢 **LOW:** Build failures → Adjust Dockerfile syntax

**Mitigation:**
- Daily monitoring of Alpine Security Advisory
- Fallback to older Alpine version (3.22) if needed
- xx toolkit documentation: https://github.com/tonistiigi/xx

### Phase 2: Dockerfile Migration (Week 2 - 12 hours)

**Tasks:**
1. **Update all build stages to Alpine** (4 hours)
   - Replace `golang:1.25-trixie` with `golang:1.25-alpine`
   - Replace `debian:trixie-slim` with `alpine:3.23`
   - Update package manager commands (apt → apk)
   - Update package names (per compatibility analysis)

2. **Test local build** (2 hours)
   - Build on amd64
   - Build on arm64 (if available)
   - Verify image size reduction

3. **Update CI/CD workflows** (3 hours)
   - Modify `.github/workflows/docker-build.yml`
   - Update image tags (add `alpine` suffix for testing)
   - Create `docker-compose.alpine-test.yml`

4. **Documentation updates** (3 hours)
   - Update `README.md` (Alpine base image)
   - Update `ARCHITECTURE.md`
   - Create migration changelog entry

**Deliverables:**
- ✅ Updated `Dockerfile` (all stages Alpine-based)
- ✅ CI workflow building Alpine image
- ✅ `docker-compose.alpine-test.yml` for testing
- ✅ Updated documentation

**Success Criteria:**
- Docker build completes without errors
- Image size reduced by ≥30%
- CI pipeline passes (build stage only)

**Timeline:** February 11-15, 2026

**Assignee:** Backend Team

**Risks:**
- 🔴 **HIGH:** CGO SQLite build failures → Adjust linker flags
- 🟡 **MEDIUM:** Cross-compilation issues with xx toolkit → Debug with ARM64 VM
- 🟢 **LOW:** Documentation drift → Use git diff to ensure completeness

### Phase 3: Comprehensive Testing (Week 2-3 - 20 hours)

**Tasks:**
1. **Runtime validation** (6 hours)
   - Container startup sequence (Test 5)
   - Database operations (Test 6)
   - DNS resolution (Test 7)
   - Health checks and monitoring

2. **E2E test execution** (10 hours)
   - Full Playwright suite (Test 8)
   - DNS provider tests (Test 9)
   - Security feature tests
   - Fix any test failures or timing issues

3. **Integration tests** (4 hours)
   - Cerberus security suite (Test 10)
   - Backend unit tests (Test 11)
   - Verify 85% coverage maintained

**Deliverables:**
- ✅ Test results documented in QA report
- ✅ 100% E2E test pass rate
- ✅ All integration tests passing
- ✅ Test failure RCA (if any)

**Success Criteria:**
- All blocking tests pass (Tests 5, 6, 8)
- No data corruption or startup failures
- Coverage threshold maintained (≥85%)

**Timeline:** February 16-22, 2026

**Assignee:** QA Team + Backend Team

**Risks:**
- 🔴 **HIGH:** E2E test failures >10% → Rollback to Debian
- 🟡 **MEDIUM:** DNS provider integration issues → Use `GODEBUG=netdns=go` workaround
- 🟡 **MEDIUM:** Performance regression → Investigate musl vs glibc trade-offs
- 🟢 **LOW:** Flaky tests → Re-run with retries, improve test stability

### Phase 4: Performance and Security Validation (Week 3 - 8 hours)

**Tasks:**
1. **Performance benchmarking** (4 hours)
   - Request latency benchmark (Test 12)
   - Memory usage analysis (Test 13)
   - Compare with Debian baseline
   - Document any regressions

2. **Security scanning** (2 hours)
   - Final CVE scan (Test 14)
   - SBOM generation and verification (Test 15)
   - Compare CVE counts with Debian

3. **Rollback testing** (2 hours)
   - Practice rollback procedure
   - Verify rollback completes in <20 minutes
   - Document rollback steps

**Deliverables:**
- ✅ Performance comparison report
- ✅ Security scan results (SARIF + reports)
- ✅ Rollback procedure validation
- ✅ Risk acceptance document (if any CVEs found)

**Success Criteria:**
- Performance within 5% of Debian (acceptable: ±10%)
- Zero HIGH/CRITICAL CVEs (or documented acceptance)
- Rollback procedure validated

**Timeline:** February 23-25, 2026

**Assignee:** DevOps + Security Teams

**Risks:**
- 🟡 **MEDIUM:** Performance regression >10% → Profile and optimize
- 🟢 **LOW:** New Alpine CVEs discovered → Document and monitor

### Phase 5: Staging Deployment (Week 4 - 4 hours)

**Tasks:**
1. **Deploy to staging environment** (1 hour)
   - Update staging `docker-compose.yml`
   - Deploy Alpine image
   - Monitor for 48 hours

2. **User acceptance testing** (2 hours)
   - Smoke test all features
   - Invite beta users to test
   - Gather feedback

3. **Documentation finalization** (1 hour)
   - Update `CHANGELOG.md`
   - Create migration announcement
   - Prepare release notes

**Deliverables:**
- ✅ Staging deployment successful
- ✅ User feedback collected
- ✅ Final documentation complete

**Success Criteria:**
- No critical bugs in staging
- Positive user feedback
- Zero production rollbacks

**Timeline:** February 26-28, 2026

**Assignee:** DevOps + Product Team

### Phase 6: Production Deployment (Week 5 - 2 hours)

**Tasks:**
1. **Production release preparation**
   - Tag Docker image: `wikid82/charon:2.x.0-alpine`
   - Create GitHub release
   - Publish release notes

2. **Gradual rollout**
   - Canary deployment (10% traffic) - 24 hours
   - Expand to 50% traffic - 24 hours
   - Full rollout - 24 hours

3. **Post-deployment monitoring**
   - Monitor error rates
   - Check performance metrics
   - Respond to user reports

**Deliverables:**
- ✅ Production deployment complete
- ✅ Alpine default for new installations
- ✅ Migration guide for existing users

**Success Criteria:**
- Zero critical incidents in first 72 hours
- <1% error rate increase
- User feedback positive

**Timeline:** March 3-5, 2026

**Assignee:** DevOps Lead

---

## Risk Assessment

### 7.1 Technical Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| **CVE-2025-60876 still present in Alpine 3.23** | 🟢 LOW (5%) | 🔴 CRITICAL | Verify with Grype scan before Phase 1 (blocking) |
| **CGO SQLite incompatibility with musl** | 🟢 LOW (10%) | 🔴 HIGH | Test database operations in Phase 2 (Test 6) |
| **DNS resolution issues with musl resolver** | 🟡 MEDIUM (30%) | 🟡 MEDIUM | Use `GODEBUG=netdns=go` workaround |
| **E2E test failures >10%** | 🟡 MEDIUM (20%) | 🔴 HIGH | Comprehensive testing in Phase 3 (Tests 8-9) |
| **Performance regression >10%** | 🟢 LOW (15%) | 🟡 MEDIUM | Benchmark in Phase 4 (Test 12), acceptable if <15% |
| **New Alpine CVEs discovered mid-migration** | 🟢 LOW (5%) | 🟡 MEDIUM | Daily CVE monitoring, risk acceptance if needed |
| **Docker Hub/GHCR Alpine image unavailable** | 🟢 VERY LOW (2%) | 🟡 MEDIUM | Pin specific SHA256, Renovate tracks updates |
| **User data corruption during migration** | 🟢 VERY LOW (1%) | 🔴 CRITICAL | No schema changes, automatic backups, rollback tested |

**Overall Risk Level:** 🟡 **MEDIUM** (manageable with comprehensive testing)

### 7.2 Business Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| **User resistance to Alpine migration** | 🟡 MEDIUM (25%) | 🟢 LOW | Clear communication, benefits highlighted |
| **Support requests increase** | 🟡 MEDIUM (30%) | 🟢 LOW | Migration guide, FAQ, troubleshooting docs |
| **Breaking change for existing users** | 🟢 LOW (10%) | 🟡 MEDIUM | No breaking changes planned, rollback available |
| **Community backlash** | 🟢 LOW (5%) | 🟢 LOW | Transparent process, user testing in staging |

### 7.3 Timeline Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| **Phase 1 delay (CVE not patched)** | 🟡 MEDIUM (20%) | 🔴 HIGH | Buffer 2 weeks, escalate to Alpine Security Team |
| **Phase 3 extended testing** | 🟡 MEDIUM (40%) | 🟡 MEDIUM | Allocate 2 weeks for comprehensive testing |
| **Production rollback required** | 🟢 LOW (10%) | 🔴 HIGH | Rollback procedure practiced, <20min downtime |

---

## Success Metrics

### 8.1 Security Metrics

| Metric | Baseline (Debian) | Target (Alpine) | Success Criteria |
|--------|------------------|-----------------|------------------|
| CRITICAL CVEs | 0 | 0 | ✅ Maintained |
| HIGH CVEs | 7 | 0 | ✅ 100% reduction |
| MEDIUM CVEs | 20 | <15 | ✅ 25% reduction |
| glibc CVEs | 7 | 0 | ✅ Eliminated (musl) |
| Attack Surface (Base Image) | 120MB | 7MB | ✅ 94% reduction |

### 8.2 Performance Metrics

| Metric | Baseline (Debian) | Target (Alpine) | Success Criteria |
|--------|------------------|-----------------|------------------|
| Image Size (Final) | 350MB | 220MB | ✅ 37% reduction |
| API Latency (P99) | 200ms | <220ms | ✅ <10% increase |
| Memory Usage (Idle) | 180MB | <200MB | ✅ <10% increase |
| Startup Time | 15s | <18s | ✅ <20% increase |

### 8.3 Quality Metrics

| Metric | Baseline (Debian) | Target (Alpine) | Success Criteria |
|--------|------------------|-----------------|------------------|
| E2E Test Pass Rate | 100% (544/544) | 100% | ✅ Maintained |
| Backend Coverage | 85% | ≥85% | ✅ Maintained |
| Frontend Coverage | 82% | ≥82% | ✅ Maintained |
| Integration Tests | 100% pass | 100% pass | ✅ Maintained |

### 8.4 User Experience Metrics

| Metric | Baseline (Debian) | Target (Alpine) | Success Criteria |
|--------|------------------|-----------------|------------------|
| Feature Parity | 100% | 100% | ✅ No regressions |
| Bug Reports (30 days) | <5 | <10 | ✅ Acceptable increase |
| User Satisfaction | 90% | ≥85% | ✅ Minor drop acceptable |

---

## Post-Migration Monitoring

### 9.1 Continuous Monitoring (First 90 Days)

**Daily Checks (Automated):**
```yaml
# .github/workflows/alpine-monitoring.yml
name: Alpine Security Monitoring
on:
  schedule:
    - cron: '0 2 * * *'  # Daily at 02:00 UTC

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - name: Pull latest Alpine image
        run: docker pull wikid82/charon:latest

      - name: Scan with Grype
        run: grype wikid82/charon:latest --fail-on high --output sarif > grype.sarif

      - name: Compare with baseline
        run: |
          diff grype-baseline.sarif grype.sarif || \
          gh issue create --title "New CVE detected in Alpine image" \
            --body "$(cat grype.sarif)"
```

**Weekly Performance Reviews:**
- API latency percentiles (P50, P95, P99)
- Memory usage trends
- Error rate changes
- User-reported issues

**Monthly CVE Reports:**
- Count of HIGH/CRITICAL CVEs
- Comparison with Debian Trixie
- Risk acceptance review
- Security advisory updates

### 9.2 Alerting Thresholds

**Immediate Escalation (Slack + PagerDuty):**
- CRITICAL CVE discovered in Alpine base image
- Container crash loop (>3 restarts in 5 minutes)
- API error rate >5%
- Memory usage >90%

**Daily Alert (Slack):**
- New HIGH CVE in Alpine packages
- E2E test failures in CI
- Performance degradation >10% vs baseline

**Weekly Report (Email):**
- CVE scan summary
- Performance metrics trend
- User feedback summary

### 9.3 Maintenance Schedule

**Monthly Tasks:**
1. Update Alpine base image to latest patch version (Renovate automated)
2. Re-run full E2E test suite
3. Review and update CVE risk acceptance documents
4. Check Alpine Security Advisory for upcoming patches

**Quarterly Tasks:**
1. Major Alpine version upgrade (e.g., 3.23 → 3.24)
2. Comprehensive security audit (Grype + Trivy + CodeQL)
3. Performance benchmarking vs Debian
4. SBOM regeneration and validation

---

## Appendices

### A. Alpine Security Resources

- **Alpine Security Advisories:** https://security.alpinelinux.org/
- **Alpine Package Search:** https://pkgs.alpinelinux.org/packages
- **Alpine Wiki - musl vs glibc:** https://wiki.alpinelinux.org/wiki/Comparison_with_other_distros
- **Go on Alpine:** https://wiki.alpinelinux.org/wiki/Go

### B. Related Documentation

- **Current Security Advisory:** `docs/security/advisory_2026-02-01_base_image_cves.md`
- **QA Report (Debian CVEs):** `docs/reports/qa_report.md` (Section 5.2)
- **Alpine Vulnerability Acceptance:** `docs/security/VULNERABILITY_ACCEPTANCE.md`
- **Docker Best Practices:** `.github/instructions/containerization-docker-best-practices.instructions.md`

### C. Contacts

- **Security Team Lead:** security-lead@example.com
- **DevOps Lead:** devops-lead@example.com
- **Alpine Security Team:** security@alpinelinux.org (for CVE inquiries)
- **Community Forum:** https://gitlab.alpinelinux.org/alpine/aports/-/issues

### D. Approval Sign-Off

**Planning Approval:**
- [ ] Security Team Lead
- [ ] Backend Team Lead
- [ ] DevOps Team Lead
- [ ] QA Team Lead
- [ ] Product Manager

**Implementation Approval (Phase 2 Go/No-Go):**
- [ ] Alpine CVE verification complete (Test 1 passed)
- [ ] PoC build successful (Test 3 passed)
- [ ] Rollback procedure validated

**Production Deployment Approval (Phase 6 Go/No-Go):**
- [ ] All blocking tests passed (Tests 5, 6, 8)
- [ ] Performance within acceptable range (<10% regression)
- [ ] Zero HIGH/CRITICAL CVEs (or documented risk acceptance)
- [ ] Staging deployment successful (48 hours stable)

---

**Document Status:** 📋 **DRAFT - AWAITING APPROVAL**

**Next Steps:**
1. Review this plan with Security Team (verify CVE research)
2. Obtain approvals from all stakeholders
3. Execute Phase 1 (CVE verification) - BLOCKING STEP
4. Schedule Phase 2 kickoff meeting (if Phase 1 successful)

**Estimated Start Date:** February 5, 2026 (pending approval)
**Estimated Completion Date:** March 5, 2026 (5 weeks total)
