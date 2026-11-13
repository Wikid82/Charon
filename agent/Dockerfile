# syntax=docker/dockerfile:1

# renovate: datasource=docker depName=golang versioning=docker
ARG GO_VERSION=1.27.4

# Build on the host platform to avoid emulation overhead; cross-compile to the target.
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

RUN apk add --no-cache ca-certificates

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.gitCommit=${GIT_COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o orthrus-agent .

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/orthrus-agent /orthrus-agent

# No USER directive: the process starts as root so it can read the mounted
# Docker socket, then drops to PUID/PGID inside the binary (see
# agent/privdrop). This is required because the scratch image has no shell,
# /etc/passwd, or entrypoint script that could do the drop, and a static USER
# would fix the uid before the binary could honour PUID/PGID.
#
# PUID defaults to 65532 and PGID defaults to 65532. Set PGID to the host
# docker group GID so the dropped process can open the Docker socket.
# Setting PUID=0 or PGID=0 is refused unless ORTHRUS_ALLOW_ROOT=true.
ENTRYPOINT ["/orthrus-agent"]
