#!/usr/bin/env bash
# Shared bats helper: builds an isolated fake repo containing the real
# scripts/lib/dockerfile-stage.sh + scripts/toolchain-key.sh +
# scripts/verify-toolchain-pin.sh alongside a synthetic Dockerfile / .trivyignore
# fixture that is structurally valid for toolchain-key.sh's sanity checks
# (>= 20 body lines per inline stage, each containing a `go build` / `xx-go build`).
#
# Usage from a .bats file:
#   load helpers/toolchain_fixture
#   setup() { tf_setup; }
#   teardown() { tf_teardown; }
#
# Exposes: $TF_ROOT (fake repo root), $TF_DF ($TF_ROOT/Dockerfile),
#          $TF_BIN (a PATH-prepended dir for command stubs).

# shellcheck shell=bash

tf_setup() {
  TF_REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  TF_ROOT="$(mktemp -d)"
  TF_BIN="$TF_ROOT/.bin"
  mkdir -p "$TF_ROOT/scripts/lib" "$TF_BIN"

  cp "$TF_REPO_ROOT/scripts/lib/dockerfile-stage.sh" "$TF_ROOT/scripts/lib/"
  cp "$TF_REPO_ROOT/scripts/toolchain-key.sh"        "$TF_ROOT/scripts/"
  cp "$TF_REPO_ROOT/scripts/verify-toolchain-pin.sh" "$TF_ROOT/scripts/"
  chmod +x "$TF_ROOT/scripts/"*.sh

  TF_DF="$TF_ROOT/Dockerfile"
  tf_write_dockerfile
  printf '.cache/\nsome-cve-id\n' > "$TF_ROOT/.trivyignore"

  export PATH="$TF_BIN:$PATH"
}

tf_teardown() {
  [[ -n "${TF_ROOT:-}" && -d "$TF_ROOT" ]] && rm -rf "$TF_ROOT"
}

# Write the fixture Dockerfile. Any already-exported TF_* knobs below let
# individual tests perturb one input at a time.
tf_write_dockerfile() {
  local caddy_version="${TF_CADDY_VERSION:-2.11.4}"
  local geoip2_version="${TF_GEOIP2_VERSION:-v0.0.0-20260623062220-3675c6e7e63d}"
  local caddy_get_line="${TF_CADDY_GET_LINE:-    _retry go get golang.org/x/net@v0.58.0; \\}"
  local golang_digest="${TF_GOLANG_DIGEST:-sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125}"

  cat > "$TF_DF" <<EOF
# syntax=docker/dockerfile:1
ARG GO_VERSION=1.27.1
ARG ALPINE_IMAGE=alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
ARG CROWDSEC_VERSION=1.8.1
ARG EXPR_LANG_VERSION=1.17.8
ARG XNET_VERSION=0.58.0
ARG XCRYPTO_VERSION=0.56.0
ARG KLAUSPOST_COMPRESS_VERSION=1.20.0
ARG GRPC_VERSION=1.83.1
ARG CADDY_VERSION=${caddy_version}
ARG CADDY_CANDIDATE_VERSION=2.11.4
ARG CADDY_USE_CANDIDATE=0
ARG CADDY_PATCH_SCENARIO=B
ARG CADDY_SECURITY_VERSION=1.1.64
ARG CORAZA_CADDY_VERSION=2.6.0
# renovate: datasource=go depName=github.com/zhangjiayin/caddy-geoip2
ARG CADDY_GEOIP2_VERSION=${geoip2_version}
# renovate: datasource=go depName=github.com/mholt/caddy-ratelimit
ARG CADDY_RATELIMIT_VERSION=0.1.0
ARG CHARON_TOOLCHAIN_IMAGE=ghcr.io/wikid82/charon-toolchain
ARG CHARON_TOOLCHAIN_TAG=caddy-crowdsec-0000000000000000
ARG CHARON_TOOLCHAIN_DIGEST=sha256:0000000000000000000000000000000000000000000000000000000000000000

FROM --platform=\$BUILDPLATFORM tonistiigi/xx:1.9.0@sha256:c64defb9ed5a91eacb37f96ccc3d4cd72521c4bd18d5442905b95e2226b0e707 AS xx

# renovate: datasource=docker depName=golang
FROM --platform=\$BUILDPLATFORM golang:\${GO_VERSION}-alpine@${golang_digest} AS caddy-inline
ARG CADDY_VERSION
ARG CADDY_SECURITY_VERSION
ARG CORAZA_CADDY_VERSION
ARG XCADDY_VERSION=0.4.7
ARG EXPR_LANG_VERSION
ARG XNET_VERSION
ARG GRPC_VERSION
ARG CADDY_GEOIP2_VERSION
ARG CADDY_RATELIMIT_VERSION
RUN apk add --no-cache bash git
RUN --mount=type=cache,target=/go/pkg/mod \\
    go install github.com/caddyserver/xcaddy/cmd/xcaddy@v\${XCADDY_VERSION}
RUN --mount=type=cache,target=/go/pkg/mod bash -c 'set -e; \\
    xcaddy build v\${CADDY_VERSION} \\
      --with github.com/zhangjiayin/caddy-geoip2@\${CADDY_GEOIP2_VERSION} \\
      --with github.com/mholt/caddy-ratelimit@v\${CADDY_RATELIMIT_VERSION} \\
      --output /tmp/caddy-initial; \\
${caddy_get_line}
    _retry go get github.com/expr-lang/expr@v\${EXPR_LANG_VERSION}; \\
    _retry go get google.golang.org/grpc@v\${GRPC_VERSION}; \\
    _retry go mod tidy; \\
    GOOS=\$TARGETOS GOARCH=\$TARGETARCH go build -o /usr/bin/caddy -ldflags "-w -s" -trimpath .; \\
    test -x /usr/bin/caddy'

# renovate: datasource=docker depName=golang
FROM --platform=\$BUILDPLATFORM golang:\${GO_VERSION}-alpine@${golang_digest} AS crowdsec-inline
COPY --from=xx / /
WORKDIR /tmp/crowdsec
ARG CROWDSEC_VERSION
ARG EXPR_LANG_VERSION
ARG XNET_VERSION
ARG XCRYPTO_VERSION
ARG KLAUSPOST_COMPRESS_VERSION
ARG GRPC_VERSION
RUN apk add --no-cache git clang lld
RUN xx-apk add --no-cache gcc musl-dev musl
RUN git clone --depth 1 --branch "v\${CROWDSEC_VERSION}" https://github.com/crowdsecurity/crowdsec.git .
RUN set -e; \\
    _retry go get github.com/expr-lang/expr@v\${EXPR_LANG_VERSION}; \\
    _retry go get golang.org/x/crypto@v\${XCRYPTO_VERSION}; \\
    _retry go get golang.org/x/net@v\${XNET_VERSION}; \\
    _retry go get github.com/klauspost/compress@v\${KLAUSPOST_COMPRESS_VERSION}; \\
    _retry go get google.golang.org/grpc@v\${GRPC_VERSION}; \\
    _retry go mod tidy
RUN sed -i 's/string(program\\.Source())/program.Source().String()/g' pkg/exprhelpers/debugger.go
RUN --mount=type=cache,target=/go/pkg/mod \\
    CGO_ENABLED=1 xx-go build -o /crowdsec-out/crowdsec ./cmd/crowdsec && \\
    xx-verify /crowdsec-out/crowdsec
RUN --mount=type=cache,target=/go/pkg/mod \\
    CGO_ENABLED=1 xx-go build -o /crowdsec-out/cscli ./cmd/crowdsec-cli && \\
    xx-verify /crowdsec-out/cscli
RUN mkdir -p /crowdsec-out/config && cp -r config/* /crowdsec-out/config/ || true

FROM \${ALPINE_IMAGE} AS toolchain-runtime
COPY --from=caddy-inline    /usr/bin/caddy         /usr/bin/caddy
COPY --from=crowdsec-inline /crowdsec-out/crowdsec /crowdsec-out/crowdsec

FROM \${ALPINE_IMAGE}
COPY --from=caddy-inline /usr/bin/caddy /usr/bin/caddy
EOF
}

# Install a fake `regctl` on PATH whose `image digest` prints $1.
tf_stub_regctl() {
  local digest="$1"
  cat > "$TF_BIN/regctl" <<EOF
#!/usr/bin/env bash
case "\$1 \$2" in
  "registry login") exit 0 ;;
  "image digest")   printf '%s\n' "${digest}"; exit 0 ;;
  *) exit 0 ;;
esac
EOF
  chmod +x "$TF_BIN/regctl"
}

# A minimal PATH that contains coreutils + the fixture stub dir but is
# guaranteed NOT to contain a system `regctl`. Use for "regctl absent" tests:
#   PATH="$(tf_min_path)" run bash "$TF_ROOT/scripts/verify-toolchain-pin.sh"
tf_min_path() {
  printf '%s' "$TF_BIN:/usr/bin:/bin"
}
