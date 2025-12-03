# Multi-stage Dockerfile for Charon with integrated Caddy
# Single container deployment for simplified home user setup

# Build arguments for versioning
ARG VERSION=dev
ARG BUILD_DATE
ARG VCS_REF

# Allow pinning Caddy version - Renovate will update this
# Build the most recent Caddy 2.x release (keeps major pinned under v3).
# Setting this to '2' tells xcaddy to resolve the latest v2.x tag so we
# avoid accidentally pulling a v3 major release. Renovate can still update
# this ARG to a specific v2.x tag when desired.
## Try to build the requested Caddy v2.x tag (Renovate can update this ARG).
## If the requested tag isn't available, fall back to a known-good v2.10.2 build.
ARG CADDY_VERSION=2.10.2
## When an official caddy image tag isn't available on the host, use a
## plain Alpine base image and overwrite its caddy binary with our
## xcaddy-built binary in the later COPY step. This avoids relying on
## upstream caddy image tags while still shipping a pinned caddy binary.
ARG CADDY_IMAGE=alpine:3.23

# ---- Cross-Compilation Helpers ----
FROM --platform=$BUILDPLATFORM tonistiigi/xx:1.8.0 AS xx

# ---- Frontend Builder ----
# Build the frontend using the BUILDPLATFORM to avoid arm64 musl Rollup native issues
FROM --platform=$BUILDPLATFORM node:24.11.1-alpine AS frontend-builder
WORKDIR /app/frontend

# Copy frontend package files
COPY frontend/package*.json ./

# Build-time project version (propagated from top-level build-arg)
ARG VERSION=dev
# Make version available to Vite as VITE_APP_VERSION during the frontend build
ENV VITE_APP_VERSION=${VERSION}

# Set environment to bypass native binary requirement for cross-arch builds
ENV npm_config_rollup_skip_nodejs_native=1 \
    ROLLUP_SKIP_NODEJS_NATIVE=1

RUN npm ci

# Copy frontend source and build
COPY frontend/ ./
RUN --mount=type=cache,target=/app/frontend/node_modules/.cache \
    npm run build

# ---- Backend Builder ----
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine AS backend-builder
# Copy xx helpers for cross-compilation
COPY --from=xx / /

WORKDIR /app/backend

# Install build dependencies
# xx-apk installs packages for the TARGET architecture
ARG TARGETPLATFORM
# hadolint ignore=DL3018
RUN apk add --no-cache clang lld
# hadolint ignore=DL3018,DL3059
RUN xx-apk add --no-cache gcc musl-dev sqlite-dev

# Install Delve (cross-compile for target)
# Note: xx-go install puts binaries in /go/bin/TARGETOS_TARGETARCH/dlv if cross-compiling.
# We find it and move it to /go/bin/dlv so it's in a consistent location for the next stage.
# hadolint ignore=DL3059,DL4006
RUN CGO_ENABLED=0 xx-go install github.com/go-delve/delve/cmd/dlv@latest && \
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

# Build the Go binary with version information injected via ldflags
# xx-go handles CGO and cross-compilation flags automatically
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 xx-go build \
    -ldflags "-s -w -X github.com/Wikid82/charon/backend/internal/version.Version=${VERSION} \
              -X github.com/Wikid82/charon/backend/internal/version.GitCommit=${VCS_REF} \
              -X github.com/Wikid82/charon/backend/internal/version.BuildTime=${BUILD_DATE}" \
    -o charon ./cmd/api

# ---- Caddy Builder ----
# Build Caddy from source to ensure we use the latest Go version and dependencies
# This fixes vulnerabilities found in the pre-built Caddy images (e.g. CVE-2025-59530, stdlib issues)
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine AS caddy-builder
ARG TARGETOS
ARG TARGETARCH
ARG CADDY_VERSION

# hadolint ignore=DL3018
RUN apk add --no-cache git
# hadolint ignore=DL3062
RUN --mount=type=cache,target=/go/pkg/mod \
    go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest

# Pre-fetch/override vulnerable module versions in the module cache so xcaddy
# will pick them up during the build. These `go get` calls attempt to pin
# fixed versions of dependencies known to cause Trivy findings (expr, quic-go).
RUN --mount=type=cache,target=/go/pkg/mod \
    go get github.com/expr-lang/expr@v1.17.0 github.com/quic-go/quic-go@v0.54.1 || true

# Build Caddy for the target architecture with security plugins.
# Try the requested v${CADDY_VERSION} tag first; if it fails (unknown tag),
# fall back to a known-good v2.10.2 build to keep the build resilient.
RUN --mount=type=cache,target=/root/.cache/go-build \
        --mount=type=cache,target=/go/pkg/mod \
        sh -c "GOOS=$TARGETOS GOARCH=$TARGETARCH xcaddy build v${CADDY_VERSION} \
            --with github.com/greenpau/caddy-security \
            --with github.com/corazawaf/coraza-caddy/v2 \
            --with github.com/hslatman/caddy-crowdsec-bouncer \
            --with github.com/zhangjiayin/caddy-geoip2 \
            --output /usr/bin/caddy || \
            (echo 'Requested Caddy tag v${CADDY_VERSION} failed; falling back to v2.10.2' && \
             GOOS=$TARGETOS GOARCH=$TARGETARCH xcaddy build v2.10.2 \
                 --with github.com/greenpau/caddy-security \
                 --with github.com/corazawaf/coraza-caddy/v2 \
                 --with github.com/hslatman/caddy-crowdsec-bouncer \
                 --with github.com/zhangjiayin/caddy-geoip2 --output /usr/bin/caddy)"

# ---- Final Runtime with Caddy ----
FROM ${CADDY_IMAGE}
WORKDIR /app

# Install runtime dependencies for Charon (no bash needed)
# hadolint ignore=DL3018
RUN apk --no-cache add ca-certificates sqlite-libs tzdata curl \
    && apk --no-cache upgrade

# Download MaxMind GeoLite2 Country database
# Note: In production, users should provide their own MaxMind license key
# This uses the publicly available GeoLite2 database
RUN mkdir -p /app/data/geoip && \
    curl -L "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" \
    -o /app/data/geoip/GeoLite2-Country.mmdb

# Copy Caddy binary from caddy-builder (overwriting the one from base image)
COPY --from=caddy-builder /usr/bin/caddy /usr/bin/caddy

# Install CrowdSec binary (default version can be overridden at build time)
ARG CROWDSEC_VERSION=1.6.0
# hadolint ignore=DL3018
RUN apk add --no-cache curl tar gzip && \
    set -eux; \
    URL="https://github.com/crowdsecurity/crowdsec/releases/download/v${CROWDSEC_VERSION}/crowdsec-v${CROWDSEC_VERSION}-linux-musl.tar.gz"; \
    curl -fSL "$URL" -o /tmp/crowdsec.tar.gz && \
    mkdir -p /tmp/crowdsec && tar -xzf /tmp/crowdsec.tar.gz -C /tmp/crowdsec --strip-components=1 || true; \
    if [ -f /tmp/crowdsec/crowdsec ]; then \
        mv /tmp/crowdsec/crowdsec /usr/local/bin/crowdsec && chmod +x /usr/local/bin/crowdsec; \
    fi && \
    rm -rf /tmp/crowdsec /tmp/crowdsec.tar.gz || true

# Copy Go binary from backend builder
COPY --from=backend-builder /app/backend/charon /app/charon
RUN ln -s /app/charon /app/cpmp || true
# Copy Delve debugger (xx-go install places it in /go/bin)
COPY --from=backend-builder /go/bin/dlv /usr/local/bin/dlv

# Copy frontend build from frontend builder
COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist

# Copy startup script
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Set default environment variables
ENV CHARON_ENV=production \
    CHARON_HTTP_PORT=8080 \
    CHARON_DB_PATH=/app/data/charon.db \
    CHARON_FRONTEND_DIR=/app/frontend/dist \
    CHARON_CADDY_ADMIN_API=http://localhost:2019 \
    CHARON_CADDY_CONFIG_DIR=/app/data/caddy \
    CHARON_GEOIP_DB_PATH=/app/data/geoip/GeoLite2-Country.mmdb \
    CPM_ENV=production \
    CPM_HTTP_PORT=8080 \
    CPM_DB_PATH=/app/data/cpm.db \
    CPM_FRONTEND_DIR=/app/frontend/dist \
    CPM_CADDY_ADMIN_API=http://localhost:2019 \
    CPM_CADDY_CONFIG_DIR=/app/data/caddy \
    CPM_GEOIP_DB_PATH=/app/data/geoip/GeoLite2-Country.mmdb

# Create necessary directories
RUN mkdir -p /app/data /app/data/caddy /config /app/data/crowdsec

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
EXPOSE 80 443 443/udp 8080 2019

# Use custom entrypoint to start both Caddy and Charon
ENTRYPOINT ["/docker-entrypoint.sh"]
