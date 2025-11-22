# Multi-stage Dockerfile for CaddyProxyManager+ with integrated Caddy
# Single container deployment for simplified home user setup

# Build arguments for versioning
ARG VERSION=dev
ARG BUILD_DATE
ARG VCS_REF

# Allow pinning Caddy base image by digest via build-arg
# Using caddy:2.9.1-alpine to fix CVE-2025-59530 and stdlib vulnerabilities
ARG CADDY_IMAGE=caddy:2.9.1-alpine

# ---- Cross-Compilation Helpers ----
FROM --platform=$BUILDPLATFORM tonistiigi/xx:1.8.0 AS xx

# ---- Frontend Builder ----
# Build the frontend using the BUILDPLATFORM to avoid arm64 musl Rollup native issues
FROM --platform=$BUILDPLATFORM node:24.11.1-alpine AS frontend-builder
WORKDIR /app/frontend

# Copy frontend package files
COPY frontend/package*.json ./

# Set environment to bypass native binary requirement for cross-arch builds
ENV npm_config_rollup_skip_nodejs_native=1 \
    ROLLUP_SKIP_NODEJS_NATIVE=1

RUN npm ci

# Copy frontend source and build
COPY frontend/ ./
RUN --mount=type=cache,target=/app/frontend/node_modules/.cache \
    npm run build

# ---- Backend Builder ----
FROM --platform=$BUILDPLATFORM golang:alpine AS backend-builder
# Copy xx helpers for cross-compilation
COPY --from=xx / /

WORKDIR /app/backend

# Install build dependencies
# xx-apk installs packages for the TARGET architecture
ARG TARGETPLATFORM
RUN apk add --no-cache clang lld
RUN xx-apk add --no-cache gcc musl-dev sqlite-dev

# Install Delve (cross-compile for target)
# Note: xx-go install puts binaries in /go/bin/TARGETOS_TARGETARCH/dlv if cross-compiling.
# We find it and move it to /go/bin/dlv so it's in a consistent location for the next stage.
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
# -gcflags "all=-N -l" disables optimizations and inlining for better debugging
# xx-go handles CGO and cross-compilation flags automatically
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 xx-go build \
    -gcflags "all=-N -l" \
    -a -installsuffix cgo \
    -ldflags "-X github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/version.Version=${VERSION} \
              -X github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/version.GitCommit=${VCS_REF} \
              -X github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/version.BuildTime=${BUILD_DATE}" \
    -o cpmp ./cmd/api

# ---- Caddy Builder ----
# Build Caddy from source to ensure we use the latest Go version and dependencies
# This fixes vulnerabilities found in the pre-built Caddy images (e.g. CVE-2025-59530, stdlib issues)
FROM --platform=$BUILDPLATFORM golang:alpine AS caddy-builder
ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache git
RUN --mount=type=cache,target=/go/pkg/mod \
    go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest

# Build Caddy for the target architecture
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS=$TARGETOS GOARCH=$TARGETARCH xcaddy build v2.9.1 \
    --replace github.com/quic-go/quic-go=github.com/quic-go/quic-go@v0.49.1 \
    --replace golang.org/x/crypto=golang.org/x/crypto@v0.35.0 \
    --output /usr/bin/caddy

# ---- Final Runtime with Caddy ----
FROM ${CADDY_IMAGE}
WORKDIR /app

# Install runtime dependencies for CPM+ (no bash needed)
RUN apk --no-cache add ca-certificates sqlite-libs \
    && apk --no-cache upgrade

# Copy Caddy binary from caddy-builder (overwriting the one from base image)
COPY --from=caddy-builder /usr/bin/caddy /usr/bin/caddy

# Copy Go binary from backend builder
COPY --from=backend-builder /app/backend/cpmp /app/cpmp
# Copy Delve debugger (xx-go install places it in /go/bin)
COPY --from=backend-builder /go/bin/dlv /usr/local/bin/dlv

# Copy frontend build from frontend builder
COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist

# Copy startup script
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Set default environment variables
ENV CPM_ENV=production \
    CPM_HTTP_PORT=8080 \
    CPM_DB_PATH=/app/data/cpm.db \
    CPM_FRONTEND_DIR=/app/frontend/dist \
    CPM_CADDY_ADMIN_API=http://localhost:2019 \
    CPM_CADDY_CONFIG_DIR=/app/data/caddy

# Create necessary directories
RUN mkdir -p /app/data /app/data/caddy /config

# Re-declare build args for LABEL usage
ARG VERSION=dev
ARG BUILD_DATE
ARG VCS_REF

# OCI image labels for version metadata
LABEL org.opencontainers.image.title="CaddyProxyManager+ (CPMP)" \
      org.opencontainers.image.description="Web UI for managing Caddy reverse proxy configurations" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.source="https://github.com/Wikid82/CaddyProxyManagerPlus" \
      org.opencontainers.image.url="https://github.com/Wikid82/CaddyProxyManagerPlus" \
      org.opencontainers.image.vendor="CaddyProxyManagerPlus" \
      org.opencontainers.image.licenses="MIT"

# Expose ports
EXPOSE 80 443 443/udp 8080 2019

# Use custom entrypoint to start both Caddy and CPM+
ENTRYPOINT ["/docker-entrypoint.sh"]
