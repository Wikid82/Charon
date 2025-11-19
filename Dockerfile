# Multi-stage Dockerfile for CaddyProxyManager+ with integrated Caddy
# Single container deployment for simplified home user setup

# Build arguments for versioning
ARG VERSION=dev
ARG BUILD_DATE
ARG VCS_REF

# Allow pinning Caddy base image by digest via build-arg
ARG CADDY_IMAGE=caddy:2-alpine

# ---- Frontend Builder ----
# Build the frontend using the BUILDPLATFORM to avoid arm64 musl Rollup native issues
FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend-builder
WORKDIR /app/frontend

# Copy frontend package files
COPY frontend/package*.json ./

# Set environment to bypass native binary requirement for cross-arch builds
ENV npm_config_rollup_skip_nodejs_native=1 \
    ROLLUP_SKIP_NODEJS_NATIVE=1

RUN npm ci

# Copy frontend source and build
COPY frontend/ ./
RUN npm run build

# ---- Backend Builder ----
FROM golang:alpine AS backend-builder
WORKDIR /app/backend

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy Go module files
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy backend source
COPY backend/ ./

# Build arguments passed from main build context
ARG VERSION=dev
ARG VCS_REF=unknown
ARG BUILD_DATE=unknown

# Build the Go binary with version information injected via ldflags
RUN CGO_ENABLED=1 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags "-X github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/version.SemVer=${VERSION} \
              -X github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/version.GitCommit=${VCS_REF} \
              -X github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/version.BuildDate=${BUILD_DATE}" \
    -o cpmp ./cmd/api

# ---- Final Runtime with Caddy ----
FROM ${CADDY_IMAGE}
WORKDIR /app

# Install runtime dependencies for CPM+ (no bash needed)
RUN apk --no-cache add ca-certificates sqlite-libs \
    && apk --no-cache upgrade

# Copy Go binary from backend builder
COPY --from=backend-builder /app/backend/cpmp /app/cpmp

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
