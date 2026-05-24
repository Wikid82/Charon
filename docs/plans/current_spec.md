## Caddy Build Path: x/crypto Pin Scope Correction

Date: 2026-05-24
Scope: Caddy path only. This plan is limited to x/crypto pinning and
verification for caddy-builder.

## Objective

Require a top-level pinned XCRYPTO_VERSION argument with a concrete default,
consume it in caddy-builder Stage 2 patching, and validate exact version
equality in built Caddy module metadata.

## Required Adjustments

1. Add a root-level build argument with a concrete pinned default:

```bash
ARG XCRYPTO_VERSION=0.52.0
```

2. Consume the shared argument in caddy-builder (no stage-local-only value):

```bash
ARG XCRYPTO_VERSION
```

3. In caddy-builder Stage 2 patch block, pin x/crypto using the shared value:

```bash
go get golang.org/x/crypto@v${XCRYPTO_VERSION}; \
```

Constraint: keep this change scoped to Caddy path patching only; do not expand
scope to CrowdSec or unrelated stages in this plan.

## Implementation Steps

1. Add XCRYPTO_VERSION at top-level ARG section with concrete default.
2. Add ARG XCRYPTO_VERSION to caddy-builder arg declarations.
3. Add explicit go get golang.org/x/crypto@v${XCRYPTO_VERSION} in Stage 2
   dependency patch list.
4. Keep existing patch ordering, tidy, and Caddy core assertions intact.

## Deterministic Validation

1. Build caddy-builder:

```bash
docker build --target caddy-builder -f Dockerfile .
```

2. Assert exact x/crypto version equality from Caddy binary metadata:

```bash
ACTUAL_XCRYPTO="$(go version -m /usr/bin/caddy | awk '$1=="dep" && $2=="golang.org/x/crypto" {print $3; exit}')" && \
EXPECTED_XCRYPTO="v${XCRYPTO_VERSION}" && \
test "${ACTUAL_XCRYPTO}" = "${EXPECTED_XCRYPTO}"
```

3. If direct host inspection of /usr/bin/caddy is not available, run the same
   assertion inside a container/image shell that contains the built binary and
   record both ACTUAL_XCRYPTO and EXPECTED_XCRYPTO values.

## Acceptance Criteria

1. Dockerfile defines top-level ARG XCRYPTO_VERSION with a concrete default.
2. caddy-builder consumes ARG XCRYPTO_VERSION and uses it for the Stage 2
   golang.org/x/crypto pin.
3. Validation includes deterministic exact equality assertion (ACTUAL equals
   EXPECTED), not presence-only matching.

## Rollback Notes

1. Revert only Caddy-path x/crypto pin changes introduced by this scope:
   top-level XCRYPTO_VERSION arg, caddy-builder arg consumption, and Stage 2
   go get line.
2. Rebuild --target caddy-builder to verify rollback behavior.
3. Document rollback reason and keep rollback isolated to Caddy path changes.
