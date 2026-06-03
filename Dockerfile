# Multi-stage Dockerfile for Charon with integrated Caddy
# Single container deployment for simplified home user setup

# Build arguments for versioning
ARG VERSION=dev
ARG BUILD_DATE
ARG VCS_REF
# Set BUILD_DEBUG=1 to build with debug symbols (required for Delve debugging)
ARG BUILD_DEBUG=0

# ---- Pinned Toolchain Versions ----
# renovate: datasource=docker depName=golang versioning=docker
ARG GO_VERSION=1.26.4

# renovate: datasource=docker depName=alpine versioning=docker
ARG ALPINE_IMAGE=alpine:3.23.4@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11

# ---- Shared CrowdSec Version ----
# renovate: datasource=github-releases depName=crowdsecurity/crowdsec
ARG CROWDSEC_VERSION=1.7.8
# CrowdSec fallback tarball checksum (v${CROWDSEC_VERSION})
ARG CROWDSEC_RELEASE_SHA256=704e37121e7ac215991441cef0d8732e33fa3b1a2b2b88b53a0bfe5e38f863bd

# ---- Shared Go Security Patches ----
# renovate: datasource=go depName=github.com/expr-lang/expr
ARG EXPR_LANG_VERSION=1.17.8
# renovate: datasource=go depName=golang.org/x/net
ARG XNET_VERSION=0.55.0
# renovate: datasource=go depName=golang.org/x/crypto
ARG XCRYPTO_VERSION=0.52.0
# renovate: datasource=npm depName=npm
ARG NPM_VERSION=11.16.0

# Allow pinning Caddy version - Renovate will update this
# Build the most recent Caddy 2.x release (keeps major pinned under v3).
# Setting this to '2' tells xcaddy to resolve the latest v2.x tag so we
# avoid accidentally pulling a v3 major release. Renovate can still update
# this ARG to a specific v2.x tag when desired.
## Try to build the requested Caddy v2.x tag (Renovate can update this ARG).
## If the requested tag isn't available, fall back to a known-good v2.11.3 build.
# renovate: datasource=go depName=github.com/caddyserver/caddy/v2
ARG CADDY_VERSION=2.11.4
# renovate: datasource=go depName=github.com/caddyserver/caddy/v2
ARG CADDY_CANDIDATE_VERSION=2.11.4
ARG CADDY_USE_CANDIDATE=0
ARG CADDY_PATCH_SCENARIO=B
# renovate: datasource=go depName=github.com/greenpau/caddy-security
ARG CADDY_SECURITY_VERSION=1.1.62
# renovate: datasource=go depName=github.com/corazawaf/coraza-caddy/v2
ARG CORAZA_CADDY_VERSION=2.5.0
## When an official caddy image tag isn't available on the host, use a
## plain Alpine base image and overwrite its caddy binary with our
## xcaddy-built binary in the later COPY step. This avoids relying on
## upstream caddy image tags while still shipping a pinned caddy binary.
## Alpine 3.23 base to reduce glibc CVE exposure and image size.

# ---- Cross-Compilation Helpers ----
# renovate: datasource=docker depName=tonistiigi/xx
FROM --platform=$BUILDPLATFORM tonistiigi/xx:1.9.0@sha256:c64defb9ed5a91eacb37f96ccc3d4cd72521c4bd18d5442905b95e2226b0e707 AS xx

# ---- Gosu Builder ----
# Build gosu from source to avoid CVEs from Debian's pre-compiled version (Go 1.19.8)
# This fixes 22 HIGH/CRITICAL CVEs in stdlib embedded in Debian's gosu package
# CVEs fixed: CVE-2023-24531, CVE-2023-24540, CVE-2023-29402, CVE-2023-29404,
#             CVE-2023-29405, CVE-2024-24790, CVE-2025-22871, and 15 more
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS gosu-builder
COPY --from=xx / /

WORKDIR /tmp/gosu

ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
# renovate: datasource=github-releases depName=tianon/gosu
ARG GOSU_VERSION=1.17

# hadolint ignore=DL3018
RUN apk add --no-cache git clang lld
# hadolint ignore=DL3059
# hadolint ignore=DL3018
# Install both musl-dev (headers) and musl (runtime library) for cross-compilation linker
RUN xx-apk add --no-cache gcc musl-dev musl

# Clone and build gosu from source with modern Go
RUN git clone --depth 1 --branch "${GOSU_VERSION}" https://github.com/tianon/gosu.git .

# Build gosu for target architecture with patched Go stdlib
# hadolint ignore=DL3059
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 xx-go build -v -ldflags '-s -w' -o /gosu-out/gosu . && \
    xx-verify /gosu-out/gosu

# ---- Frontend Builder ----
# Build the frontend using the BUILDPLATFORM to avoid arm64 musl Rollup native issues
# renovate: datasource=docker depName=node
FROM --platform=$BUILDPLATFORM node:24.16.0-alpine@sha256:2bdb65ed1dab192432bc31c95f94155ca5ad7fc1392fb7eb7526ab682fa5bf14 AS frontend-builder
WORKDIR /app/frontend

# Copy frontend package files
COPY frontend/package*.json ./

# Build-time project version (propagated from top-level build-arg)
ARG VERSION=dev
# Make version available to Vite as VITE_APP_VERSION during the frontend build
ENV VITE_APP_VERSION=${VERSION}

# Vite 8: Rolldown native bindings auto-resolved per platform via optionalDependencies
ARG NPM_VERSION
# hadolint ignore=DL3017
RUN apk upgrade --no-cache && \
    npm install -g npm@${NPM_VERSION} --no-fund --no-audit \
        --fetch-retries=5 --fetch-retry-mintimeout=10000 && \
    npm cache clean --force

# Patch CVE-2026-33671: picomatch ReDoS (fixed in 4.0.4) — bundled in Node.js 24.15.0 npm toolchain.
# Remove when a patched Node.js 24 image is available.
# hadolint ignore=DL3059
RUN npm install -g picomatch@4.0.4 --no-fund --no-audit

RUN npm ci

# Copy frontend source and build
COPY frontend/ ./
RUN --mount=type=cache,target=/app/frontend/node_modules/.cache \
    npm run build

# ---- Backend Builder ----
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS backend-builder
# Copy xx helpers for cross-compilation
COPY --from=xx / /

WORKDIR /app/backend

SHELL ["/bin/ash", "-o", "pipefail", "-c"]

# Install build dependencies
# xx-apk installs packages for the TARGET architecture
ARG TARGETPLATFORM
ARG TARGETARCH
# hadolint ignore=DL3018
RUN apk add --no-cache git clang lld
# hadolint ignore=DL3059
# hadolint ignore=DL3018
# Install musl (headers + runtime) and gcc for cross-compilation linker
# The musl runtime library and gcc crt/libgcc are required by the linker
RUN xx-apk add --no-cache gcc musl-dev musl sqlite-dev

# Ensure the ARM64 musl loader exists for qemu-aarch64 cross-linking
# Without this, the linker fails with: qemu-aarch64: Could not open '/lib/ld-musl-aarch64.so.1'
RUN set -eux; \
    if [ "$TARGETARCH" = "arm64" ]; then \
        LOADER="/lib/ld-musl-aarch64.so.1"; \
        LOADER_PATH="$LOADER"; \
        if [ ! -e "$LOADER" ]; then \
            FOUND="$(find / -path '*/ld-musl-aarch64.so.1' -type f 2>/dev/null | head -n 1)"; \
            if [ -n "$FOUND" ]; then \
                mkdir -p /lib; \
                ln -sf "$FOUND" "$LOADER"; \
                LOADER_PATH="$FOUND"; \
            fi; \
        fi; \
        echo "Using musl loader at: $LOADER_PATH"; \
        test -e "$LOADER"; \
    fi

# Install Delve (cross-compile for target)
# Note: xx-go install puts binaries in /go/bin/TARGETOS_TARGETARCH/dlv if cross-compiling.
# We find it and move it to /go/bin/dlv so it's in a consistent location for the next stage.
# renovate: datasource=go depName=github.com/go-delve/delve
ARG DLV_VERSION=1.26.3
# hadolint ignore=DL3059,DL4006
RUN CGO_ENABLED=0 xx-go install github.com/go-delve/delve/cmd/dlv@v${DLV_VERSION} && \
    DLV_PATH=$(find /go/bin -name dlv -type f | head -n 1) && \
    if [ -n "$DLV_PATH" ] && [ "$DLV_PATH" != "/go/bin/dlv" ]; then \
        mv "$DLV_PATH" /go/bin/dlv; \
    fi && \
    xx-verify /go/bin/dlv

# Copy Go module files
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy backend source
COPY backend/ ./

# Build arguments passed from main build context
ARG VERSION=dev
ARG VCS_REF=unknown
ARG BUILD_DATE=unknown
ARG BUILD_DEBUG=0

# Build the Go binary with version information injected via ldflags
# xx-go handles CGO and cross-compilation flags automatically
# Note: Go 1.26 defaults to gold linker for ARM64, but clang doesn't support -fuse-ld=gold
# Use lld for ARM64 cross-linking; keep bfd for amd64 to preserve prior behavior
# PIE is required for arm64 cross-linking with lld to avoid relocation conflicts under
# QEMU emulation and improves security posture.
# When BUILD_DEBUG=1, we preserve debug symbols (no -s -w) and disable optimizations
# for Delve debugging. Otherwise, strip symbols for smaller production binaries.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    EXT_LD_FLAGS="-fuse-ld=bfd"; \
    BUILD_MODE=""; \
    if [ "$TARGETARCH" = "arm64" ]; then \
        EXT_LD_FLAGS="-fuse-ld=lld"; \
        BUILD_MODE="-buildmode=pie"; \
    fi; \
    if [ "$BUILD_DEBUG" = "1" ]; then \
        echo "Building with debug symbols for Delve..."; \
        CGO_ENABLED=1 CC=xx-clang CXX=xx-clang++ xx-go build ${BUILD_MODE} \
            -gcflags="all=-N -l" \
            -ldflags "-extldflags=${EXT_LD_FLAGS} \
                      -X github.com/Wikid82/charon/backend/internal/version.Version=${VERSION} \
                      -X github.com/Wikid82/charon/backend/internal/version.GitCommit=${VCS_REF} \
                      -X github.com/Wikid82/charon/backend/internal/version.BuildTime=${BUILD_DATE}" \
            -o charon ./cmd/api; \
    else \
        echo "Building optimized production binary..."; \
        CGO_ENABLED=1 CC=xx-clang CXX=xx-clang++ xx-go build ${BUILD_MODE} \
            -ldflags "-s -w -extldflags=${EXT_LD_FLAGS} \
                      -X github.com/Wikid82/charon/backend/internal/version.Version=${VERSION} \
                      -X github.com/Wikid82/charon/backend/internal/version.GitCommit=${VCS_REF} \
                      -X github.com/Wikid82/charon/backend/internal/version.BuildTime=${BUILD_DATE}" \
            -o charon ./cmd/api; \
    fi

# ---- Caddy Builder ----
# Build Caddy from source to ensure we use the latest Go version and dependencies
# This fixes vulnerabilities found in the pre-built Caddy images (e.g. CVE-2025-59530, stdlib issues)
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS caddy-builder
ARG TARGETOS
ARG TARGETARCH
ARG CADDY_VERSION
ARG CADDY_CANDIDATE_VERSION
ARG CADDY_USE_CANDIDATE
ARG CADDY_PATCH_SCENARIO
ARG CADDY_SECURITY_VERSION
ARG CORAZA_CADDY_VERSION
# renovate: datasource=go depName=github.com/caddyserver/xcaddy
ARG XCADDY_VERSION=0.4.6
ARG EXPR_LANG_VERSION
ARG XNET_VERSION
ARG XCRYPTO_VERSION
ARG CROWDSEC_VERSION

# hadolint ignore=DL3018
RUN apk add --no-cache bash git
# hadolint ignore=DL3062
RUN --mount=type=cache,target=/go/pkg/mod \
    go install github.com/caddyserver/xcaddy/cmd/xcaddy@v${XCADDY_VERSION}

# Build Caddy for the target architecture with security plugins.
# Two-stage approach: xcaddy generates go.mod, we patch it, then build from scratch.
# This ensures the final binary is compiled with fully patched dependencies.
# NOTE: Keep patching deterministic and explicit. Avoid silent fallbacks.
# hadolint ignore=SC2016
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    bash -c 'set -e; \
        # Restore any module cache files patched by a previous build run.
        # xcaddy Stage 1 resolves crowdsec to its native version (v1.6.x, IPEquals *string).
        # If a prior build left IPEquals: value, (plain string) in the cache, xcaddy fails.
        _GOMC="$(go env GOMODCACHE)"; \
        for _PF in \
            "${_GOMC}/github.com/hslatman/caddy-crowdsec-bouncer@v0.12.1/internal/bouncer/live.go" \
            "${_GOMC}/github.com/crowdsecurity/go-cs-bouncer@v0.0.14/live_bouncer.go"; do \
            if [ -f "${_PF}" ]; then \
                chmod +w "${_PF}"; \
                sed -i "s/IPEquals: value,/IPEquals: \&value,/g" "${_PF}"; \
            fi; \
        done; \
        CADDY_TARGET_VERSION="${CADDY_VERSION}"; \
        if [ "${CADDY_USE_CANDIDATE}" = "1" ]; then \
            CADDY_TARGET_VERSION="${CADDY_CANDIDATE_VERSION}"; \
        fi; \
        echo "Using Caddy target version: v${CADDY_TARGET_VERSION}"; \
        echo "Using Caddy patch scenario: ${CADDY_PATCH_SCENARIO}"; \
        export XCADDY_SKIP_CLEANUP=1; \
        echo "Stage 1: Generate go.mod with xcaddy..."; \
        # Run xcaddy to generate the build directory and go.mod
        GOOS=$TARGETOS GOARCH=$TARGETARCH xcaddy build v${CADDY_TARGET_VERSION} \
            --with github.com/caddyserver/caddy/v2@v${CADDY_TARGET_VERSION} \
            --with github.com/greenpau/caddy-security@v${CADDY_SECURITY_VERSION} \
            --with github.com/corazawaf/coraza-caddy/v2@v${CORAZA_CADDY_VERSION} \
            --with github.com/hslatman/caddy-crowdsec-bouncer@v0.12.1 \
            --with github.com/zhangjiayin/caddy-geoip2 \
            --with github.com/mholt/caddy-ratelimit \
            --output /tmp/caddy-initial; \
        # Find the build directory created by xcaddy
        BUILDDIR=$(ls -td /tmp/buildenv_* 2>/dev/null | head -1); \
        if [ ! -d "$BUILDDIR" ] || [ ! -f "$BUILDDIR/go.mod" ]; then \
            echo "ERROR: Build directory not found or go.mod missing"; \
            exit 1; \
        fi; \
        echo "Found build directory: $BUILDDIR"; \
        cd "$BUILDDIR"; \
        echo "Stage 2: Apply security patches to go.mod..."; \
        # Patch ALL dependencies BEFORE building the final binary
        # These patches fix CVEs in transitive dependencies
        # Renovate tracks these via regex manager in renovate.json
        go get github.com/expr-lang/expr@v${EXPR_LANG_VERSION}; \
        # renovate: datasource=go depName=github.com/hslatman/ipstore
        go get github.com/hslatman/ipstore@v0.4.0; \
        go get golang.org/x/crypto@v${XCRYPTO_VERSION}; \
        go get golang.org/x/net@v${XNET_VERSION}; \
        # CVE-2026-33186: gRPC-Go auth bypass (fixed in v1.79.3)
        # CVE-2026-34986: go-jose/v4 transitive fix (requires grpc >= v1.80.0)
        # Pin here so the Caddy binary is patched immediately;
        # remove once Caddy ships a release built with grpc >= v1.80.0.
        # renovate: datasource=go depName=google.golang.org/grpc
        go get google.golang.org/grpc@v1.80.0; \
        # CVE-2026-34986: go-jose JOSE/JWT validation bypass
        # renovate: datasource=go depName=github.com/go-jose/go-jose/v3
        go get github.com/go-jose/go-jose/v3@v3.0.5; \
        # renovate: datasource=go depName=github.com/go-jose/go-jose/v4
        go get github.com/go-jose/go-jose/v4@v4.1.4; \
        # CVE-2026-39883: OTel SDK resource leak
        # renovate: datasource=go depName=go.opentelemetry.io/otel/sdk
        go get go.opentelemetry.io/otel/sdk@v1.43.0; \
        # CVE-2026-39882: OTel HTTP exporter request smuggling
        # renovate: datasource=go depName=go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp
        go get go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp@v0.19.0; \
        # renovate: datasource=go depName=go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp
        go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp@v1.43.0; \
        # renovate: datasource=go depName=go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
        go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.43.0; \
        # GHSA-479m-364c-43vc: goxmldsig XML signature validation bypass (loop variable capture)
        # Fix available at v1.6.0. Pin here so the Caddy binary is patched immediately;
        # remove once caddy-security ships a release built with goxmldsig >= v1.6.0.
        # renovate: datasource=go depName=github.com/russellhaering/goxmldsig
        go get github.com/russellhaering/goxmldsig@v1.6.0; \
        # CVE-2026-32952: go-ntlmssp DoS via malicious NTLM challenge response
        # Affects /usr/bin/caddy (transitive dependency). Fix available at v0.1.1.
        # renovate: datasource=go depName=github.com/Azure/go-ntlmssp
        go get github.com/Azure/go-ntlmssp@v0.1.1; \
        # CVE-2026-44982 (GHSA-rw47-hm26-6wr7): CrowdSec AppSec silently drops HTTP request
        # body for chunked/HTTP-2 requests, bypassing WAF body inspection rules.
        # caddy-crowdsec-bouncer@v0.12.1 was built against crowdsec v1.6.3 whose
        # DecisionsListOpts fields were *string; v1.7.8 changed them to plain string.
        # The source-level incompatibility is patched below via local copy + go.mod replace.
        # Remove once bouncer ships against crowdsec >= v1.7.8.
        go get github.com/crowdsecurity/crowdsec@v${CROWDSEC_VERSION}; \
        if [ "${CADDY_PATCH_SCENARIO}" = "A" ]; then \
            # Rollback scenario: keep explicit nebula pin if upstream compatibility regresses.
            # NOTE: smallstep/certificates (pulled by caddy-security stack) currently
            # uses legacy nebula APIs removed in nebula v1.10+, which causes compile
            # failures in authority/provisioner. Keep this pinned to a known-compatible
            # v1.9.x release until upstream stack supports nebula v1.10+.
            # renovate: datasource=go depName=github.com/slackhq/nebula
            go get github.com/slackhq/nebula@v1.9.7; \
        elif [ "${CADDY_PATCH_SCENARIO}" = "B" ] || [ "${CADDY_PATCH_SCENARIO}" = "C" ]; then \
            # Default PR-2 posture: retire explicit nebula pin and use upstream resolution.
            echo "Skipping nebula pin for scenario ${CADDY_PATCH_SCENARIO}"; \
        else \
            echo "Unsupported CADDY_PATCH_SCENARIO=${CADDY_PATCH_SCENARIO}"; \
            exit 1; \
        fi; \
        # Final re-pin: enforce requested Caddy core version after plugin/security updates.
        go get github.com/caddyserver/caddy/v2@v${CADDY_TARGET_VERSION}; \
        # Clean up go.mod and ensure all dependencies are resolved
        go mod tidy; \
        # Patch DecisionsListOpts API: crowdsec v1.7.8 changed fields (IPEquals, ScopeEquals,
        # etc.) from *string to plain string. caddy-crowdsec-bouncer@v0.12.1 and its transitive
        # dep go-cs-bouncer@v0.0.14 still use the old pointer form.
        # Strategy: copy modules to ephemeral /tmp dirs and use go.mod replace directives.
        # This avoids modifying the shared BuildKit module cache, which would corrupt xcaddy
        # Stage 1 of subsequent builds (where these modules are compiled with crowdsec v1.6.x).
        go mod download github.com/hslatman/caddy-crowdsec-bouncer@v0.12.1; \
        BOUNCER_CACHE="${_GOMC}/github.com/hslatman/caddy-crowdsec-bouncer@v0.12.1"; \
        BOUNCER_LOCAL="/tmp/bouncer-patched"; \
        rm -rf "${BOUNCER_LOCAL}"; \
        cp -r "${BOUNCER_CACHE}/." "${BOUNCER_LOCAL}/"; \
        chmod -R +w "${BOUNCER_LOCAL}"; \
        sed -i "s/IPEquals: &value,/IPEquals: value,/g" "${BOUNCER_LOCAL}/internal/bouncer/live.go"; \
        echo "Patched caddy-crowdsec-bouncer at ${BOUNCER_LOCAL}"; \
        go mod edit -replace "github.com/hslatman/caddy-crowdsec-bouncer@v0.12.1=${BOUNCER_LOCAL}"; \
        GO_CS_CACHE="${_GOMC}/github.com/crowdsecurity/go-cs-bouncer@v0.0.14"; \
        if [ -d "${GO_CS_CACHE}" ]; then \
            GO_CS_LOCAL="/tmp/go-cs-bouncer-patched"; \
            rm -rf "${GO_CS_LOCAL}"; \
            cp -r "${GO_CS_CACHE}/." "${GO_CS_LOCAL}/"; \
            chmod -R +w "${GO_CS_LOCAL}"; \
            sed -i "s/IPEquals: &value,/IPEquals: value,/g" "${GO_CS_LOCAL}/live_bouncer.go"; \
            sed -i "s/ScopeEquals: &value,/ScopeEquals: value,/g" "${GO_CS_LOCAL}/live_bouncer.go"; \
            sed -i "s/ValueEquals: &value,/ValueEquals: value,/g" "${GO_CS_LOCAL}/live_bouncer.go"; \
            sed -i "s/TypeEquals: &value,/TypeEquals: value,/g" "${GO_CS_LOCAL}/live_bouncer.go"; \
            sed -i "s/RangeEquals: &value,/RangeEquals: value,/g" "${GO_CS_LOCAL}/live_bouncer.go"; \
            sed -i "s/osName, osVersion := version.DetectOS()/osName, osVersion, _ := version.DetectOS()/g" "${GO_CS_LOCAL}/metrics.go"; \
            echo "Patched go-cs-bouncer at ${GO_CS_LOCAL}"; \
            go mod edit -replace "github.com/crowdsecurity/go-cs-bouncer@v0.0.14=${GO_CS_LOCAL}"; \
        fi; \
        # Hard assertion: fail if module graph resolves to a different Caddy core version.
        ACTUAL_CADDY_VERSION="$(go list -m -f "{{.Version}}" github.com/caddyserver/caddy/v2)"; \
        if [ "$ACTUAL_CADDY_VERSION" != "v${CADDY_TARGET_VERSION}" ]; then \
            echo "ERROR: Resolved Caddy version ${ACTUAL_CADDY_VERSION} does not match target v${CADDY_TARGET_VERSION}"; \
            exit 1; \
        fi; \
        echo "Verified Caddy module version: ${ACTUAL_CADDY_VERSION}"; \
        echo "Dependencies patched successfully"; \
        # Remove any temporary binaries from initial xcaddy run
        rm -f /tmp/caddy-initial; \
        echo "Stage 3: Build final Caddy binary with patched dependencies..."; \
        # Build the final binary from scratch with the fully patched go.mod
        # This ensures no vulnerable metadata is embedded
        GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /usr/bin/caddy \
            -ldflags "-w -s" -trimpath -tags "nobadger,nomysql,nopgx" .; \
        echo "Build successful with patched dependencies"; \
        # Verify the binary exists and is executable (no execution to avoid hang)
        test -x /usr/bin/caddy || exit 1; \
        echo "Caddy binary verified"; \
        # Clean up temporary build directories
        rm -rf /tmp/buildenv_* /tmp/caddy-initial'

# ---- CrowdSec Builder ----
# Build CrowdSec from source to ensure we use Go 1.26.3+ and avoid stdlib vulnerabilities
# (CVE-2025-58183, CVE-2025-58186, CVE-2025-58187, CVE-2025-61729)
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS crowdsec-builder
COPY --from=xx / /

WORKDIR /tmp/crowdsec

ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG CROWDSEC_VERSION
ARG CROWDSEC_RELEASE_SHA256
ARG EXPR_LANG_VERSION
ARG XNET_VERSION

# hadolint ignore=DL3018
RUN apk add --no-cache git clang lld
# hadolint ignore=DL3059
# hadolint ignore=DL3018
# Install both musl-dev (headers) and musl (runtime library) for cross-compilation linker
RUN xx-apk add --no-cache gcc musl-dev musl

# Clone CrowdSec source
RUN git clone --depth 1 --branch "v${CROWDSEC_VERSION}" https://github.com/crowdsecurity/crowdsec.git .

# Patch dependencies to fix CVEs in transitive dependencies
# This follows the same pattern as Caddy's dependency patches
# renovate: datasource=go depName=golang.org/x/crypto
RUN go get github.com/expr-lang/expr@v${EXPR_LANG_VERSION} && \
    go get golang.org/x/crypto@v0.52.0 && \
    go get golang.org/x/net@v${XNET_VERSION} && \
    # CVE-2026-33186 (GHSA-p77j-4mvh-x3m3): gRPC-Go auth bypass via missing leading slash
    # Fix available at v1.79.3. Pin here so the CrowdSec binary is patched immediately;
    # remove once CrowdSec ships a release built with grpc >= v1.79.3.
    # renovate: datasource=go depName=google.golang.org/grpc
    go get google.golang.org/grpc@v1.81.1 && \
    # CVE-2026-32286: pgproto3/v2 buffer overflow (no v2 fix exists; bump pgx/v4 to latest patch)
    # renovate: datasource=github-tags depName=jackc/pgx
    go get github.com/jackc/pgx/v4@v4.18.3 && \
    # CVE-2026-29181 (GHSA-mh2q-q3fh-2475): OpenTelemetry-Go baggage header multi-value DoS
    # go.opentelemetry.io/otel >= 1.36.0 and <= 1.40.0 is vulnerable; fix available at v1.41.0.
    # Pin here so the CrowdSec binary is patched immediately;
    # remove once CrowdSec ships a release built with go.opentelemetry.io/otel >= v1.41.0.
    # renovate: datasource=go depName=go.opentelemetry.io/otel
    go get go.opentelemetry.io/otel@v1.44.0 && \
    # GHSA-xmrv-pmrh-hhx2: AWS SDK v2 event stream injection
    # renovate: datasource=go depName=github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream
    go get github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream@v1.7.11 && \
    # renovate: datasource=go depName=github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs
    go get github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs@v1.74.2 && \
    go get github.com/aws/aws-sdk-go-v2/service/kinesis@v1.43.7 && \
    go get github.com/aws/aws-sdk-go-v2/service/s3@v1.102.1 && \
    # CVE-2026-32952: go-ntlmssp DoS via malicious NTLM challenge response
    # Affects /usr/local/bin/cscli (transitive dependency). Fix available at v0.1.1.
    # renovate: datasource=go depName=github.com/Azure/go-ntlmssp
    go get github.com/Azure/go-ntlmssp@v0.1.1 && \
    go mod tidy

# Fix compatibility issues with expr-lang v1.17.7
# In v1.17.7, program.Source() returns file.Source struct instead of string
# The upstream fix is in main branch but not yet released
RUN sed -i 's/string(program\.Source())/program.Source().String()/g' pkg/exprhelpers/debugger.go

# Build CrowdSec binaries for target architecture with patched dependencies
# hadolint ignore=DL3059
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 xx-go build -o /crowdsec-out/crowdsec \
        -ldflags "-s -w -X github.com/crowdsecurity/crowdsec/pkg/cwversion.Version=v${CROWDSEC_VERSION}" \
        ./cmd/crowdsec && \
    xx-verify /crowdsec-out/crowdsec

# hadolint ignore=DL3059
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 xx-go build -o /crowdsec-out/cscli \
        -ldflags "-s -w -X github.com/crowdsecurity/crowdsec/pkg/cwversion.Version=v${CROWDSEC_VERSION}" \
        ./cmd/crowdsec-cli && \
    xx-verify /crowdsec-out/cscli

# Copy config files
RUN mkdir -p /crowdsec-out/config && \
    cp -r config/* /crowdsec-out/config/ || true

# ---- CrowdSec Fallback (for architectures where build fails) ----
FROM ${ALPINE_IMAGE} AS crowdsec-fallback

SHELL ["/bin/ash", "-o", "pipefail", "-c"]

WORKDIR /tmp/crowdsec

ARG TARGETARCH
ARG CROWDSEC_VERSION
ARG CROWDSEC_RELEASE_SHA256

# hadolint ignore=DL3018
RUN apk add --no-cache curl ca-certificates

# Download static binaries as fallback (only available for amd64)
# For other architectures, create empty placeholder files so COPY doesn't fail
# hadolint ignore=DL3059,SC2015
RUN set -eux; \
    mkdir -p /crowdsec-out/bin /crowdsec-out/config; \
    if [ "$TARGETARCH" = "amd64" ]; then \
        echo "Downloading CrowdSec binaries for amd64 (fallback)..."; \
        curl -fSL "https://github.com/crowdsecurity/crowdsec/releases/download/v${CROWDSEC_VERSION}/crowdsec-release.tgz" \
            -o /tmp/crowdsec.tar.gz && \
        echo "${CROWDSEC_RELEASE_SHA256}  /tmp/crowdsec.tar.gz" | sha256sum -c - && \
        tar -xzf /tmp/crowdsec.tar.gz -C /tmp && \
        cp "/tmp/crowdsec-v${CROWDSEC_VERSION}/cmd/crowdsec-cli/cscli" /crowdsec-out/bin/ && \
        cp "/tmp/crowdsec-v${CROWDSEC_VERSION}/cmd/crowdsec/crowdsec" /crowdsec-out/bin/ && \
        chmod +x /crowdsec-out/bin/* && \
        if [ -d "/tmp/crowdsec-v${CROWDSEC_VERSION}/config" ]; then \
            cp -r "/tmp/crowdsec-v${CROWDSEC_VERSION}/config/"* /crowdsec-out/config/; \
        fi && \
        echo "CrowdSec fallback binaries installed successfully"; \
    else \
        echo "CrowdSec binaries not available for $TARGETARCH - skipping"; \
        touch /crowdsec-out/bin/.placeholder /crowdsec-out/config/.placeholder; \
    fi

# ---- Final Runtime with Caddy ----
FROM ${ALPINE_IMAGE}
WORKDIR /app

# Install runtime dependencies for Charon, including bash for maintenance scripts
# Note: gosu is now built from source (see gosu-builder stage) to avoid CVEs from Debian's pre-compiled version
# Explicitly upgrade packages to fix security vulnerabilities
# hadolint ignore=DL3018
RUN apk add --no-cache \
    bash ca-certificates sqlite-libs sqlite tzdata gettext libcap libcap-utils \
    c-ares busybox-extras \
    && apk upgrade --no-cache zlib libcrypto3 libssl3 musl musl-utils \
    # CVE-2026-34743: xz-libs DoS via buffer overflow in index decoding (fixed in 5.8.3-r0)
    xz-libs \
    # CVE-2026-6732: libxml2 HIGH vulnerability (fixed in 2.13.9-r1)
    libxml2

# Copy gosu binary from gosu-builder (built with Go 1.26+ to avoid stdlib CVEs)
COPY --from=gosu-builder /gosu-out/gosu /usr/sbin/gosu
RUN chmod +x /usr/sbin/gosu

# Security: Create non-root user and group for running the application
# This follows the principle of least privilege (CIS Docker Benchmark 4.1)
RUN addgroup -g 1000 -S charon && \
    adduser -u 1000 -S -G charon -h /app -s /sbin/nologin charon

SHELL ["/bin/ash", "-o", "pipefail", "-c"]

# Download MaxMind GeoLite2 Country database
# Note: In production, users should provide their own MaxMind license key
# This uses the publicly available GeoLite2 database
# In CI, timeout quickly rather than retrying to save build time
ARG GEOLITE2_COUNTRY_SHA256=c77ac1d7e64b3fcd1447045615fc3aefb3ed886e176608c568b01f29f955e21a
RUN mkdir -p /app/data/geoip && \
        if [ "$CI" = "true" ] || [ "$CI" = "1" ]; then \
            echo "⏱️  CI detected - quick download (10s timeout, no retries)"; \
            if wget -qO /app/data/geoip/GeoLite2-Country.mmdb \
                -T 10 "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" 2>/dev/null \
                && [ -s /app/data/geoip/GeoLite2-Country.mmdb ]; then \
                echo "✅ GeoIP downloaded"; \
            else \
                echo "⚠️  GeoIP skipped"; \
                touch /app/data/geoip/GeoLite2-Country.mmdb.placeholder; \
            fi; \
        else \
            echo "Local - full download (30s timeout, 3 retries)"; \
            if wget -qO /app/data/geoip/GeoLite2-Country.mmdb \
                -T 30 -t 4 "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" \
                && [ -s /app/data/geoip/GeoLite2-Country.mmdb ]; then \
                echo "✅ GeoIP downloaded"; \
            else \
                echo "⚠️  GeoIP download failed or empty — skipping"; \
                touch /app/data/geoip/GeoLite2-Country.mmdb.placeholder; \
            fi; \
        fi

# Copy Caddy binary from caddy-builder (overwriting the one from base image)
COPY --from=caddy-builder /usr/bin/caddy /usr/bin/caddy

# Allow non-root to bind privileged ports (80/443) securely
RUN setcap 'cap_net_bind_service=+ep' /usr/bin/caddy

# Copy CrowdSec binaries from the crowdsec-builder stage (built with Go 1.26.3+)
# This ensures we don't have stdlib vulnerabilities from older Go versions
COPY --from=crowdsec-builder /crowdsec-out/crowdsec /usr/local/bin/crowdsec
COPY --from=crowdsec-builder /crowdsec-out/cscli /usr/local/bin/cscli
# Copy CrowdSec configuration files to .dist directory (will be used at runtime)
COPY --from=crowdsec-builder /crowdsec-out/config /etc/crowdsec.dist
# Verify config files were copied successfully
RUN if [ ! -f /etc/crowdsec.dist/config.yaml ]; then \
        echo "WARNING: config.yaml not found in /etc/crowdsec.dist"; \
        echo "Available files in /etc/crowdsec.dist:"; \
        ls -la /etc/crowdsec.dist/ 2>/dev/null || echo "Directory empty or missing"; \
    else \
        echo "✓ config.yaml found in /etc/crowdsec.dist"; \
    fi

# Verify CrowdSec binaries and configuration
RUN chmod +x /usr/local/bin/crowdsec /usr/local/bin/cscli 2>/dev/null || true; \
    if [ -x /usr/local/bin/cscli ]; then \
        echo "CrowdSec installed (built from source with Go 1.26):"; \
        cscli version || echo "CrowdSec version check failed"; \
        echo ""; \
        echo "Configuration source: /etc/crowdsec.dist"; \
        ls -la /etc/crowdsec.dist/ | head -10 || echo "ERROR: /etc/crowdsec.dist directory not found"; \
    else \
        echo "CrowdSec not available for this architecture"; \
    fi

# Create required CrowdSec directories in runtime image
# NOTE: Do NOT create /etc/crowdsec here - it must be a symlink created at runtime by non-root user
RUN mkdir -p /var/lib/crowdsec/data /var/log/crowdsec /var/log/caddy \
             /app/data/crowdsec/config /app/data/crowdsec/data && \
    chown -R charon:charon /var/lib/crowdsec /var/log/crowdsec \
                           /app/data/crowdsec

# Ensure config.yaml exists in .dist (required for runtime)
# Skip cscli config restore at build time (no valid /etc/crowdsec at this stage)
# The runtime entrypoint will handle config initialization from .dist
RUN if [ ! -f /etc/crowdsec.dist/config.yaml ]; then \
        echo "⚠️  WARNING: config.yaml not in /etc/crowdsec.dist after builder COPY"; \
        echo "   This file is critical for CrowdSec initialization at runtime"; \
    else \
        echo "✓ /etc/crowdsec.dist/config.yaml verified"; \
    fi

# Copy CrowdSec configuration templates from source
COPY configs/crowdsec/acquis.yaml /etc/crowdsec.dist/acquis.yaml
COPY configs/crowdsec/install_hub_items.sh /usr/local/bin/install_hub_items.sh
COPY configs/crowdsec/register_bouncer.sh /usr/local/bin/register_bouncer.sh

# Make CrowdSec scripts executable
RUN chmod +x /usr/local/bin/install_hub_items.sh /usr/local/bin/register_bouncer.sh

# Copy Go binary from backend builder
COPY --from=backend-builder /app/backend/charon /app/charon
RUN ln -s /app/charon /app/cpmp || true
# Copy Delve debugger (xx-go install places it in /go/bin)
COPY --from=backend-builder /go/bin/dlv /usr/local/bin/dlv

# Copy frontend build from frontend builder
COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist

# Copy startup script
COPY .docker/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Copy utility scripts (used for DB recovery and maintenance)
COPY scripts/ /app/scripts/
RUN chmod +x /app/scripts/db-recovery.sh

# Set default environment variables
ENV CHARON_ENV=production \
    CHARON_DB_PATH=/app/data/charon.db \
    CHARON_FRONTEND_DIR=/app/frontend/dist \
    CHARON_CADDY_ADMIN_API=http://localhost:2019 \
    CHARON_CADDY_CONFIG_DIR=/app/data/caddy \
    CHARON_GEOIP_DB_PATH=/app/data/geoip/GeoLite2-Country.mmdb \
    CHARON_HTTP_PORT=8080 \
    CHARON_CROWDSEC_CONFIG_DIR=/app/data/crowdsec \
    CHARON_PLUGINS_DIR=/app/plugins
# Create necessary directories
RUN mkdir -p /app/data /app/data/caddy /config /app/data/crowdsec

# Security: Create plugins directory with secure permissions
# Mode 0755: owner rwx, group rx, other rx (NOT world-writable)
# This satisfies the PluginLoaderService security check (mode & 0002 == 0)
RUN mkdir -p /app/plugins && chmod 755 /app/plugins

# Security: Set ownership of all application directories to non-root charon user
# Note: /etc/crowdsec will be created as a symlink at runtime, not owned directly
# Note: /app/plugins has 755 permissions (NOT world-writable) for security
RUN chown -R charon:charon /app /config /var/log/crowdsec /var/log/caddy && \
    chown -R charon:charon /etc/crowdsec.dist 2>/dev/null || true && \
    chown -R charon:charon /var/lib/crowdsec 2>/dev/null || true

# Re-declare build args for LABEL usage
ARG VERSION=dev
ARG BUILD_DATE
ARG VCS_REF

# OCI image labels for version metadata
LABEL org.opencontainers.image.title="Charon (CPMP legacy)" \
      org.opencontainers.image.description="Web UI for managing Caddy reverse proxy configurations" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}" \
    org.opencontainers.image.source="https://github.com/Wikid82/charon" \
    org.opencontainers.image.url="https://github.com/Wikid82/charon" \
    org.opencontainers.image.vendor="charon" \
      org.opencontainers.image.licenses="MIT"

# Expose ports
EXPOSE 80 443 443/udp 2019 8080

# Security: Add healthcheck to monitor container health
# Verifies the Charon API is responding correctly
HEALTHCHECK --interval=30s --timeout=10s --start-period=4m --retries=3 \
    CMD wget -q -O /dev/null http://localhost:8080/api/v1/health || exit 1

# Create CrowdSec symlink as root before switching to non-root user
# This symlink allows CrowdSec to use persistent storage at /app/data/crowdsec/config
# while maintaining the expected /etc/crowdsec path for compatibility
RUN ln -sf /app/data/crowdsec/config /etc/crowdsec

# Security: Run the container as non-root by default.
USER charon

# Use custom entrypoint to start both Caddy and Charon
ENTRYPOINT ["/docker-entrypoint.sh"]
