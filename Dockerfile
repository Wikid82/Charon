# Multi-stage Dockerfile for CaddyProxyManager+ (Go backend + React frontend)

# ---- Frontend Builder ----
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend

# Copy frontend package files
COPY frontend/package*.json ./
RUN npm ci

# Copy frontend source and build
COPY frontend/ ./
RUN npm run build

# ---- Backend Builder ----
FROM golang:1.22-alpine AS backend-builder
WORKDIR /app/backend

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy Go module files
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy backend source
COPY backend/ ./

# Build the Go binary
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o api ./cmd/api

# ---- Final Runtime ----
FROM alpine:latest
WORKDIR /app

# Install runtime dependencies
RUN apk --no-cache add ca-certificates sqlite-libs

# Copy Go binary from backend builder
COPY --from=backend-builder /app/backend/api /app/api

# Copy frontend build from frontend builder
COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist

# Set default environment variables
ENV CPM_ENV=production
ENV CPM_HTTP_PORT=8080
ENV CPM_DB_PATH=/app/data/cpm.db
ENV CPM_FRONTEND_DIR=/app/frontend/dist

# Create data directory
RUN mkdir -p /app/data

# Expose HTTP port
EXPOSE 8080

# Run the application
CMD ["/app/api"]
